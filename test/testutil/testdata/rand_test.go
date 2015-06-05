// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil_test

import (
	"testing"

	"v.io/x/ref/test/testutil"
)

func TestRandSeed(t *testing.T) {
	defer testutil.InitRandGenerator(t.Logf)()
	t.Logf("rand: %d", testutil.RandomInt())
	t.FailNow()
}
