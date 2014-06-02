package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"veyron/examples/tunnel"
	"veyron/examples/tunnel/lib"
	"veyron/lib/signals"
	"veyron2"
	"veyron2/rt"
	"veyron2/vlog"
)

var (
	disablePty = flag.Bool("T", false, "Disable pseudo-terminal allocation.")
	forcePty   = flag.Bool("t", false, "Force allocation of pseudo-terminal.")
	vname      = flag.String("vname", "", "Veyron name (or endpoint) for tunneling service.")

	portforward = flag.String("L", "", "localaddr,remoteaddr Forward local 'localaddr' to 'remoteaddr'")
	lprotocol   = flag.String("local_protocol", "tcp", "Local network protocol for port forwarding")
	rprotocol   = flag.String("remote_protocol", "tcp", "Remote network protocol for port forwarding")

	noshell = flag.Bool("N", false, "Do not execute a shell. Only do port forwarding.")
)

func init() {
	flag.Usage = func() {
		bname := path.Base(os.Args[0])
		fmt.Fprintf(os.Stderr, `%s: Veyron SHell.

This tool is used to run shell commands or an interactive shell on a remote
tunneld service.

To open an interactive shell, use:
  %s --host=<veyron name or endpoint>

To run a shell command, use:
  %s --host=<veyron name or endpoint> <command to run>

The -L flag will forward connections from a local port to a remote address
through the tunneld service. The flag value is localaddr,remoteaddr. E.g.
  -L :14141,www.google.com:80

%s can't be used directly with tools like rsync because veyron addresses don't
look like traditional hostnames, which rsync doesn't understand. For
compatibility with such tools, %s has a special feature that allows passing the
veyron address via the VSH_NAME environment variable.

  $ VSH_NAME=<veyron address> rsync -avh -e %s /foo/* veyron:/foo/

In this example, the "veyron" host will be substituted with $VSH_NAME by %s
and rsync will work as expected.

Full flags:
`, os.Args[0], bname, bname, bname, bname, os.Args[0], bname)
		flag.PrintDefaults()
	}
}

func main() {
	// Work around the fact that os.Exit doesn't run deferred functions.
	os.Exit(realMain())
}

func realMain() int {
	r := rt.Init()
	defer r.Shutdown()

	host, cmd, err := veyronNameAndCommandLine()
	if err != nil {
		flag.Usage()
		fmt.Fprintf(os.Stderr, "\n%v\n", err)
		return 1
	}

	t, err := tunnel.BindTunnel(host)
	if err != nil {
		vlog.Fatalf("BindTunnel(%q) failed: %v", host, err)
	}

	if len(*portforward) > 0 {
		go runPortForwarding(t, host)
	}

	if *noshell {
		<-signals.ShutdownOnSignals()
		return 0
	}

	opts := shellOptions(cmd)

	stream, err := t.Shell(rt.R().TODOContext(), cmd, opts, veyron2.CallTimeout(24*time.Hour))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	saved := lib.EnterRawTerminalMode()
	defer lib.RestoreTerminalSettings(saved)
	runIOManager(os.Stdin, os.Stdout, os.Stderr, stream)

	exitMsg := fmt.Sprintf("Connection to %s closed.", host)
	exitStatus, err := stream.Finish()
	if err != nil {
		exitMsg += fmt.Sprintf(" (%v)", err)
	}
	vlog.VI(1).Info(exitMsg)
	// Only show the exit message on stdout for interactive shells.
	// Otherwise, the exit message might get confused with the output
	// of the command that was run.
	if err != nil {
		fmt.Fprintln(os.Stderr, exitMsg)
	} else if len(cmd) == 0 {
		fmt.Println(exitMsg)
	}
	return int(exitStatus)
}

func shellOptions(cmd string) (opts tunnel.ShellOpts) {
	opts.UsePty = (len(cmd) == 0 || *forcePty) && !*disablePty
	opts.Environment = environment()
	ws, err := lib.GetWindowSize()
	if err != nil {
		vlog.VI(1).Infof("GetWindowSize failed: %v", err)
	} else {
		opts.Rows = uint32(ws.Row)
		opts.Cols = uint32(ws.Col)
	}
	return
}

func environment() []string {
	env := []string{}
	for _, name := range []string{"TERM", "COLORTERM"} {
		if value := os.Getenv(name); value != "" {
			env = append(env, fmt.Sprintf("%s=%s", name, value))
		}
	}
	return env
}

// veyronNameAndCommandLine extracts the veyron name and the remote command to
// send to the server. The name can be specified with the --vname flag or as the
// first non-flag argument. The command line is the concatenation of all the
// non-flag arguments, minus the veyron name.
func veyronNameAndCommandLine() (string, string, error) {
	name := *vname
	args := flag.Args()
	if len(name) == 0 {
		if len(args) > 0 {
			name = args[0]
			args = args[1:]
		}
	}
	if len(name) == 0 {
		return "", "", errors.New("veyron name missing")
	}
	// For compatibility with tools like rsync. Because veyron addresses
	// don't look like traditional hostnames, tools that work with rsh and
	// ssh can't work directly with vsh. This trick makes the following
	// possible:
	//   $ VSH_NAME=<veyron address> rsync -avh -e vsh /foo/* veyron:/foo/
	// The "veyron" host will be substituted with <veyron address>.
	if envName := os.Getenv("VSH_NAME"); len(envName) > 0 && name == "veyron" {
		name = envName
	}
	cmd := strings.Join(args, " ")
	return name, cmd, nil
}

func runPortForwarding(t tunnel.Tunnel, host string) {
	// *portforward is localaddr,remoteaddr
	parts := strings.Split(*portforward, ",")
	var laddr, raddr string
	if len(parts) != 2 {
		vlog.Fatalf("-L flag expects 2 values separated by a comma")
	}
	laddr = parts[0]
	raddr = parts[1]

	ln, err := net.Listen(*lprotocol, laddr)
	if err != nil {
		vlog.Fatalf("net.Listen(%q, %q) failed: %v", *lprotocol, laddr, err)
	}
	defer ln.Close()
	vlog.VI(1).Infof("Listening on %q", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			vlog.Infof("Accept failed: %v", err)
			continue
		}
		stream, err := t.Forward(rt.R().TODOContext(), *rprotocol, raddr, veyron2.CallTimeout(24*time.Hour))
		if err != nil {
			vlog.Infof("Tunnel(%q, %q) failed: %v", *rprotocol, raddr, err)
			conn.Close()
			continue
		}
		name := fmt.Sprintf("%v-->%v-->(%v)-->%v", conn.RemoteAddr(), conn.LocalAddr(), host, raddr)
		go func() {
			vlog.VI(1).Infof("TUNNEL START: %v", name)
			errf := lib.Forward(conn, stream)
			err := stream.Finish()
			vlog.VI(1).Infof("TUNNEL END  : %v (%v, %v)", name, errf, err)
		}()
	}
}
