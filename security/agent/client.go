// Package agent provides a client for communicating with an "Agent"
// process holding the private key for an identity.
package agent

import (
	"fmt"
	"net"
	"os"

	"v.io/core/veyron/lib/unixfd"
	"v.io/core/veyron/security/agent/cache"
	"v.io/v23/context"
	"v.io/v23/ipc"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/security"
	"v.io/v23/vlog"
	"v.io/v23/vtrace"
)

// FdVarName is the name of the environment variable containing
// the file descriptor for talking to the agent.
const FdVarName = "VEYRON_AGENT_FD"

type client struct {
	caller caller
	key    security.PublicKey
}

type caller struct {
	ctx    *context.T
	client ipc.Client
	name   string
}

func (c *caller) call(name string, results []interface{}, args ...interface{}) error {
	call, err := c.startCall(name, args...)
	if err != nil {
		return err
	}
	if err := call.Finish(results...); err != nil {
		return err
	}
	return nil
}

func (c *caller) startCall(name string, args ...interface{}) (ipc.Call, error) {
	ctx, _ := vtrace.SetNewTrace(c.ctx)
	// VCSecurityNone is safe here since we're using anonymous unix sockets.
	return c.client.StartCall(ctx, c.name, name, args, options.VCSecurityNone, options.NoResolve{})
}

func results(inputs ...interface{}) []interface{} {
	return inputs
}

// NewAgentPrincipal returns a security.Pricipal using the PrivateKey held in a remote agent process.
// 'fd' is the socket for connecting to the agent, typically obtained from
// os.GetEnv(agent.FdVarName).
// 'ctx' should not have a deadline, and should never be cancelled while the
// principal is in use.
func NewAgentPrincipal(ctx *context.T, fd int, insecureClient ipc.Client) (security.Principal, error) {
	p, err := newUncachedPrincipal(ctx, fd, insecureClient)
	if err != nil {
		return p, err
	}
	call, callErr := p.caller.startCall("NotifyWhenChanged")
	if callErr != nil {
		return nil, callErr
	}
	return cache.NewCachedPrincipal(p.caller.ctx, p, call)
}
func newUncachedPrincipal(ctx *context.T, fd int, insecureClient ipc.Client) (*client, error) {
	f := os.NewFile(uintptr(fd), "agent_client")
	defer f.Close()
	conn, err := net.FileConn(f)
	if err != nil {
		return nil, err
	}
	// This is just an arbitrary 1 byte string. The value is ignored.
	data := make([]byte, 1)
	addr, err := unixfd.SendConnection(conn.(*net.UnixConn), data)
	if err != nil {
		return nil, err
	}
	caller := caller{
		client: insecureClient,
		name:   naming.JoinAddressName(naming.FormatEndpoint(addr.Network(), addr.String()), ""),
		ctx:    ctx,
	}
	agent := &client{caller: caller}
	if err := agent.fetchPublicKey(); err != nil {
		return nil, err
	}
	return agent, nil
}

func (c *client) fetchPublicKey() (err error) {
	var b []byte
	if err = c.caller.call("PublicKey", results(&b)); err != nil {
		return
	}
	c.key, err = security.UnmarshalPublicKey(b)
	return
}

func (c *client) Bless(key security.PublicKey, with security.Blessings, extension string, caveat security.Caveat, additionalCaveats ...security.Caveat) (security.Blessings, error) {
	var blessings security.WireBlessings
	marshalledKey, err := key.MarshalBinary()
	if err != nil {
		return security.Blessings{}, err
	}
	if err = c.caller.call("Bless", results(&blessings), marshalledKey, security.MarshalBlessings(with), extension, caveat, additionalCaveats); err != nil {
		return security.Blessings{}, err
	}
	return security.NewBlessings(blessings)
}

func (c *client) BlessSelf(name string, caveats ...security.Caveat) (security.Blessings, error) {
	var blessings security.WireBlessings
	if err := c.caller.call("BlessSelf", results(&blessings), name, caveats); err != nil {
		return security.Blessings{}, err
	}
	return security.NewBlessings(blessings)
}

func (c *client) Sign(message []byte) (sig security.Signature, err error) {
	err = c.caller.call("Sign", results(&sig), message)
	return
}

func (c *client) MintDischarge(forCaveat, caveatOnDischarge security.Caveat, additionalCaveatsOnDischarge ...security.Caveat) (security.Discharge, error) {
	var discharge security.WireDischarge
	if err := c.caller.call("MintDischarge", results(&discharge), forCaveat, caveatOnDischarge, additionalCaveatsOnDischarge); err != nil {
		return security.Discharge{}, err
	}
	return security.NewDischarge(discharge), nil
}

func (c *client) PublicKey() security.PublicKey {
	return c.key
}

