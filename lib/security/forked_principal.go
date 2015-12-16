// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"fmt"
	"sync"

	"v.io/v23/security"
	"v.io/v23/verror"
)

var (
	errImmutable         = verror.Register(pkgPath+".errImmutable", verror.NoRetry, "{1:}{2:} mutation not supported on this immutable type (type={3:} method={4:}")
	errPublicKeyMismatch = verror.Register(pkgPath+".errPublicKeyMismatch", verror.NoRetry, "{1:}{2:} principal's public key {3:} does not match store's public key {4:}")
)

// ForkPrincipal returns a principal that has the same private key as p but
// uses store and roots instead of the BlessingStore and BlessingRoots in p.
func ForkPrincipal(p security.Principal, store security.BlessingStore, roots security.BlessingRoots) (security.Principal, error) {
	k1, err := p.PublicKey().MarshalBinary()
	if err != nil {
		return nil, err
	}
	k2, err := store.PublicKey().MarshalBinary()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(k1, k2) {
		return nil, verror.New(errPublicKeyMismatch, nil, p.PublicKey(), store.PublicKey())
	}
	return &forkedPrincipal{p, store, roots}, nil
}

// MustForkPrincipal is identical to ForkPrincipal, except that it panics on
// error (such as if store is bound to a different PublicKey than p).
func MustForkPrincipal(p security.Principal, store security.BlessingStore, roots security.BlessingRoots) security.Principal {
	p, err := ForkPrincipal(p, store, roots)
	if err != nil {
		panic(err)
	}
	return p
}

// ImmutableBlessingRoots returns a BlessingRoots implementation that is
// identical to r, except that all mutation operations fail.
func ImmutableBlessingRoots(r security.BlessingRoots) security.BlessingRoots {
	return &immutableBlessingRoots{impl: r}
}

// ImmutableBlessingStore returns a BlessingStore implementation that is
// identical to r, except that Set* methods will fail.
// (Mutation in the form of adding discharges via CacheDischarge are still allowed).
func ImmutableBlessingStore(s security.BlessingStore) security.BlessingStore {
	return &immutableBlessingStore{impl: s}
}

// FixedBlessingsStore returns a BlessingStore implementation that always
// returns a fixed set of blessings (b) for both Default and ForPeer.
func FixedBlessingsStore(b security.Blessings) security.BlessingStore {
	return &fixedBlessingsStore{b: b, dcache: make(map[dischargeCacheKey]security.Discharge)}
}

type forkedPrincipal struct {
	security.Principal
	store security.BlessingStore
	roots security.BlessingRoots
}

func (p *forkedPrincipal) BlessingStore() security.BlessingStore {
	return p.store
}

func (p *forkedPrincipal) Roots() security.BlessingRoots {
	return p.roots
}

type immutableBlessingStore struct {
	// Do not embed BlessingRoots since that will make it easy to miss
	// interface changes if a mutating method is added to the interface.
	impl security.BlessingStore
}

func (s *immutableBlessingStore) Set(security.Blessings, security.BlessingPattern) (security.Blessings, error) {
	return security.Blessings{}, verror.New(errImmutable, nil, fmt.Sprintf("%T", s), "Set")
}
func (s *immutableBlessingStore) ForPeer(peerBlessings ...string) security.Blessings {
	return s.impl.ForPeer(peerBlessings...)
}
func (s *immutableBlessingStore) SetDefault(security.Blessings) error {
	return verror.New(errImmutable, nil, fmt.Sprintf("%T", s), "SetDefault")
}
func (s *immutableBlessingStore) Default() security.Blessings {
	return s.impl.Default()
}
func (s *immutableBlessingStore) PublicKey() security.PublicKey {
	return s.impl.PublicKey()
}
func (s *immutableBlessingStore) PeerBlessings() map[security.BlessingPattern]security.Blessings {
	return s.impl.PeerBlessings()
}
func (s *immutableBlessingStore) CacheDischarge(discharge security.Discharge, caveat security.Caveat, impetus security.DischargeImpetus) {
	s.impl.CacheDischarge(discharge, caveat, impetus)
}
func (s *immutableBlessingStore) ClearDischarges(discharges ...security.Discharge) {
	s.impl.ClearDischarges(discharges...)
}
func (s *immutableBlessingStore) Discharge(caveat security.Caveat, impetus security.DischargeImpetus) security.Discharge {
	return s.impl.Discharge(caveat, impetus)
}
func (s *immutableBlessingStore) DebugString() string {
	return s.impl.DebugString()
}

type fixedBlessingsStore struct {
	b      security.Blessings
	mu     sync.Mutex
	dcache map[dischargeCacheKey]security.Discharge
}

func (s *fixedBlessingsStore) Set(security.Blessings, security.BlessingPattern) (security.Blessings, error) {
	return security.Blessings{}, verror.New(errImmutable, nil, fmt.Sprintf("%T", s), "Set")
}
func (s *fixedBlessingsStore) ForPeer(peerBlessings ...string) security.Blessings {
	return s.b
}
func (s *fixedBlessingsStore) SetDefault(security.Blessings) error {
	return verror.New(errImmutable, nil, fmt.Sprintf("%T", s), "SetDefault")
}
func (s *fixedBlessingsStore) Default() security.Blessings {
	return s.b
}
func (s *fixedBlessingsStore) PublicKey() security.PublicKey {
	return s.b.PublicKey()
}
func (s *fixedBlessingsStore) PeerBlessings() map[security.BlessingPattern]security.Blessings {
	return map[security.BlessingPattern]security.Blessings{security.AllPrincipals: s.b}
}
func (s *fixedBlessingsStore) CacheDischarge(discharge security.Discharge, caveat security.Caveat, impetus security.DischargeImpetus) {
	id := discharge.ID()
	key, cacheable := dcacheKey(caveat.ThirdPartyDetails(), impetus)
	if id == "" || !cacheable {
		return
	}
	s.mu.Lock()
	s.dcache[key] = discharge
	s.mu.Unlock()
}
func (s *fixedBlessingsStore) ClearDischarges(discharges ...security.Discharge) {
	s.mu.Lock()
	clearDischargesFromCache(s.dcache, discharges...)
	s.mu.Unlock()
}
func (s *fixedBlessingsStore) Discharge(caveat security.Caveat, impetus security.DischargeImpetus) security.Discharge {
	key, cacheable := dcacheKey(caveat.ThirdPartyDetails(), impetus)
	if !cacheable {
		return security.Discharge{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return dischargeFromCache(s.dcache, key)
}
func (s *fixedBlessingsStore) DebugString() string {
	return fmt.Sprintf("FixedBlessingsStore:[%v]", s.b)
}

type immutableBlessingRoots struct {
	// Do not embed BlessingRoots since that will make it easy to miss
	// interface changes if a mutation method is added to the interface.
	impl security.BlessingRoots
}

func (r *immutableBlessingRoots) Recognized(root []byte, blessing string) error {
	return r.impl.Recognized(root, blessing)
}
func (r *immutableBlessingRoots) Dump() map[security.BlessingPattern][]security.PublicKey {
	return r.impl.Dump()
}
func (r *immutableBlessingRoots) DebugString() string { return r.impl.DebugString() }
func (r *immutableBlessingRoots) Add([]byte, security.BlessingPattern) error {
	return verror.New(errImmutable, nil, fmt.Sprintf("%T", r), "Add")
}
