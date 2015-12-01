// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements the Signpost part of the localblobstore interface.
// It passes the calls through to blobmap.

package fs_cablobstore

import "v.io/v23/context"
import "v.io/v23/services/syncbase/nosql"
import "v.io/x/ref/services/syncbase/localblobstore"
import "v.io/x/ref/services/syncbase/server/interfaces"

// SetSignpost() sets the Signpost associated with a blob to *sp.
func (fscabs *FsCaBlobStore) SetSignpost(ctx *context.T, blobID nosql.BlobRef, sp *interfaces.Signpost) error {
	return fscabs.bm.SetSignpost(ctx, blobID, sp)
}

// GetSignpost() yields in *sp the Signpost associated with a blob.
func (fscabs *FsCaBlobStore) GetSignpost(ctx *context.T, blobID nosql.BlobRef, sp *interfaces.Signpost) error {
	return fscabs.bm.GetSignpost(ctx, blobID, sp)
}

// DeleteSignpost() deletes the Signpost for the specified blob.
func (fscabs *FsCaBlobStore) DeleteSignpost(ctx *context.T, blobID nosql.BlobRef) error {
	return fscabs.bm.DeleteSignpost(ctx, blobID)
}

// NewSignpostStream() returns a pointer to a SignpostStream
// that allows the client to iterate over each blob for which a Signpost
// has been specified.
func (fscabs *FsCaBlobStore) NewSignpostStream(ctx *context.T) localblobstore.SignpostStream {
	return fscabs.bm.NewSignpostStream(ctx)
}
