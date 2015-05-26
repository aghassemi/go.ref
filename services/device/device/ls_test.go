// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"v.io/v23/naming"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/test"

	cmd_device "v.io/x/ref/services/device/device"
)

// TestLsCommand verifies the device ls command.  It also acts as a test for the
// glob functionality, by trying out various combinations of
// instances/installations in glob results.
func TestLsCommand(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	tapes := newTapeMap()
	server, endpoint, err := startServer(t, ctx, tapes)
	if err != nil {
		return
	}
	defer stopServer(t, server)

	cmd := cmd_device.CmdRoot
	appName := naming.JoinAddressName(endpoint.String(), "app")
	rootTape := tapes.forSuffix("")
	cannedGlobResponses := [][]string{
		[]string{"app/3", "app/4", "app/6", "app/5"},
		[]string{"app/2", "app/1"},
	}
	cannedStatusResponses := map[string][]interface{}{
		"app/1": []interface{}{instanceRunning},
		"app/2": []interface{}{installationUninstalled},
		"app/3": []interface{}{instanceUpdating},
		"app/4": []interface{}{installationActive},
		"app/5": []interface{}{instanceNotRunning},
		"app/6": []interface{}{installationActive},
	}
	joinLines := func(args ...string) string {
		return strings.Join(args, "\n")
	}
	for _, c := range []struct {
		globResponses   [][]string
		statusResponses map[string][]interface{}
		lsFlags         []string
		globPatterns    []string
		expected        string
	}{
		{
			cannedGlobResponses,
			cannedStatusResponses,
			[]string{},
			[]string{"glob1", "glob2"},
			joinLines(appName+"/2", appName+"/4", appName+"/6", appName+"/1", appName+"/3", appName+"/5"),
		},
		{
			cannedGlobResponses,
			cannedStatusResponses,
			[]string{"--only-instances"},
			[]string{"glob1", "glob2"},
			joinLines(appName+"/1", appName+"/3", appName+"/5"),
		},
		{
			cannedGlobResponses,
			cannedStatusResponses,
			[]string{"--only-installations"},
			[]string{"glob1", "glob2"},
			joinLines(appName+"/2", appName+"/4", appName+"/6"),
		},
		{
			cannedGlobResponses,
			cannedStatusResponses,
			[]string{"--instance-state=Running,Updating"},
			[]string{"glob1", "glob2"},
			joinLines(appName+"/2", appName+"/4", appName+"/6", appName+"/1", appName+"/3"),
		},
		{
			cannedGlobResponses,
			cannedStatusResponses,
			[]string{"--installation-state=Active"},
			[]string{"glob1", "glob2"},
			joinLines(appName+"/4", appName+"/6", appName+"/1", appName+"/3", appName+"/5"),
		},
		{
			cannedGlobResponses,
			cannedStatusResponses,
			[]string{"--only-installations", "--installation-state=Active"},
			[]string{"glob1", "glob2"},
			joinLines(appName+"/4", appName+"/6"),
		},
		{
			cannedGlobResponses,
			cannedStatusResponses,
			[]string{"--only-instances", "--installation-state=Active"},
			[]string{"glob1", "glob2"},
			joinLines(appName+"/1", appName+"/3", appName+"/5"),
		},
		{
			[][]string{
				[]string{"app/1", "app/2"},
				[]string{"app/2", "app/3"},
				[]string{"app/2", "app/3"},
			},
			map[string][]interface{}{
				"app/1": []interface{}{instanceRunning},
				"app/2": []interface{}{installationUninstalled, installationUninstalled},
				"app/3": []interface{}{instanceUpdating},
			},
			[]string{},
			[]string{"glob1", "glob2"},
			joinLines(appName+"/2", appName+"/2", appName+"/1", appName+"/3"),
		},
		{
			[][]string{
				[]string{"app/1", "app/2"},
				[]string{"app/2", "app/3"},
				[]string{"app/2", "app/3"},
			},
			map[string][]interface{}{
				"app/1": []interface{}{instanceRunning},
				"app/2": []interface{}{installationUninstalled, installationUninstalled, installationUninstalled},
				"app/3": []interface{}{instanceUpdating, instanceUpdating},
			},
			[]string{"--only-installations"},
			[]string{"glob1", "glob2", "glob3"},
			joinLines(appName+"/2", appName+"/2", appName+"/2"),
		},
	} {
		var stdout, stderr bytes.Buffer
		env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
		tapes.rewind()
		var rootTapeResponses []interface{}
		for _, r := range c.globResponses {
			rootTapeResponses = append(rootTapeResponses, GlobResponse{r})
		}
		rootTape.SetResponses(rootTapeResponses...)
		for n, r := range c.statusResponses {
			tapes.forSuffix(n).SetResponses(r...)
		}
		args := append([]string{"ls"}, c.lsFlags...)
		for _, p := range c.globPatterns {
			args = append(args, naming.JoinAddressName(endpoint.String(), p))
		}
		if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, args); err != nil {
			fmt.Println("run test case error", err)
			t.Errorf("%v", err)
		}

		if expected, got := c.expected, strings.TrimSpace(stdout.String()); got != expected {
			t.Errorf("Unexpected output from ls. Got %q, expected %q", got, expected)
		}
		cmd_device.ResetGlobFlags()
	}
}