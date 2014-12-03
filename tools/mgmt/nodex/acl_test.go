package main

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"veyron.io/veyron/veyron2/naming"
	"veyron.io/veyron/veyron2/security"
	"veyron.io/veyron/veyron2/services/security/access"
	"veyron.io/veyron/veyron2/verror"
)

func TestACLGetCommand(t *testing.T) {
	tape := NewTape()
	server, endpoint, err := startServer(t, runtime, tape)
	if err != nil {
		return
	}
	defer stopServer(t, server)

	// Setup the command-line.
	cmd := root()
	var stdout, stderr bytes.Buffer
	cmd.Init(nil, &stdout, &stderr)
	nodeName := naming.JoinAddressName(endpoint.String(), "")

	// Test the 'get' command.
	tape.SetResponses([]interface{}{GetACLResponse{
		acl: access.TaggedACLMap{
			"Admin": access.ACL{
				In:    []security.BlessingPattern{"self/..."},
				NotIn: []string{"self/bad"},
			},
			"Read": access.ACL{
				In: []security.BlessingPattern{"other", "self/..."},
			},
		},
		etag: "anEtagForToday",
		err:  nil,
	}})

	if err := cmd.Execute([]string{"acl", "get", nodeName}); err != nil {
		t.Fatalf("%v, output: %v, error: %v", err)
	}
	if expected, got := strings.TrimSpace(`
other Read
self/... Admin,Read
self/bad !Admin
`), strings.TrimSpace(stdout.String()); got != expected {
		t.Errorf("Unexpected output from get. Got %q, expected %q", got, expected)
	}
	if got, expected := tape.Play(), []interface{}{"GetACL"}; !reflect.DeepEqual(expected, got) {
		t.Errorf("invalid call sequence. Got %#v, want %#v", got, expected)
	}
	tape.Rewind()
	stdout.Reset()
}

