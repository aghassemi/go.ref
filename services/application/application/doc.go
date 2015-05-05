// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command application manages the Vanadium application repository.

Usage:
   application <command>

The application commands are:
   match       Shows the first matching envelope that matches the given
               profiles.
   put         Add the given envelope to the application for the given profiles.
   remove      removes the application envelope for the given profile.
   edit        edits the application envelope for the given profile.
   help        Display help for commands or topics

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
 -stderrthreshold=2
   logs at or above this threshold go to stderr
 -v=0
   log level for V logs
 -v23.credentials=
   directory to use for storing security credentials
 -v23.i18n-catalogue=
   18n catalogue files to load, comma separated
 -v23.metadata=<just specify -v23.metadata to activate>
   Displays metadata for the program and exits.
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

Application match

Shows the first matching envelope that matches the given profiles.

Usage:
   application match <application> <profiles>

<application> is the full name of the application. <profiles> is a
comma-separated list of profiles.

Application put

Add the given envelope to the application for the given profiles.

Usage:
   application put <application> <profiles> [<envelope>]

<application> is the full name of the application. <profiles> is a
comma-separated list of profiles. <envelope> is the file that contains a
JSON-encoded envelope. If this file is not provided, the user will be prompted
to enter the data manually.

Application remove

removes the application envelope for the given profile.

Usage:
   application remove <application> <profile>

<application> is the full name of the application. <profile> is a profile.

Application edit

edits the application envelope for the given profile.

Usage:
   application edit <application> <profile>

<application> is the full name of the application. <profile> is a profile.

Application help

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

Output is formatted to a target width in runes, determined by checking the
CMDLINE_WIDTH environment variable, falling back on the terminal width, falling
back on 80 chars.  By setting CMDLINE_WIDTH=x, if x > 0 the width is x, if x < 0
the width is unlimited, and if x == 0 or is unset one of the fallbacks is used.

Usage:
   application help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The application help flags are:
 -style=compact
   The formatting style for help output:
      compact - Good for compact cmdline output.
      full    - Good for cmdline output, shows all global flags.
      godoc   - Good for godoc processing.
   Override the default by setting the CMDLINE_STYLE environment variable.
*/
package main
