// +build linux

// Package gce provides a Profile for Google Compute Engine and should be
// used by binaries that only ever expect to be run on GCE.
package gce

import (
	"net"
	"veyron.io/veyron/veyron/profiles"

	"veyron.io/veyron/veyron2"
	"veyron.io/veyron/veyron2/config"
	"veyron.io/veyron/veyron2/rt"

	"veyron.io/veyron/veyron/profiles/internal/gce"
)

func init() {
	rt.RegisterProfile(&profile{})
}

type profile struct {
	publicAddress net.Addr
}

func (p *profile) Name() string {
	return "GCE"
}

func (p *profile) Runtime() string {
	return ""
}

func (p *profile) Platform() *veyron2.Platform {
	platform, _ := profiles.Platform()
	return platform
}

func (p *profile) String() string {
	return "net " + p.Platform().String()
}

func (p *profile) AddressChooser() veyron2.AddressChooser {
	return func(network string, addrs []net.Addr) (net.Addr, error) {
		return p.publicAddress, nil
	}
}

func (p *profile) Init(rt veyron2.Runtime, publisher *config.Publisher) {
	if !gce.RunningOnGCE() {
		return
		// TODO(cnicolaou): add error return to init
		//return fmt.Errorf("GCE profile used on a non-GCE system")
	}
	if ip, err := gce.ExternalIPAddress(); err != nil {
		return
		// TODO(cnicolaou): add error return to init
		//		return err
	} else {
		p.publicAddress = &net.IPAddr{IP: ip}
	}
}
