package security

import (
	"fmt"
	"strings"

	"veyron.io/veyron/veyron2/security"
)

func matchesError(got error, want string) error {
	if (got == nil) && len(want) == 0 {
		return nil
	}
	if got == nil {
		return fmt.Errorf("Got nil error, wanted to match %q", want)
	}
	if !strings.Contains(got.Error(), want) {
		return fmt.Errorf("Got error %q, wanted to match %q", got, want)
	}
	return nil
}

func newPrincipal(selfblessings ...string) (security.Principal, security.Blessings) {
	p, err := NewPrincipal()
	if err != nil {
		panic(err)
	}
	if len(selfblessings) == 0 {
		return p, nil
	}
	var def security.Blessings
	for _, str := range selfblessings {
		b, err := p.BlessSelf(str)
		if err != nil {
			panic(err)
		}
		if def, err = security.UnionOfBlessings(def, b); err != nil {
			panic(err)
		}
	}
	if err := p.AddToRoots(def); err != nil {
		panic(err)
	}
	if err := p.BlessingStore().SetDefault(def); err != nil {
		panic(err)
	}
	if _, err := p.BlessingStore().Set(def, security.AllPrincipals); err != nil {
		panic(err)
	}
	return p, def
}

func bless(blesser, blessed security.Principal, with security.Blessings, extension string) security.Blessings {
	b, err := blesser.Bless(blessed.PublicKey(), with, extension, security.UnconstrainedUse())
	if err != nil {
		panic(err)
	}
	return b
}

func blessSelf(p security.Principal, name string) security.Blessings {
	b, err := p.BlessSelf(name)
	if err != nil {
		panic(err)
	}
	return b
}

func unionOfBlessings(blessings ...security.Blessings) security.Blessings {
	b, err := security.UnionOfBlessings(blessings...)
	if err != nil {
		panic(err)
	}
	return b
}
