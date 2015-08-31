// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Source: sync_types.vdl

package interfaces

import (
	// VDL system imports
	"fmt"
	"v.io/v23/vdl"

	// VDL user imports
	"time"
	"v.io/v23/services/syncbase/nosql"
	_ "v.io/v23/vdlroot/time"
)

// PrefixGenVector is the generation vector for a data prefix, which maps each
// device id to its last locally known generation in the scope of that prefix.
type PrefixGenVector map[uint64]uint64

func (PrefixGenVector) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/server/interfaces.PrefixGenVector"`
}) {
}

// GenVector is the generation vector for a Database, and maps prefixes to their
// generation vectors. Note that the prefixes in a GenVector are relative to the
// the Application and Database name.
type GenVector map[string]PrefixGenVector

func (GenVector) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/server/interfaces.GenVector"`
}) {
}

// LogRecMetadata represents the metadata of a single log record that is
// exchanged between two peers. Each log record represents a change made to an
// object in the store.
//
// TODO(hpucha): Add readset/scanset. Look into sending tx metadata only once
// per transaction.
type LogRecMetadata struct {
	// Log related information.
	Id      uint64 // device id that created the log record.
	Gen     uint64 // generation number for the log record.
	RecType byte   // type of log record.
	// Id of the object that was updated. This id is relative to Application
	// and Database names and is the store key for a particular row in a
	// table.
	ObjId      string
	CurVers    string    // current version number of the object.
	Parents    []string  // 0, 1 or 2 parent versions that the current version is derived from.
	UpdTime    time.Time // timestamp when the update is generated.
	Delete     bool      // indicates whether the update resulted in object being deleted from the store.
	BatchId    uint64    // unique id of the Batch this update belongs to.
	BatchCount uint64    // number of objects in the Batch.
}

func (LogRecMetadata) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/server/interfaces.LogRecMetadata"`
}) {
}

// LogRec represents the on-wire representation of an entire log record: its
// metadata and data. Value is the actual value of a store object.
type LogRec struct {
	Metadata LogRecMetadata
	Value    []byte
}

func (LogRec) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/server/interfaces.LogRec"`
}) {
}

// GroupId is a globally unique SyncGroup ID.
type GroupId uint64

func (GroupId) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/server/interfaces.GroupId"`
}) {
}

// Possible states for a SyncGroup.
type SyncGroupStatus int

const (
	SyncGroupStatusPublishPending SyncGroupStatus = iota
	SyncGroupStatusPublishRejected
	SyncGroupStatusRunning
)

// SyncGroupStatusAll holds all labels for SyncGroupStatus.
var SyncGroupStatusAll = [...]SyncGroupStatus{SyncGroupStatusPublishPending, SyncGroupStatusPublishRejected, SyncGroupStatusRunning}

// SyncGroupStatusFromString creates a SyncGroupStatus from a string label.
func SyncGroupStatusFromString(label string) (x SyncGroupStatus, err error) {
	err = x.Set(label)
	return
}

// Set assigns label to x.
func (x *SyncGroupStatus) Set(label string) error {
	switch label {
	case "PublishPending", "publishpending":
		*x = SyncGroupStatusPublishPending
		return nil
	case "PublishRejected", "publishrejected":
		*x = SyncGroupStatusPublishRejected
		return nil
	case "Running", "running":
		*x = SyncGroupStatusRunning
		return nil
	}
	*x = -1
	return fmt.Errorf("unknown label %q in interfaces.SyncGroupStatus", label)
}

// String returns the string label of x.
func (x SyncGroupStatus) String() string {
	switch x {
	case SyncGroupStatusPublishPending:
		return "PublishPending"
	case SyncGroupStatusPublishRejected:
		return "PublishRejected"
	case SyncGroupStatusRunning:
		return "Running"
	}
	return ""
}

func (SyncGroupStatus) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/server/interfaces.SyncGroupStatus"`
	Enum struct{ PublishPending, PublishRejected, Running string }
}) {
}

// SyncGroup contains the state of a SyncGroup object.
type SyncGroup struct {
	Id          GroupId                              // globally unique identifier generated by Syncbase
	Name        string                               // globally unique Vanadium name chosen by app
	SpecVersion string                               // version on SyncGroup spec for concurrency control
	Spec        nosql.SyncGroupSpec                  // app-given specification
	Creator     string                               // Creator's Vanadium name
	AppName     string                               // Globally unique App name
	DbName      string                               // Database name within the App
	Status      SyncGroupStatus                      // Status of the SyncGroup
	Joiners     map[string]nosql.SyncGroupMemberInfo // map of joiners to their metadata
}

