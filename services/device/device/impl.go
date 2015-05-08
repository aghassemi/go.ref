// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/services/application"
	"v.io/v23/services/device"
	"v.io/x/lib/cmdline2"
	"v.io/x/ref/lib/v23cmd"
)

type configFlag device.Config

func (c *configFlag) String() string {
	jsonConfig, _ := json.Marshal(c)
	return string(jsonConfig)
}
func (c *configFlag) Set(s string) error {
	if err := json.Unmarshal([]byte(s), c); err != nil {
		return fmt.Errorf("Unmarshal(%v) failed: %v", s, err)
	}
	return nil
}

var configOverride configFlag = configFlag{}

type packagesFlag application.Packages

func (c *packagesFlag) String() string {
	jsonPackages, _ := json.Marshal(c)
	return string(jsonPackages)
}
func (c *packagesFlag) Set(s string) error {
	if err := json.Unmarshal([]byte(s), c); err != nil {
		return fmt.Errorf("Unmarshal(%v) failed: %v", s, err)
	}
	return nil
}

var packagesOverride packagesFlag = packagesFlag{}

func init() {
	cmdInstall.Flags.Var(&configOverride, "config", "JSON-encoded device.Config object, of the form: '{\"flag1\":\"value1\",\"flag2\":\"value2\"}'")
	cmdInstall.Flags.Var(&packagesOverride, "packages", "JSON-encoded application.Packages object, of the form: '{\"pkg1\":{\"File\":\"object name 1\"},\"pkg2\":{\"File\":\"object name 2\"}}'")
}

var cmdInstall = &cmdline2.Command{
	Runner:   v23cmd.RunnerFunc(runInstall),
	Name:     "install",
	Short:    "Install the given application.",
	Long:     "Install the given application and print the name of the new installation.",
	ArgsName: "<device> <application>",
	ArgsLong: `
<device> is the vanadium object name of the device manager's app service.

<application> is the vanadium object name of the application.
`,
}

