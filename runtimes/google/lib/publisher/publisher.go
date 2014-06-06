// Package publisher provides a type to publish names to a mounttable.
package publisher

// TODO(toddw): Add unittests.

import (
	"fmt"
	"strings"
	"time"

	"veyron2/naming"
	"veyron2/vlog"
)

// Publisher manages the publishing of servers in mounttable.
type Publisher interface {
	// AddServer adds a new server to be mounted.
	AddServer(server string)
	// AddName adds a new name for all servers to be mounted as.
	AddName(name string)
	// Published returns the published names rooted at the mounttable.
	Published() []string
	// DebugString returns a string representation of the publisher
	// meant solely for debugging.
	DebugString() string
	// Stop causes the publishing to stop and initiates unmounting of the
	// mounted names.  Stop performs the unmounting asynchronously, and
	// WaitForStop should be used to wait until it is done.
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

type serverCmd struct {
	server string        // server to add
	done   chan struct{} // closed when the cmd is done
}

type nameCmd struct {
	name string        // name to add
	done chan struct{} // closed when the cmd is done
}

type debugCmd chan string // debug string is sent when the cmd is done

type publishedCmd chan []string // published names are sent when cmd is done

// New returns a new publisher that updates mounts on mt every period.
func New(mt naming.MountTable, period time.Duration) Publisher {
	p := &publisher{
		cmdchan:  make(chan interface{}, 10),
		donechan: make(chan struct{}),
	}
	go p.runLoop(mt, period)
	return p
}

func (p *publisher) AddServer(server string) {
	done := make(chan struct{})
	defer func() { recover() }()
	p.cmdchan <- serverCmd{server, done}
	<-done
}

func (p *publisher) AddName(name string) {
	done := make(chan struct{})
	defer func() { recover() }()
	p.cmdchan <- nameCmd{name, done}
	<-done
}

// Published returns the published name(s) for this publisher, where each name
// is rooted at the mount table(s) where the name has been mounted.
// The names are returned grouped by published name, where all the names
// corresponding the the mount table replicas are grouped together.
func (p *publisher) Published() []string {
	published := make(publishedCmd)
	defer func() { recover() }()
	p.cmdchan <- published
	return <-published
}

func (p *publisher) DebugString() (dbg string) {
	debug := make(debugCmd)
	defer func() {
		recover()
		dbg = "stopped"
	}()
	p.cmdchan <- debug
	dbg = <-debug
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
	defer func() { recover() }()
	close(p.cmdchan)
}

func (p *publisher) WaitForStop() {
	<-p.donechan
}

func (p *publisher) runLoop(mt naming.MountTable, period time.Duration) {
	vlog.VI(1).Info("ipc pub: start runLoop")
	state := newPubState(mt, period)
	for {
		select {
		case cmd, ok := <-p.cmdchan:
			if !ok {
				// Closing the cmdchan signals us to break out of the loop.  Unmount
				// everything and signal that we're done by closing the donechan.
				state.unmountAll()
				vlog.VI(1).Info("ipc pub: exit runLoop")
				close(p.donechan)
				return
			}
			switch tcmd := cmd.(type) {
			case serverCmd:
				state.addServer(tcmd.server)
				close(tcmd.done)
			case nameCmd:
				state.addName(tcmd.name)
				close(tcmd.done)
			case publishedCmd:
				tcmd <- state.published()
				close(tcmd)
			case debugCmd:
				tcmd <- state.debugString()
				close(tcmd)
			}
		case <-state.timeout():
			// Remount everything once every period, to refresh the ttls.
			state.mountAll()
		}
	}
}

// pubState maintains the state for our periodic mounts.  It is not thread-safe;
// it's only used in the sequential publisher runLoop.
type pubState struct {
	mt       naming.MountTable
	period   time.Duration
	deadline time.Time                 // deadline for the next mountAll call
	names    []string                  // names that have been added
	servers  map[string]bool           // servers that have been added
	mounts   map[mountKey]*mountStatus // map each (name,server) to its status
}

type mountKey struct {
	name   string
	server string
}

type mountStatus struct {
	lastMount      time.Time
	lastMountErr   error
	lastUnmount    time.Time
	lastUnmountErr error
}

func newPubState(mt naming.MountTable, period time.Duration) *pubState {
	return &pubState{
		mt:       mt,
		period:   period,
		deadline: time.Now().Add(period),
		servers:  make(map[string]bool),
		mounts:   make(map[mountKey]*mountStatus),
	}
}

func (ps *pubState) timeout() <-chan time.Time {
	return time.After(ps.deadline.Sub(time.Now()))
}

func (ps *pubState) addName(name string) {
	// Each non-dup name that is added causes new mounts to be created for all
	// existing servers.
	for _, n := range ps.names {
		if n == name {
			return
		}
	}
	ps.names = append(ps.names, name)
	for server, _ := range ps.servers {
		status := new(mountStatus)
		ps.mounts[mountKey{name, server}] = status
		ps.mount(name, server, status)
	}
}

func (ps *pubState) addServer(server string) {
	// Each non-dup server that is added causes new mounts to be created for all
	// existing names.
	if !ps.servers[server] {
		ps.servers[server] = true
		for _, name := range ps.names {
			status := new(mountStatus)
			ps.mounts[mountKey{name, server}] = status
			ps.mount(name, server, status)
		}
	}
}

func (ps *pubState) mount(name, server string, status *mountStatus) {
	// Always mount with ttl = period + slack, regardless of whether this is
	// triggered by a newly added server or name, or by mountAll.  The next call
	// to mountAll will occur within the next period, and refresh all mounts.
	ttl := ps.period + mountTTLSlack
	status.lastMount = time.Now()
	status.lastMountErr = ps.mt.Mount(name, server, ttl)
	if status.lastMountErr != nil {
		vlog.Errorf("ipc pub: couldn't mount(%v, %v, %v): %v", name, server, ttl, status.lastMountErr)
	} else {
		vlog.VI(2).Infof("ipc pub: mount(%v, %v, %v)", name, server, ttl)
	}
}

func (ps *pubState) mountAll() {
	ps.deadline = time.Now().Add(ps.period) // set deadline for the next mountAll
	for key, status := range ps.mounts {
		ps.mount(key.name, key.server, status)
	}
}

func (ps *pubState) unmount(name, server string, status *mountStatus) {
	status.lastUnmount = time.Now()
	status.lastUnmountErr = ps.mt.Unmount(name, server)
	if status.lastUnmountErr != nil {
		vlog.Errorf("ipc pub: couldn't unmount(%v, %v): %v", name, server, status.lastUnmountErr)
	} else {
		vlog.VI(2).Infof("ipc pub: unmount(%v, %v)", name, server)
	}
}

func (ps *pubState) unmountAll() {
	for key, status := range ps.mounts {
		ps.unmount(key.name, key.server, status)
	}
}

func (ps *pubState) published() []string {
	var ret []string
	for _, name := range ps.names {
		mtServers, err := ps.mt.ResolveToMountTable(name)
		if err != nil {
			vlog.Errorf("ipc pub: couldn't resolve %v to mount table: %v", name, err)
			continue
		}
		if len(mtServers) == 0 {
			vlog.Errorf("ipc pub: no mount table found for %v", name)
			continue
		}
		for _, s := range mtServers {
			ret = append(ret, naming.MakeResolvable(s))
		}
	}
	return ret
}

func (ps *pubState) debugString() string {
	l := make([]string, 2+len(ps.mounts))
	l = append(l, fmt.Sprintf("Publisher period:%v deadline:%v", ps.period, ps.deadline))
	l = append(l, "==============================Mounts============================================")
	for key, status := range ps.mounts {
		l = append(l, fmt.Sprintf("[%s,%s] mount(%v, %v) unmount(%v, %v)", key.name, key.server, status.lastMount, status.lastMountErr, status.lastUnmount, status.lastUnmountErr))
	}
	return strings.Join(l, "\n")
}
