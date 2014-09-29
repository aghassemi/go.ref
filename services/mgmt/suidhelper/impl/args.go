package impl

import (
	"flag"
	"fmt"
	"os/user"
	"strconv"
)

type WorkParameters struct {
	uid       uint32
	gid       uint32
	workspace string
	stderrLog string
	stdoutLog string
	argv0     string
	argv      []string
	envv      []string
}

var flagUsername, flagWorkspace, flagStdoutLog, flagStderrLog, flagRun *string
var flagMinimumUid *int64

func init() {
	// Add flags to global set.
	setupFlags(flag.CommandLine)
}

func setupFlags(fs *flag.FlagSet) {
	flagUsername = fs.String("username", "", "The UNIX user name used for the other functions of this tool.")
	flagWorkspace = fs.String("workspace", "", "Path to the application's workspace directory.")
	flagStdoutLog = fs.String("stdoutlog", "", "Path to the stdout log file.")
	flagStderrLog = fs.String("stderrlog", "", "Path to the stdin log file.")
	flagRun = fs.String("run", "", "Path to the application to exec.")
	flagMinimumUid = fs.Int64("minuid", 501, "UIDs cannot be less than this number.")
}

// ParseArguments populates the WorkParameter object from the provided args
// and env strings.
func (wp *WorkParameters) ProcessArguments(fs *flag.FlagSet, env []string) error {
	username := *flagUsername
	if username == "" {
		return fmt.Errorf("--username missing")
	}

	usr, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("--username %s: unknown user", username)
	}

	uid, err := strconv.ParseUint(usr.Uid, 0, 32)
	if err != nil {
		return fmt.Errorf("user.Lookup() returned an invalid uid %v", usr.Uid)
	}
	gid, err := strconv.ParseUint(usr.Gid, 0, 32)
	if err != nil {
		return fmt.Errorf("user.Lookup() returned an invalid gid %v", usr.Gid)
	}

	// Uids less than 501 can be special so we forbid running as them.
	if uint32(uid) < uint32(*flagMinimumUid) {
		return fmt.Errorf("suidhelper does not permit uids less than %d", uint32(*flagMinimumUid))
	}

	wp.uid = uint32(uid)
	wp.gid = uint32(gid)
	wp.workspace = *flagWorkspace
	wp.argv0 = *flagRun
	wp.stdoutLog = *flagStdoutLog
	wp.stderrLog = *flagStderrLog
	wp.argv = fs.Args()
	wp.envv = env

	return nil
}
