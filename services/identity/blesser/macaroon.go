package blesser

import (
	"fmt"
	"time"

	"v.io/core/veyron/services/identity"
	"v.io/core/veyron/services/identity/oauth"
	"v.io/core/veyron/services/identity/util"

	"v.io/v23/ipc"
	"v.io/v23/security"
	"v.io/v23/vom"
)

type macaroonBlesser struct {
	key []byte
}

// NewMacaroonBlesserServer provides an identity.MacaroonBlesser Service that generates blessings
// after unpacking a BlessingMacaroon.
func NewMacaroonBlesserServer(key []byte) identity.MacaroonBlesserServerStub {
	return identity.MacaroonBlesserServer(&macaroonBlesser{key})
}

func (b *macaroonBlesser) Bless(ctx ipc.ServerContext, macaroon string) (security.WireBlessings, error) {
	var empty security.WireBlessings
	inputs, err := util.Macaroon(macaroon).Decode(b.key)
	if err != nil {
		return empty, err
	}
	var m oauth.BlessingMacaroon
	if err := vom.Decode(inputs, &m); err != nil {
		return empty, err
	}
	if time.Now().After(m.Creation.Add(time.Minute * 5)) {
		return empty, fmt.Errorf("macaroon has expired")
	}
	if ctx.LocalPrincipal() == nil {
		return empty, fmt.Errorf("server misconfiguration: no authentication happened")
	}
	if len(m.Caveats) == 0 {
		m.Caveats = []security.Caveat{security.UnconstrainedUse()}
	}
	// TODO(ashankar,toddw): After the VDL configuration files have the
	// scheme to translate between "wire" types and "in-memory" types, this
	// should just become return ctx.LocalPrincipal().....
	blessings, err := ctx.LocalPrincipal().Bless(ctx.RemoteBlessings().PublicKey(), ctx.LocalBlessings(), m.Name, m.Caveats[0], m.Caveats[1:]...)
	if err != nil {
		return empty, err
	}
	return security.MarshalBlessings(blessings), nil
}
