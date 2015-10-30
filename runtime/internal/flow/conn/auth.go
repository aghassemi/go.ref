// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"crypto/rand"
	"io"
	"reflect"
	"sync"
	"time"

	"golang.org/x/crypto/nacl/box"
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/flow/message"
	"v.io/v23/naming"
	"v.io/v23/rpc/version"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/v23/vom"
	slib "v.io/x/ref/lib/security"
	iflow "v.io/x/ref/runtime/internal/flow"
	inaming "v.io/x/ref/runtime/internal/naming"
)

var (
	authDialerTag   = []byte("AuthDial\x00")
	authAcceptorTag = []byte("AuthAcpt\x00")
)

func (c *Conn) dialHandshake(ctx *context.T, versions version.RPCVersionRange, auth flow.PeerAuthorizer) error {
	binding, remoteEndpoint, err := c.setup(ctx, versions)
	if err != nil {
		return err
	}
	c.isProxy = c.remote.RoutingID() != naming.NullRoutingID && c.remote.RoutingID() != remoteEndpoint.RoutingID()
	// We use the remote ends local endpoint as our remote endpoint when the routingID's
	// of the endpoints differ. This is an indicator that we are talking to a proxy.
	// This means that the manager will need to dial a subsequent conn on this conn
	// to the end server.
	c.remote.(*inaming.Endpoint).RID = remoteEndpoint.RoutingID()
	bflow := c.newFlowLocked(ctx, blessingsFlowID, 0, 0, nil, true, true)
	bflow.releaseLocked(DefaultBytesBufferedPerFlow)
	c.blessingsFlow = newBlessingsFlow(ctx, &c.loopWG, bflow)

	rBlessings, rDischarges, err := c.readRemoteAuth(ctx, binding, true)
	if err != nil {
		return err
	}
	if rBlessings.IsZero() {
		return NewErrAcceptorBlessingsMissing(ctx)
	}
	if !c.isProxy {
		if _, _, err := auth.AuthorizePeer(ctx, c.local, c.remote, rBlessings, rDischarges); err != nil {
			return iflow.MaybeWrapError(verror.ErrNotTrusted, ctx, err)
		}
	}
	signedBinding, err := v23.GetPrincipal(ctx).Sign(append(authDialerTag, binding...))
	if err != nil {
		return err
	}
	lAuth := &message.Auth{
		ChannelBinding: signedBinding,
	}
	// We only send our real blessings if we are a server in addition to being a client,
	// and we are not talking through a proxy.
	// Otherwise, we only send our public key through a nameless blessings object.
	if c.lBlessings.IsZero() || c.handler == nil || c.isProxy {
		c.lBlessings, _ = security.NamelessBlessing(v23.GetPrincipal(ctx).PublicKey())
	}
	if lAuth.BlessingsKey, _, err = c.blessingsFlow.send(ctx, c.lBlessings, nil); err != nil {
		return err
	}
	if err = c.mp.writeMsg(ctx, lAuth); err != nil {
		return err
	}
	// We send discharges asynchronously to prevent making a second RPC while
	// trying to build up the connection for another. If the two RPCs happen to
	// go to the same server a deadlock will result.
	// This commonly happens when we make a Resolve call.  During the Resolve we
	// will try to fetch discharges to send to the mounttable, leading to a
	// Resolve of the discharge server name.  The two resolve calls may be to
	// the same mounttable.
	c.loopWG.Add(1)
	go func() {
		c.refreshDischarges(ctx)
		c.loopWG.Done()
	}()
	return nil
}

func (c *Conn) acceptHandshake(ctx *context.T, versions version.RPCVersionRange) error {
	binding, remoteEndpoint, err := c.setup(ctx, versions)
	if err != nil {
		return err
	}
	c.isProxy = false
	c.remote = remoteEndpoint
	c.blessingsFlow = newBlessingsFlow(ctx, &c.loopWG,
		c.newFlowLocked(ctx, blessingsFlowID, 0, 0, nil, true, true))
	signedBinding, err := v23.GetPrincipal(ctx).Sign(append(authAcceptorTag, binding...))
	if err != nil {
		return err
	}
	lAuth := &message.Auth{
		ChannelBinding: signedBinding,
	}
	if lAuth.BlessingsKey, lAuth.DischargeKey, err = c.refreshDischarges(ctx); err != nil {
		return err
	}
	if err = c.mp.writeMsg(ctx, lAuth); err != nil {
		return err
	}
	_, _, err = c.readRemoteAuth(ctx, binding, false)
	return err
}

