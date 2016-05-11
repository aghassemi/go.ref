// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package control

import (
	"v.io/v23/context"
	"v.io/v23/security"
	vsecurity "v.io/x/ref/lib/security"
)

// InternalCtx returns the controller's internal context so that it may be used
// by tests.
// TODO(nlacasse): Once we have better idea of how syncbase clients will
// operate, we should consider getting rid of this.
func (c *Controller) InternalCtx() *context.T {
	return c.ctx
}

// InternalGetInstance returns the instance with the given name.
// TODO(nlacasse): This might be a good thing to export for more than just
// tests.
func (c *Controller) InternalGetInstance(name string) *instance {
	c.instancesMu.Lock()
	defer c.instancesMu.Unlock()
	return c.instances[name]
}

// InternalPrincipal returns the principal for the instance.
func (i *instance) InternalPrincipal() security.Principal {
	return i.principal
}

// InternalDefaultBlessings returns the default blessings for the instance.
// Returns empty blessings in the case of an error.
func (i *instance) InternalDefaultBlessings() security.Blessings {
	p, err := vsecurity.LoadPersistentPrincipal(i.credsDir, nil)
	if err != nil {
		return security.Blessings{}
	}
	b, _ := p.BlessingStore().Default()
	return b
}

// InternalResetClientRegistry resets the client registry to an empty map.
func InternalResetClientRegistry() {
	clientRegistryMu.Lock()
	defer clientRegistryMu.Unlock()
	clientRegistry = make(map[string]ClientGenerator)
}

// InternalConfigureContext exposes controller.configureContext to tests.
func (c *Controller) InternalConfigureContext(ctx *context.T, blessingName string) (*context.T, error) {
	return c.configureContext(ctx, blessingName)
}
