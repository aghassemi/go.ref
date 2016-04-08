// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package control starts a number of syncbase instances and configures their
// connectivity based on a model.
//
// Usage:
//
//  // Create a new universe model.
//  u := model.GenerateUniverse(universeOpts)
//
//  // Run the universe model on a new controller.
//  c := NewController(controllerOpts)
//  err := c.Run(u)
//
//  // Then update universe however you want, and run it on the controller.
//  u = ...
//  err := c.Run(u)
//
//  // Pause all instances.
//  err := c.Pause()
//
//  // Resume all instances.
//  err := c.Resume()
//
//  // Tear it all down.
//  err := c.TearDown()

package control

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/lib/gosh"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/runtime/protocols/vine"
	"v.io/x/ref/services/mounttable/mounttablelib"
	"v.io/x/ref/services/syncbase/longevity_tests/model"
	"v.io/x/ref/services/syncbase/longevity_tests/mounttabled"
	"v.io/x/ref/test/testutil"
)

var (
	mounttabledMain = gosh.RegisterFunc("mounttabledMain", mounttabled.Main)
)

// Controller handles starting and stopping syncbase instances according to
// model universes.
type Controller struct {
	// Context used to send RPCs to vine servers.
	ctx *context.T

	// identityProvider used to bless mounttable and all syncbase instances.
	identityProvider *testutil.IDProvider

	// Map of syncbase instance names to instances.
	instances   map[string]*instance
	instancesMu sync.Mutex // Locks instances.

	// Gosh command for mounttabled process.  Will be nil if mounttabled is not
	// running.
	mtCmd *gosh.Cmd

	// Namespace root of all instances.
	namespaceRoot string

	// Directory containing all instance working directories.
	rootDir string

	// Gosh shell used to spawn all processes.
	sh *gosh.Shell

	// Name of the controller's vine server.  Even though the controller only
	// acts as a client, it needs to run a vine server so that other servers
	// can be configured to allow RPCs from the controller.
	vineName string
}

// Opts is used to configure the Controller.
type Opts struct {
	// Directory that will contain all instance output.
	RootDir string

	// Send all child process output to stdout/stderr.  Usefull for tests.
	DebugOutput bool

	// Tests should pass their testing.TB instance, non-tests should pass nil.
	TB gosh.TB
}

