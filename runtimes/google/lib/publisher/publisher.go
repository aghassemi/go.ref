// Package publisher provides a type to publish names to a mounttable.
package publisher

// TODO(toddw): Add unittests.

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"v.io/v23/context"
	"v.io/v23/ipc"
	"v.io/v23/naming"
	"v.io/v23/naming/ns"
	"v.io/x/lib/vlog"
)

// Publisher manages the publishing of servers in mounttable.
type Publisher interface {
	// AddServer adds a new server to be mounted.
	AddServer(server string, ServesMountTable bool)
	// RemoveServer removes a server from the list of mounts.
	RemoveServer(server string)
	// AddName adds a new name for all servers to be mounted as.
	AddName(name string)
	// RemoveName removes a name.
	RemoveName(name string)
	// Status returns a snapshot of the publisher's current state.
	Status() ipc.MountState
	// DebugString returns a string representation of the publisher
	// meant solely for debugging.
	DebugString() string
	// Stop causes the publishing to stop and initiates unmounting of the
	// mounted names.  Stop performs the unmounting asynchronously, and
	// WaitForStop should be used to wait until it is done.
	// Once Stop is called Add/RemoveServer and AddName become noops.
	Stop()
	// WaitForStop waits until all unmounting initiated by Stop is finished.
	WaitForStop()
}

// The publisher adds this much slack to each TTL.
const mountTTLSlack = 20 * time.Second

// publisher maintains the name->server associations in the mounttable.  It
// spawns its own goroutine that does the actual work; the publisher itself
// simply coordinates concurrent access by sending and receiving on the
// appropriate channels.
type publisher struct {
	cmdchan  chan interface{} // value is one of {server,name,debug}Cmd
	donechan chan struct{}    // closed when the publisher is done
}

type addServerCmd struct {
	server string        // server to add
	mt     bool          // true if server serves a mount table
	done   chan struct{} // closed when the cmd is done
}

type removeServerCmd struct {
	server string        // server to remove
	done   chan struct{} // closed when the cmd is done
}

type addNameCmd struct {
	name string        // name to add
	done chan struct{} // closed when the cmd is done
}

type removeNameCmd struct {
	name string        // name to remove
	done chan struct{} // closed when the cmd is done
}

type debugCmd chan string // debug string is sent when the cmd is done

type statusCmd chan ipc.MountState // status info is sent when cmd is done

type stopCmd struct{} // sent to the runloop when we want it to exit.

// New returns a new publisher that updates mounts on ns every period.
func New(ctx *context.T, ns ns.Namespace, period time.Duration) Publisher {
	p := &publisher{
		cmdchan:  make(chan interface{}),
		donechan: make(chan struct{}),
	}
	go runLoop(ctx, p.cmdchan, p.donechan, ns, period)
	return p
}

func (p *publisher) sendCmd(cmd interface{}) bool {
	select {
	case p.cmdchan <- cmd:
		return true
	case <-p.donechan:
		return false
	}
}

func (p *publisher) AddServer(server string, mt bool) {
	done := make(chan struct{})
	if p.sendCmd(addServerCmd{server, mt, done}) {
		<-done
	}
}

func (p *publisher) RemoveServer(server string) {
	done := make(chan struct{})
	if p.sendCmd(removeServerCmd{server, done}) {
		<-done
	}
}

func (p *publisher) AddName(name string) {
	done := make(chan struct{})
	if p.sendCmd(addNameCmd{name, done}) {
		<-done
	}
}

func (p *publisher) RemoveName(name string) {
	done := make(chan struct{})
	if p.sendCmd(removeNameCmd{name, done}) {
		<-done
	}
}

func (p *publisher) Status() ipc.MountState {
	status := make(statusCmd)
	if p.sendCmd(status) {
		return <-status
	}
	return ipc.MountState{}
}

func (p *publisher) DebugString() (dbg string) {
	debug := make(debugCmd)
	if p.sendCmd(debug) {
		dbg = <-debug
	} else {
		dbg = "stopped"
	}
	return
}

// Stop stops the publisher, which in practical terms means un-mounting
// everything and preventing any further publish operations.  The caller can
// be confident that no new names or servers will get published once Stop
// returns.  To wait for existing mounts to be cleaned up, use WaitForStop.
//
// Stopping the publisher is irreversible.
//
// Once the publisher is stopped, any further calls on its public methods
// (including Stop) are no-ops.
func (p *publisher) Stop() {
	p.sendCmd(stopCmd{})
}

func (p *publisher) WaitForStop() {
	<-p.donechan
}

func runLoop(ctx *context.T, cmdchan chan interface{}, donechan chan struct{}, ns ns.Namespace, period time.Duration) {
	vlog.VI(2).Info("ipc pub: start runLoop")
	state := newPubState(ctx, ns, period)

	for {
		select {
		case cmd := <-cmdchan:
			switch tcmd := cmd.(type) {
			case stopCmd:
				state.unmountAll()
				close(donechan)
				vlog.VI(2).Info("ipc pub: exit runLoop")
				return
			case addServerCmd:
				state.addServer(tcmd.server, tcmd.mt)
				close(tcmd.done)
			case removeServerCmd:
				state.removeServer(tcmd.server)
				close(tcmd.done)
			case addNameCmd:
				state.addName(tcmd.name)
				close(tcmd.done)
			case removeNameCmd:
				state.removeName(tcmd.name)
				close(tcmd.done)
			case statusCmd:
				tcmd <- state.getStatus()
				close(tcmd)
			case debugCmd:
				tcmd <- state.debugString()
				close(tcmd)
			}
		case <-state.timeout():
			// Sync everything once every period, to refresh the ttls.
			state.sync()
		}
	}
}