func (c *Conn) setup(ctx *context.T, versions version.RPCVersionRange) ([]byte, naming.Endpoint, error) {
	pk, sk, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	lSetup := &message.Setup{
		Versions:          versions,
		PeerLocalEndpoint: c.local,
		PeerNaClPublicKey: pk,
	}
	if c.remote != nil {
		lSetup.PeerRemoteEndpoint = c.remote
	}
	ch := make(chan error)
	go func() {
		ch <- c.mp.writeMsg(ctx, lSetup)
	}()
	msg, err := c.mp.readMsg(ctx)
	if err != nil {
		<-ch
		if verror.ErrorID(err) == message.ErrWrongProtocol.ID {
			return nil, nil, err
		}
		return nil, nil, NewErrRecv(ctx, "unknown", err)
	}
	rSetup, valid := msg.(*message.Setup)
	if !valid {
		<-ch
		return nil, nil, NewErrUnexpectedMsg(ctx, reflect.TypeOf(msg).String())
	}
	if err := <-ch; err != nil {
		return nil, nil, NewErrSend(ctx, "setup", c.remote.String(), err)
	}
	if c.version, err = version.CommonVersion(ctx, lSetup.Versions, rSetup.Versions); err != nil {
		return nil, nil, err
	}
	if c.local == nil {
		c.local = rSetup.PeerRemoteEndpoint
	}
	if rSetup.PeerNaClPublicKey == nil {
		return nil, nil, NewErrMissingSetupOption(ctx, "peerNaClPublicKey")
	}
	binding := c.mp.setupEncryption(ctx, pk, sk, rSetup.PeerNaClPublicKey)
	// if we're encapsulated in another flow, tell that flow to stop
	// encrypting now that we've started.
	if f, ok := c.mp.rw.(*flw); ok {
		f.disableEncryption()
	}
	return binding, rSetup.PeerLocalEndpoint, nil
}

func (c *Conn) readRemoteAuth(ctx *context.T, binding []byte, dialer bool) (security.Blessings, map[string]security.Discharge, error) {
	tag := authDialerTag
	if dialer {
		tag = authAcceptorTag
	}
	var rauth *message.Auth
	for {
		msg, err := c.mp.readMsg(ctx)
		if err != nil {
			return security.Blessings{}, nil, NewErrRecv(ctx, c.remote.String(), err)
		}
		if rauth, _ = msg.(*message.Auth); rauth != nil {
			break
		}
		if err = c.handleMessage(ctx, msg); err != nil {
			return security.Blessings{}, nil, err
		}
	}
	c.rBKey = rauth.BlessingsKey
	// Only read the blessings if we were the dialer. Any blessings from the dialer
	// will be sent later.
	var rBlessings security.Blessings
	var rDischarges map[string]security.Discharge
	if rauth.BlessingsKey != 0 {
		var err error
		// TODO(mattr): Make sure we cancel out of this at some point.
		rBlessings, rDischarges, err = c.blessingsFlow.getRemote(ctx, rauth.BlessingsKey, rauth.DischargeKey)
		if err != nil {
			return security.Blessings{}, nil, err
		}
		c.mu.Lock()
		c.rPublicKey = rBlessings.PublicKey()
		c.mu.Unlock()
	}
	if c.rPublicKey == nil {
		return security.Blessings{}, nil, NewErrNoPublicKey(ctx)
	}
	if !rauth.ChannelBinding.Verify(c.rPublicKey, append(tag, binding...)) {
		return security.Blessings{}, nil, NewErrInvalidChannelBinding(ctx)
	}
	return rBlessings, rDischarges, nil
}

func (c *Conn) refreshDischarges(ctx *context.T) (bkey, dkey uint64, err error) {
	dis := slib.PrepareDischarges(ctx, c.lBlessings,
		security.DischargeImpetus{}, time.Minute)
	// Schedule the next update.
	dur, expires := minExpiryTime(c.lBlessings, dis)
	c.mu.Lock()
	if expires && c.status < Closing {
		c.loopWG.Add(1)
		c.dischargeTimer = time.AfterFunc(dur, func() {
			c.refreshDischarges(ctx)
			c.loopWG.Done()
		})
	}
	c.mu.Unlock()
	bkey, dkey, err = c.blessingsFlow.send(ctx, c.lBlessings, dis)
	return
}

