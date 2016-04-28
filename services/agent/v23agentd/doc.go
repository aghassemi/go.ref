// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command v23agentd runs the security agent daemon, which holds the private key,
blessings and recognized roots of a principal in memory and makes the principal
available to other processes.

Other processes can access the agent credentials when V23_AGENT_PATH is set to
<credential dir>/agent/sock.

Exits right away if another agent is already serving the credentials. Exits when
there are no processes accessing the credentials (after a grace period).

Example:
 $ v23agentd $HOME/.credentials
 $ V23_AGENT_PATH=$HOME/.credentials/agent/sock principal dump

Usage:
   v23agentd [flags] credentials

The path for the directory containing the credentials to be served by the agent.

The v23agentd flags are:
 -daemon=false
   Run the agent as a daemon (returns right away but leaves the agent running in
   the background)
 -stop=false
   Stop the agent serving the credentials, if any is running
 -timeout=1m0s
   How long the agent stays alive without any client connections
 -with-version=0
   Version that the agent should use.  Will fail if the version is not in the
   range of supported versions (obtained from the --metadata flag)

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
 -vmodule=
   comma-separated list of globpattern=N settings for filename-filtered logging
   (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns baz or
   *az or b* but not by bar/baz or baz.go or az or b.*
 -vpath=
   comma-separated list of regexppattern=N settings for file pathname-filtered
   logging (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns
   foo/bar/baz or fo.*az or oo/ba or b.z but not by foo/bar/baz.go or fo*az
*/
package main
