package vc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"

	"v.io/v23/security"
	"v.io/v23/vlog"
)

var AnonymousPrincipal security.Principal

func init() {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		vlog.Fatalf("could not create private key for anonymous principal: %v", err)
	}
	store := &anonymousBlessingStore{k: security.NewECDSAPublicKey(&key.PublicKey)}
	if AnonymousPrincipal, err = security.CreatePrincipal(security.NewInMemoryECDSASigner(key), store, nil); err != nil {
		vlog.Fatalf("could not create anonymous principal: %v", err)
	}
	if store.b, err = AnonymousPrincipal.BlessSelf("anonymous"); err != nil {
		vlog.Fatalf("failed to generate the one blessing to be used by the anonymous principal: %v", err)
	}
}

// TODO(ashankar,ataly): Figure out what to do with this!
// (Most likely move the BlessingStore implementation from veyron/runtimes/google/rt to veyron/security
// and use that?)
type anonymousBlessingStore struct {
	k security.PublicKey
	b security.Blessings
}

func (s *anonymousBlessingStore) Set(security.Blessings, security.BlessingPattern) (security.Blessings, error) {
	return security.Blessings{}, fmt.Errorf("cannot store blessings with an anonymous principal")
}

func (s *anonymousBlessingStore) ForPeer(...string) security.Blessings {
	return s.b
}

func (s *anonymousBlessingStore) SetDefault(security.Blessings) error {
	return fmt.Errorf("cannot change default blessing associated with the anonymous principal")
}

func (s *anonymousBlessingStore) Default() security.Blessings {
	return s.b
}

func (s *anonymousBlessingStore) PublicKey() security.PublicKey {
	return s.k
}

func (s *anonymousBlessingStore) PeerBlessings() map[security.BlessingPattern]security.Blessings {
	return nil
}

func (anonymousBlessingStore) DebugString() string {
	return "anonymous BlessingStore"
}