func runInstall(ctx *context.T, env *cmdline2.Env, args []string) error {
	if expected, got := 2, len(args); expected != got {
		return env.UsageErrorf("install: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	deviceName, appName := args[0], args[1]
	appID, err := device.ApplicationClient(deviceName).Install(ctx, appName, device.Config(configOverride), application.Packages(packagesOverride))
	// Reset the value for any future invocations of "install" or
	// "install-local" (we run more than one command per process in unit
	// tests).
	configOverride = configFlag{}
	packagesOverride = packagesFlag{}
	if err != nil {
		return fmt.Errorf("Install failed: %v", err)
	}
	fmt.Fprintf(env.Stdout, "%s\n", naming.Join(deviceName, appID))
	return nil
}

var cmdUninstall = &cmdline2.Command{
	Runner:   v23cmd.RunnerFunc(runUninstall),
	Name:     "uninstall",
	Short:    "Uninstall the given application installation.",
	Long:     "Uninstall the given application installation.",
	ArgsName: "<installation>",
	ArgsLong: `
<installation> is the vanadium object name of the application installation to
uninstall.
`,
}

func runUninstall(ctx *context.T, env *cmdline2.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("uninstall: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	installName := args[0]
	if err := device.ApplicationClient(installName).Uninstall(ctx); err != nil {
		return fmt.Errorf("Uninstall failed: %v", err)
	}
	fmt.Fprintf(env.Stdout, "Successfully uninstalled: %q\n", installName)
	return nil
}

var cmdInstantiate = &cmdline2.Command{
	Runner:   v23cmd.RunnerFunc(runInstantiate),
	Name:     "instantiate",
	Short:    "Create an instance of the given application.",
	Long:     "Create an instance of the given application, provide it with a blessing, and print the name of the new instance.",
	ArgsName: "<application installation> <grant extension>",
	ArgsLong: `
<application installation> is the vanadium object name of the
application installation from which to create an instance.

<grant extension> is used to extend the default blessing of the
current principal when blessing the app instance.`,
}

type granter struct {
	rpc.CallOpt
	extension string
}

func (g *granter) Grant(ctx *context.T, call security.Call) (security.Blessings, error) {
	p := call.LocalPrincipal()
	return p.Bless(call.RemoteBlessings().PublicKey(), p.BlessingStore().Default(), g.extension, security.UnconstrainedUse())
}

func runInstantiate(ctx *context.T, env *cmdline2.Env, args []string) error {
	if expected, got := 2, len(args); expected != got {
		return env.UsageErrorf("instantiate: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	appInstallation, grant := args[0], args[1]

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	principal := v23.GetPrincipal(ctx)

	call, err := device.ApplicationClient(appInstallation).Instantiate(ctx)
	if err != nil {
		return fmt.Errorf("Instantiate failed: %v", err)
	}
	for call.RecvStream().Advance() {
		switch msg := call.RecvStream().Value().(type) {
		case device.BlessServerMessageInstancePublicKey:
			pubKey, err := security.UnmarshalPublicKey(msg.Value)
			if err != nil {
				return fmt.Errorf("Instantiate failed: %v", err)
			}
			// TODO(caprita,rthellend): Get rid of security.UnconstrainedUse().
			blessings, err := principal.Bless(pubKey, principal.BlessingStore().Default(), grant, security.UnconstrainedUse())
			if err != nil {
				return fmt.Errorf("Instantiate failed: %v", err)
			}
			call.SendStream().Send(device.BlessClientMessageAppBlessings{blessings})
		default:
			fmt.Fprintf(env.Stderr, "Received unexpected message: %#v\n", msg)
		}
	}
	var instanceID string
	if instanceID, err = call.Finish(); err != nil {
		return fmt.Errorf("Instantiate failed: %v", err)
	}
	fmt.Fprintf(env.Stdout, "%s\n", naming.Join(appInstallation, instanceID))
	return nil
}

var cmdClaim = &cmdline2.Command{
	Runner:   v23cmd.RunnerFunc(runClaim),
	Name:     "claim",
	Short:    "Claim the device.",
	Long:     "Claim the device.",
	ArgsName: "<device> <grant extension> <pairing token> <device publickey>",
	ArgsLong: `
<device> is the vanadium object name of the device manager's device service.

<grant extension> is used to extend the default blessing of the
current principal when blessing the app instance.

<pairing token> is a token that the device manager expects to be replayed
during a claim operation on the device.

<device publickey> is the marshalled public key of the device manager we
are claiming.`,
}

func runClaim(ctx *context.T, env *cmdline2.Env, args []string) error {
	if expected, max, got := 2, 4, len(args); expected > got || got > max {
		return env.UsageErrorf("claim: incorrect number of arguments, expected atleast %d (max: %d), got %d", expected, max, got)
	}
	deviceName, grant := args[0], args[1]
	var pairingToken string
	if len(args) > 2 {
		pairingToken = args[2]
	}
	var serverKeyOpts rpc.CallOpt
	if len(args) > 3 {
		marshalledPublicKey, err := base64.URLEncoding.DecodeString(args[3])
		if err != nil {
			return fmt.Errorf("Failed to base64 decode publickey: %v", err)
		}
		if deviceKey, err := security.UnmarshalPublicKey(marshalledPublicKey); err != nil {
			return fmt.Errorf("Failed to unmarshal device public key:%v", err)
		} else {
			serverKeyOpts = options.ServerPublicKey{deviceKey}
		}
	}
	// Skip server endpoint authorization since an unclaimed device might have
	// roots that will not be recognized by the claimer.
	if err := device.ClaimableClient(deviceName).Claim(ctx, pairingToken, &granter{extension: grant}, serverKeyOpts, options.SkipServerEndpointAuthorization{}); err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, "Successfully claimed.")
	return nil
}

var cmdDescribe = &cmdline2.Command{
	Runner:   v23cmd.RunnerFunc(runDescribe),
	Name:     "describe",
	Short:    "Describe the device.",
	Long:     "Describe the device.",
	ArgsName: "<device>",
	ArgsLong: `
<device> is the vanadium object name of the device manager's device service.`,
}

func runDescribe(ctx *context.T, env *cmdline2.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("describe: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	deviceName := args[0]
	if description, err := device.DeviceClient(deviceName).Describe(ctx); err != nil {
		return fmt.Errorf("Describe failed: %v", err)
	} else {
		fmt.Fprintf(env.Stdout, "%+v\n", description)
	}
	return nil
}

var cmdUpdate = &cmdline2.Command{
	Runner:   v23cmd.RunnerFunc(runUpdate),
	Name:     "update",
	Short:    "Update the device manager or application",
	Long:     "Update the device manager or application",
	ArgsName: "<object>",
	ArgsLong: `
<object> is the vanadium object name of the device manager or application
installation or instance to update.`,
}

func runUpdate(ctx *context.T, env *cmdline2.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("update: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	if err := device.ApplicationClient(name).Update(ctx); err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, "Update successful.")
	return nil
}

var cmdRevert = &cmdline2.Command{
	Runner:   v23cmd.RunnerFunc(runRevert),
	Name:     "revert",
	Short:    "Revert the device manager or application",
	Long:     "Revert the device manager or application to its previous version",
	ArgsName: "<object>",
	ArgsLong: `
<object> is the vanadium object name of the device manager or application
installation to revert.`,
}

func runRevert(ctx *context.T, env *cmdline2.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("revert: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	deviceName := args[0]
	if err := device.ApplicationClient(deviceName).Revert(ctx); err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, "Revert successful.")
	return nil
}

var cmdDebug = &cmdline2.Command{
	Runner:   v23cmd.RunnerFunc(runDebug),
	Name:     "debug",
	Short:    "Debug the device.",
	Long:     "Debug the device.",
	ArgsName: "<app name>",
	ArgsLong: `
<app name> is the vanadium object name of an app installation or instance.`,
}

func runDebug(ctx *context.T, env *cmdline2.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("debug: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	appName := args[0]
	if description, err := device.DeviceClient(appName).Debug(ctx); err != nil {
		return fmt.Errorf("Debug failed: %v", err)
	} else {
		fmt.Fprintf(env.Stdout, "%v\n", description)
	}
	return nil
}

var cmdStatus = &cmdline2.Command{
	Runner:   v23cmd.RunnerFunc(runStatus),
	Name:     "status",
	Short:    "Get application status.",
	Long:     "Get the status of an application installation or instance.",
	ArgsName: "<app name>",
	ArgsLong: `
<app name> is the vanadium object name of an app installation or instance.`,
}

func runStatus(ctx *context.T, env *cmdline2.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("status: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	appName := args[0]
	status, err := device.DeviceClient(appName).Status(ctx)
	if err != nil {
		return fmt.Errorf("Status failed: %v", err)
	}
	switch s := status.(type) {
	case device.StatusInstance:
		fmt.Fprintf(env.Stdout, "Instance [State:%v,Version:%v]\n", s.Value.State, s.Value.Version)
	case device.StatusInstallation:
		fmt.Fprintf(env.Stdout, "Installation [State:%v,Version:%v]\n", s.Value.State, s.Value.Version)
	default:
		return fmt.Errorf("Status returned unknown type: %T", s)
	}
	return nil
}
