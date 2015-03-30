// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
The vomtestgen tool generates vom test data, using the vomdata file as input,
and creating a vdl file as output.

Usage:
   vomtestgen [flags] [vomdata]

[vomdata] is the path to the vomdata input file, specified in the vdl config
file format.  It must be of the form "NAME.vdl.config", and the output vdl file
will be generated at "NAME.vdl".

The config file should export a const []any that contains all of the values that
will be tested.  Here's an example:
   config = []any{
     bool(true), uint64(123), string("abc"),
   }

If not specified, we'll try to find the file at its canonical location:
   v.io/v23/vom/testdata/vomdata.vdl.config

The vomtestgen flags are:
 -exts=.vdl
   Comma-separated list of valid VDL file name extensions.
 -max_errors=-1
   Stop processing after this many errors, or -1 for unlimited.

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
 -vanadium.i18n_catalogue=
   18n catalogue files to load, comma separated
 -veyron.acl.file=map[]
   specify an acl file as <name>:<aclfile>
 -veyron.acl.literal=
   explicitly specify the runtime acl as a JSON-encoded access.Permissions.
   Overrides all --veyron.acl.file flags.
 -veyron.credentials=
   directory to use for storing security credentials
 -veyron.namespace.root=[/ns.dev.v.io:8101]
   local namespace root; can be repeated to provided multiple roots
 -veyron.proxy=
   object name of proxy service to use to export services across network
   boundaries
 -veyron.tcp.address=
   address to listen on
 -veyron.tcp.protocol=wsh
   protocol to listen with
 -veyron.vtrace.cache_size=1024
   The number of vtrace traces to store in memory.
 -veyron.vtrace.collect_regexp=
   Spans and annotations that match this regular expression will trigger trace
   collection.
 -veyron.vtrace.dump_on_shutdown=true
   If true, dump all stored traces on runtime shutdown.
 -veyron.vtrace.sample_rate=0
   Rate (from 0.0 to 1.0) to sample vtrace traces.
 -vmodule=
   comma-separated list of pattern=N settings for file-filtered logging
*/
package main
