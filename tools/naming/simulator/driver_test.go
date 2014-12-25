package main

import (
	"fmt"
	"reflect"
	"testing"

	"v.io/veyron/veyron/lib/modules"
)

func TestFields(t *testing.T) {
	cases := []struct {
		input  string
		output []string
	}{
		{"", []string{}},
		{"a", []string{"a"}},
		{"  z", []string{"z"}},
		{"  zz  zz", []string{"zz", "zz"}},
		{"ab", []string{"ab"}},
		{"a b", []string{"a", "b"}},
		{`a " b"`, []string{"a", " b"}},
		{`a "  b  zz"`, []string{"a", "  b  zz"}},
		{`a "  b		zz"`, []string{"a", "  b		zz"}},
		{`a " b" cc`, []string{"a", " b", "cc"}},
		{`a "z b" cc`, []string{"a", "z b", "cc"}},
	}
	for i, c := range cases {
		got, err := splitQuotedFields(c.input)
		if err != nil {
			t.Errorf("%d: %q: unexpected error: %v", i, c.input, err)
		}
		if !reflect.DeepEqual(got, c.output) {
			t.Errorf("%d: %q: got %#v, want %#v", i, c.input, got, c.output)
		}
	}
	if _, err := splitQuotedFields(`a b "c`); err == nil {
		t.Errorf("expected error for unterminated quote")
	}
}

func TestVariables(t *testing.T) {
	sh, err := modules.NewShell(nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	defer sh.Cleanup(nil, nil)
	sh.SetVar("foo", "bar")
	cases := []struct {
		input  string
		output []string
	}{
		{"a b", []string{"a", "b"}},
		{"a $foo", []string{"a", "bar"}},
		{"$foo a", []string{"bar", "a"}},
		{`a "$foo "`, []string{"a", "bar "}},
		{"a xx$foo", []string{"a", "xxbar"}},
		{"a xx${foo}yy", []string{"a", "xxbaryy"}},
		{`a "foo"`, []string{"a", "foo"}},
	}
	for i, c := range cases {
		fields, err := splitQuotedFields(c.input)
		if err != nil {
			t.Errorf("%d: %q: unexpected error: %v", i, c.input, err)
		}
		got, err := subVariables(sh, fields)
		if err != nil {
			t.Errorf("%d: %q: unexpected error: %v", i, c.input, err)
		}
		if !reflect.DeepEqual(got, c.output) {
			t.Errorf("%d: %q: got %#v, want %#v", i, c.input, got, c.output)
		}
	}

	errors := []struct {
		input string
		err   error
	}{
		{"$foox", fmt.Errorf("unknown variable: %q", "foox")},
		{"${fo", fmt.Errorf("unterminated variable: %q", "{fo")},
	}
	for i, c := range errors {
		vars, got := subVariables(sh, []string{c.input})
		if (got == nil && c.err != nil) || got.Error() != c.err.Error() {
			t.Errorf("%d: %q: expected error: got %v (with results %#v) want %v", i, c.input, got, vars, c.err)
		}
	}
}
