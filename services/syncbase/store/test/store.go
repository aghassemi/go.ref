// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"v.io/syncbase/x/ref/services/syncbase/store"
)

type operation int

const (
	Put    operation = 0
	Delete operation = 1
)

type testStep struct {
	op  operation
	key int
}

func randomBytes(rnd *rand.Rand, length int) []byte {
	var res []byte
	for i := 0; i < length; i++ {
		res = append(res, '0'+byte(rnd.Intn(10)))
	}
	return res
}

// storeState is the in-memory representation of the store state.
type storeState struct {
	// We assume that the database has keys [0..size).
	size     int
	rnd      *rand.Rand
	memtable map[string][]byte
}

func newStoreState(size int) *storeState {
	return &storeState{
		size,
		rand.New(rand.NewSource(239017)),
		make(map[string][]byte),
	}
}

func (s *storeState) clone() *storeState {
	other := &storeState{
		s.size,
		s.rnd,
		make(map[string][]byte),
	}
	for k, v := range s.memtable {
		other.memtable[k] = v
	}
	return other
}

// nextKey returns the smallest key in the store that is not less than the
// provided key. If there is no such key, returns size.
func (s *storeState) lowerBound(key int) int {
	for key < s.size {
		if _, ok := s.memtable[fmt.Sprintf("%05d", key)]; ok {
			return key
		}
		key++
	}
	return key
}

// verify checks that various read operations on store.Store and memtable return
// the same results.
func (s *storeState) verify(t *testing.T, st store.StoreReader) {
	// Verify Get().
	for i := 0; i < s.size; i++ {
		keystr := fmt.Sprintf("%05d", i)
		answer, ok := s.memtable[keystr]
		if ok {
			verifyGet(t, st, []byte(keystr), answer)
		} else {
			verifyGet(t, st, []byte(keystr), nil)
		}
	}
	// Verify 10 random Scan() calls.
	for i := 0; i < 10; i++ {
		start, limit := s.rnd.Intn(s.size), s.rnd.Intn(s.size)
		if start > limit {
			start, limit = limit, start
		}
		limit++
		stream := st.Scan([]byte(fmt.Sprintf("%05d", start)), []byte(fmt.Sprintf("%05d", limit)))
		for start = s.lowerBound(start); start < limit; start = s.lowerBound(start + 1) {
			keystr := fmt.Sprintf("%05d", start)
			verifyAdvance(t, stream, []byte(keystr), s.memtable[keystr])
		}
		verifyAdvance(t, stream, nil, nil)
	}
}

// runReadWriteTest verifies read/write/snapshot operations.
func runReadWriteTest(t *testing.T, st store.Store, size int, steps []testStep) {
	s := newStoreState(size)
	// We verify database state no more than ~100 times to prevent the test from
	// being slow.
	frequency := (len(steps) + 99) / 100
	var states []*storeState
	var snapshots []store.Snapshot
	for i, step := range steps {
		if step.key < 0 || step.key >= s.size {
			t.Fatalf("invalid test step %v", step)
		}
		switch step.op {
		case Put:
			key := fmt.Sprintf("%05d", step.key)
			value := randomBytes(s.rnd, 100)
			s.memtable[key] = value
			st.Put([]byte(key), value)
		case Delete:
			key := fmt.Sprintf("%05d", step.key)
			if _, ok := s.memtable[key]; ok {
				delete(s.memtable, key)
				st.Delete([]byte(key))
			}
		default:
			t.Fatalf("invalid test step %v", step)
		}
		if i%frequency == 0 {
			s.verify(t, st)
			states = append(states, s.clone())
			snapshots = append(snapshots, st.NewSnapshot())
		}
	}
	s.verify(t, st)
	for i := 0; i < len(states); i++ {
		states[i].verify(t, snapshots[i])
		snapshots[i].Close()
	}
}

// RunReadWriteBasicTest runs a basic test that verifies reads, writes and
// snapshots.
func RunReadWriteBasicTest(t *testing.T, st store.Store) {
	runReadWriteTest(t, st, 3, []testStep{
		testStep{Put, 1},
		testStep{Put, 2},
		testStep{Delete, 1},
		testStep{Put, 1},
		testStep{Put, 2},
	})
}

