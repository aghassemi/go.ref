// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package interfaces

import (
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/watchable"
)

// Database is an internal interface to the database layer.
type Database interface {
	// St returns the storage engine instance for this database.
	St() *watchable.Store

	// App returns the app handle for this database.
	App() App

	// Collection returns the Collection with the specified name.
	Collection(ctx *context.T, collectionName string) Collection

	// CheckPermsInternal checks whether the given RPC (ctx, call) is allowed per
	// the database perms.
	// Designed for use from within App.DestroyDatabase.
	CheckPermsInternal(ctx *context.T, call rpc.ServerCall, st store.StoreReader) error

	// SetPermsInternal updates the database perms.
	// Designed for use from within App.SetDatabasePerms.
	SetPermsInternal(ctx *context.T, call rpc.ServerCall, perms access.Permissions, version string) error

	// Name returns the name of this database.
	Name() string

	// GetSchemaMetadataInternal returns SchemaMetadata stored for this db
	// without checking any credentials.
	GetSchemaMetadataInternal(ctx *context.T) (*wire.SchemaMetadata, error)

	// CrConnectionStream returns the current conflict resolution stream
	// established between an app and this database.
	CrConnectionStream() wire.ConflictManagerStartConflictResolverServerStream

	// ResetCrConnectionStream resets the current conflict resolution stream.
	// This can be used to either close an active stream or to remove a dead
	// stream.
	// Note: Resetting a stream does not reconnect the stream. Its upto the
	// client to reconnect.
	ResetCrConnectionStream()
}
