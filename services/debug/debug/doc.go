// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command debug supports debugging Vanadium servers.

Usage:
   debug [flags] <command>

The debug commands are:
   glob        Returns all matching entries from the namespace.
   vtrace      Returns vtrace traces.
   logs        Accesses log files
   stats       Accesses stats
   pprof       Accesses profiling data
   browse      Starts an interactive interface for debugging
   help        Display help for commands or topics

The debug flags are:
 -timeout=10s
   Time to wait for various RPCs

The global flags are:
 -alsologtostderr=true
   log to standard error as well as files
 -log_backtrace_at=:0
   when logging hits line file:N, emit a stack trace
 -log_dir=
   if non-empty, write log files to this directory
 -logtostderr=false
   log to standard error instead of files
 -max_stack_buf_size=4292608
   max size in bytes of the buffer to use for logging stack traces
 -metadata=<just specify -metadata to activate>
   Displays metadata for the program and exits.
 -stderrthreshold=2
   logs at or above this threshold go to stderr
 -time=false
   Dump timing information to stderr before exiting the program.
 -v=0
   log level for V logs
 -v23.credentials=
   directory to use for storing security credentials
 -v23.i18n-catalogue=
   18n catalogue files to load, comma separated
 -v23.namespace.root=[/(dev.v.io:role:vprod:service:mounttabled)@ns.dev.v.io:8101]
   local namespace root; can be repeated to provided multiple roots
 -v23.proxy=
   object name of proxy service to use to export services across network
   boundaries
 -v23.tcp.address=
   address to listen on
 -v23.tcp.protocol=wsh
   protocol to listen with
 -v23.vtrace.cache-size=1024
   The number of vtrace traces to store in memory.
 -v23.vtrace.collect-regexp=
   Spans and annotations that match this regular expression will trigger trace
   collection.
 -v23.vtrace.dump-on-shutdown=true
   If true, dump all stored traces on runtime shutdown.
 -v23.vtrace.sample-rate=0
   Rate (from 0.0 to 1.0) to sample vtrace traces.
 -vmodule=
   comma-separated list of globpattern=N settings for filename-filtered logging
   (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns baz or
   *az or b* but not by bar/baz or baz.go or az or b.*
 -vpath=
   comma-separated list of regexppattern=N settings for file pathname-filtered
   logging (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns
   foo/bar/baz or fo.*az or oo/ba or b.z but not by foo/bar/baz.go or fo*az

Debug glob

Returns all matching entries from the namespace.

Usage:
   debug glob [flags] <pattern> ...

<pattern> is a glob pattern to match.

The debug glob flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug vtrace

Returns matching vtrace traces (or all stored traces if no ids are given).

Usage:
   debug vtrace [flags] <name> [id ...]

<name> is the name of a vtrace object. [id] is a vtrace trace id.

The debug vtrace flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug logs - Accesses log files

Accesses log files

Usage:
   debug logs [flags] <command>

The debug logs commands are:
   read        Reads the content of a log file object.
   size        Returns the size of a log file object.

The debug logs flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug logs read

Reads the content of a log file object.

Usage:
   debug logs read [flags] <name>

<name> is the name of the log file object.

The debug logs read flags are:
 -f=false
   When true, read will wait for new log entries when it reaches the end of the
   file.
 -n=-1
   The number of log entries to read.
 -o=0
   The position, in bytes, from which to start reading the log file.
 -v=false
   When true, read will be more verbose.

 -timeout=10s
   Time to wait for various RPCs

Debug logs size

Returns the size of a log file object.

Usage:
   debug logs size [flags] <name>

<name> is the name of the log file object.

The debug logs size flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug stats - Accesses stats

Accesses stats

Usage:
   debug stats [flags] <command>

The debug stats commands are:
   read        Returns the value of stats objects.
   watch       Returns a stream of all matching entries and their values as they
               change.

The debug stats flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug stats read

Returns the value of stats objects.

Usage:
   debug stats read [flags] <name> ...

<name> is the name of a stats object, or a glob pattern to match against stats
object names.

The debug stats read flags are:
 -json=false
   When true, the command will display the raw value of the object in json
   format.
 -raw=false
   When true, the command will display the raw value of the object.
 -type=false
   When true, the type of the values will be displayed.

 -timeout=10s
   Time to wait for various RPCs

Debug stats watch

Returns a stream of all matching entries and their values as they change.

Usage:
   debug stats watch [flags] <pattern> ...

<pattern> is a glob pattern to match.

The debug stats watch flags are:
 -raw=false
   When true, the command will display the raw value of the object.
 -type=false
   When true, the type of the values will be displayed.

 -timeout=10s
   Time to wait for various RPCs

Debug pprof - Accesses profiling data

Accesses profiling data

Usage:
   debug pprof [flags] <command>

The debug pprof commands are:
   run         Runs the pprof tool.
   proxy       Runs an http proxy to a pprof object.

The debug pprof flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug pprof run

Runs the pprof tool.

Usage:
   debug pprof run [flags] <name> <profile> [passthru args] ...

<name> is the name of the pprof object. <profile> the name of the profile to
use.

All the [passthru args] are passed to the pprof tool directly, e.g.

  $ debug pprof run a/b/c/__debug/pprof heap --text
  $ debug pprof run a/b/c/__debug/pprof profile -gv

The debug pprof run flags are:
 -pprofcmd=jiri go tool pprof
   The pprof command to use.

 -timeout=10s
   Time to wait for various RPCs

Debug pprof proxy

Runs an http proxy to a pprof object.

Usage:
   debug pprof proxy [flags] <name>

<name> is the name of the pprof object.

The debug pprof proxy flags are:
 -timeout=10s
   Time to wait for various RPCs

Debug browse - Starts an interactive interface for debugging

Starts a webserver with a URL that when visited allows for inspection of a
remote process via a web browser.

This differs from browser.v.io in a few important ways:

  (a) Does not require a chrome extension,
  (b) Is not tied into the v.io cloud services
  (c) Can be setup with alternative different credentials,
  (d) The interface is more geared towards debugging a server than general purpose namespace browsing.

While (d) is easily overcome by sharing code between the two, (a), (b) & (c) are
not easy to work around.  Of course, the down-side here is that this requires
explicit command-line invocation instead of being just a URL anyone can visit
(https://browser.v.io).

A dump of some possible future features: TODO(ashankar):?

  (1) Profiling: Should be able to use the webserver to profile the remote
  process (via 'go tool pprof' for example).  In the mean time, use the 'pprof'
  command (instead of the 'browse' command) for this purpose.
  (2) Trace browsing: Browse traces at the remote server, and possible force
  the collection of some traces (avoiding the need to restart the remote server
  with flags like --v23.vtrace.collect-regexp for example). In the mean time,
  use the 'vtrace' command (instead of the 'browse' command) for this purpose.
  (3) Log offsets: Log files can be large and currently the logging endpoint
  of this interface downloads the full log file from the beginning. The ability
  to start looking at the logs only from a specified offset might be useful
  for these large files.
  (4) Delegation: The 'browse' command requires the appropriate credentials to
  inspect a remote process. Make delegation of these credentials to another
  instance of the 'browse' command easier so that, for example, Bob can conveniently
  ask Alice to debug his service without worrying about giving Alice the ability to
  modify his service.
  (5) Signature: Display the interfaces, types etc. defined by any suffix in the
  remote process. in the mean time, use the 'vrpc signature' command for this purpose.

Usage:
   debug browse [flags] <name>

<name> is the vanadium object name of the remote process to inspec

The debug browse flags are:
 -addr=
   Address on which the interactive HTTP server will listen. For example,
   localhost:14141. If empty, defaults to localhost:<some random port>
 -log=true
   If true, log debug data obtained so that if a subsequent refresh from the
   browser fails, previously obtained information is available from the log file

 -timeout=10s
   Time to wait for various RPCs

Debug help - Display help for commands or topics

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

Usage:
   debug help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The debug help flags are:
 -style=compact
   The formatting style for help output:
      compact   - Good for compact cmdline output.
      full      - Good for cmdline output, shows all global flags.
      godoc     - Good for godoc processing.
      shortonly - Only output short description.
   Override the default by setting the CMDLINE_STYLE environment variable.
 -width=<terminal width>
   Format output to this target width in runes, or unlimited if width < 0.
   Defaults to the terminal width if available.  Override the default by setting
   the CMDLINE_WIDTH environment variable.
*/
package main
