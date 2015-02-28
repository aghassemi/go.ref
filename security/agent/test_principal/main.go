package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"os"
	"reflect"
	"runtime"

	"v.io/v23"
	"v.io/v23/security"
	"v.io/x/ref/lib/testutil"
	_ "v.io/x/ref/profiles"
)

func newKey() security.PublicKey {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return security.NewECDSAPublicKey(&k.PublicKey)
}

func main() {
	var errors []string
	defer func() {
		if len(errors) == 0 {
			return
		}
		// Print out all errors and exit with failure.
		for _, e := range errors {
			fmt.Fprintln(os.Stderr, e)
		}
		os.Exit(1)
	}()
	errorf := func(format string, args ...interface{}) {
		_, file, line, _ := runtime.Caller(1)
		errors = append(errors, fmt.Sprintf("%v:%d: %v", file, line, fmt.Sprintf(format, args...)))
	}

	ctx, shutdown := testutil.InitForTest()
	defer shutdown()

	p := v23.GetPrincipal(ctx)

	// BlessSelf
	b, err := p.BlessSelf("batman")
	if err != nil {
		errorf("BlessSelf: %v", err)
	}
	// Bless
	if _, err := p.Bless(newKey(), b, "delegate", security.UnconstrainedUse()); err != nil {
		errorf("Bless: %v", err)
	}
	// Sign & PublicKey
	signature, err := p.Sign([]byte("bugs bunny"))
	if err != nil {
		errorf("Sign: %v", err)
	}
	if !signature.Verify(p.PublicKey(), []byte("bugs bunny")) {
		errorf("signature.Verify: %v", err)
	}
	// MintDischarge
	cav, err := security.MethodCaveat("method")
	if err != nil {
		errorf("security.MethodCaveat: %v", err)
	}
	tpcav, err := security.NewPublicKeyCaveat(p.PublicKey(), "location", security.ThirdPartyRequirements{}, cav)
	if err != nil {
		errorf("security.NewPublicKeyCaveat: %v", err)
	}
	if _, err := p.MintDischarge(tpcav, cav); err != nil {
		errorf("MintDischarge: %v", err)
	}
	// BlessingRoots
	if err := p.Roots().Recognized(p.PublicKey(), "batman"); err == nil {
		errorf("Roots().Recognized returned nil")
	}
	if err := p.AddToRoots(b); err != nil {
		errorf("AddToRoots: %v", err)
	}
	if err := p.Roots().Recognized(p.PublicKey(), "batman"); err != nil {
		errorf("Roots().Recognized: %v", err)
	}
	// BlessingStore: Defaults
	if err := p.BlessingStore().SetDefault(security.Blessings{}); err != nil {
		errorf("BlessingStore().SetDefault: %v", err)
	}
	if def := p.BlessingStore().Default(); !def.IsZero() {
		errorf("BlessingStore().Default returned %v, want empty", def)
	}
	if err := p.BlessingStore().SetDefault(b); err != nil {
		errorf("BlessingStore().SetDefault: %v", err)
	}
	if def := p.BlessingStore().Default(); !reflect.DeepEqual(def, b) {
		errorf("BlessingStore().Default returned [%v], want [%v]", def, b)
	}
	// BlessingStore: Set & ForPeer
	// First, clear out the self-generated default of the blessing store.
	if _, err := p.BlessingStore().Set(security.Blessings{}, security.AllPrincipals); err != nil {
		errorf("BlessingStore().Set(nil, %q): %v", security.AllPrincipals, err)
	}
	if forpeer := p.BlessingStore().ForPeer("superman/friend"); !forpeer.IsZero() {
		errorf("BlessingStore().ForPeer unexpectedly returned %v", forpeer)
	}
	if old, err := p.BlessingStore().Set(b, "superman"); err != nil {
		errorf("BlessingStore().Set returned (%v, %v)", old, err)
	}
	if forpeer := p.BlessingStore().ForPeer("superman/friend"); !reflect.DeepEqual(forpeer, b) {
		errorf("BlessingStore().ForPeer returned %v and not %v", forpeer, b)
	}
}
