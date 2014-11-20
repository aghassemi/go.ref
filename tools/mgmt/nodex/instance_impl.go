package main

// Commands to modify instance.

import (
	"fmt"

	"veyron.io/lib/cmdline"
	"veyron.io/veyron/veyron2/rt"
	"veyron.io/veyron/veyron2/services/mgmt/node"
)

var cmdStop = &cmdline.Command{
	Run:      runStop,
	Name:     "stop",
	Short:    "Stop the given application instance.",
	Long:     "Stop the given application instance.",
	ArgsName: "<app instance>",
	ArgsLong: `
<app instance> is the veyron object name of the application instance to stop.`,
}

func runStop(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("stop: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	appName := args[0]

	if err := node.ApplicationClient(appName).Stop(rt.R().NewContext(), 5); err != nil {
		return fmt.Errorf("Stop failed: %v", err)
	}
	fmt.Fprintf(cmd.Stdout(), "Stop succeeded\n")
	return nil
}

var cmdSuspend = &cmdline.Command{
	Run:      runSuspend,
	Name:     "suspend",
	Short:    "Suspend the given application instance.",
	Long:     "Suspend the given application instance.",
	ArgsName: "<app instance>",
	ArgsLong: `
<app instance> is the veyron object name of the application instance to suspend.`,
}

func runSuspend(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("suspend: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	appName := args[0]

	if err := node.ApplicationClient(appName).Suspend(rt.R().NewContext()); err != nil {
		return fmt.Errorf("Suspend failed: %v", err)
	}
	fmt.Fprintf(cmd.Stdout(), "Suspend succeeded\n")
	return nil
}

var cmdResume = &cmdline.Command{
	Run:      runResume,
	Name:     "resume",
	Short:    "Resume the given application instance.",
	Long:     "Resume the given application instance.",
	ArgsName: "<app instance>",
	ArgsLong: `
<app instance> is the veyron object name of the application instance to resume.`,
}

func runResume(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("resume: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	appName := args[0]

	if err := node.ApplicationClient(appName).Resume(rt.R().NewContext()); err != nil {
		return fmt.Errorf("Resume failed: %v", err)
	}
	fmt.Fprintf(cmd.Stdout(), "Resume succeeded\n")
	return nil
}
