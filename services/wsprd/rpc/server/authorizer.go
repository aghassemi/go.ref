package server

import (
	"v.io/v23/context"
	"v.io/v23/security"
)

type authorizer struct {
	authFunc remoteAuthFunc
}

func (a *authorizer) Authorize(ctx *context.T) error {
	return a.authFunc(security.GetCall(ctx))
}