// NewController returns a new Controller configured by opts.
func NewController(ctx *context.T, opts Opts) (*Controller, error) {
	sh := gosh.NewShell(opts.TB)
	// Gosh by default will panic when it encounters an error.  By setting
	// sh.ContinueOnError to true, we prevent the panic.  We must handle any
	// error ourselves by checking sh.Err after each Shell method invocation,
	// and cmd.Err after each Cmd method invocation.
	sh.ContinueOnError = true
	if opts.DebugOutput {
		sh.PropagateChildOutput = true
	}
	c := &Controller{
		instances: make(map[string]*instance),
		rootDir:   opts.RootDir,
		sh:        sh,
		vineName:  "vine-controller",
	}
	if err := c.init(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// init initializes the controller.  It is safe to call multiple times.
func (c *Controller) init(ctx *context.T) error {
	// Initialize rootDir.
	if c.rootDir == "" {
		return fmt.Errorf("controller rootDir must not be empty")
	}
	if err := os.MkdirAll(c.rootDir, 0755); err != nil {
		return err
	}

	// Initialize identity provider.
	c.identityProvider = testutil.NewIDProvider("root")

	// Create blessings for mounttable.
	mtCreds := filepath.Join(c.rootDir, "mounttable-creds")
	var p security.Principal
	p, err := vsecurity.CreatePersistentPrincipal(mtCreds, nil)
	if err != nil {
		return err
	}
	if err = c.identityProvider.Bless(p, "mounttable"); err != nil {
		return err
	}

	// Start mounttable and set namespace root.
	opts := mounttablelib.Opts{}
	c.mtCmd = c.sh.FuncCmd(mounttabledMain, opts)
	if c.sh.Err != nil {
		return c.sh.Err
	}
	c.mtCmd.Args = append(c.mtCmd.Args, "--v23.credentials="+mtCreds)
	c.mtCmd.Start()
	vars := c.mtCmd.AwaitVars("NAME")
	name := vars["NAME"]
	if name == "" {
		return fmt.Errorf("empty NAME variable returned by mounttable")
	}
	c.namespaceRoot = name

	// Configure context.
	c.ctx, err = c.configureContext(ctx, "controller")
	return err
}

// TearDown stops all instances and the root mounttable.
func (c *Controller) TearDown() error {
	c.sh.Cleanup()
	return c.sh.Err
}

// Run runs a mounttable and a number of syncbased instances with connectivity
// defined by the given model.
// Run can be called multiple times, and will update the running syncbases to
// reflect the model.  Any running instances not contained in the model will be
// stopped.  Any non-running instances contained it the model will be started.
func (c *Controller) Run(u *model.Universe) error {
	if err := c.updateInstances(u.Devices); err != nil {
		return err
	}
	return c.updateTopology(u.Topology)
}

// PauseUniverse stops all instances "gracefully".
func (c *Controller) PauseUniverse() error {
	return fmt.Errorf("not implemented")
}

// ResumeUniverse restarts all instances.
func (c *Controller) ResumeUniverse() error {
	return fmt.Errorf("not implemented")
}

func (c *Controller) updateInstances(devices model.DeviceSet) error {
	c.instancesMu.Lock()
	defer c.instancesMu.Unlock()

	for _, d := range devices {
		// If instance is already running, update it to match model.
		if inst, ok := c.instances[d.Name]; ok {
			inst.update(d)
			continue
		}
		// Otherwise start a new instance for the model.
		inst, err := c.newInstance(d)
		if err != nil {
			return err
		}
		if err := inst.start(); err != nil {
			return err
		}
		c.instances[inst.name] = inst
	}

	// Stop all running instances that are not in the model.
	// TODO(nlacasse): Does it make sense to allow universes to shrink like
	// this?  Maybe once a device is in the universe, it cannot be taken out?
	// Pausing and disconnecting should still be allowed, of course.
	for _, inst := range c.instances {
		// TODO(nlacasse): Consider using a map to represent DeviceSet, so we
		// don't have to iterate like this.
		inModel := false
		for _, d := range devices {
			if inst.name == d.Name {
				inModel = true
				break
			}
		}
		if !inModel {
			if err := inst.kill(); err != nil {
				return err
			}
			delete(c.instances, inst.name)
		}
	}
	return nil
}

func (c *Controller) updateTopology(top model.Topology) error {
	c.instancesMu.Lock()
	defer c.instancesMu.Unlock()

	behaviorMap := map[vine.PeerKey]vine.PeerBehavior{}

	// Start with all instances unreachable.
	// TODO(nlacasse,suharshs): I should not have to do this.  If the
	// behaviorMap is missing a PeerKey for some client and server pair, then
	// that pair should be unreachable.
	for _, clientInst := range c.instances {
		for _, serverInst := range c.instances {
			pk := vine.PeerKey{
				Dialer:   clientInst.vineName,
				Acceptor: serverInst.vineName,
			}
			behaviorMap[pk] = vine.PeerBehavior{Reachable: false}
		}
	}

	// Overwrite the reachable pairs based on the given topology.
	for clientDevice, serviceDevices := range top {
		clientInst, ok := c.instances[clientDevice.Name]
		if !ok {
			return fmt.Errorf("no instance associated with device %v", clientDevice)
		}

		for _, serviceDevice := range serviceDevices {
			serviceInst, ok := c.instances[serviceDevice.Name]
			if !ok {
				return fmt.Errorf("no instance associated with device %v", clientDevice)
			}
			peerKey := vine.PeerKey{
				Dialer:   clientInst.vineName,
				Acceptor: serviceInst.vineName,
			}
			behaviorMap[peerKey] = vine.PeerBehavior{Reachable: true}
		}
	}

	// Allow the controller to access every device.
	for _, inst := range c.instances {
		pk1 := vine.PeerKey{
			Dialer:   c.vineName,
			Acceptor: inst.vineName,
		}
		behaviorMap[pk1] = vine.PeerBehavior{Reachable: true}
		pk2 := vine.PeerKey{
			Dialer:   inst.vineName,
			Acceptor: c.vineName,
		}
		behaviorMap[pk2] = vine.PeerBehavior{Reachable: true}
	}

	// Set the behavior on each instance.
	for _, inst := range c.instances {
		vineClient := vine.VineClient(inst.vineName)
		if err := vineClient.SetBehaviors(c.ctx, behaviorMap); err != nil {
			return err
		}
	}

	// Set the controller's behavior.
	vineClient := vine.VineClient(c.vineName)
	if err := vineClient.SetBehaviors(c.ctx, behaviorMap); err != nil {
		return err
	}

	return nil
}

// newInstance creates a new instance corresponding to the given device model.
func (c *Controller) newInstance(d *model.Device) (*instance, error) {
	wd := filepath.Join(c.rootDir, "instances", d.Name)
	if err := os.MkdirAll(wd, 0755); err != nil {
		return nil, err
	}

	// Create principal blessed by identity provider.
	// TODO(nlacasse): Once we have credentials per user, these should be
	// blessed by the user who owns the device.  For now we give each device
	// its own credentails blessed directly by the identity provider.
	credsDir := filepath.Join(wd, "credentials")
	p, err := vsecurity.CreatePersistentPrincipal(credsDir, nil)
	if err != nil {
		return nil, err
	}
	if err := c.identityProvider.Bless(p, d.Name); err != nil {
		return nil, err
	}

	vineName := "vine-" + d.Name
	inst := &instance{
		credsDir:      credsDir,
		name:          d.Name,
		namespaceRoot: c.namespaceRoot,
		vineName:      vineName,
		sh:            c.sh,
		wd:            wd,
	}
	return inst, nil
}

// configureContext returns a new context based off the given one, which is
// blessed by the controller's identity provider, configured with the
// controller's namespace root, and associated with a vine server.
func (c *Controller) configureContext(ctx *context.T, blessingName string) (*context.T, error) {
	// Create a principal blessed by the identity provider.
	p := testutil.NewPrincipal()
	if err := c.identityProvider.Bless(p, blessingName); err != nil {
		return nil, err
	}
	newCtx, err := v23.WithPrincipal(ctx, p)
	if err != nil {
		return nil, err
	}
	// Set proper namespace root.
	newCtx, _, err = v23.WithNewNamespace(newCtx, c.namespaceRoot)
	if err != nil {
		return nil, err
	}
	// Modify ctx to use vine protocol.
	newCtx, err = vine.Init(newCtx, c.vineName, security.AllowEveryone(), c.vineName, 0)
	if err != nil {
		return nil, nil
	}

	return newCtx, nil
}
