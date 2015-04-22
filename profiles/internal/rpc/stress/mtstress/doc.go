// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Usage:
   mtstress <command>

The mtstress commands are:
   mount       Measure latency of the Mount RPC at a fixed request rate
   resolve     Measure latency of the Resolve RPC at a fixed request rate
   help        Display help for commands or topics
Run "mtstress help [command]" for command usage.

The global flags are:
 -alsologtostderr=true
   log to standard error as well as files
 -duration=10s
   Duration for sending test traffic and measuring latency
 -log_backtrace_at=:0
   when logging hits line file:N, emit a stack trace
 -log_dir=
   if non-empty, write log files to this directory
 -logtostderr=false
   log to standard error instead of files
 -max_stack_buf_size=4292608
   max size in bytes of the buffer to use for logging stack traces
 -rate=1
   Rate, in RPCs per second, to send to the test server
 -reauthenticate=false
   If true, establish a new authenticated connection for each RPC, simulating
   load from a distinct process
 -stderrthreshold=2
   logs at or above this threshold go to stderr
 -v=0
   log level for V logs
 -v23.credentials=
   directory to use for storing security credentials
 -v23.i18n-catalogue=
   18n catalogue files to load, comma separated
 -v23.namespace.root=[/(dev.v.io/role/vprod/service/mounttabled)@ns.dev.v.io:8101]
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
   comma-separated list of pattern=N settings for file-filtered logging

Mtstress Mount

Repeatedly issues a Mount request (at --rate) and measures latency

Usage:
   mtstress mount <mountpoint> <ttl>

<mountpoint> defines the name to be mounted

<ttl> specfies the time-to-live of the mount point. For example: 5s for 5
seconds, 1m for 1 minute etc. Valid time units are "ms", "s", "m", "h".

Mtstress Resolve

Repeatedly issues a Resolve request (at --rate) to a name and measures latency

Usage:
   mtstress resolve <name>

<name> the object name to resolve

Mtstress Help

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

The output is formatted to a target width in runes.  The target width is
determined by checking the environment variable CMDLINE_WIDTH, falling back on
the terminal width from the OS, falling back on 80 chars.  By setting
CMDLINE_WIDTH=x, if x > 0 the width is x, if x < 0 the width is unlimited, and
if x == 0 or is unset one of the fallbacks is used.

Usage:
   mtstress help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The mtstress help flags are:
 -style=default
   The formatting style for help output, either "default" or "godoc".
*/
package main