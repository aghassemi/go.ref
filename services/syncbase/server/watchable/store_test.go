// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package watchable

import (
	"runtime"
	"testing"

	"v.io/syncbase/x/ref/services/syncbase/store"
	"v.io/syncbase/x/ref/services/syncbase/store/test"
)

func init() {
	runtime.GOMAXPROCS(10)
}

func TestStream(t *testing.T) {
	runTest(t, []string{}, test.RunStreamTest)
	runTest(t, nil, test.RunStreamTest)
}

func TestSnapshot(t *testing.T) {
	runTest(t, []string{}, test.RunSnapshotTest)
	runTest(t, nil, test.RunSnapshotTest)
}

func TestStoreState(t *testing.T) {
	runTest(t, []string{}, test.RunStoreStateTest)
	runTest(t, nil, test.RunStoreStateTest)
}

func TestClose(t *testing.T) {
	runTest(t, []string{}, test.RunCloseTest)
	runTest(t, nil, test.RunCloseTest)
}

func TestReadWriteBasic(t *testing.T) {
	runTest(t, []string{}, test.RunReadWriteBasicTest)
	runTest(t, nil, test.RunReadWriteBasicTest)
}

func TestReadWriteRandom(t *testing.T) {
	runTest(t, []string{}, test.RunReadWriteRandomTest)
	runTest(t, nil, test.RunReadWriteRandomTest)
}

func TestConcurrentTransactions(t *testing.T) {
	runTest(t, []string{}, test.RunConcurrentTransactionsTest)
	runTest(t, nil, test.RunConcurrentTransactionsTest)
}

func TestTransactionState(t *testing.T) {
	runTest(t, []string{}, test.RunTransactionStateTest)
	runTest(t, nil, test.RunTransactionStateTest)
}

func TestTransactionsWithGet(t *testing.T) {
	runTest(t, []string{}, test.RunTransactionsWithGetTest)
	runTest(t, nil, test.RunTransactionsWithGetTest)
}

// With Memstore, TestReadWriteRandom is slow with ManagedPrefixes=nil since
// every watchable.Store.Get() takes a snapshot, and memstore snapshots are
// relatively expensive since the entire data map is copied. LevelDB snapshots
// are cheap, so with LevelDB ManagedPrefixes=nil is still reasonably fast.
const useMemstore = false

func runTest(t *testing.T, mp []string, f func(t *testing.T, st store.Store)) {
	st, destroy := createStore(useMemstore)
	defer destroy()
	st, err := Wrap(st, &Options{ManagedPrefixes: mp})
	if err != nil {
		t.Fatal(err)
	}
	f(t, st)
}
