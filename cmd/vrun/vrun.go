// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $V23_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/lib/cmdline"
	"v.io/x/lib/vlog"
	"v.io/x/ref/envvar"
	"v.io/x/ref/services/agent/agentlib"
	"v.io/x/ref/services/agent/keymgr"
	"v.io/x/ref/services/role"

	_ "v.io/x/ref/profiles"
)

var (
	durationFlag time.Duration
	nameFlag     string
	roleFlag     string
)

var cmdVrun = &cmdline.Command{
	Run:      vrun,
	Name:     "vrun",
	Short:    "executes commands with a derived Vanadium principal",
	Long:     "Command vrun executes commands with a derived Vanadium principal.",
	ArgsName: "<command> [command args...]",
}

func main() {
	cmdline.HideGlobalFlagsExcept()
	syscall.CloseOnExec(3)
	syscall.CloseOnExec(4)

	cmdVrun.Flags.DurationVar(&durationFlag, "duration", 1*time.Hour, "Duration for the blessing.")
	cmdVrun.Flags.StringVar(&nameFlag, "name", "", "Name to use for the blessing. Uses the command name if unset.")
	cmdVrun.Flags.StringVar(&roleFlag, "role", "", "Role object from which to request the blessing. If set, the blessings from this role server are used and --name is ignored. If not set, the default blessings of the calling principal are extended with --name.")

	os.Exit(cmdVrun.Main())
}

func vrun(cmd *cmdline.Command, args []string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	if len(args) == 0 {
		args = []string{"bash", "--norc"}
	}
	principal, conn, err := createPrincipal(ctx)
	if err != nil {
		return err
	}
	if len(roleFlag) == 0 {
		if len(nameFlag) == 0 {
			nameFlag = filepath.Base(args[0])
		}
		if err := bless(ctx, principal, nameFlag); err != nil {
			return err
		}
	} else {
		// The role server expects the client's blessing name to end
		// with RoleSuffix. This is to avoid accidentally granting role
		// access to anything else that might have been blessed by the
		// same principal.
		if err := bless(ctx, principal, role.RoleSuffix); err != nil {
			return err
		}
		rCtx, err := v23.WithPrincipal(ctx, principal)
		if err != nil {
			return err
		}
		if err := setupRoleBlessings(rCtx, roleFlag); err != nil {
			return err
		}
	}

	return doExec(args, conn)
}

func bless(ctx *context.T, p security.Principal, name string) error {
	caveat, err := security.NewExpiryCaveat(time.Now().Add(durationFlag))
	if err != nil {
		vlog.Errorf("Couldn't create caveat")
		return err
	}

	rp := v23.GetPrincipal(ctx)
	blessing, err := rp.Bless(p.PublicKey(), rp.BlessingStore().Default(), name, caveat)
	if err != nil {
		vlog.Errorf("Couldn't bless")
		return err
	}

	if err = p.BlessingStore().SetDefault(blessing); err != nil {
		vlog.Errorf("Couldn't set default blessing")
		return err
	}
	if _, err = p.BlessingStore().Set(blessing, security.AllPrincipals); err != nil {
		vlog.Errorf("Couldn't set default client blessing")
		return err
	}
	if err = p.AddToRoots(blessing); err != nil {
		vlog.Errorf("Couldn't set trusted roots")
		return err
	}
	return nil
}

func doExec(cmd []string, conn *os.File) error {
	if conn.Fd() != 3 {
		if err := syscall.Dup2(int(conn.Fd()), 3); err != nil {
			vlog.Errorf("Couldn't dup fd")
			return err
		}
		conn.Close()
	}
	p, err := exec.LookPath(cmd[0])
	if err != nil {
		vlog.Errorf("Couldn't find %q", cmd[0])
		return err
	}
	err = syscall.Exec(p, cmd, os.Environ())
	vlog.Errorf("Couldn't exec %s.", cmd[0])
	return err
}

func createPrincipal(ctx *context.T) (security.Principal, *os.File, error) {
	kagent, err := keymgr.NewAgent()
	if err != nil {
		vlog.Errorf("Could not initialize agent")
		return nil, nil, err
	}

	_, conn, err := kagent.NewPrincipal(ctx, true)
	if err != nil {
		vlog.Errorf("Couldn't create principal")
		return nil, nil, err
	}

	ep, err := v23.NewEndpoint(os.Getenv(envvar.AgentEndpoint))
	if err != nil {
		vlog.Errorf("Couldn't parse %v=%q: %v", envvar.AgentEndpoint, os.Getenv(envvar.AgentEndpoint), err)
		return nil, nil, err
	}
	// Connect to the Principal
	fd, err := syscall.Dup(int(conn.Fd()))
	if err != nil {
		vlog.Errorf("Couldn't copy fd")
		return nil, nil, err
	}
	syscall.CloseOnExec(fd)
	ep, err = v23.NewEndpoint(agentlib.AgentEndpoint(fd))
	if err != nil {
		vlog.Errorf("Error creating endpoint: %v", err)
		return nil, nil, err
	}
	principal, err := agentlib.NewAgentPrincipal(ctx, ep, v23.GetClient(ctx))
	if err != nil {
		vlog.Errorf("Couldn't connect to principal")
	}
	return principal, conn, nil
}

func setupRoleBlessings(ctx *context.T, roleStr string) error {
	b, err := role.RoleClient(roleStr).SeekBlessings(ctx)
	if err != nil {
		return err
	}
	p := v23.GetPrincipal(ctx)
	// TODO(rthellend,ashankar): Revisit this configuration.
	// SetDefault: Should we expect users to want to act as a server on behalf of the role (by default?)
	// AllPrincipals: Do we not want to be discriminating about which services we use the role blessing at.
	if err := p.BlessingStore().SetDefault(b); err != nil {
		return err
	}
	if _, err := p.BlessingStore().Set(b, security.AllPrincipals); err != nil {
		return err
	}
	return nil
}
