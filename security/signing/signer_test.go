package signing

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"testing"
)

func TestSigner(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Couldn't generate ECDSA private key: %v", err)
	}
	signer := NewClearSigner(key)
	testdata := [][]byte{
		nil,
		[]byte{},
		[]byte(""),
		[]byte("withorwithoutyou"),
		[]byte("wherethestreetshavenoname"),
	}
	for _, d := range testdata {
		hash := sha256.Sum256(d)
		sig, err := signer.Sign(hash[:])
		if err != nil {
			t.Errorf("Sign(%q) returned err: %q, expected success.", d, err)
			continue
		}
		if !sig.Verify(signer.PublicKey(), hash[:]) {
			t.Errorf("Sign(%q) signature couldn't be verified.", d)
		}
	}
}
