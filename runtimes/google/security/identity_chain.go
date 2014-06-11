package security

// This file describes a certificate chain based implementation of security.PublicID.

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"math/big"
	"reflect"
	"time"

	icaveat "veyron/runtimes/google/security/caveat"
	"veyron/runtimes/google/security/keys"
	"veyron/security/caveat"

	"veyron2/security"
	"veyron2/security/wire"
	"veyron2/vom"
)

// chainPublicID implements security.PublicID.
type chainPublicID struct {
	certificates []wire.Certificate

	// Fields derived from certificates in VomDecode
	publicKey *ecdsa.PublicKey
	rootKey   *ecdsa.PublicKey
	name      string
}

func (id *chainPublicID) Names() []string {
	// Return a name only if the identity provider is trusted.
	if keys.LevelOfTrust(id.rootKey, id.certificates[0].Name) == keys.Trusted {
		return []string{id.name}
	}
	return nil
}

// Match determines if the PublicID's chained name can be extended to match the
// provided PrincipalPattern. An extension of a chained name is any name obtained
// by joining additional strings to the name using wire.ChainSeparator. Ex: extensions
// of the name "foo/bar" are the names "foo/bar", "foo/bar/baz", "foo/bar/baz/car", and
// so on.
func (id *chainPublicID) Match(pattern security.PrincipalPattern) bool {
	return matchPrincipalPattern(id.Names(), pattern)
}

func (id *chainPublicID) PublicKey() *ecdsa.PublicKey { return id.publicKey }

func (id *chainPublicID) String() string {
	// Add a prefix if the identity provider is not trusted.
	if keys.LevelOfTrust(id.rootKey, id.certificates[0].Name) != keys.Trusted {
		return wire.UntrustedIDProviderPrefix + id.name
	}
	return id.name
}

func (id *chainPublicID) VomEncode() (*wire.ChainPublicID, error) {
	return &wire.ChainPublicID{Certificates: id.certificates}, nil
}

func (id *chainPublicID) VomDecode(w *wire.ChainPublicID) error {
	if err := w.VerifyIntegrity(); err != nil {
		return err
	}
	firstKey, err := w.Certificates[0].PublicKey.Decode()
	if err != nil {
		return err
	}
	lastKey, err := w.Certificates[len(w.Certificates)-1].PublicKey.Decode()
	if err != nil {
		return err
	}
	id.name = w.Name()
	id.certificates = w.Certificates
	id.publicKey = lastKey
	id.rootKey = firstKey
	return err
}

// Authorize checks if all caveats on the PublicID validate with respect to the
// provided context and that the identity provider (root public key) is not
// mistrusted. If so returns the original PublicID. This method assumes that
// the existing PublicID was obtained after successfully decoding a serialized
// PublicID and hence has integrity.
func (id *chainPublicID) Authorize(context security.Context) (security.PublicID, error) {
	rootCert := id.certificates[0]
	rootKey, err := rootCert.PublicKey.Decode()
	if err != nil {
		// unlikely to hit this case, as chainPublicID would have integrity.
		return nil, err
	}
	// Implicit "caveat": The identity provider should not be mistrusted.
	switch tl := keys.LevelOfTrust(rootKey, rootCert.Name); tl {
	case keys.Unknown, keys.Trusted:
		// No-op
	default:
		return nil, fmt.Errorf("%v public key(%v) for identity provider %q", tl, rootKey, rootCert.Name)
	}
	for _, c := range id.certificates {
		if err := c.ValidateCaveats(context); err != nil {
			return nil, fmt.Errorf("not authorized because %v", err)
		}
	}
	return id, nil
}

func (id *chainPublicID) ThirdPartyCaveats() (thirdPartyCaveats []security.ServiceCaveat) {
	for _, c := range id.certificates {
		thirdPartyCaveats = append(thirdPartyCaveats, wire.DecodeThirdPartyCaveats(c.Caveats)...)
	}
	return
}

// chainPrivateID implements security.PrivateID
type chainPrivateID struct {
	publicID   *chainPublicID
	privateKey *ecdsa.PrivateKey
}

func (id *chainPrivateID) PublicID() security.PublicID { return id.publicID }

func (id *chainPrivateID) Sign(message []byte) (signature security.Signature, err error) {
	signature.R, signature.S, err = ecdsa.Sign(rand.Reader, id.privateKey, message)
	return
}