// RunReadWriteRandomTest runs a random-generated test that verifies reads,
// writes and snapshots.
func RunReadWriteRandomTest(t *testing.T, st store.Store) {
	rnd := rand.New(rand.NewSource(239017))
	var testcase []testStep
	size := 50
	for i := 0; i < 10000; i++ {
		testcase = append(testcase, testStep{operation(rnd.Intn(2)), rnd.Intn(size)})
	}
	runReadWriteTest(t, st, size, testcase)
}

// RunStoreStateTest verifies operations that modify the state of a store.Store.
func RunStoreStateTest(t *testing.T, st store.Store) {
	key1, value1 := []byte("key1"), []byte("value1")
	st.Put(key1, value1)
	key2 := []byte("key2")

	// Test Get and Scan.
	verifyGet(t, st, key1, value1)
	verifyGet(t, st, key2, nil)
	s := st.Scan([]byte("a"), []byte("z"))
	verifyAdvance(t, s, key1, value1)
	verifyAdvance(t, s, nil, nil)

	// Test functions after Close.
	if err := st.Close(); err != nil {
		t.Fatalf("can't close the store: %v", err)
	}
	expectedErr := "closed store"
	if err := st.Close(); !strings.Contains(err.Error(), expectedErr) {
		t.Fatalf("unexpected error: got %v, want %v", err, expectedErr)
	}
	s = st.Scan([]byte("a"), []byte("z"))
	verifyAdvance(t, s, nil, nil)
	if err := s.Err(); !strings.Contains(err.Error(), expectedErr) {
		t.Fatalf("unexpected error: got %v, want %v", err, expectedErr)
	}
	snapshot := st.NewSnapshot()
	if _, err := snapshot.Get(key1, nil); !strings.Contains(err.Error(), expectedErr) {
		t.Fatalf("unexpected error: got %v, want %v", err, expectedErr)
	}
	tx := st.NewTransaction()
	if _, err := tx.Get(key1, nil); !strings.Contains(err.Error(), expectedErr) {
		t.Fatalf("unexpected error: got %v, want %v", err, expectedErr)
	}
	if _, err := st.Get(key1, nil); !strings.Contains(err.Error(), expectedErr) {
		t.Fatalf("unexpected error: got %v, want %v", err, expectedErr)
	}
	if err := st.Put(key1, value1); !strings.Contains(err.Error(), expectedErr) {
		t.Fatalf("unexpected error: got %v, want %v", err, expectedErr)
	}
	if err := st.Delete(key1); !strings.Contains(err.Error(), expectedErr) {
		t.Fatalf("unexpected error: got %v, want %v", err, expectedErr)
	}
}

// RunCloseTest verifies that child objects are closed when the parent object is
// closed.
func RunCloseTest(t *testing.T, st store.Store) {
	key1, value1 := []byte("key1"), []byte("value1")
	st.Put(key1, value1)

	var streams []store.Stream
	var snapshots []store.Snapshot
	var transactions []store.Transaction
	// TODO(rogulenko): make multiple transactions.
	tx := st.NewTransaction()
	for i := 0; i < 10; i++ {
		streams = append(streams, st.Scan([]byte("a"), []byte("z")))
		snapshot := st.NewSnapshot()
		for j := 0; j < 10; j++ {
			streams = append(streams, snapshot.Scan([]byte("a"), []byte("z")))
			streams = append(streams, tx.Scan([]byte("a"), []byte("z")))
		}
		snapshots = append(snapshots, snapshot)
		transactions = append(transactions, tx)
	}
	st.Close()

	for _, stream := range streams {
		if got, want := stream.Err().Error(), "canceled stream"; !strings.Contains(got, want) {
			t.Fatalf("unexpected error: got %v, want %v", got, want)
		}
	}
	for _, snapshot := range snapshots {
		_, err := snapshot.Get(key1, nil)
		if got, want := err.Error(), "closed snapshot"; !strings.Contains(got, want) {
			t.Fatalf("unexpected error: got %v, want %v", got, want)
		}
	}
	for _, tx := range transactions {
		_, err := tx.Get(key1, nil)
		if got, want := err.Error(), "aborted transaction"; !strings.Contains(got, want) {
			t.Fatalf("unexpected error: got %v, want %v", got, want)
		}
	}
}
