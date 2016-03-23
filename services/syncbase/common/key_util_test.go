// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common_test

import (
	"reflect"
	"testing"

	"v.io/x/ref/services/syncbase/common"
)

var keyPartTests = []struct {
	parts []string
	key   string
}{
	{[]string{"a", "b"}, "a\xfeb"},
	{[]string{"aa", "bb"}, "aa\xfebb"},
	{[]string{"a", "b", "c"}, "a\xfeb\xfec"},
}

func TestJoinKeyParts(t *testing.T) {
	for _, test := range keyPartTests {
		got, want := common.JoinKeyParts(test.parts...), test.key
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%v: got %q, want %q", test.parts, got, want)
		}
	}
}

func TestSplitKeyParts(t *testing.T) {
	for _, test := range keyPartTests {
		got, want := common.SplitKeyParts(test.key), test.parts
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", test.key, got, want)
		}
	}
}

func TestSplitNKeyParts(t *testing.T) {
	for _, test := range keyPartTests {
		got, want := common.SplitNKeyParts(test.key, 1), []string{test.key}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", test.key, got, want)
		}
	}
	for _, test := range keyPartTests {
		// Note, all test cases in keyPartTests have <= 3 parts.
		got, want := common.SplitNKeyParts(test.key, 3), test.parts
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", test.key, got, want)
		}
	}
}

func TestStripFirstKeyPartOrDie(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{"a\xfe", ""},
		{"a\xfeb", "b"},
		{"a\xfe\xfe", "\xfe"},
		{"a\xfeb\xfe", "b\xfe"},
		{"a\xfeb\xfec", "b\xfec"},
	}
	for _, test := range tests {
		got, want := common.StripFirstKeyPartOrDie(test.in), test.out
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", test.in, got, want)
		}
	}
}

func TestFirstKeyPart(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{"", ""},
		{"a", "a"},
		{"a\xfe", "a"},
		{"a\xfeb", "a"},
		{"\xfe", ""},
		{"\xfeb", ""},
	}
	for _, test := range tests {
		got, want := common.FirstKeyPart(test.in), test.out
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", test.in, got, want)
		}
	}
}

func TestIsRowKey(t *testing.T) {
	tests := []struct {
		in  string
		out bool
	}{
		{"", false},
		{"a", false},
		{"a\xfe", false},
		{"a\xfeb", false},
		{common.RowPrefix, true},
		{common.RowPrefix + "\xfe", true},
		{common.RowPrefix + "\xfeb", true},
		{common.PermsPrefix, false},
		{common.PermsPrefix + "\xfe", false},
		{common.PermsPrefix + "\xfeb", false},
	}
	for _, test := range tests {
		got, want := common.IsRowKey(test.in), test.out
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", test.in, got, want)
		}
	}
}

func TestIsPermsKey(t *testing.T) {
	tests := []struct {
		in  string
		out bool
	}{
		{"", false},
		{"a", false},
		{"a\xfe", false},
		{"a\xfeb", false},
		{common.RowPrefix, false},
		{common.RowPrefix + "\xfe", false},
		{common.RowPrefix + "\xfeb", false},
		{common.PermsPrefix, true},
		{common.PermsPrefix + "\xfe", true},
		{common.PermsPrefix + "\xfeb", true},
	}
	for _, test := range tests {
		got, want := common.IsPermsKey(test.in), test.out
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", test.in, got, want)
		}
	}
}

func TestParseTableAndRow(t *testing.T) {
	tests := []struct {
		key   string
		table string
		row   string
		err   bool
	}{
		{common.RowPrefix + "\xfetb\xferow", "tb", "row", false},
		{common.RowPrefix + "\xfetb\xfe", "tb", "", false},
		{common.RowPrefix + "\xfe\xferow", "", "row", false},
		{common.RowPrefix + "\xfe\xfe", "", "", false},
		{common.PermsPrefix + "\xfetb\xferow", "tb", "row", false},
		{common.PermsPrefix + "\xfetb\xfe", "tb", "", false},
		{common.PermsPrefix + "\xfe\xferow", "", "row", false},
		{common.PermsPrefix + "\xfe\xfe", "", "", false},
		{"pfx\xfetb\xferow", "", "", true},
		{"pfx\xfetb\xfe", "", "", true},
		{"pfx\xfe\xferow", "", "", true},
		{"pfx\xfe\xfe", "", "", true},
		{"\xfetb\xferow", "", "", true},
		{"\xfetb\xfe", "", "", true},
		{"\xfe\xferow", "", "", true},
		{"\xfe\xfe", "", "", true},
		{common.RowPrefix, "", "", true},
		{common.RowPrefix + "\xfetb", "", "", true},
		{common.RowPrefix + "\xfe", "", "", true},
	}
	for _, test := range tests {
		table, row, err := common.ParseTableAndRow(test.key)
		if !reflect.DeepEqual(table, test.table) {
			t.Errorf("%q: got %v, want %v", test.key, table, test.table)
		}
		if !reflect.DeepEqual(row, test.row) {
			t.Errorf("%q: got %v, want %v", test.key, table, test.table)
		}
		if !reflect.DeepEqual(err != nil, test.err) {
			t.Errorf("%q: got %v, want %v", test.key, err != nil, test.err)
		}
	}
}

func TestScanPrefixArgs(t *testing.T) {
	tests := []struct {
		stKeyPrefix, prefix, wantStart, wantLimit string
	}{
		{"x", "", "x\xfe", "x\xff"},
		{"x", "a", "x\xfea", "x\xfeb"},
		{"x", "a\xfe", "x\xfea\xfe", "x\xfea\xff"},
	}
	for _, test := range tests {
		start, limit := common.ScanPrefixArgs(test.stKeyPrefix, test.prefix)
		gotStart, gotLimit := string(start), string(limit)
		if gotStart != test.wantStart {
			t.Errorf("{%q, %q} start: got %q, want %q", test.stKeyPrefix, test.prefix, gotStart, test.wantStart)
		}
		if gotLimit != test.wantLimit {
			t.Errorf("{%q, %q} limit: got %q, want %q", test.stKeyPrefix, test.prefix, gotLimit, test.wantLimit)
		}
	}
}

func TestScanRangeArgs(t *testing.T) {
	tests := []struct {
		stKeyPrefix, start, limit, wantStart, wantLimit string
	}{
		{"x", "", "", "x\xfe", "x\xff"},   // limit "" means "no limit"
		{"x", "a", "", "x\xfea", "x\xff"}, // limit "" means "no limit"
		{"x", "a", "b", "x\xfea", "x\xfeb"},
		{"x", "a", "a", "x\xfea", "x\xfea"}, // empty range
		{"x", "b", "a", "x\xfeb", "x\xfea"}, // empty range
	}
	for _, test := range tests {
		start, limit := common.ScanRangeArgs(test.stKeyPrefix, test.start, test.limit)
		gotStart, gotLimit := string(start), string(limit)
		if gotStart != test.wantStart {
			t.Errorf("{%q, %q, %q} start: got %q, want %q", test.stKeyPrefix, test.start, test.limit, gotStart, test.wantStart)
		}
		if gotLimit != test.wantLimit {
			t.Errorf("{%q, %q, %q} limit: got %q, want %q", test.stKeyPrefix, test.start, test.limit, gotLimit, test.wantLimit)
		}
	}
}
