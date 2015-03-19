// Package security contains utility testing functions related to
// v.io/v23/security.
//
// Suggested Usage:
//
// Create a new in-memory principal with an empty BlessingStore.
// p := NewPrincipal()
//
// Create a new in-memory principal with self-signed blessing for "alice".
// p := NewPrincipal("alice")
//
// Use a common "identity" provider that blesses other principals allowing
// them to share a single root of trust:
// idp := NewIDProvider("identity_provider")
// p1, p2 := NewPrincipal(), NewPrincipal()
// idp.Bless(p1, "alice")  // Providing the blessing "identity_provider/alice" to p1
// idp.Bless(p2, "bob)     // Providing the blessing "identity_provider/bob" to p2.
package security

import (
	"io/ioutil"
	"os"

	vsecurity "v.io/x/ref/security"

	"v.io/v23/security"
	"v.io/v23/services/security/access"
)

func newCredentials() (string, security.Principal) {
	dir, err := ioutil.TempDir("", "veyron_credentials")
	if err != nil {
		panic(err)
	}
	p, err := vsecurity.CreatePersistentPrincipal(dir, nil)
	if err != nil {
		panic(err)
	}
	return dir, p
}

func selfBlessings(p security.Principal, names ...string) security.Blessings {
	var blessings security.Blessings
	for _, name := range names {
		b, err := p.BlessSelf(name)
		if err != nil {
			panic(err)
		}
		if blessings, err = security.UnionOfBlessings(blessings, b); err != nil {
			panic(err)
		}
	}
	return blessings
}

// NewCredentials generates a directory with a new principal with
// self-signed blessings.
//
// In particular, the principal is initialized with self-signed
// blessings for the provided 'names', marked as  default and shareable
// with all peers on the principal's blessing store.
//
// It returns the path to the directory created and the principal.
// The directory can be used as a value for the VEYRON_CREDENTIALS
// environment variable (or the --veyron.credentials flag) used to
// initialize a Runtime.
func NewCredentials(requiredName string, otherNames ...string) (string, security.Principal) {
	dir, p := newCredentials()
	def := selfBlessings(p, append([]string{requiredName}, otherNames...)...)
	SetDefaultBlessings(p, def)
	return dir, p
}

// ForkCredentials creates a new Principal with blessings extended
// from those of the parent.
//
// In particular, the principal is initialized with blessings from
// 'parent' under the provided 'extensions', and marked as default and
// shareable with all peers on the principal's blessing store.
func ForkCredentials(parent security.Principal, requiredExtension string, otherExtensions ...string) security.Principal {
	p, err := vsecurity.NewPrincipal()
	if err != nil {
		panic(err)
	}
	var def security.Blessings
	for _, extension := range append([]string{requiredExtension}, otherExtensions...) {
		b, err := parent.Bless(p.PublicKey(), parent.BlessingStore().Default(), extension, security.UnconstrainedUse())
		if err != nil {
			panic(err)
		}
		if def, err = security.UnionOfBlessings(def, b); err != nil {
			panic(err)
		}
	}
	SetDefaultBlessings(p, def)
	return p
}

// NewPrincipal creates a new security.Principal.
//
// It also creates self-certified blessings for defaultBlessings and
// marks the union of these blessings as default and shareable with all
// peers on the principal's blessing store.
func NewPrincipal(defaultBlessings ...string) security.Principal {
	p, err := vsecurity.NewPrincipal()
	if err != nil {
		panic(err)
	}
	def := selfBlessings(p, defaultBlessings...)
	SetDefaultBlessings(p, def)
	return p
}

// SetDefaultBlessings updates the BlessingStore and BlessingRoots of p
// so that:
// (1) b is revealed to all clients that connect to Servers operated
// by 'p' (BlessingStore.Default)
// (2) b is revealed  to all servers that clients connect to on behalf
// of p (BlessingStore.Set(..., security.AllPrincipals))
// (3) p recognizes all blessings that have the same root certificate as b.
// (AddToRoots)
func SetDefaultBlessings(p security.Principal, b security.Blessings) {
	if err := p.BlessingStore().SetDefault(b); err != nil {
		panic(err)
	}
	if _, err := p.BlessingStore().Set(b, security.AllPrincipals); err != nil {
		panic(err)
	}
	if err := p.AddToRoots(b); err != nil {
		panic(err)
	}
}

// SaveAccessListToFile saves the provided AccessList in JSON format to a randomly created
// temporary file, and returns the path to the file. This function is meant
// to be used for testing purposes only, it panics if there is an error. The
// caller must ensure that the created file is removed once it is no longer needed.
func SaveAccessListToFile(acl access.Permissions) string {
	f, err := ioutil.TempFile("", "saved_acl")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := acl.WriteTo(f); err != nil {
		defer os.Remove(f.Name())
		panic(err)
	}
	return f.Name()
}

// IDProvider is a convenience wrapper over security.Principal that
// makes a Principal act as an "identity provider" (i.e., provides
// other principals with a blessing from it).
type IDProvider struct {
	p security.Principal
	b security.Blessings
}

func NewIDProvider(name string) *IDProvider {
	p, err := vsecurity.NewPrincipal()
	if err != nil {
		panic(err)
	}
	b, err := p.BlessSelf(name)
	if err != nil {
		panic(err)
	}
	return &IDProvider{p, b}
}

// Bless sets up the provided principal to use blessings from idp as its
// default.
func (idp *IDProvider) Bless(who security.Principal, extension string, caveats ...security.Caveat) error {
	blessings, err := idp.NewBlessings(who, extension, caveats...)
	if err != nil {
		return err
	}
	SetDefaultBlessings(who, blessings)
	return nil
}

// NewBlessings returns Blessings that extend the identity provider's blessing
// with 'extension' and binds it to 'p.PublicKey'.
func (idp *IDProvider) NewBlessings(p security.Principal, extension string, caveats ...security.Caveat) (security.Blessings, error) {
	if len(caveats) == 0 {
		caveats = append(caveats, security.UnconstrainedUse())
	}
	return idp.p.Bless(p.PublicKey(), idp.b, extension, caveats[0], caveats[1:]...)
}

// PublicKey is the public key of the identity provider.
func (idp *IDProvider) PublicKey() security.PublicKey {
	return idp.p.PublicKey()
}
