// This app provides a simple scripted environment for running common veyron
// services as subprocesses and testing interactions between them. It is
// structured as an interpreter, with global variables and variable
// expansion, but no control flow. The command set that it supports is
// extendable by adding new 'commands' that implement the API defined
// by veyron/lib/modules.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"veyron.io/veyron/veyron2/rt"

	"veyron.io/veyron/veyron/lib/expect"
	"veyron.io/veyron/veyron/lib/modules"
	"veyron.io/veyron/veyron/lib/modules/core"
)

type cmdState struct {
	modules.Handle
	*expect.Session
	line string
}

var (
	interactive bool
	handles     map[string]*cmdState
)

func init() {
	flag.BoolVar(&interactive, "interactive", true, "set interactive/batch mode")
	handles = make(map[string]*cmdState)
	flag.Usage = usage
}

var usage = func() {
	fmt.Println(
		`Welcome to this simple shell that lets you run mount tables, a simple server
and sundry other commands from an interactive command line or as scripts. Type
'help' at the prompt to see a list of available commands, or 'help command' to
get specific help about that command. The shell provides environment variables
with expansion and intrinsic support for managing subprocess, but it does not
provide any flow control commands.

All commands, except builtin ones (such as help, set, eval etc) are run
asynchronously in background. That is, the prompt returns as soon as they are
started and no output is displayed from them unless an error is encountered
when they are being started. Each input line is numbered and that number is
used to refer to the standard output of previous started commands. The variable
_ always contains the number of the immediately preceeding line. It is
possible to read the output of a command (using the 'read' builtin) and assign
it that output to an environment variable. The 'eval' builtin parses output of
the form <var>=<val>. In this way subproccess may be started, their output
read and used to configure subsequent subprocesses. For example:

1> time
2> read 1 t
3> print $t

will print the first line of output from the time command, as will the
following:

or:
time
read $_ t
print $t

The eval builtin is used to directly to assign to variables specified
in the output of the command. For example, if the root command
prints out MT_NAME=foo then eval will set MT_NAME to foo as follows:

root
eval $_
print $MT_NAME

will print the value of MT_NAME that is output by the root command.
`)
	flag.PrintDefaults()
}

func prompt(lineno int) {
	if interactive {
		fmt.Printf("%d> ", lineno)
	}
}

func main() {
	rt.Init()

	// Subprocesses commands are run by fork/execing this binary
	// so we must test to see if this instance is a subprocess or the
	// the original command line instance.
	if os.Getenv(modules.ShellEntryPoint) != "" {
		// Subprocess, run the requested command.
		if err := modules.Dispatch(); err != nil {
			fmt.Fprintf(os.Stdout, "failed: %v\n", err)
			fmt.Fprintf(os.Stderr, "failed: %v\n", err)
			return
		}
		return
	}

	shell := modules.NewShell()
	defer shell.Cleanup(os.Stderr)

	core.Install(shell)

	scanner := bufio.NewScanner(os.Stdin)
	lineno := 1
	prompt(lineno)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "#") && len(line) > 0 {
			if line == "eof" {
				break
			}
			if err := process(shell, line, lineno); err != nil {
				fmt.Printf("ERROR: %d> %q: %v\n", lineno, line, err)
				os.Exit(1)
			}
		}
		shell.SetVar("_", strconv.Itoa(lineno))
		lineno++
		prompt(lineno)
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("error reading input: %v\n", err)
	}

}

func output(lineno int, line string) {
	if len(line) > 0 {
		if !interactive {
			fmt.Printf("%d> ", lineno)
		}
		line = strings.TrimSuffix(line, "\n")
		fmt.Printf("%s\n", line)
	}
}

func process(sh *modules.Shell, line string, lineno int) error {
	fields, err := splitQuotedFields(line)
	if err != nil {
		return err
	}
	if len(fields) == 0 {
		return fmt.Errorf("no input")
	}
	name := fields[0]

	var args []string
	if len(fields) > 1 {
		args = fields[1:]
	} else {
		args = []string{}
	}
	sub, err := subVariables(sh, args)
	if cmd := builtins[name]; cmd != nil {
		if cmd.nargs >= 0 && len(sub) != cmd.nargs {
			return fmt.Errorf("wrong (%d) # args for %q: usage %s", len(sub), name, cmd.usage)
		}
		l := ""
		var err error
		if cmd.needsHandle {
			l, err = handleWrapper(sh, cmd.fn, sub...)
		} else {
			l, err = cmd.fn(sh, nil, sub...)
		}
		if err != nil {
			return err
		}
		output(lineno, l)
	} else {
		handle, err := sh.Start(name, sub...)
		if err != nil {
			return err
		}
		handles[strconv.Itoa(lineno)] = &cmdState{
			handle,
			expect.NewSession(nil, handle.Stdout(), time.Minute),
			line,
		}
		if !interactive {
			fmt.Printf("%d> %s\n", lineno, line)
		}
	}

	return nil
}

// splitQuotedFields a line into fields, allowing for quoted strings.
func splitQuotedFields(line string) ([]string, error) {
	fields := []string{}
	inquote := false
	var field []rune
	for _, c := range line {
		switch {
		case c == '"':
			if inquote {
				fields = append(fields, string(field))
				field = nil
				inquote = false
			} else {
				inquote = true
			}
		case unicode.IsSpace(c):
			if inquote {
				field = append(field, c)
			} else {
				if len(field) > 0 {
					fields = append(fields, string(field))
				}
				field = nil
			}
		default:
			field = append(field, c)
		}
	}
	if inquote {
		return nil, fmt.Errorf("unterminated quoted input")
	}

	if len(field) > 0 {
		fields = append(fields, string(field))
	}
	return fields, nil
}

// subVariables substitutes variables that occur in the string slice
// args with values from the Shell.
func subVariables(sh *modules.Shell, args []string) ([]string, error) {
	var results []string
	for _, a := range args {
		if r, err := subVariablesInArgument(sh, a); err != nil {
			return results, err
		} else {
			results = append(results, r)
		}
	}
	return results, nil
}

// subVariablesInArgument substitutes variables that occur in the string
// parameter with values from vars.
//
// A variable, is introduced by $, terminated by \t, space, / , : or !.
// Variables may also be enclosed by {} (as in ${VAR}) to allow for embedding
// within strings.
func subVariablesInArgument(sh *modules.Shell, a string) (string, error) {
	first := strings.Index(a, "$")
	if first < 0 {
		return a, nil
	}
	parts := strings.Split(a, "$")
	result := parts[0]
	vn := ""
	rem := 0
	for _, p := range parts[1:] {
		start := 0
		end := -1
		if strings.HasPrefix(p, "{") {
			start = 1
			end = strings.Index(p, "}")
			if end < 0 {
				return "", fmt.Errorf("unterminated variable: %q", p)
			}
			rem = end + 1
		} else {
			end = strings.IndexAny(p, "\t/,:!= ")
			if end < 0 {
				end = len(p)
			}
			rem = end
		}
		vn = p[start:end]
		r := p[rem:]
		v, present := sh.GetVar(vn)
		if !present {
			return a, nil
		}
		result += v
		result += r
	}
	return result, nil
}
