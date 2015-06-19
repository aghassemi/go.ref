// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package debuglib

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/services/logreader"
	"v.io/v23/services/stats"
	s_vtrace "v.io/v23/services/vtrace"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/v23/vtrace"

	libstats "v.io/x/ref/lib/stats"
	"v.io/x/ref/lib/xrpc"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

//go:generate v23 test generate

func TestDebugServer(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	tracedContext := func(ctx *context.T) *context.T {
		ctx, _ = vtrace.WithNewTrace(ctx)
		vtrace.ForceCollect(ctx)
		return ctx
	}
	rootName = "debug"

	workdir, err := ioutil.TempDir("", "logreadertest")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(workdir)
	if err = ioutil.WriteFile(filepath.Join(workdir, "test.INFO"), []byte("test"), os.FileMode(0644)); err != nil {
		t.Fatalf("ioutil.WriteFile failed: %v", err)
	}

	disp := NewDispatcher(func() string { return workdir }, nil)
	server, err := xrpc.NewDispatchingServer(ctx, "", disp)
	if err != nil {
		t.Fatalf("failed to start debug server: %v", err)
	}
	endpoint := server.Status().Endpoints[0].String()

	// Access a logs directory that exists.
	{
		results, _, err := testutil.GlobName(ctx, naming.JoinAddressName(endpoint, "debug/logs"), "*")
		if err != nil {
			t.Errorf("Glob failed: %v", err)
		}
		if len(results) != 1 || results[0] != "test.INFO" {
			t.Errorf("unexpected result. Got %v, want 'test.INFO'", results)
		}
	}

	// Access a logs directory that doesn't exist.
	{
		results, _, err := testutil.GlobName(ctx, naming.JoinAddressName(endpoint, "debug/logs/nowheretobefound"), "*")
		if len(results) != 0 {
			t.Errorf("unexpected result. Got %v, want ''", results)
		}
		if err != nil {
			t.Errorf("unexpected error value: %v", err)
		}
	}

	// Access a log file that exists.
	{
		lf := logreader.LogFileClient(naming.JoinAddressName(endpoint, "debug/logs/test.INFO"))
		size, err := lf.Size(tracedContext(ctx))
		if err != nil {
			t.Errorf("Size failed: %v", err)
		}
		if expected := int64(len("test")); size != expected {
			t.Errorf("unexpected result. Got %v, want %v", size, expected)
		}
	}

	// Access a log file that doesn't exist.
	{
		lf := logreader.LogFileClient(naming.JoinAddressName(endpoint, "debug/logs/nosuchfile.INFO"))
		_, err = lf.Size(tracedContext(ctx))
		if expected := verror.ErrNoExist.ID; verror.ErrorID(err) != expected {
			t.Errorf("unexpected error value, got %v, want: %v", err, expected)
		}
	}

	// Access a stats object that exists.
	{
		foo := libstats.NewInteger("testing/foo")
		foo.Set(123)

		st := stats.StatsClient(naming.JoinAddressName(endpoint, "debug/stats/testing/foo"))
		v, err := st.Value(tracedContext(ctx))
		if err != nil {
			t.Errorf("Value failed: %v", err)
		}
		if want := vdl.Int64Value(123); !vdl.EqualValue(v, want) {
			t.Errorf("unexpected result. got %v, want %v", v, want)
		}
	}

	// Access a stats object that doesn't exists.
	{
		st := stats.StatsClient(naming.JoinAddressName(endpoint, "debug/stats/testing/nobodyhome"))
		_, err = st.Value(tracedContext(ctx))
		if expected := verror.ErrNoExist.ID; verror.ErrorID(err) != expected {
			t.Errorf("unexpected error value, got %v, want: %v", err, expected)
		}
	}

	// Access vtrace.
	{
		vt := s_vtrace.StoreClient(naming.JoinAddressName(endpoint, "debug/vtrace"))
		call, err := vt.AllTraces(ctx)
		if err != nil {
			t.Errorf("AllTraces failed: %v", err)
		}
		ntraces := 0
		stream := call.RecvStream()
		for stream.Advance() {
			stream.Value()
			ntraces++
		}
		if err = stream.Err(); err != nil && err != io.EOF {
			t.Fatalf("Unexpected error reading trace stream: %s", err)
		}
		if ntraces != 4 {
			t.Errorf("We expected 4 traces, got: %d", ntraces)
		}
	}

	// Glob from the root.
	{
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		ns := v23.GetNamespace(ctx)
		ns.SetRoots(naming.JoinAddressName(endpoint, "debug"))

		c, err := ns.Glob(ctx, "logs/...")
		if err != nil {
			t.Errorf("ns.Glob failed: %v", err)
		}
		results := []string{}
		for res := range c {
			switch v := res.(type) {
			case *naming.GlobReplyEntry:
				results = append(results, v.Value.Name)
			}
		}
		sort.Strings(results)
		expected := []string{
			"logs",
			"logs/test.INFO",
		}
		if !reflect.DeepEqual(expected, results) {
			t.Errorf("unexpected result. Got %v, want %v", results, expected)
		}

		c, err = ns.Glob(ctx, "stats/testing/*")
		if err != nil {
			t.Errorf("ns.Glob failed: %v", err)
		}
		results = []string{}
		for res := range c {
			t.Logf("got %v", res)
			switch v := res.(type) {
			case *naming.GlobReplyEntry:
				results = append(results, v.Value.Name)
			}
		}
		sort.Strings(results)
		expected = []string{
			"stats/testing/foo",
		}
		if !reflect.DeepEqual(expected, results) {
			t.Errorf("unexpected result. Got %v, want %v", results, expected)
		}

		c, err = ns.Glob(ctx, "*")
		if err != nil {
			t.Errorf("ns.Glob failed: %v", err)
		}
		results = []string{}
		for res := range c {
			switch v := res.(type) {
			case *naming.GlobReplyEntry:
				results = append(results, v.Value.Name)
			}
		}
		sort.Strings(results)
		expected = []string{
			"logs",
			"pprof",
			"stats",
			"vtrace",
		}
		if !reflect.DeepEqual(expected, results) {
			t.Errorf("unexpected result. Got %v, want %v", results, expected)
		}

		c, err = ns.Glob(ctx, "...")
		if err != nil {
			t.Errorf("ns.Glob failed: %v", err)
		}
		results = []string{}
		for res := range c {
			switch v := res.(type) {
			case *naming.GlobReplyEntry:
				if strings.HasPrefix(v.Value.Name, "stats/") && !strings.HasPrefix(v.Value.Name, "stats/testing/") {
					// Skip any non-testing stats.
					continue
				}
				results = append(results, v.Value.Name)
			}
		}
		sort.Strings(results)
		expected = []string{
			"",
			"logs",
			"logs/test.INFO",
			"pprof",
			"stats",
			"stats/testing/foo",
			"vtrace",
		}
		if !reflect.DeepEqual(expected, results) {
			t.Errorf("unexpected result. Got %v, want %v", results, expected)
		}
	}
}
