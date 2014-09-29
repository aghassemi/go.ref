package impl

import (
	"fmt"
	"time"

	"veyron.io/veyron/veyron/lib/cmdline"
	"veyron.io/veyron/veyron/services/mgmt/profile"
	"veyron.io/veyron/veyron/services/mgmt/repository"

	"veyron.io/veyron/veyron2/rt"
	"veyron.io/veyron/veyron2/services/mgmt/build"
)

var cmdLabel = &cmdline.Command{
	Run:      runLabel,
	Name:     "label",
	Short:    "Shows a human-readable profile key for the profile.",
	Long:     "Shows a human-readable profile key for the profile.",
	ArgsName: "<profile>",
	ArgsLong: "<profile> is the full name of the profile.",
}

func runLabel(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("label: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	p, err := repository.BindProfile(name)
	if err != nil {
		return fmt.Errorf("BindProfile(%v) failed: %v", name, err)
	}
	ctx, cancel := rt.R().NewContext().WithTimeout(time.Minute)
	defer cancel()
	label, err := p.Label(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.Stdout(), label)
	return nil
}

var cmdDescription = &cmdline.Command{
	Run:      runDescription,
	Name:     "description",
	Short:    "Shows a human-readable profile description for the profile.",
	Long:     "Shows a human-readable profile description for the profile.",
	ArgsName: "<profile>",
	ArgsLong: "<profile> is the full name of the profile.",
}

func runDescription(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("description: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	p, err := repository.BindProfile(name)
	if err != nil {
		return fmt.Errorf("BindProfile(%v) failed: %v", name, err)
	}
	ctx, cancel := rt.R().NewContext().WithTimeout(time.Minute)
	defer cancel()
	desc, err := p.Description(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.Stdout(), desc)
	return nil
}

var cmdSpecification = &cmdline.Command{
	Run:      runSpecification,
	Name:     "spec",
	Short:    "Shows the specification of the profile.",
	Long:     "Shows the specification of the profile.",
	ArgsName: "<profile>",
	ArgsLong: "<profile> is the full name of the profile.",
}

func runSpecification(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("spec: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	p, err := repository.BindProfile(name)
	if err != nil {
		return fmt.Errorf("BindProfile(%v) failed: %v", name, err)
	}
	ctx, cancel := rt.R().NewContext().WithTimeout(time.Minute)
	defer cancel()
	spec, err := p.Specification(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.Stdout(), "%#v\n", spec)
	return nil
}

var cmdPut = &cmdline.Command{
	Run:      runPut,
	Name:     "put",
	Short:    "Sets a placeholder specification for the profile.",
	Long:     "Sets a placeholder specification for the profile.",
	ArgsName: "<profile>",
	ArgsLong: "<profile> is the full name of the profile.",
}

func runPut(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("put: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	p, err := repository.BindProfile(name)
	if err != nil {
		return fmt.Errorf("BindProfile(%v) failed: %v", name, err)
	}

	// TODO(rthellend): Read an actual specification from a file.
	spec := profile.Specification{
		Arch:        build.AMD64,
		Description: "Example profile to test the profile manager implementation.",
		Format:      build.ELF,
		Libraries:   map[profile.Library]struct{}{profile.Library{Name: "foo", MajorVersion: "1", MinorVersion: "0"}: struct{}{}},
		Label:       "example",
		OS:          build.Linux,
	}
	ctx, cancel := rt.R().NewContext().WithTimeout(time.Minute)
	defer cancel()
	if err := p.Put(ctx, spec); err != nil {
		return err
	}
	fmt.Fprintln(cmd.Stdout(), "Profile added successfully.")
	return nil
}

var cmdRemove = &cmdline.Command{
	Run:      runRemove,
	Name:     "remove",
	Short:    "removes the profile specification for the profile.",
	Long:     "removes the profile specification for the profile.",
	ArgsName: "<profile>",
	ArgsLong: "<profile> is the full name of the profile.",
}

func runRemove(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("remove: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	p, err := repository.BindProfile(name)
	if err != nil {
		return fmt.Errorf("BindProfile(%v) failed: %v", name, err)
	}
	ctx, cancel := rt.R().NewContext().WithTimeout(time.Minute)
	defer cancel()
	if err = p.Remove(ctx); err != nil {
		return err
	}
	fmt.Fprintln(cmd.Stdout(), "Profile removed successfully.")
	return nil
}

func Root() *cmdline.Command {
	return &cmdline.Command{
		Name:     "profile",
		Short:    "Command-line tool for interacting with the veyron profile repository",
		Long:     "Command-line tool for interacting with the veyron profile repository",
		Children: []*cmdline.Command{cmdLabel, cmdDescription, cmdSpecification, cmdPut, cmdRemove},
	}
}
