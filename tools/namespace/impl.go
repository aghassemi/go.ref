package main

import (
	"fmt"
	"time"

	"veyron.io/lib/cmdline"
	"veyron.io/veyron/veyron2/naming"
	"veyron.io/veyron/veyron2/rt"
	"veyron.io/veyron/veyron2/vlog"
)

var cmdGlob = &cmdline.Command{
	Run:      runGlob,
	Name:     "glob",
	Short:    "Returns all matching entries from the namespace",
	Long:     "Returns all matching entries from the namespace.",
	ArgsName: "<pattern>",
	ArgsLong: `
<pattern> is a glob pattern that is matched against all the names below the
specified mount name.
`,
}

func runGlob(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("glob: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	pattern := args[0]
	ns := rt.R().Namespace()
	ctx, cancel := rt.R().NewContext().WithTimeout(time.Minute)
	defer cancel()
	c, err := ns.Glob(ctx, pattern)
	if err != nil {
		vlog.Infof("ns.Glob(%q) failed: %v", pattern, err)
		return err
	}
	for res := range c {
		fmt.Fprint(cmd.Stdout(), res.Name)
		for _, s := range res.Servers {
			fmt.Fprintf(cmd.Stdout(), " %s (Expires %s)", s.Server, s.Expires)
		}
		fmt.Fprintln(cmd.Stdout())
	}
	return nil
}

var cmdMount = &cmdline.Command{
	Run:      runMount,
	Name:     "mount",
	Short:    "Adds a server to the namespace",
	Long:     "Adds server <server> to the namespace with name <name>.",
	ArgsName: "<name> <server> <ttl>",
	ArgsLong: `
<name> is the name to add to the namespace.
<server> is the object address of the server to add.
<ttl> is the TTL of the new entry. It is a decimal number followed by a unit
suffix (s, m, h). A value of 0s represents an infinite duration.
`,
}

func runMount(cmd *cmdline.Command, args []string) error {
	if expected, got := 3, len(args); expected != got {
		return cmd.UsageErrorf("mount: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	server := args[1]
	ttlArg := args[2]

	ttl, err := time.ParseDuration(ttlArg)
	if err != nil {
		return fmt.Errorf("TTL parse error: %v", err)
	}
	ns := rt.R().Namespace()
	ctx, cancel := rt.R().NewContext().WithTimeout(time.Minute)
	defer cancel()
	if err = ns.Mount(ctx, name, server, ttl); err != nil {
		vlog.Infof("ns.Mount(%q, %q, %s) failed: %v", name, server, ttl, err)
		return err
	}
	fmt.Fprintln(cmd.Stdout(), "Server mounted successfully.")
	return nil
}

var cmdUnmount = &cmdline.Command{
	Run:      runUnmount,
	Name:     "unmount",
	Short:    "Removes a server from the namespace",
	Long:     "Removes server <server> with name <name> from the namespace.",
	ArgsName: "<name> <server>",
	ArgsLong: `
<name> is the name to remove from the namespace.
<server> is the object address of the server to remove.
`,
}

func runUnmount(cmd *cmdline.Command, args []string) error {
	if expected, got := 2, len(args); expected != got {
		return cmd.UsageErrorf("unmount: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	server := args[1]
	ns := rt.R().Namespace()
	ctx, cancel := rt.R().NewContext().WithTimeout(time.Minute)
	defer cancel()
	if err := ns.Unmount(ctx, name, server); err != nil {
		vlog.Infof("ns.Unmount(%q, %q) failed: %v", name, server, err)
		return err
	}
	fmt.Fprintln(cmd.Stdout(), "Server unmounted successfully.")
	return nil
}

var cmdResolve = &cmdline.Command{
	Run:      runResolve,
	Name:     "resolve",
	Short:    "Translates a object name to its object address(es)",
	Long:     "Translates a object name to its object address(es).",
	ArgsName: "<name>",
	ArgsLong: "<name> is the name to resolve.",
}

func runResolve(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("resolve: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	ns := rt.R().Namespace()
	ctx, cancel := rt.R().NewContext().WithTimeout(time.Minute)
	defer cancel()
	servers, err := ns.Resolve(ctx, name)
	if err != nil {
		vlog.Infof("ns.Resolve(%q) failed: %v", name, err)
		return err
	}
	for _, s := range servers {
		fmt.Fprintln(cmd.Stdout(), s)
	}
	return nil
}

var cmdResolveToMT = &cmdline.Command{
	Run:      runResolveToMT,
	Name:     "resolvetomt",
	Short:    "Finds the address of the mounttable that holds an object name",
	Long:     "Finds the address of the mounttable that holds an object name.",
	ArgsName: "<name>",
	ArgsLong: "<name> is the name to resolve.",
}

func runResolveToMT(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("resolvetomt: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	ns := rt.R().Namespace()
	ctx, cancel := rt.R().NewContext().WithTimeout(time.Minute)
	defer cancel()
	e, err := ns.ResolveToMountTableX(ctx, name)
	if err != nil {
		vlog.Infof("ns.ResolveToMountTableX(%q) failed: %v", name, err)
		return err
	}
	for _, s := range e.Servers {
		fmt.Fprintln(cmd.Stdout(), naming.JoinAddressName(s.Server, e.Name))
	}
	return nil
}

var cmdUnresolve = &cmdline.Command{
	Run:      runUnresolve,
	Name:     "unresolve",
	Short:    "Returns the rooted object names for the given object name",
	Long:     "Returns the rooted object names for the given object name.",
	ArgsName: "<name>",
	ArgsLong: "<name> is the object name to unresolve.",
}

func runUnresolve(cmd *cmdline.Command, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return cmd.UsageErrorf("unresolve: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	ns := rt.R().Namespace()
	ctx, cancel := rt.R().NewContext().WithTimeout(time.Minute)
	defer cancel()
	servers, err := ns.Unresolve(ctx, name)
	if err != nil {
		vlog.Infof("ns.Unresolve(%q) failed: %v", name, err)
		return err
	}
	for _, s := range servers {
		fmt.Fprintln(cmd.Stdout(), s)
	}
	return nil
}

func root() *cmdline.Command {
	return &cmdline.Command{
		Name:  "namespace",
		Short: "Tool for interacting with the Veyron namespace",
		Long: `
The namespace tool facilitates interaction with the Veyron namespace.

The namespace roots are set from the command line via veyron.namespace.root options or from environment variables that have a name
starting with NAMESPACE_ROOT, e.g. NAMESPACE_ROOT, NAMESPACE_ROOT_2,
NAMESPACE_ROOT_GOOGLE, etc. The command line options override the environment.
`,
		Children: []*cmdline.Command{cmdGlob, cmdMount, cmdUnmount, cmdResolve, cmdResolveToMT, cmdUnresolve},
	}
}
