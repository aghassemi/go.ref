// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"crypto/rand"
	"reflect"
	"sync"

	"golang.org/x/crypto/nacl/box"
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/rpc/version"
	"v.io/v23/security"
	"v.io/v23/vom"
)

func (c *Conn) dialHandshake(ctx *context.T, versions version.RPCVersionRange) error {
	binding, err := c.setup(ctx, versions)
	if err != nil {
		return err
	}
	c.blessingsFlow = newBlessingsFlow(ctx, c.newFlowLocked(ctx, blessingsFlowID, 0, 0, true, true), true)
	if err = c.readRemoteAuth(ctx, binding); err != nil {
		return err
	}
	if c.rBlessings.IsZero() {
		return NewErrAcceptorBlessingsMissing(ctx)
	}
	signedBinding, err := v23.GetPrincipal(ctx).Sign(binding)
	if err != nil {
		return err
	}
	lAuth := &auth{
		channelBinding: signedBinding,
	}
	// We only send our blessings if we are a server in addition to being a client.
	// If we are a pure client, we only send our public key.
	if c.handler != nil {
		bkey, dkey, err := c.blessingsFlow.put(ctx, c.lBlessings, c.lDischarges)
		if err != nil {
			return err
		}
		lAuth.bkey, lAuth.dkey = bkey, dkey
	} else {
		lAuth.publicKey = c.lBlessings.PublicKey()
	}
	if err = c.mp.writeMsg(ctx, lAuth); err != nil {
		return err
	}
	return err
}

func (c *Conn) acceptHandshake(ctx *context.T, versions version.RPCVersionRange) error {
	binding, err := c.setup(ctx, versions)
	if err != nil {
		return err
	}
	c.blessingsFlow = newBlessingsFlow(ctx, c.newFlowLocked(ctx, blessingsFlowID, 0, 0, true, true), false)
	signedBinding, err := v23.GetPrincipal(ctx).Sign(binding)
	if err != nil {
		return err
	}
	bkey, dkey, err := c.blessingsFlow.put(ctx, c.lBlessings, c.lDischarges)
	if err != nil {
		return err
	}
	err = c.mp.writeMsg(ctx, &auth{
		bkey:           bkey,
		dkey:           dkey,
		channelBinding: signedBinding,
	})
	if err != nil {
		return err
	}
	return c.readRemoteAuth(ctx, binding)
}

func (c *Conn) setup(ctx *context.T, versions version.RPCVersionRange) ([]byte, error) {
	pk, sk, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	lSetup := &setup{
		versions:          versions,
		peerLocalEndpoint: c.local,
		peerNaClPublicKey: pk,
	}
	if c.remote != nil {
		lSetup.peerRemoteEndpoint = c.remote
	}
	ch := make(chan error)
	go func() {
		ch <- c.mp.writeMsg(ctx, lSetup)
	}()
	msg, err := c.mp.readMsg(ctx)
	if err != nil {
		return nil, NewErrRecv(ctx, "unknown", err)
	}
	rSetup, valid := msg.(*setup)
	if !valid {
		return nil, NewErrUnexpectedMsg(ctx, reflect.TypeOf(msg).String())
	}
	if err := <-ch; err != nil {
		return nil, NewErrSend(ctx, "setup", c.remote.String(), err)
	}
	if c.version, err = version.CommonVersion(ctx, lSetup.versions, rSetup.versions); err != nil {
		return nil, err
	}
	// TODO(mattr): Decide which endpoints to actually keep, the ones we know locally
	// or what the remote side thinks.
	if rSetup.peerRemoteEndpoint != nil {
		c.local = rSetup.peerRemoteEndpoint
	}
	if rSetup.peerLocalEndpoint != nil {
		c.remote = rSetup.peerLocalEndpoint
	}
	if rSetup.peerNaClPublicKey == nil {
		return nil, NewErrMissingSetupOption(ctx, peerNaClPublicKeyOption)
	}
	return c.mp.setupEncryption(ctx, pk, sk, rSetup.peerNaClPublicKey), nil
}

