// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command namespace resolves and manages names in the Vanadium namespace.

The namespace roots are set from the command line via --v23.namespace.root
command line option or from environment variables that have a name starting with
V23_NAMESPACE, e.g.  V23_NAMESPACE, V23_NAMESPACE_2, V23_NAMESPACE_GOOGLE, etc.
The command line options override the environment.

Usage:
   namespace <command>

The namespace commands are:
   glob        Returns all matching entries from the namespace
   mount       Adds a server to the namespace
   unmount     Removes a server from the namespace
   resolve     Translates a object name to its object address(es)
   resolvetomt Finds the address of the mounttable that holds an object name
   permissions Manipulates permissions on an entry in the namespace
   help        Display help for commands or topics
Run "namespace help [command]" for command usage.

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

Namespace Glob

Returns all matching entries from the namespace.

Usage:
   namespace glob [flags] <pattern>

<pattern> is a glob pattern that is matched against all the names below the
specified mount name.

The namespace glob flags are:
 -l=false
   Long listing format.

Namespace Mount

Adds server <server> to the namespace with name <name>.

Usage:
   namespace mount <name> <server> <ttl>

<name> is the name to add to the namespace. <server> is the object address of
the server to add. <ttl> is the TTL of the new entry. It is a decimal number
followed by a unit suffix (s, m, h). A value of 0s represents an infinite
duration.

Namespace Unmount

Removes server <server> with name <name> from the namespace.

Usage:
   namespace unmount <name> <server>

<name> is the name to remove from the namespace. <server> is the object address
of the server to remove.

Namespace Resolve

Translates a object name to its object address(es).

Usage:
   namespace resolve [flags] <name>

<name> is the name to resolve.

The namespace resolve flags are:
 -insecure=false
   Insecure mode: May return results from untrusted servers and invoke Resolve
   on untrusted mounttables

Namespace Resolvetomt

Finds the address of the mounttable that holds an object name.

Usage:
   namespace resolvetomt [flags] <name>

<name> is the name to resolve.

The namespace resolvetomt flags are:
 -insecure=false
   Insecure mode: May return results from untrusted servers and invoke Resolve
   on untrusted mounttables

Namespace Permissions

Commands to get and set the permissions on a name - controlling the blessing
names required to resolve the name.

The permissions are provided as an JSON-encoded version of the Permissions type
defined in v.io/v23/security/access/types.vdl.

Usage:
   namespace permissions <command>

The namespace permissions commands are:
   get         Gets permissions on a mount name
   set         Sets permissions on a mount name

Namespace Permissions Get

Get retrieves the permissions on the usage of a name.

The output is a JSON-encoded Permissions object (defined in
v.io/v23/security/access/types.vdl).

Usage:
   namespace permissions get <name>

<name> is a name in the namespace.

Namespace Permissions Set

Set replaces the permissions controlling usage of a mount name.

Usage:
   namespace permissions set <name> <permissions>

<name> is the name on which permissions are to be set.

<permissions> is the path to a file containing a JSON-encoded Permissions object
(defined in v.io/v23/security/access/types.vdl), or "-" for STDIN.

Namespace Help

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

The output is formatted to a target width in runes.  The target width is
determined by checking the environment variable CMDLINE_WIDTH, falling back on
the terminal width from the OS, falling back on 80 chars.  By setting
CMDLINE_WIDTH=x, if x > 0 the width is x, if x < 0 the width is unlimited, and
if x == 0 or is unset one of the fallbacks is used.

Usage:
   namespace help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The namespace help flags are:
 -style=default
   The formatting style for help output, either "default" or "godoc".
*/
package main