func (SyncGroup) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/server/interfaces.SyncGroup"`
}) {
}

// DeltaReq contains the initiator's genvector and the set of SyncGroups it is
// interested in within a Database (specified by the AppName/DbName) when
// requesting deltas for that Database.
type DeltaReq struct {
	AppName string
	DbName  string
	SgIds   map[GroupId]struct{}
	InitVec GenVector
}

func (DeltaReq) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/server/interfaces.DeltaReq"`
}) {
}

type (
	// DeltaResp represents any single field of the DeltaResp union type.
	//
	// DeltaResp contains the responder's genvector or the missing log records
	// returned in response to an initiator's request for deltas for a Database.
	DeltaResp interface {
		// Index returns the field index.
		Index() int
		// Interface returns the field value as an interface.
		Interface() interface{}
		// Name returns the field name.
		Name() string
		// __VDLReflect describes the DeltaResp union type.
		__VDLReflect(__DeltaRespReflect)
	}
	// DeltaRespStart represents field Start of the DeltaResp union type.
	DeltaRespStart struct{ Value bool }
	// DeltaRespFinish represents field Finish of the DeltaResp union type.
	DeltaRespFinish struct{ Value bool }
	// DeltaRespRec represents field Rec of the DeltaResp union type.
	DeltaRespRec struct{ Value LogRec }
	// DeltaRespRespVec represents field RespVec of the DeltaResp union type.
	DeltaRespRespVec struct{ Value GenVector }
	// __DeltaRespReflect describes the DeltaResp union type.
	__DeltaRespReflect struct {
		Name  string `vdl:"v.io/x/ref/services/syncbase/server/interfaces.DeltaResp"`
		Type  DeltaResp
		Union struct {
			Start   DeltaRespStart
			Finish  DeltaRespFinish
			Rec     DeltaRespRec
			RespVec DeltaRespRespVec
		}
	}
)

func (x DeltaRespStart) Index() int                      { return 0 }
func (x DeltaRespStart) Interface() interface{}          { return x.Value }
func (x DeltaRespStart) Name() string                    { return "Start" }
func (x DeltaRespStart) __VDLReflect(__DeltaRespReflect) {}

func (x DeltaRespFinish) Index() int                      { return 1 }
func (x DeltaRespFinish) Interface() interface{}          { return x.Value }
func (x DeltaRespFinish) Name() string                    { return "Finish" }
func (x DeltaRespFinish) __VDLReflect(__DeltaRespReflect) {}

func (x DeltaRespRec) Index() int                      { return 2 }
func (x DeltaRespRec) Interface() interface{}          { return x.Value }
func (x DeltaRespRec) Name() string                    { return "Rec" }
func (x DeltaRespRec) __VDLReflect(__DeltaRespReflect) {}

func (x DeltaRespRespVec) Index() int                      { return 3 }
func (x DeltaRespRespVec) Interface() interface{}          { return x.Value }
func (x DeltaRespRespVec) Name() string                    { return "RespVec" }
func (x DeltaRespRespVec) __VDLReflect(__DeltaRespReflect) {}

// ChunkHash contains the hash of a chunk that is part of a blob's recipe.
type ChunkHash struct {
	Hash []byte
}

func (ChunkHash) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/server/interfaces.ChunkHash"`
}) {
}

// ChunkData contains the data of a chunk.
type ChunkData struct {
	Data []byte
}

func (ChunkData) __VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/syncbase/server/interfaces.ChunkData"`
}) {
}

func init() {
	vdl.Register((*PrefixGenVector)(nil))
	vdl.Register((*GenVector)(nil))
	vdl.Register((*LogRecMetadata)(nil))
	vdl.Register((*LogRec)(nil))
	vdl.Register((*GroupId)(nil))
	vdl.Register((*SyncGroupStatus)(nil))
	vdl.Register((*SyncGroup)(nil))
	vdl.Register((*DeltaReq)(nil))
	vdl.Register((*DeltaResp)(nil))
	vdl.Register((*ChunkHash)(nil))
	vdl.Register((*ChunkData)(nil))
}

const NoGroupId = GroupId(0)

// NodeRec type log record adds a new node in the dag.
const NodeRec = byte(0)

// LinkRec type log record adds a new link in the dag. Link records are
// added when a conflict is resolved by picking the local or the remote
// version as the resolution of a conflict, instead of creating a new
// version.
const LinkRec = byte(1)