func (id *chainPrivateID) String() string { return fmt.Sprintf("PrivateID:%v", id.publicID) }

func (id *chainPrivateID) VomEncode() (*wire.ChainPrivateID, error) {
	pub, err := id.publicID.VomEncode()
	if err != nil {
		return nil, err
	}
	return &wire.ChainPrivateID{Secret: id.privateKey.D.Bytes(), PublicID: *pub}, nil
}

func (id *chainPrivateID) VomDecode(w *wire.ChainPrivateID) error {
	id.publicID = new(chainPublicID)
	if err := id.publicID.VomDecode(&w.PublicID); err != nil {
		return err
	}
	id.privateKey = &ecdsa.PrivateKey{
		PublicKey: *id.publicID.publicKey,
		D:         new(big.Int).SetBytes(w.Secret),
	}
	return nil
}

// Bless returns a new PublicID by extending the ceritificate chain of the PrivateID's
// PublicID with a new certificate that has the provided blessingName, caveats, and an
// additional expiry caveat for the given duration.
func (id *chainPrivateID) Bless(blessee security.PublicID, blessingName string, duration time.Duration, caveats []security.ServiceCaveat) (security.PublicID, error) {
	// The integrity of the PublicID blessee is assumed to have been verified
	// (typically by a Vom decode).
	if err := wire.ValidateBlessingName(blessingName); err != nil {
		return nil, err
	}
	cert := wire.Certificate{Name: blessingName}
	if err := cert.PublicKey.Encode(blessee.PublicKey()); err != nil {
		return nil, err
	}
	now := time.Now()
	caveats = append(caveats, security.UniversalCaveat(&caveat.Expiry{IssueTime: now, ExpiryTime: now.Add(duration)}))
	var err error
	if cert.Caveats, err = wire.EncodeCaveats(caveats); err != nil {
		return nil, err
	}
	vomID, err := id.VomEncode()
	if err != nil {
		return nil, err
	}
	if err := cert.Sign(vomID); err != nil {
		return nil, err
	}
	w := &wire.ChainPublicID{
		Certificates: append(id.publicID.certificates, cert),
	}
	return &chainPublicID{
		certificates: w.Certificates,
		publicKey:    blessee.PublicKey(),
		rootKey:      id.publicID.rootKey,
		name:         w.Name(),
	}, nil
}

func (id *chainPrivateID) Derive(pub security.PublicID) (security.PrivateID, error) {
	if !reflect.DeepEqual(pub.PublicKey(), id.publicID.publicKey) {
		return nil, errDeriveMismatch
	}
	switch p := pub.(type) {
	case *chainPublicID:
		return &chainPrivateID{
			publicID:   p,
			privateKey: id.privateKey,
		}, nil
	case *setPublicID:
		privs := make([]security.PrivateID, len(*p))
		var err error
		for ix, ip := range *p {
			if privs[ix], err = id.Derive(ip); err != nil {
				return nil, fmt.Errorf("Derive failed for public id %d of %d in set: %v", ix, len(*p), err)
			}
		}
		return setPrivateID(privs), nil
	default:
		return nil, fmt.Errorf("PrivateID of type %T cannot be used to Derive from PublicID of type %T", id, pub)
	}
}

func (id *chainPrivateID) MintDischarge(cav security.ThirdPartyCaveat, ctx security.Context, duration time.Duration, dischargeCaveats []security.ServiceCaveat) (security.ThirdPartyDischarge, error) {
	return icaveat.NewPublicKeyDischarge(id, cav, ctx, duration, dischargeCaveats)
}

// newChainPrivateID returns a new PrivateID containing a freshly generated
// private key, and a single self-signed certificate specifying the provided
// name and the public key corresponding to the generated private key.
func newChainPrivateID(name string) (security.PrivateID, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	id := &chainPrivateID{
		publicID: &chainPublicID{
			certificates: []wire.Certificate{{Name: name}},
			name:         name,
			publicKey:    &key.PublicKey,
			rootKey:      &key.PublicKey,
		},
		privateKey: key,
	}
	cert := &id.publicID.certificates[0]
	if err := cert.PublicKey.Encode(&key.PublicKey); err != nil {
		return nil, err
	}
	vomID, err := id.VomEncode()
	if err != nil {
		return nil, err
	}
	if err := cert.Sign(vomID); err != nil {
		return nil, err
	}
	return id, nil
}

func init() {
	vom.Register(chainPublicID{})
	vom.Register(chainPrivateID{})
}
