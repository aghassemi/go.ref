// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package store

import (
	"v.io/v23/verror"
)

// TODO(sadovsky): Add retry loop.
func RunInTransaction(st Store, fn func(st StoreReadWriter) error) error {
	tx := st.NewTransaction()
	if err := fn(tx); err != nil {
		tx.Abort()
		return err
	}
	if err := tx.Commit(); err != nil {
		// TODO(sadovsky): Commit() can fail for a number of reasons, e.g. RPC
		// failure or ErrConcurrentTransaction. Depending on the cause of failure,
		// it may be desirable to retry the Commit() and/or to call Abort(). For
		// now, we always abort on a failed commit.
		tx.Abort()
		return err
	}
	return nil
}

// CopyBytes copies elements from a source slice into a destination slice.
// The returned slice may be a sub-slice of dst if dst was large enough to hold
// src. Otherwise, a newly allocated slice will be returned.
// TODO(rogulenko): add some tests.
func CopyBytes(dst, src []byte) []byte {
	if cap(dst) < len(src) {
		newlen := cap(dst)*2 + 2
		if newlen < len(src) {
			newlen = len(src)
		}
		dst = make([]byte, newlen)
	}
	dst = dst[:len(src)]
	copy(dst, src)
	return dst
}

// WrapError wraps the given err with a verror. It is a no-op if the err is nil.
// The returned error contains the current stack trace, but retains the input
// error's IDAction pair. If the given error is not a verror, the IDAction pair
// of the returned error will be (ErrUnknown.ID, NoRetry).
func WrapError(err error) error {
	if err == nil {
		return nil
	}
	return verror.New(verror.IDAction{
		verror.ErrorID(err),
		verror.Action(err),
	}, nil, err)
}
