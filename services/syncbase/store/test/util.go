// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"

	"v.io/syncbase/x/ref/services/syncbase/store"
	"v.io/v23/verror"
)

// verifyGet verifies that st.Get(key) == value. If value is nil, verifies that
// the key is not found.
func verifyGet(t *testing.T, st store.StoreReader, key, value []byte) {
	valbuf := []byte("tmp")
	var err error
	if value != nil {
		if valbuf, err = st.Get(key, valbuf); err != nil {
			Fatalf(t, "can't get value of %q: %v", key, err)
		}
		if !bytes.Equal(valbuf, value) {
			Fatalf(t, "unexpected value: got %q, want %q", valbuf, value)
		}
	} else {
		valbuf, err = st.Get(key, valbuf)
		verifyError(t, err, string(key), store.ErrUnknownKey.ID)
		valcopy := []byte("tmp")
		// Verify that valbuf is not modified if the key is not found.
		if !bytes.Equal(valbuf, valcopy) {
			Fatalf(t, "unexpected value: got %q, want %q", valbuf, valcopy)
		}
	}
}

// verifyGet verifies the next key/value pair of the provided stream.
// If key is nil, verifies that next Advance call on the stream returns false.
func verifyAdvance(t *testing.T, s store.Stream, key, value []byte) {
	ok := s.Advance()
	if key == nil {
		if ok {
			Fatalf(t, "advance returned true unexpectedly")
		}
		return
	}
	if !ok {
		Fatalf(t, "can't advance the stream")
	}
	var k, v []byte
	for i := 0; i < 2; i++ {
		if k = s.Key(k); !bytes.Equal(k, key) {
			Fatalf(t, "unexpected key: got %q, want %q", k, key)
		}
		if v = s.Value(v); !bytes.Equal(v, value) {
			Fatalf(t, "unexpected value: got %q, want %q", v, value)
		}
	}
}

func verifyError(t *testing.T, err error, substr string, errorID verror.ID) {
	if got := verror.ErrorID(err); got != errorID {
		Fatalf(t, "unexpected error ID: got %v, want %v", got, errorID)
	}
	if !strings.Contains(err.Error(), substr) {
		Fatalf(t, "unexpected error: got %v, want %v", err, substr)
	}
}

func Fatalf(t *testing.T, format string, args ...interface{}) {
	debug.PrintStack()
	t.Fatalf(format, args...)
}
