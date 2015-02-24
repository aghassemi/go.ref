package audit_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"v.io/core/veyron/security/audit"
	"v.io/v23/security"
)

func TestAuditingPrincipal(t *testing.T) {
	var (
		thirdPartyCaveat, discharge = newThirdPartyCaveatAndDischarge(t)
		wantErr                     = errors.New("call failed") // The error returned by call calls to mockID operations

		mockP   = new(mockPrincipal)
		auditor = new(mockAuditor)
		p       = audit.NewPrincipal(mockP, auditor)
	)
	tests := []struct {
		Method      string
		Args        V
		Result      interface{} // Result returned by the Method call.
		AuditResult bool        // If true, Result should appear in the audit log. If false, it should not.
	}{
		{"BlessSelf", V{"self"}, newBlessing(t, "blessing"), true},
		{"Bless", V{newPrincipal(t).PublicKey(), newBlessing(t, "root"), "extension", security.UnconstrainedUse()}, newBlessing(t, "root/extension"), true},
		{"MintDischarge", V{thirdPartyCaveat, security.UnconstrainedUse()}, discharge, false},
		{"Sign", V{make([]byte, 10)}, security.Signature{R: []byte{1}, S: []byte{1}}, false},
	}
	for _, test := range tests {
		// Test1: If the underlying operation fails, the error should be returned and nothing should be audited.
		mockP.NextError = wantErr
		results, err := call(p, test.Method, test.Args)
		if err != nil {
			t.Errorf("failed to invoke p.%v(%#v): %v", test.Method, test.Args, err)
			continue
		}
		if got, ok := results[len(results)-1].(error); !ok || got != wantErr {
			t.Errorf("p.%v(%#v) returned (..., %v), want (..., %v)", test.Method, test.Args, got, wantErr)
		}
		if audited := auditor.Release(); !reflect.DeepEqual(audited, audit.Entry{}) {
			t.Errorf("p.%v(%#v) resulted in [%+v] being written to the audit log, nothing should have been", test.Method, test.Args, audited)
		}

		// Test2: If the auditor fails, then the operation should fail too.
		auditor.NextError = errors.New("auditor failed")
		results, err = call(p, test.Method, test.Args)
		if err != nil {
			t.Errorf("failed to invoke p.%v(%#v): %v", test.Method, test.Args, err)
			continue
		}
		if got, ok := results[len(results)-1].(error); !ok || !strings.HasSuffix(got.Error(), "auditor failed") {
			t.Errorf("p.%v(%#v) returned %v when auditor failed, wanted (..., %v)", test.Method, test.Args, results, "... auditor failed")
		}

		// Test3: If the underlying operation succeeds, should return the same value and write to the audit log.
		now := time.Now()
		mockP.NextResult = test.Result
		results, err = call(p, test.Method, test.Args)
		audited := auditor.Release()
		if err != nil {
			t.Errorf("failed to invoke p.%v(%#v): %v", test.Method, test.Args, err)
			continue
		}
		if got := results[len(results)-1]; got != nil {
			t.Errorf("p.%v(%#v) returned an error: %v", test.Method, test.Args, got)
		}
		if got := results[0]; !reflect.DeepEqual(got, test.Result) {
			t.Errorf("p.%v(%#v) returned %v(%T) want %v(%T)", test.Method, test.Args, got, got, test.Result, test.Result)
		}
		if audited.Timestamp.Before(now) || audited.Timestamp.IsZero() {
			t.Errorf("p.%v(%#v) audited the time as %v, should have been a time after %v", test.Method, test.Args, audited.Timestamp, now)
		}
		if want := (audit.Entry{
			Method:    test.Method,
			Arguments: []interface{}(test.Args),
			Results:   sliceOrNil(test.AuditResult, test.Result),
			Timestamp: audited.Timestamp, // Hard to come up with the expected timestamp, relying on sanity check above.
		}); !reflect.DeepEqual(audited, want) {
			t.Errorf("p.%v(%#v) resulted in [%#v] being audited, wanted [%#v]", test.Method, test.Args, audited, want)
		}
	}
}

