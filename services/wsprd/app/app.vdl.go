// This file was auto-generated by the veyron vdl tool.
// Source: app.vdl

// The app package contains the struct that keeps per javascript app state and handles translating
// javascript requests to veyron requests and vice versa.
package app

import (
	"v.io/core/veyron2/security"

	// The non-user imports are prefixed with "__" to prevent collisions.
	__vdl "v.io/core/veyron2/vdl"
)

type VeyronRPC struct {
	Name        string
	Method      string
	InArgs      []__vdl.AnyRep
	NumOutArgs  int32
	IsStreaming bool
	Timeout     int64
}

func (VeyronRPC) __VDLReflect(struct {
	Name string "v.io/wspr/veyron/services/wsprd/app.VeyronRPC"
}) {
}

type BlessingRequest struct {
	Handle     int32
	Caveats    []security.Caveat
	DurationMs int32
	Extension  string
}

func (BlessingRequest) __VDLReflect(struct {
	Name string "v.io/wspr/veyron/services/wsprd/app.BlessingRequest"
}) {
}

func init() {
	__vdl.Register(VeyronRPC{})
	__vdl.Register(BlessingRequest{})
}