func minExpiryTime(blessings security.Blessings, discharges map[string]security.Discharge) (time.Duration, bool) {
	var min time.Time
	cavCount := len(blessings.ThirdPartyCaveats())
	if cavCount == 0 {
		return 0, false
	}
	for _, d := range discharges {
		if exp := d.Expiry(); min.IsZero() || (!exp.IsZero() && exp.Before(min)) {
			min = exp
		}
	}
	if min.IsZero() && cavCount == len(discharges) {
		return 0, false
	}
	now := time.Now()
	d := min.Sub(now)
	if d > time.Minute && cavCount > len(discharges) {
		d = time.Minute
	}
	return d, true
}

func newBlessingsFlow(ctx *context.T, loopWG *sync.WaitGroup, f *flw) *blessingsFlow {
	b := &blessingsFlow{
		f:       f,
		enc:     vom.NewEncoder(f),
		dec:     vom.NewDecoder(f),
		nextKey: 1,
		incoming: &inCache{
			blessings:  make(map[uint64]security.Blessings),
			dkeys:      make(map[uint64]uint64),
			discharges: make(map[uint64][]security.Discharge),
		},
		outgoing: &outCache{
			bkeys:      make(map[string]uint64),
			dkeys:      make(map[uint64]uint64),
			blessings:  make(map[uint64]security.Blessings),
			discharges: make(map[uint64][]security.Discharge),
		},
	}
	b.cond = sync.NewCond(&b.mu)
	loopWG.Add(1)
	go b.readLoop(ctx, loopWG)
	return b
}

type blessingsFlow struct {
	enc *vom.Encoder
	dec *vom.Decoder
	f   *flw

	mu       sync.Mutex
	cond     *sync.Cond
	closeErr error
	nextKey  uint64
	incoming *inCache
	outgoing *outCache
}

// inCache keeps track of incoming blessings, discharges, and keys.
type inCache struct {
	dkeys      map[uint64]uint64               // bkey -> dkey of the latest discharges.
	blessings  map[uint64]security.Blessings   // keyed by bkey
	discharges map[uint64][]security.Discharge // keyed by dkey
}

// outCache keeps track of outgoing blessings, discharges, and keys.
type outCache struct {
	bkeys map[string]uint64 // blessings uid -> bkey

	dkeys      map[uint64]uint64               // blessings bkey -> dkey of latest discharges
	blessings  map[uint64]security.Blessings   // keyed by bkey
	discharges map[uint64][]security.Discharge // keyed by dkey
}

func (b *blessingsFlow) receive(ctx *context.T, bd BlessingsFlowMessage) error {
	switch bd := bd.(type) {
	case BlessingsFlowMessageBlessings:
		bkey, blessings := bd.Value.BKey, bd.Value.Blessings
		// When accepting, make sure the blessings received are bound to the conn's
		// remote public key.
		b.f.conn.mu.Lock()
		if pk := b.f.conn.rPublicKey; pk != nil && !reflect.DeepEqual(blessings.PublicKey(), pk) {
			b.f.conn.mu.Unlock()
			return NewErrBlessingsNotBound(ctx)
		}
		b.f.conn.mu.Unlock()
		b.mu.Lock()
		b.incoming.blessings[bkey] = blessings
		b.mu.Unlock()
	case BlessingsFlowMessageDischarges:
		bkey, dkey, discharges := bd.Value.BKey, bd.Value.DKey, bd.Value.Discharges
		b.mu.Lock()
		b.incoming.discharges[dkey] = discharges
		b.incoming.dkeys[bkey] = dkey
		b.mu.Unlock()
	}
	b.cond.Broadcast()
	return nil
}

func (b *blessingsFlow) getRemote(ctx *context.T, bkey, dkey uint64) (security.Blessings, map[string]security.Discharge, error) {
	defer b.mu.Unlock()
	b.mu.Lock()
	for {
		blessings, hasB := b.incoming.blessings[bkey]
		if hasB {
			if dkey == 0 {
				return blessings, nil, nil
			}
			discharges, hasD := b.incoming.discharges[dkey]
			if hasD {
				return blessings, dischargeMap(discharges), nil
			}
		}
		// We check closeErr after we check the map to allow gets to succeed even after
		// the blessings flow is closed.
		if b.closeErr != nil {
			break
		}
		b.cond.Wait()
	}
	return security.Blessings{}, nil, b.closeErr
}

