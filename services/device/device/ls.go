// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"

	"v.io/x/lib/cmdline"
)

var cmdLs = &cmdline.Command{
	Runner:   globRunner(runLs),
	Name:     "ls",
	Short:    "List applications.",
	Long:     "List application installations or instances.",
	ArgsName: "<app name patterns...>",
	ArgsLong: `
<app name patterns...> are vanadium object names or glob name patterns corresponding to app installations and instances.`,
}

func runLs(entry globResult, stdout, stderr io.Writer) error {
	fmt.Fprintf(stdout, "%v\n", entry.name)
	return nil
}
