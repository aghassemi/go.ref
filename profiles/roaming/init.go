// +build linux darwin

// Package roaming provides a network-aware Profile that provides appropriate
// options and configuration for a variety of network configurations, including
// being behind 1-1 NATs, using dhcp and auto-configuration for being on
// Google Compute Engine.
//
// The config.Publisher mechanism is used for communicating networking
// settings to the ipc.Server implementation of the runtime and publishes
// the Settings it expects.
package roaming

import (
	"flag"
	"fmt"
	"net"

	"veyron2"
	"veyron2/config"
	"veyron2/ipc"
	"veyron2/rt"

	"veyron/lib/netconfig"
	"veyron/lib/netstate"
	"veyron/profiles"
)

const (
	SettingsStreamName = "dhcp"
)

var (
	listenProtocolFlag = config.TCPProtocolFlag{"tcp"}
	listenSpecFlag     = config.IPHostPortFlag{Port: "0"}
)

func init() {
	flag.Var(&listenProtocolFlag, "veyron.tcp.protocol", "protocol to listen with")
	flag.Var(&listenSpecFlag, "veyron.tcp.address", "address to listen on")
	rt.RegisterProfile(New())
}

type profile struct {
	addrChooser veyron2.AddressChooser
	gce         string
}

func preferredIPAddress(network string, addrs []net.Addr) (net.Addr, error) {
	if !netstate.IsIPProtocol(network) {
		return nil, fmt.Errorf("can't support network protocol %q", network)
	}
	al := netstate.AddrList(addrs).Map(netstate.ConvertToIPHost)
	for _, predicate := range []netstate.Predicate{netstate.IsPublicUnicastIPv4,
		netstate.IsUnicastIPv4, netstate.IsPublicUnicastIPv6} {
		if a := al.First(predicate); a != nil {
			return a, nil
		}
	}
	return nil, fmt.Errorf("failed to find any usable address for %q", network)
}

func New() veyron2.Profile {
	return &profile{addrChooser: preferredIPAddress}
}

func (p *profile) Platform() *veyron2.Platform {
	platform, _ := profiles.Platform()
	return platform
}

func (p *profile) Name() string {
	return "dhcp" + p.gce
}

func (p *profile) Runtime() string {
	return ""
}

func (p *profile) String() string {
	return p.Name() + " " + p.Platform().String()
}

func (p *profile) Init(rt veyron2.Runtime, publisher *config.Publisher) {
	log := rt.Logger()

	state, err := netstate.GetAccessibleIPs()
	if err != nil {
		log.Infof("failed to determine network state")
		// TODO(cnicolaou): in a subsequent CL, change Init to return an error.
		return
		//return err
	}
	first := state.First(netstate.IsUnicastIP)
	if first == nil {
		log.Infof("failed to find any usable IP addresses at startup")
	}
	public := netstate.IsPublicUnicastIPv4(first)

	// We now know that there is an IP address to listen on, and whether
	// it's public or private.

	// Our address is private, so we test for running on GCE and for its
	// 1:1 NAT configuration. handleGCE returns a non-nil addr
	// if we are indeed running on GCE.
	if !public {
		if addr := handleGCE(rt, publisher); addr != nil {
			p.addrChooser = func(string, []net.Addr) (net.Addr, error) {
				return addr, nil
			}
			p.gce = "+gce"
			return
		}
	}

	// Create stream in Init function to avoid a race between any
	// goroutines started here and consumers started after Init returns.
	ch := make(chan config.Setting)
	stop, err := publisher.CreateStream(SettingsStreamName, "dhcp", ch)
	if err != nil {
		log.Errorf("failed to create publisher: %s", err)
		return
	}

	protocol := listenProtocolFlag.Protocol
	log.VI(2).Infof("Initial Network Settings: %s %s available: %s", protocol, listenSpecFlag, state)
	publishInitialSettings(ch, protocol, listenSpecFlag.String(), state)
	go monitorNetworkSettings(rt, stop, ch, state, protocol, p.addrChooser)
}

func (p *profile) AddressChooser() veyron2.AddressChooser {
	return p.addrChooser
}

func publishInitialSettings(ch chan<- config.Setting, protocol, listenSpec string, addrs []net.Addr) {
	// TODO(cnicolaou): consider applying the address chooser here and not in
	// the server, or better yet not sending the InitialAddrsSetting.
	for _, setting := range []config.Setting{
		ipc.NewProtocolSetting(protocol),
		ipc.NewListenSpecSetting(listenSpecFlag),
		ipc.NewInitialAddrsSetting(addrs),
	} {
		ch <- setting
	}
}

// monitorNetworkSettings will monitor network configuration changes and
// publish subsequent Settings to reflect any changes detected.
func monitorNetworkSettings(rt veyron2.Runtime, stop <-chan struct{},
	ch chan<- config.Setting, prev netstate.AddrList, protocol string, chooser veyron2.AddressChooser) {
	defer close(ch)

	log := rt.Logger()

	// Start the dhcp watcher.
	watcher, err := netconfig.NewNetConfigWatcher()
	if err != nil {
		log.VI(2).Infof("Failed to get new config watcher: %s", err)
		// TODO(cnicolaou): add support for shutting down profiles
		//<-stop
		return
	}

	for {
		select {
		case <-watcher.Channel():
			cur, err := netstate.GetAccessibleIPs()
			if err != nil {
				log.Errorf("failed to read network state: %s", err)
				continue
			}
			removed := netstate.FindRemoved(prev, cur)
			added := netstate.FindAdded(prev, cur)
			log.VI(2).Infof("Previous: %d: %s", len(prev), prev)
			log.VI(2).Infof("Current : %d: %s", len(cur), cur)
			log.VI(2).Infof("Added   : %d: %s", len(added), added)
			log.VI(2).Infof("Removed : %d: %s", len(removed), removed)
			if len(removed) == 0 && len(added) == 0 {
				log.VI(2).Infof("Network event that lead to no address changes since our last 'baseline'")
				continue
			}
			if len(removed) > 0 {
				log.VI(2).Infof("Sending removed: %s", removed)
				ch <- ipc.NewRmAddrsSetting(removed)
			}
			// We will always send the best currently available address
			if chosen, err := chooser(protocol, cur); err == nil && chosen != nil {
				ch <- ipc.NewAddAddrsSetting([]net.Addr{chosen})
			}
			prev = cur
			// TODO(cnicolaou): add support for shutting down profiles.
			//case <-stop:
			//	return
		}
	}
}