func TestUnauditedMethodsOnPrincipal(t *testing.T) {
	var (
		auditor  = new(mockAuditor)
		p        = newPrincipal(t)
		auditedP = audit.NewPrincipal(p, auditor)
	)
	blessing, err := p.BlessSelf("self")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		Method string
		Args   V
	}{
		{"PublicKey", V{}},
		{"Roots", V{}},
		{"AddToRoots", V{blessing}},
		{"BlessingStore", V{}},
	}

	for _, test := range tests {
		want, err := call(p, test.Method, test.Args)
		if err != nil {
			t.Fatalf("%v: %v", test.Method, err)
		}
		got, err := call(auditedP, test.Method, test.Args)
		if err != nil {
			t.Fatalf("%v: %v", test.Method, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Got %v, want %v", got, want)
		}
		if gotEntry := auditor.Release(); !reflect.DeepEqual(gotEntry, audit.Entry{}) {
			t.Errorf("Unexpected entry in audit log: %v", gotEntry)
		}
	}
}

type mockPrincipal struct {
	NextResult interface{}
	NextError  error
}

func (p *mockPrincipal) reset() {
	p.NextError = nil
	p.NextResult = nil
}

func (p *mockPrincipal) Bless(security.PublicKey, security.Blessings, string, security.Caveat, ...security.Caveat) (security.Blessings, error) {
	defer p.reset()
	b, _ := p.NextResult.(security.Blessings)
	return b, p.NextError
}

func (p *mockPrincipal) BlessSelf(string, ...security.Caveat) (security.Blessings, error) {
	defer p.reset()
	b, _ := p.NextResult.(security.Blessings)
	return b, p.NextError
}

func (p *mockPrincipal) Sign([]byte) (sig security.Signature, err error) {
	defer p.reset()
	sig, _ = p.NextResult.(security.Signature)
	err = p.NextError
	return
}

func (p *mockPrincipal) MintDischarge(security.Caveat, security.Caveat, ...security.Caveat) (security.Discharge, error) {
	defer p.reset()
	d, _ := p.NextResult.(security.Discharge)
	return d, p.NextError
}

func (p *mockPrincipal) BlessingsByName(name security.BlessingPattern) []security.Blessings {
	return nil
}

func (p *mockPrincipal) BlessingsInfo(b security.Blessings) map[string][]security.Caveat {
	return nil
}

func (p *mockPrincipal) PublicKey() security.PublicKey         { return p.NextResult.(security.PublicKey) }
func (p *mockPrincipal) Roots() security.BlessingRoots         { return nil }
func (p *mockPrincipal) BlessingStore() security.BlessingStore { return nil }
func (p *mockPrincipal) AddToRoots(b security.Blessings) error { return nil }

type mockAuditor struct {
	LastEntry audit.Entry
	NextError error
}

func (a *mockAuditor) Audit(entry audit.Entry) error {
	if a.NextError != nil {
		err := a.NextError
		a.NextError = nil
		return err
	}
	a.LastEntry = entry
	return nil
}

func (a *mockAuditor) Release() audit.Entry {
	entry := a.LastEntry
	a.LastEntry = audit.Entry{}
	return entry
}

type V []interface{}

func call(receiver interface{}, method string, args V) (results []interface{}, err interface{}) {
	defer func() {
		err = recover()
	}()
	callargs := make([]reflect.Value, len(args))
	for idx, arg := range args {
		callargs[idx] = reflect.ValueOf(arg)
	}
	callresults := reflect.ValueOf(receiver).MethodByName(method).Call(callargs)
	results = make([]interface{}, len(callresults))
	for idx, res := range callresults {
		results[idx] = res.Interface()
	}
	return
}

func sliceOrNil(include bool, item interface{}) []interface{} {
	if item != nil && include {
		return []interface{}{item}
	}
	return nil
}

func newPrincipal(t *testing.T) security.Principal {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := security.NewInMemoryECDSASigner(key)
	p, err := security.CreatePrincipal(signer, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func newCaveat(c security.Caveat, err error) security.Caveat {
	if err != nil {
		panic(err)
	}
	return c
}

func newBlessing(t *testing.T, name string) security.Blessings {
	b, err := newPrincipal(t).BlessSelf(name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func newThirdPartyCaveatAndDischarge(t *testing.T) (security.Caveat, security.Discharge) {
	p := newPrincipal(t)
	c, err := security.NewPublicKeyCaveat(p.PublicKey(), "location", security.ThirdPartyRequirements{}, newCaveat(security.MethodCaveat("method")))
	if err != nil {
		t.Fatal(err)
	}
	d, err := p.MintDischarge(c, security.UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	return c, d
}
