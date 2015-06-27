// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Source: types.vdl

package watchable

import (
	// VDL system imports
	"v.io/v23/vdl"
)

// GetOp represents a store get operation.
type GetOp struct {
	Key []byte
}

func (GetOp) __VDLReflect(struct {
	Name string `vdl:"v.io/syncbase/x/ref/services/syncbase/server/watchable.GetOp"`
}) {
}

// ScanOp represents a store scan operation.
type ScanOp struct {
	Start []byte
	Limit []byte
}

func (ScanOp) __VDLReflect(struct {
	Name string `vdl:"v.io/syncbase/x/ref/services/syncbase/server/watchable.ScanOp"`
}) {
}

// PutOp represents a store put operation.  The new version is written instead
// of the value to avoid duplicating the user data in the store.  The version
// is used to access the user data of that specific mutation.
type PutOp struct {
	Key     []byte
	Version []byte
}

func (PutOp) __VDLReflect(struct {
	Name string `vdl:"v.io/syncbase/x/ref/services/syncbase/server/watchable.PutOp"`
}) {
}

// DeleteOp represents a store delete operation.
type DeleteOp struct {
	Key []byte
}

func (DeleteOp) __VDLReflect(struct {
	Name string `vdl:"v.io/syncbase/x/ref/services/syncbase/server/watchable.DeleteOp"`
}) {
}

// SyncGroupOp represents a change in SyncGroup tracking, adding or removing
// key prefixes to sync.  SyncGroup prefixes cannot be changed, this is used
// to track changes due to SyncGroup create/join/leave/destroy.
type SyncGroupOp struct {
	Prefixes []string
	Remove   bool
}

func (SyncGroupOp) __VDLReflect(struct {
	Name string `vdl:"v.io/syncbase/x/ref/services/syncbase/server/watchable.SyncGroupOp"`
}) {
}

type (
	// Op represents any single field of the Op union type.
	//
	// Op represents a store operation.
	Op interface {
		// Index returns the field index.
		Index() int
		// Interface returns the field value as an interface.
		Interface() interface{}
		// Name returns the field name.
		Name() string
		// __VDLReflect describes the Op union type.
		__VDLReflect(__OpReflect)
	}
	// OpGet represents field Get of the Op union type.
	OpGet struct{ Value GetOp }
	// OpScan represents field Scan of the Op union type.
	OpScan struct{ Value ScanOp }
	// OpPut represents field Put of the Op union type.
	OpPut struct{ Value PutOp }
	// OpDelete represents field Delete of the Op union type.
	OpDelete struct{ Value DeleteOp }
	// OpSyncGroup represents field SyncGroup of the Op union type.
	OpSyncGroup struct{ Value SyncGroupOp }
	// __OpReflect describes the Op union type.
	__OpReflect struct {
		Name  string `vdl:"v.io/syncbase/x/ref/services/syncbase/server/watchable.Op"`
		Type  Op
		Union struct {
			Get       OpGet
			Scan      OpScan
			Put       OpPut
			Delete    OpDelete
			SyncGroup OpSyncGroup
		}
	}
)

func (x OpGet) Index() int               { return 0 }
func (x OpGet) Interface() interface{}   { return x.Value }
func (x OpGet) Name() string             { return "Get" }
func (x OpGet) __VDLReflect(__OpReflect) {}

func (x OpScan) Index() int               { return 1 }
func (x OpScan) Interface() interface{}   { return x.Value }
func (x OpScan) Name() string             { return "Scan" }
func (x OpScan) __VDLReflect(__OpReflect) {}

func (x OpPut) Index() int               { return 2 }
func (x OpPut) Interface() interface{}   { return x.Value }
func (x OpPut) Name() string             { return "Put" }
func (x OpPut) __VDLReflect(__OpReflect) {}

func (x OpDelete) Index() int               { return 3 }
func (x OpDelete) Interface() interface{}   { return x.Value }
func (x OpDelete) Name() string             { return "Delete" }
func (x OpDelete) __VDLReflect(__OpReflect) {}

func (x OpSyncGroup) Index() int               { return 4 }
func (x OpSyncGroup) Interface() interface{}   { return x.Value }
func (x OpSyncGroup) Name() string             { return "SyncGroup" }
func (x OpSyncGroup) __VDLReflect(__OpReflect) {}

// LogEntry represents a single store operation. This operation may have been
// part of a transaction, as signified by the Continued boolean. Read-only
// operations (and read-only transactions) are not logged.
type LogEntry struct {
	// The store operation that was performed.
	Op Op
	// Time when the operation was committed.
	CommitTimestamp int64
	// If true, this entry is followed by more entries that belong to the same
	// commit as this entry.
	Continued bool
}

func (LogEntry) __VDLReflect(struct {
	Name string `vdl:"v.io/syncbase/x/ref/services/syncbase/server/watchable.LogEntry"`
}) {
}

func init() {
	vdl.Register((*GetOp)(nil))
	vdl.Register((*ScanOp)(nil))
	vdl.Register((*PutOp)(nil))
	vdl.Register((*DeleteOp)(nil))
	vdl.Register((*SyncGroupOp)(nil))
	vdl.Register((*Op)(nil))
	vdl.Register((*LogEntry)(nil))
}