type mountKey struct {
	name, server string
}

// pubState maintains the state for our periodic mounts.  It is not thread-safe;
// it's only used in the sequential publisher runLoop.
type pubState struct {
	ctx      *context.T
	ns       ns.Namespace
	period   time.Duration
	deadline time.Time       // deadline for the next sync call
	names    map[string]bool // names that have been added
	servers  map[string]bool // servers that have been added, true
	// map each (name,server) to its status.
	mounts map[mountKey]*ipc.MountStatus
}

func newPubState(ctx *context.T, ns ns.Namespace, period time.Duration) *pubState {
	return &pubState{
		ctx:      ctx,
		ns:       ns,
		period:   period,
		deadline: time.Now().Add(period),
		names:    make(map[string]bool),
		servers:  make(map[string]bool),
		mounts:   make(map[mountKey]*ipc.MountStatus),
	}
}

func (ps *pubState) timeout() <-chan time.Time {
	return time.After(ps.deadline.Sub(time.Now()))
}

func (ps *pubState) addName(name string) {
	// Each non-dup name that is added causes new mounts to be created for all
	// existing servers.
	if ps.names[name] {
		return
	}
	ps.names[name] = true
	for server, servesMT := range ps.servers {
		status := new(ipc.MountStatus)
		ps.mounts[mountKey{name, server}] = status
		ps.mount(name, server, status, servesMT)
	}
}

func (ps *pubState) removeName(name string) {
	if !ps.names[name] {
		return
	}
	for server, _ := range ps.servers {
		if status, exists := ps.mounts[mountKey{name, server}]; exists {
			ps.unmount(name, server, status)
		}
	}
	delete(ps.names, name)
}

func (ps *pubState) addServer(server string, servesMT bool) {
	// Each non-dup server that is added causes new mounts to be created for all
	// existing names.
	if !ps.servers[server] {
		ps.servers[server] = servesMT
		for name, _ := range ps.names {
			status := new(ipc.MountStatus)
			ps.mounts[mountKey{name, server}] = status
			ps.mount(name, server, status, servesMT)
		}
	}
}

func (ps *pubState) removeServer(server string) {
	if _, exists := ps.servers[server]; !exists {
		return
	}
	delete(ps.servers, server)
	for name, _ := range ps.names {
		if status, exists := ps.mounts[mountKey{name, server}]; exists {
			ps.unmount(name, server, status)
		}
	}
}

func (ps *pubState) mount(name, server string, status *ipc.MountStatus, servesMT bool) {
	// Always mount with ttl = period + slack, regardless of whether this is
	// triggered by a newly added server or name, or by sync.  The next call
	// to sync will occur within the next period, and refresh all mounts.
	ttl := ps.period + mountTTLSlack
	status.LastMount = time.Now()
	status.LastMountErr = ps.ns.Mount(ps.ctx, name, server, ttl, naming.ServesMountTableOpt(servesMT))
	status.TTL = ttl
	if status.LastMountErr != nil {
		vlog.Errorf("ipc pub: couldn't mount(%v, %v, %v): %v", name, server, ttl, status.LastMountErr)
	} else {
		vlog.VI(2).Infof("ipc pub: mount(%v, %v, %v)", name, server, ttl)
	}
}

func (ps *pubState) sync() {
	ps.deadline = time.Now().Add(ps.period) // set deadline for the next sync
	for key, status := range ps.mounts {
		if status.LastUnmountErr != nil {
			// Desired state is "unmounted", failed at previous attempt. Retry.
			ps.unmount(key.name, key.server, status)
		} else {
			ps.mount(key.name, key.server, status, ps.servers[key.server])
		}
	}
}

func (ps *pubState) unmount(name, server string, status *ipc.MountStatus) {
	status.LastUnmount = time.Now()
	status.LastUnmountErr = ps.ns.Unmount(ps.ctx, name, server)
	if status.LastUnmountErr != nil {
		vlog.Errorf("ipc pub: couldn't unmount(%v, %v): %v", name, server, status.LastUnmountErr)
	} else {
		vlog.VI(2).Infof("ipc pub: unmount(%v, %v)", name, server)
		delete(ps.mounts, mountKey{name, server})
	}
}

func (ps *pubState) unmountAll() {
	for key, status := range ps.mounts {
		ps.unmount(key.name, key.server, status)
	}
}

func copyToSlice(sl map[string]bool) []string {
	var ret []string
	for s, _ := range sl {
		if len(s) == 0 {
			continue
		}
		ret = append(ret, s)
	}
	return ret
}

func (ps *pubState) getStatus() ipc.MountState {
	st := make([]ipc.MountStatus, 0, len(ps.mounts))
	names := copyToSlice(ps.names)
	servers := copyToSlice(ps.servers)
	sort.Strings(names)
	sort.Strings(servers)
	for _, name := range names {
		for _, server := range servers {
			if v := ps.mounts[mountKey{name, server}]; v != nil {
				mst := *v
				mst.Name = name
				mst.Server = server
				st = append(st, mst)
			}
		}
	}
	return st
}

// TODO(toddw): sort the names/servers so that the output order is stable.
func (ps *pubState) debugString() string {
	l := make([]string, 2+len(ps.mounts))
	l = append(l, fmt.Sprintf("Publisher period:%v deadline:%v", ps.period, ps.deadline))
	l = append(l, "==============================Mounts============================================")
	for key, status := range ps.mounts {
		l = append(l, fmt.Sprintf("[%s,%s] mount(%v, %v, %s) unmount(%v, %v)", key.name, key.server, status.LastMount, status.LastMountErr, status.TTL, status.LastUnmount, status.LastUnmountErr))
	}
	return strings.Join(l, "\n")
}