func (c *Conn) readRemoteAuth(ctx *context.T, binding []byte) error {
	var rauth *auth
	for {
		msg, err := c.mp.readMsg(ctx)
		if err != nil {
			return NewErrRecv(ctx, c.remote.String(), err)
		}
		if rauth, _ = msg.(*auth); rauth != nil {
			break
		}
		if err = c.handleMessage(ctx, msg); err != nil {
			return err
		}
	}
	var rPublicKey security.PublicKey
	if rauth.bkey != 0 {
		var err error
		// TODO(mattr): Make sure we cancel out of this at some point.
		c.rBlessings, c.rDischarges, err = c.blessingsFlow.get(ctx, rauth.bkey, rauth.dkey)
		if err != nil {
			return err
		}
		rPublicKey = c.rBlessings.PublicKey()
	} else {
		rPublicKey = rauth.publicKey
	}
	if rPublicKey == nil {
		return NewErrNoPublicKey(ctx)
	}
	if !rauth.channelBinding.Verify(rPublicKey, binding) {
		return NewErrInvalidChannelBinding(ctx)
	}
	return nil
}

type blessingsFlow struct {
	enc *vom.Encoder
	dec *vom.Decoder

	mu      sync.Mutex
	cond    *sync.Cond
	closed  bool
	nextKey uint64
	byUID   map[string]*Blessings
	byBKey  map[uint64]*Blessings
}

func newBlessingsFlow(ctx *context.T, f flow.Flow, dialed bool) *blessingsFlow {
	b := &blessingsFlow{
		enc:     vom.NewEncoder(f),
		dec:     vom.NewDecoder(f),
		nextKey: 1,
		byUID:   make(map[string]*Blessings),
		byBKey:  make(map[uint64]*Blessings),
	}
	b.cond = sync.NewCond(&b.mu)
	if !dialed {
		b.nextKey++
	}
	go b.readLoop(ctx)
	return b
}

func (b *blessingsFlow) put(ctx *context.T, blessings security.Blessings, discharges map[string]security.Discharge) (bkey, dkey uint64, err error) {
	defer b.mu.Unlock()
	b.mu.Lock()
	buid := string(blessings.UniqueID())
	element, has := b.byUID[buid]
	if has && equalDischarges(discharges, element.Discharges) {
		return element.BKey, element.DKey, nil
	}
	defer b.cond.Broadcast()
	if has {
		element.Discharges = dischargeList(discharges)
		element.DKey = b.nextKey
		b.nextKey += 2
		return element.BKey, element.DKey, b.enc.Encode(Blessings{
			Discharges: element.Discharges,
			DKey:       element.DKey,
		})
	}
	element = &Blessings{
		Blessings:  blessings,
		Discharges: dischargeList(discharges),
		BKey:       b.nextKey,
	}
	b.nextKey += 2
	if len(discharges) > 0 {
		element.DKey = b.nextKey
		b.nextKey += 2
	}
	b.byUID[buid] = element
	b.byBKey[element.BKey] = element
	return element.BKey, element.DKey, b.enc.Encode(element)
}

func (b *blessingsFlow) get(ctx *context.T, bkey, dkey uint64) (security.Blessings, map[string]security.Discharge, error) {
	defer b.mu.Unlock()
	b.mu.Lock()
	for !b.closed {
		element, has := b.byBKey[bkey]
		if has && element.DKey == dkey {
			return element.Blessings, dischargeMap(element.Discharges), nil
		}
		b.cond.Wait()
	}
	return security.Blessings{}, nil, NewErrBlessingsFlowClosed(ctx)
}

func (b *blessingsFlow) readLoop(ctx *context.T) {
	for {
		var received Blessings
		err := b.dec.Decode(&received)
		b.mu.Lock()
		if err != nil {
			b.closed = true
			b.mu.Unlock()
			return
		}
		b.byUID[string(received.Blessings.UniqueID())] = &received
		b.byBKey[received.BKey] = &received
		b.cond.Broadcast()
		b.mu.Unlock()
	}
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
