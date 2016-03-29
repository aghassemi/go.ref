// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"reflect"
	"testing"

	"v.io/v23"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/memstore"
	"v.io/x/ref/services/syncbase/store/watchable"
	"v.io/x/ref/services/syncbase/vclock"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

type mockCall struct {
	security.Call
	b security.Blessings
}

func (c *mockCall) Server() rpc.Server                   { return nil }
func (c *mockCall) GrantedBlessings() security.Blessings { return c.b }
func (c *mockCall) Security() security.Call              { return c }
func (c *mockCall) LocalBlessings() security.Blessings   { return c.b }
func (c *mockCall) RemoteBlessings() security.Blessings  { return c.b }

func putOp(st store.Store, key, permKey string, permVersion []byte) *watchable.PutOp {
	version, _ := watchable.GetVersion(nil, st, []byte(key))
	return &watchable.PutOp{
		Key:         []byte(key),
		Version:     version,
		PermKey:     []byte(permKey),
		PermVersion: permVersion,
	}
}

// TestWatchLogPerms checks that the recorded prefix permissions object
// used to grant access to Put/Delete operations is correct.
func TestWatchLogPerms(t *testing.T) {
	// Prepare V23.
	ctx, shutdown := test.V23Init()
	defer shutdown()
	ctx, _ = v23.WithPrincipal(ctx, testutil.NewPrincipal("root"))
	// Mock the service, store, db, collection.
	clk := vclock.NewVClockForTests(nil)
	st, _ := watchable.Wrap(memstore.New(), clk, &watchable.Options{
		ManagedPrefixes: []string{common.RowPrefix, common.PermsPrefix},
	})
	db := &databaseReq{database: &database{name: "d", st: st}}
	c := &collectionReq{name: "c", d: db}
	// Mock create the collection.
	perms := access.Permissions{}
	for _, tag := range access.AllTypicalTags() {
		perms.Add(security.BlessingPattern("root"), string(tag))
	}
	store.Put(ctx, st, c.stKey(), &CollectionData{
		Name:  c.name,
		Perms: perms,
	})
	store.Put(ctx, st, c.prefixPermsKey(""), perms)
	store.Put(ctx, st, c.permsIndexStart(""), "")
	store.Put(ctx, st, c.permsIndexLimit(""), "")
	blessings, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
	call := &mockCall{b: blessings}
	var expected []interface{}
	resumeMarker, _ := watchable.GetResumeMarker(st)
	// Generate Put/Delete events.
	for i := 0; i < 5; i++ {
		// Set initial prefix permissions.
		if err := c.SetPrefixPermissions(ctx, call, 0, "foo", perms); err != nil {
			t.Fatalf("c.SetPrefixPermissions failed: %v", err)
		}
		// Put.
		row := &rowReq{key: "foobar", c: c}
		if err := row.Put(ctx, call, 0, []byte("value")); err != nil {
			t.Fatalf("row.Put failed: %v", err)
		}
		permVersion, _ := watchable.GetVersion(ctx, st, []byte(c.prefixPermsKey("foo")))
		expected = append(expected, putOp(st, row.stKey(), c.prefixPermsKey("foo"), permVersion))
		// Delete.
		if err := row.Delete(ctx, call, 0); err != nil {
			t.Fatalf("row.Delete failed: %v", err)
		}
		deleteOp := &watchable.DeleteOp{
			Key:         []byte(row.stKey()),
			PermKey:     []byte(c.prefixPermsKey("foo")),
			PermVersion: permVersion,
		}
		expected = append(expected, deleteOp)
		// DeleteRange.
		if err := row.Put(ctx, call, 0, []byte("value")); err != nil {
			t.Fatalf("row.Put failed: %v", err)
		}
		if err := c.DeleteRange(ctx, call, 0, []byte("foo"), nil); err != nil {
			t.Fatalf("c.DeleteRange failed: %v", err)
		}
		expected = append(expected, deleteOp)
		// SetPrefixPermissions.
		if err := c.SetPrefixPermissions(ctx, call, 0, "foobaz", perms); err != nil {
			t.Fatalf("c.SetPrefixPermissions failed: %v", err)
		}
		expected = append(expected, putOp(st, c.prefixPermsKey("foobaz"), c.prefixPermsKey("foo"), permVersion))
		// SetPrefixPermissions again.
		permVersion, _ = watchable.GetVersion(ctx, st, []byte(c.prefixPermsKey("foobaz")))
		if err := c.SetPrefixPermissions(ctx, call, 0, "foobaz", perms); err != nil {
			t.Fatalf("c.SetPrefixPermissions failed: %v", err)
		}
		expected = append(expected, putOp(st, c.prefixPermsKey("foobaz"), c.prefixPermsKey("foobaz"), permVersion))
		// DeletePrefixPermissions.
		permVersion, _ = watchable.GetVersion(ctx, st, []byte(c.prefixPermsKey("foobaz")))
		if err := c.DeletePrefixPermissions(ctx, call, 0, "foobaz"); err != nil {
			t.Fatalf("c.DeletePrefixPermissions failed: %v", err)
		}
		expected = append(expected, &watchable.DeleteOp{
			Key:         []byte(c.prefixPermsKey("foobaz")),
			PermKey:     []byte(c.prefixPermsKey("foobaz")),
			PermVersion: permVersion,
		})
	}
	expectedIndex := 0
	for {
		var logs []*watchable.LogEntry
		if logs, resumeMarker, _ = watchable.ReadBatchFromLog(st, resumeMarker); logs == nil {
			break
		}
		for _, logRecord := range logs {
			var op interface{}
			if err := logRecord.Op.ToValue(&op); err != nil {
				t.Fatalf("RawBytes.ToValue failed: %v", err)
			}
			if expectedIndex < len(expected) && reflect.DeepEqual(op, expected[expectedIndex]) {
				expectedIndex++
			}
		}
	}
	if expectedIndex != len(expected) {
		t.Fatalf("only %d out of %d record were found", expectedIndex, len(expected))
	}
}