func TestACLSetCommand(t *testing.T) {
	tape := NewTape()
	server, endpoint, err := startServer(t, runtime, tape)
	if err != nil {
		return
	}
	defer stopServer(t, server)

	// Setup the command-line.
	cmd := root()
	var stdout, stderr bytes.Buffer
	cmd.Init(nil, &stdout, &stderr)
	nodeName := naming.JoinAddressName(endpoint.String(), "")

	// Some tests to validate parse.
	if err := cmd.Execute([]string{"acl", "set", nodeName}); err == nil {
		t.Fatalf("failed to correctly detect insufficient parameters")
	}
	if expected, got := "ERROR: set: incorrect number of arguments 1, must be 1 + 2n", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from list. Got %q, expected prefix %q", got, expected)
	}

	stderr.Reset()
	stdout.Reset()
	if err := cmd.Execute([]string{"acl", "set", nodeName, "foo"}); err == nil {
		t.Fatalf("failed to correctly detect insufficient parameters")
	}
	if expected, got := "ERROR: set: incorrect number of arguments 2, must be 1 + 2n", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from list. Got %q, expected prefix %q", got, expected)
	}

	stderr.Reset()
	stdout.Reset()
	if err := cmd.Execute([]string{"acl", "set", nodeName, "foo", "bar", "ohno"}); err == nil {
		t.Fatalf("failed to correctly detect insufficient parameters")
	}
	if expected, got := "ERROR: set: incorrect number of arguments 4, must be 1 + 2n", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from list. Got %q, expected prefix %q", got, expected)
	}

	stderr.Reset()
	stdout.Reset()
	if err := cmd.Execute([]string{"acl", "set", nodeName, "foo", "!"}); err == nil {
		t.Fatalf("failed to detect invalid parameter")
	}
	if expected, got := "ERROR: failed to parse access tags for \"foo\": empty access tag", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Errorf("Unexpected output from list. Got %q, expected prefix %q", got, expected)
	}

	// Correct operation in the absence of errors.
	stderr.Reset()
	stdout.Reset()
	tape.SetResponses([]interface{}{GetACLResponse{
		acl: access.TaggedACLMap{
			"Admin": access.ACL{
				In: []security.BlessingPattern{"self/..."},
			},
			"Read": access.ACL{
				In:    []security.BlessingPattern{"other/...", "self/..."},
				NotIn: []string{"other/bob"},
			},
		},
		etag: "anEtagForToday",
		err:  nil,
	},
		verror.Make(access.ErrBadEtag, fmt.Sprintf("etag mismatch in:%s vers:%s", "anEtagForToday", "anEtagForTomorrow")),
		GetACLResponse{
			acl: access.TaggedACLMap{
				"Admin": access.ACL{
					In: []security.BlessingPattern{"self/..."},
				},
				"Read": access.ACL{
					In:    []security.BlessingPattern{"other/...", "self/..."},
					NotIn: []string{"other/bob/baddevice"},
				},
			},
			etag: "anEtagForTomorrow",
			err:  nil,
		},
		nil,
	})

	// set command that:
	// - Adds entry for "friends/..." to "Write" & "Admin"
	// - Adds a blacklist entry for "friend/alice"  for "Admin"
	// - Edits existing entry for "self/..." (adding "Write" access)
	// - Removes entry for "other/bob/baddevice"
	if err := cmd.Execute([]string{
		"acl",
		"set",
		nodeName,
		"friends/...", "Admin,Write",
		"friends/alice", "!Admin,Write",
		"self/...", "Admin,Write,Read",
		"other/bob/baddevice", "^",
	}); err != nil {
		t.Fatalf("SetACL failed: %v", err)
	}

	if expected, got := "", strings.TrimSpace(stdout.String()); got != expected {
		t.Fatalf("Unexpected output from list. Got %q, expected %q", got, expected)
	}
	if expected, got := "WARNING: trying again because of asynchronous change", strings.TrimSpace(stderr.String()); got != expected {
		t.Fatalf("Unexpected output from list. Got %q, expected %q", got, expected)
	}
	expected := []interface{}{
		"GetACL",
		SetACLStimulus{
			fun: "SetACL",
			acl: access.TaggedACLMap{
				"Admin": access.ACL{
					In:    []security.BlessingPattern{"friends/...", "self/..."},
					NotIn: []string{"friends/alice"},
				},
				"Read": access.ACL{
					In:    []security.BlessingPattern{"other/...", "self/..."},
					NotIn: []string{"other/bob"},
				},
				"Write": access.ACL{
					In:    []security.BlessingPattern{"friends/...", "friends/alice", "self/..."},
					NotIn: []string{},
				},
			},
			etag: "anEtagForToday",
		},
		"GetACL",
		SetACLStimulus{
			fun: "SetACL",
			acl: access.TaggedACLMap{
				"Admin": access.ACL{
					In:    []security.BlessingPattern{"friends/...", "self/..."},
					NotIn: []string{"friends/alice"},
				},
				"Read": access.ACL{
					In:    []security.BlessingPattern{"other/...", "self/..."},
					NotIn: []string{},
				},
				"Write": access.ACL{
					In:    []security.BlessingPattern{"friends/...", "friends/alice", "self/..."},
					NotIn: []string{},
				},
			},
			etag: "anEtagForTomorrow",
		},
	}

	if got := tape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("invalid call sequence. Got %#v, want %#v", got, expected)
	}
	tape.Rewind()
	stdout.Reset()
	stderr.Reset()

	// GetACL fails.
	tape.SetResponses([]interface{}{GetACLResponse{
		acl:  access.TaggedACLMap{},
		etag: "anEtagForToday",
		err:  verror.BadArgf("oops!"),
	},
	})

	if err := cmd.Execute([]string{"acl", "set", nodeName, "vana/bad", "Read"}); err == nil {
		t.Fatalf("GetACL RPC inside acl set command failed but error wrongly not detected")
	}
	if expected, got := `^ERROR: GetACL\(`+nodeName+`\) failed:.*oops!`, strings.TrimSpace(stderr.String()); !regexp.MustCompile(expected).MatchString(got) {
		t.Fatalf("Unexpected output from list. Got %q, regexp %q", got, expected)
	}
	if expected, got := "", strings.TrimSpace(stdout.String()); got != expected {
		t.Fatalf("Unexpected output from list. Got %q, expected %q", got, expected)
	}
	expected = []interface{}{
		"GetACL",
	}

	if got := tape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("invalid call sequence. Got %#v, want %#v", got, expected)
	}
	tape.Rewind()
	stdout.Reset()
	stderr.Reset()

	// SetACL fails with something other than a bad etag failure.
	tape.SetResponses([]interface{}{GetACLResponse{
		acl: access.TaggedACLMap{
			"Read": access.ACL{
				In: []security.BlessingPattern{"other", "self/..."},
			},
		},
		etag: "anEtagForToday",
		err:  nil,
	},
		verror.BadArgf("oops!"),
	})

	if err := cmd.Execute([]string{"acl", "set", nodeName, "friend", "Read"}); err == nil {
		t.Fatalf("SetACL should have failed: %v", err)
	}
	if expected, got := "", strings.TrimSpace(stdout.String()); got != expected {
		t.Fatalf("Unexpected output from list. Got %q, expected %q", got, expected)
	}
	if expected, got := `^ERROR: SetACL\(`+nodeName+`\) failed:.*oops!`, strings.TrimSpace(stderr.String()); !regexp.MustCompile(expected).MatchString(got) {
		t.Fatalf("Unexpected output from list. Got %q, regexp %q", got, expected)
	}

	expected = []interface{}{
		"GetACL",
		SetACLStimulus{
			fun: "SetACL",
			acl: access.TaggedACLMap{
				"Read": access.ACL{
					In:    []security.BlessingPattern{"friend", "other", "self/..."},
					NotIn: []string{},
				},
			},
			etag: "anEtagForToday",
		},
	}

	if got := tape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("invalid call sequence. Got %#v, want %#v", got, expected)
	}
	tape.Rewind()
	stdout.Reset()
	stderr.Reset()
}
