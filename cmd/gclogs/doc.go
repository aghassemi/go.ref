// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
gclogs is a utility that safely deletes old log files.

It looks for file names that match the format of files produced by the vlog
package, and deletes the ones that have not changed in the amount of time
specified by the --cutoff flag.

Only files produced by the same user as the one running the gclogs command are
considered for deletion.

Usage:
   gclogs [flags] <dir> ...

<dir> ... A list of directories where to look for log files.

The gclogs flags are:
 -cutoff=24h0m0s
   The age cut-off for a log file to be considered for garbage collection.
 -n=false
   If true, log files that would be deleted are shown on stdout, but not
   actually deleted.
 -program=.*
   A regular expression to apply to the program part of the log file name, e.g
   ".*test".
 -verbose=false
   If true, each deleted file is shown on stdout.

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
 -v23.namespace.root=[/ns.dev.v.io:8101]
   local namespace root; can be repeated to provided multiple roots
 -v23.permissions.file=map[]
   specify an acl file as <name>:<aclfile>
 -v23.permissions.literal=
   explicitly specify the runtime acl as a JSON-encoded access.Permissions.
   Overrides all --v23.permissions.file flags.
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
*/
package main
