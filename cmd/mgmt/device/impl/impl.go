package impl

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/ipc"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/security"
	"v.io/v23/services/mgmt/application"
	"v.io/v23/services/mgmt/device"
	"v.io/x/lib/cmdline"
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

var cmdInstall = &cmdline.Command{
	Run:      runInstall,
	Name:     "install",
	Short:    "Install the given application.",
	Long:     "Install the given application.",
	ArgsName: "<device> <application>",
	ArgsLong: `
<device> is the vanadium object name of the device manager's app service.

<application> is the vanadium object name of the application.
`,
}

func runInstall(cmd *cmdline.Command, args []string) error {
	if expected, got := 2, len(args); expected != got {
		return cmd.UsageErrorf("install: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	deviceName, appName := args[0], args[1]
	appID, err := device.ApplicationClient(deviceName).Install(gctx, appName, device.Config(configOverride), application.Packages(packagesOverride))
	// Reset the value for any future invocations of "install" or
	// "install-local" (we run more than one command per process in unit
	// tests).
	configOverride = configFlag{}
	packagesOverride = packagesFlag{}
	if err != nil {
		return fmt.Errorf("Install failed: %v", err)
	}
	fmt.Fprintf(cmd.Stdout(), "Successfully installed: %q\n", naming.Join(deviceName, appID))
	return nil
}

var cmdUninstall = &cmdline.Command{
	Run:      runUninstall,
	Name:     "uninstall",
	Short:    "Uninstall the given application installation.",
	Long:     "Uninstall the given application installation.",
	ArgsName: "<installation>",
	ArgsLong: `
<installation> is the vanadium object name of the application installation to
uninstall.
`,
}

func runUninstall(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("uninstall: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	installName := args[0]
	if err := device.ApplicationClient(installName).Uninstall(gctx); err != nil {
		return fmt.Errorf("Uninstall failed: %v", err)
	}
	fmt.Fprintf(cmd.Stdout(), "Successfully uninstalled: %q\n", installName)
	return nil
}

var cmdStart = &cmdline.Command{
	Run:      runStart,
	Name:     "start",
	Short:    "Start an instance of the given application.",
	Long:     "Start an instance of the given application.",
	ArgsName: "<application installation> <grant extension>",
	ArgsLong: `
<application installation> is the vanadium object name of the
application installation from which to start an instance.

<grant extension> is used to extend the default blessing of the
current principal when blessing the app instance.`,
}

type granter struct {
	ipc.CallOpt
	p         security.Principal
	extension string
}

func (g *granter) Grant(other security.Blessings) (security.Blessings, error) {
	return g.p.Bless(other.PublicKey(), g.p.BlessingStore().Default(), g.extension, security.UnconstrainedUse())
}

func runStart(cmd *cmdline.Command, args []string) error {
	if expected, got := 2, len(args); expected != got {
		return cmd.UsageErrorf("start: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	appInstallation, grant := args[0], args[1]

	ctx, cancel := context.WithCancel(gctx)
	defer cancel()
	principal := v23.GetPrincipal(ctx)

	call, err := device.ApplicationClient(appInstallation).Start(ctx)
	if err != nil {
		return fmt.Errorf("Start failed: %v", err)
	}
	var appInstanceIDs []string
	for call.RecvStream().Advance() {
		switch msg := call.RecvStream().Value().(type) {
		case device.StartServerMessageInstanceName:
			appInstanceIDs = append(appInstanceIDs, msg.Value)
		case device.StartServerMessageInstancePublicKey:
			pubKey, err := security.UnmarshalPublicKey(msg.Value)
			if err != nil {
				return fmt.Errorf("Start failed: %v", err)
			}
			// TODO(caprita,rthellend): Get rid of security.UnconstrainedUse().
			blessings, err := principal.Bless(pubKey, principal.BlessingStore().Default(), grant, security.UnconstrainedUse())
			if err != nil {
				return fmt.Errorf("Start failed: %v", err)
			}
			call.SendStream().Send(device.StartClientMessageAppBlessings{blessings})
		default:
			fmt.Fprintf(cmd.Stderr(), "Received unexpected message: %#v\n", msg)
		}
	}
	if err := call.Finish(); err != nil {
		if len(appInstanceIDs) == 0 {
			return fmt.Errorf("Start failed: %v", err)
		} else {
			return fmt.Errorf(
				"Start failed: %v,\nView log with:\n debug logs read `debug glob %s/logs/STDERR-*`",
				err, naming.Join(appInstallation, appInstanceIDs[0]))
		}

	}
	for _, id := range appInstanceIDs {
		fmt.Fprintf(cmd.Stdout(), "Successfully started: %q\n", naming.Join(appInstallation, id))
	}
	return nil
}

var cmdClaim = &cmdline.Command{
	Run:      runClaim,
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

func runClaim(cmd *cmdline.Command, args []string) error {
	if expected, max, got := 2, 4, len(args); expected > got || got > max {
		return cmd.UsageErrorf("claim: incorrect number of arguments, expected atleast %d (max: %d), got %d", expected, max, got)
	}
	deviceName, grant := args[0], args[1]
	var pairingToken string
	if len(args) > 2 {
		pairingToken = args[2]
	}
	var serverKeyOpts ipc.CallOpt
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
	// Skip server resolve authorization since an unclaimed device might have
	// roots that will not be recognized by the claimer.
	if err := device.ClaimableClient(deviceName).Claim(gctx, pairingToken, &granter{p: v23.GetPrincipal(gctx), extension: grant}, serverKeyOpts, options.SkipResolveAuthorization{}); err != nil {
		return err
	}
	fmt.Fprintln(cmd.Stdout(), "Successfully claimed.")
	return nil
}

var cmdDescribe = &cmdline.Command{
	Run:      runDescribe,
	Name:     "describe",
	Short:    "Describe the device.",
	Long:     "Describe the device.",
	ArgsName: "<device>",
	ArgsLong: `
<device> is the vanadium object name of the device manager's device service.`,
}

func runDescribe(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("describe: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	deviceName := args[0]
	if description, err := device.DeviceClient(deviceName).Describe(gctx); err != nil {
		return fmt.Errorf("Describe failed: %v", err)
	} else {
		fmt.Fprintf(cmd.Stdout(), "%+v\n", description)
	}
	return nil
}

var cmdUpdate = &cmdline.Command{
	Run:      runUpdate,
	Name:     "update",
	Short:    "Update the device manager or application",
	Long:     "Update the device manager or application",
	ArgsName: "<object>",
	ArgsLong: `
<object> is the vanadium object name of the device manager or application
installation or instance to update.`,
}

func runUpdate(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("update: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	if err := device.ApplicationClient(name).Update(gctx); err != nil {
		return err
	}
	fmt.Fprintln(cmd.Stdout(), "Update successful.")
	return nil
}

var cmdRevert = &cmdline.Command{
	Run:      runRevert,
	Name:     "revert",
	Short:    "Revert the device manager or application",
	Long:     "Revert the device manager or application to its previous version",
	ArgsName: "<object>",
	ArgsLong: `
<object> is the vanadium object name of the device manager or application
installation to revert.`,
}

func runRevert(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("revert: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	deviceName := args[0]
	if err := device.ApplicationClient(deviceName).Revert(gctx); err != nil {
		return err
	}
	fmt.Fprintln(cmd.Stdout(), "Revert successful.")
	return nil
}

var cmdDebug = &cmdline.Command{
	Run:      runDebug,
	Name:     "debug",
	Short:    "Debug the device.",
	Long:     "Debug the device.",
	ArgsName: "<device>",
	ArgsLong: `
<device> is the vanadium object name of an app installation or instance.`,
}

func runDebug(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("debug: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	deviceName := args[0]
	if description, err := device.DeviceClient(deviceName).Debug(gctx); err != nil {
		return fmt.Errorf("Debug failed: %v", err)
	} else {
		fmt.Fprintf(cmd.Stdout(), "%v\n", description)
	}
	return nil
}
