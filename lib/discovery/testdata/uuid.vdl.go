// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Source: uuid.vdl

package testdata

import (
	// VDL system imports
	"v.io/v23/vdl"
)

// UuidTestData represents the inputs and outputs for a uuid test.
type UuidTestData struct {
	// In is the input string.
	In string
	// Want is the expected uuid's human-readable string form.
	Want string
}

func (UuidTestData) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/discovery/testdata.UuidTestData"`
}) {
}

func init() {
	vdl.Register((*UuidTestData)(nil))
}

var InterfaceNameTest = []UuidTestData{
	{
		In:   "v.io",
		Want: "2101363c-688d-548a-a600-34d506e1aad0",
	},
	{
		In:   "v.io/v23/abc",
		Want: "6726c4e5-b6eb-5547-9228-b2913f4fad52",
	},
	{
		In:   "v.io/v23/abc/xyz",
		Want: "be8a57d7-931d-5ee4-9243-0bebde0029a5",
	},
}