func (b *blessingsFlow) getLatestRemote(ctx *context.T, bkey uint64) (security.Blessings, map[string]security.Discharge, error) {
	defer b.mu.Unlock()
	b.mu.Lock()
	for {
		blessings, has := b.incoming.blessings[bkey]
		if has {
			dkey := b.incoming.dkeys[bkey]
			discharges := b.incoming.discharges[dkey]
			return blessings, dischargeMap(discharges), nil
		}
		// We check closeErr after we check the map to allow gets to succeed even after
		// the blessings flow is closed.
		if b.closeErr != nil {
			break
		}
		b.cond.Wait()
	}
	return security.Blessings{}, nil, b.closeErr
}

func (b *blessingsFlow) send(ctx *context.T, blessings security.Blessings, discharges map[string]security.Discharge) (bkey, dkey uint64, err error) {
	if blessings.IsZero() {
		return 0, 0, nil
	}
	defer b.mu.Unlock()
	b.mu.Lock()
	buid := string(blessings.UniqueID())
	bkey, hasB := b.outgoing.bkeys[buid]
	if !hasB {
		bkey = b.nextKey
		b.nextKey++
		b.outgoing.bkeys[buid] = bkey
		b.outgoing.blessings[bkey] = blessings
		if err := b.enc.Encode(BlessingsFlowMessageBlessings{Blessings{
			BKey:      bkey,
			Blessings: blessings,
		}}); err != nil {
			return 0, 0, err
		}
	}
	if len(discharges) == 0 {
		return bkey, 0, nil
	}
	dkey, hasD := b.outgoing.dkeys[bkey]
	if hasD && equalDischarges(discharges, b.outgoing.discharges[dkey]) {
		return bkey, dkey, nil
	}
	dlist := dischargeList(discharges)
	dkey = b.nextKey
	b.nextKey++
	b.outgoing.dkeys[bkey] = dkey
	b.outgoing.discharges[dkey] = dlist
	return bkey, dkey, b.enc.Encode(BlessingsFlowMessageDischarges{Discharges{
		BKey:       bkey,
		DKey:       dkey,
		Discharges: dlist,
	}})
}

func (b *blessingsFlow) getLocal(ctx *context.T, bkey, dkey uint64) (security.Blessings, map[string]security.Discharge) {
	defer b.mu.Unlock()
	b.mu.Lock()
	blessings := b.outgoing.blessings[bkey]
	discharges := b.outgoing.discharges[dkey]
	return blessings, dischargeMap(discharges)
}

func (b *blessingsFlow) getLatestLocal(ctx *context.T, blessings security.Blessings) map[string]security.Discharge {
	defer b.mu.Unlock()
	b.mu.Lock()
	buid := string(blessings.UniqueID())
	bkey := b.outgoing.bkeys[buid]
	dkey := b.outgoing.dkeys[bkey]
	discharges := b.outgoing.discharges[dkey]
	return dischargeMap(discharges)
}

func (b *blessingsFlow) readLoop(ctx *context.T, loopWG *sync.WaitGroup) {
	defer loopWG.Done()
	for {
		var received BlessingsFlowMessage
		err := b.dec.Decode(&received)
		if err != nil {
			if err != io.EOF {
				// TODO(mattr): In practice this is very spammy,
				// figure out how to log it more effectively.
				ctx.VI(3).Infof("Blessings flow closed: %v", err)
			}
			b.mu.Lock()
			b.closeErr = NewErrBlessingsFlowClosed(ctx, err)
			b.mu.Unlock()
			return
		}
		if err := b.receive(ctx, received); err != nil {
			b.f.conn.mu.Lock()
			b.f.conn.internalCloseLocked(ctx, err)
			b.f.conn.mu.Unlock()
			return
		}
	}
}

func (b *blessingsFlow) close(ctx *context.T, err error) {
	defer b.mu.Unlock()
	b.mu.Lock()
	err = NewErrBlessingsFlowClosed(ctx, err)
	b.f.close(ctx, err)
	b.closeErr = err
	b.cond.Broadcast()
}

func dischargeList(in map[string]security.Discharge) []security.Discharge {
	out := make([]security.Discharge, 0, len(in))
	for _, d := range in {
		out = append(out, d)
	}
	return out
}
func dischargeMap(in []security.Discharge) map[string]security.Discharge {
	out := make(map[string]security.Discharge, len(in))
	for _, d := range in {
		out[d.ID()] = d
	}
	return out
}
func equalDischarges(m map[string]security.Discharge, s []security.Discharge) bool {
	if len(m) != len(s) {
		return false
	}
	for _, d := range s {
		if !d.Equivalent(m[d.ID()]) {
			return false
		}
	}
	return true
}
