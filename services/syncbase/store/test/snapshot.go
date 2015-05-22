// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"testing"

	"v.io/syncbase/x/ref/services/syncbase/store"
	"v.io/v23/verror"
)

// RunSnapshotTest verifies store.Snapshot operations.
func RunSnapshotTest(t *testing.T, st store.Store) {
	key1, value1 := []byte("key1"), []byte("value1")
	st.Put(key1, value1)
	snapshot := st.NewSnapshot()
	key2, value2 := []byte("key2"), []byte("value2")
	st.Put(key2, value2)

	// Test Get and Scan.
	verifyGet(t, snapshot, key1, value1)
	verifyGet(t, snapshot, key2, nil)
	s := snapshot.Scan([]byte("a"), []byte("z"))
	verifyAdvance(t, s, key1, value1)
	verifyAdvance(t, s, nil, nil)

	// Test functions after Close.
	if err := snapshot.Close(); err != nil {
		t.Fatalf("can't close the snapshot: %v", err)
	}
	expectedErr := "closed snapshot"
	verifyError(t, snapshot.Close(), expectedErr, verror.ErrCanceled.ID)

	_, err := snapshot.Get(key1, nil)
	verifyError(t, err, expectedErr, verror.ErrCanceled.ID)

	s = snapshot.Scan([]byte("a"), []byte("z"))
	verifyAdvance(t, s, nil, nil)
	verifyError(t, s.Err(), expectedErr, verror.ErrCanceled.ID)
}
