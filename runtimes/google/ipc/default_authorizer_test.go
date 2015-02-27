package ipc

import (
	"testing"
	"time"

	vsecurity "v.io/core/veyron/security"
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/vdl"
)

func TestDefaultAuthorizer(t *testing.T) {
	var (
		pali, _ = vsecurity.NewPrincipal()
		pbob, _ = vsecurity.NewPrincipal()
		pche, _ = vsecurity.NewPrincipal()

		che, _ = pche.BlessSelf("che")
		ali, _ = pali.BlessSelf("ali")
		bob, _ = pbob.BlessSelf("bob")

		// bless(ali, bob, "friend") will generate a blessing for ali, calling him "bob/friend".
		bless = func(target, extend security.Blessings, extension string) security.Blessings {
			var p security.Principal
			switch extend.PublicKey() {
			case ali.PublicKey():
				p = pali
			case bob.PublicKey():
				p = pbob
			case che.PublicKey():
				p = pche
			default:
				panic(extend)
			}
			ret, err := p.Bless(target.PublicKey(), extend, extension, security.UnconstrainedUse())
			if err != nil {
				panic(err)
			}
			return ret
		}

		U = func(blessings ...security.Blessings) security.Blessings {
			u, err := security.UnionOfBlessings(blessings...)
			if err != nil {
				panic(err)
			}
			return u
		}

		// Shorthands for getting blessings for Ali and Bob.
		A = func(as security.Blessings, extension string) security.Blessings { return bless(ali, as, extension) }
		B = func(as security.Blessings, extension string) security.Blessings { return bless(bob, as, extension) }

		authorizer defaultAuthorizer
	)
	// Make ali, bob (the two ends) recognize all three blessings
	for ip, p := range []security.Principal{pali, pbob} {
		for _, b := range []security.Blessings{ali, bob, che} {
			if err := p.AddToRoots(b); err != nil {
				t.Fatalf("%d: %v - %v", ip, b, err)
			}
		}
	}
	// All tests are run as if "ali" is the local end and "bob" is the remote.
	tests := []struct {
		local, remote security.Blessings
		authorized    bool
	}{
		{ali, ali, true},
		{ali, bob, false},
		{ali, B(ali, "friend"), true},               // ali talking to ali/friend
		{A(bob, "friend"), bob, true},               // bob/friend talking to bob
		{A(che, "friend"), B(che, "family"), false}, // che/friend talking to che/family
		{U(ali, A(bob, "friend"), A(che, "friend")),
			U(bob, B(che, "family")),
			true}, // {ali, bob/friend, che/friend} talking to {bob, che/family}
	}
	ctx, shutdown := v23.Init()
	defer shutdown()
	for _, test := range tests {
		err := authorizer.Authorize(&mockSecurityContext{
			p: pali,
			l: test.local,
			r: test.remote,
			c: ctx,
		})
		if (err == nil) != test.authorized {
			t.Errorf("Local:%v Remote:%v. Got %v", test.local, test.remote, err)
		}
	}
}

type mockSecurityContext struct {
	p    security.Principal
	l, r security.Blessings
	c    *context.T
}

func (c *mockSecurityContext) Timestamp() (t time.Time)                        { return }
func (c *mockSecurityContext) Method() string                                  { return "" }
func (c *mockSecurityContext) MethodTags() []*vdl.Value                        { return nil }
func (c *mockSecurityContext) Suffix() string                                  { return "" }
func (c *mockSecurityContext) RemoteDischarges() map[string]security.Discharge { return nil }
func (c *mockSecurityContext) LocalEndpoint() naming.Endpoint                  { return nil }
func (c *mockSecurityContext) RemoteEndpoint() naming.Endpoint                 { return nil }
func (c *mockSecurityContext) LocalPrincipal() security.Principal              { return c.p }
func (c *mockSecurityContext) LocalBlessings() security.Blessings              { return c.l }
func (c *mockSecurityContext) RemoteBlessings() security.Blessings             { return c.r }
func (c *mockSecurityContext) Context() *context.T                             { return c.c }