func (c *client) BlessingsByName(pattern security.BlessingPattern) []security.Blessings {
	var wbResults []security.WireBlessings
	err := c.caller.call("BlessingsByName", results(&wbResults), pattern)
	if err != nil {
		vlog.Errorf("error calling BlessingsByName: %v", err)
		return nil
	}
	blessings := make([]security.Blessings, len(wbResults))
	for i, wb := range wbResults {
		var err error
		blessings[i], err = security.NewBlessings(wb)
		if err != nil {
			vlog.Errorf("error creating Blessing from WireBlessings: %v", err)
		}
	}
	return blessings
}

func (c *client) BlessingsInfo(blessings security.Blessings) map[string][]security.Caveat {
	var bInfo map[string][]security.Caveat
	err := c.caller.call("BlessingsInfo", results(&bInfo), security.MarshalBlessings(blessings))
	if err != nil {
		vlog.Errorf("error calling BlessingsInfo: %v", err)
		return nil
	}
	return bInfo
}
func (c *client) BlessingStore() security.BlessingStore {
	return &blessingStore{c.caller, c.key}
}

func (c *client) Roots() security.BlessingRoots {
	return &blessingRoots{c.caller}
}

func (c *client) AddToRoots(blessings security.Blessings) error {
	return c.caller.call("AddToRoots", results(), security.MarshalBlessings(blessings))
}

type blessingStore struct {
	caller caller
	key    security.PublicKey
}

func (b *blessingStore) Set(blessings security.Blessings, forPeers security.BlessingPattern) (security.Blessings, error) {
	var resultBlessings security.WireBlessings
	if err := b.caller.call("BlessingStoreSet", results(&resultBlessings), security.MarshalBlessings(blessings), forPeers); err != nil {
		return security.Blessings{}, err
	}
	return security.NewBlessings(resultBlessings)
}

func (b *blessingStore) ForPeer(peerBlessings ...string) security.Blessings {
	var resultBlessings security.WireBlessings
	err := b.caller.call("BlessingStoreForPeer", results(&resultBlessings), peerBlessings)
	if err != nil {
		vlog.Errorf("error calling BlessingStoreForPeer: %v", err)
		return security.Blessings{}
	}
	blessings, err := security.NewBlessings(resultBlessings)
	if err != nil {
		vlog.Errorf("error creating Blessings from WireBlessings: %v", err)
		return security.Blessings{}
	}
	return blessings
}

func (b *blessingStore) SetDefault(blessings security.Blessings) error {
	return b.caller.call("BlessingStoreSetDefault", results(), security.MarshalBlessings(blessings))
}

func (b *blessingStore) Default() security.Blessings {
	var resultBlessings security.WireBlessings
	err := b.caller.call("BlessingStoreDefault", results(&resultBlessings))
	if err != nil {
		vlog.Errorf("error calling BlessingStoreDefault: %v", err)
		return security.Blessings{}
	}
	blessings, err := security.NewBlessings(resultBlessings)
	if err != nil {
		vlog.Errorf("error creating Blessing from WireBlessings: %v", err)
	}
	return blessings
}

func (b *blessingStore) PublicKey() security.PublicKey {
	return b.key
}

func (b *blessingStore) PeerBlessings() map[security.BlessingPattern]security.Blessings {
	var wbMap map[security.BlessingPattern]security.WireBlessings
	err := b.caller.call("BlessingStorePeerBlessings", results(&wbMap))
	if err != nil {
		vlog.Errorf("error calling BlessingStorePeerBlessings: %v", err)
		return nil
	}
	bMap := make(map[security.BlessingPattern]security.Blessings)
	for pattern, wb := range wbMap {
		blessings, err := security.NewBlessings(wb)
		if err != nil {
			vlog.Errorf("error creating Blessing from WireBlessings: %v", err)
			return nil
		}
		bMap[pattern] = blessings
	}
	return bMap
}

func (b *blessingStore) DebugString() (s string) {
	err := b.caller.call("BlessingStoreDebugString", results(&s))
	if err != nil {
		s = fmt.Sprintf("error calling BlessingStoreDebugString: %v", err)
		vlog.Errorf(s)
	}
	return
}

type blessingRoots struct {
	caller caller
}

func (b *blessingRoots) Add(root security.PublicKey, pattern security.BlessingPattern) error {
	marshalledKey, err := root.MarshalBinary()
	if err != nil {
		return err
	}
	return b.caller.call("BlessingRootsAdd", results(), marshalledKey, pattern)
}

func (b *blessingRoots) Recognized(root security.PublicKey, blessing string) error {
	marshalledKey, err := root.MarshalBinary()
	if err != nil {
		return err
	}
	return b.caller.call("BlessingRootsRecognized", results(), marshalledKey, blessing)
}

func (b *blessingRoots) DebugString() (s string) {
	err := b.caller.call("BlessingRootsDebugString", results(&s))
	if err != nil {
		s = fmt.Sprintf("error calling BlessingRootsDebugString: %v", err)
		vlog.Errorf(s)
	}
	return
}
