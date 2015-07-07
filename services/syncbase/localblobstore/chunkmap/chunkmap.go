// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package chunkmap implements a map from chunk checksums to chunk locations
// and vice versa, using a store.Store (currently, one implemented with
// leveldb).
package chunkmap

import "encoding/binary"

import "v.io/syncbase/x/ref/services/syncbase/store"
import "v.io/syncbase/x/ref/services/syncbase/store/leveldb"
import "v.io/v23/context"
import "v.io/v23/verror"

const pkgPath = "v.io/syncbase/x/ref/services/syncbase/localblobstore/chunkmap"

var (
	errBadBlobIDLen        = verror.Register(pkgPath+".errBadBlobIDLen", verror.NoRetry, "{1:}{2:} chunkmap {3}: bad blob length {4} should be {5}{:_}")
	errBadChunkHashLen     = verror.Register(pkgPath+".errBadChunkHashLen", verror.NoRetry, "{1:}{2:} chunkmap {3}: bad chunk hash length {4} should be {5}{:_}")
	errNoSuchBlob          = verror.Register(pkgPath+".errNoSuchBlob", verror.NoRetry, "{1:}{2:} chunkmap {3}: no such blob{:_}")
	errMalformedChunkEntry = verror.Register(pkgPath+".errMalformedChunkEntry", verror.NoRetry, "{1:}{2:} chunkmap {3}: malfored chunk entry{:_}")
	errNoSuchChunk         = verror.Register(pkgPath+".errNoSuchChunk", verror.NoRetry, "{1:}{2:} chunkmap {3}: no such chunk{:_}")
	errMalformedBlobEntry  = verror.Register(pkgPath+".errMalformedBlobEntry", verror.NoRetry, "{1:}{2:} chunkmap {3}: malfored blob entry{:_}")
)

// There are two tables: chunk-to-location, and blob-to-chunk.
// Each chunk is represented by one entry in each table.
// On deletion, the latter is used to find the former, so the latter is added
// first, and deleted last.
//
// chunk-to-location:
//    Key:    1-byte containing chunkPrefix, 16-byte chunk hash, 16-byte blob ID
//    Value:  Varint offset, Varint length.
// The chunk with the specified 16-byte hash had the specified length, and is
// (or was) found at the specified offset in the blob.
//
// blob-to-chunk:
//    Key:    1-byte containing blobPrefix, 16-byte blob ID, 8-byte bigendian offset
//    Value:  16-byte chunk hash, Varint length.
//
// The varint encoded fields are written/read with
// encoding/binary.{Put,Read}Varint.  The blob-to-chunk keys encode the offset
// as raw big-endian (encoding/binary.{Put,}Uint64) so that it will sort in
// increasing offset order.

const chunkHashLen = 16 // length of chunk hash
const blobIDLen = 16    // length of blob ID
const offsetLen = 8     // length of offset in blob-to-chunk key

const maxKeyLen = 64 // conservative maximum key length
const maxValLen = 64 // conservative maximum value length

var chunkPrefix []byte = []byte{0} // key prefix for chunk-to-location
var blobPrefix []byte = []byte{1}  // key prefix for blob-to-chunk

// offsetLimit is an offset that's greater than, and one byte longer than, any
// real offset.
var offsetLimit []byte = []byte{
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff,
}

// blobLimit is a blobID that's greater than, and one byte longer than, any
// real blob ID
var blobLimit []byte = []byte{
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
	0xff,
}

// A Location describes chunk's location within a blob.
type Location struct {
	Blob   []byte // ID of blob
	Offset int64  // byte offset of chunk within blob
	Size   int64  // size of chunk
}

// A ChunkMap maps chunk checksums to Locations, and vice versa.
type ChunkMap struct {
	dir string      // the directory where the store is held
	st  store.Store // private store that holds the mapping.
}

// New() returns a pointer to a ChunkMap, backed by storage in directory dir.
func New(ctx *context.T, dir string) (cm *ChunkMap, err error) {
	cm = new(ChunkMap)
	cm.dir = dir
	cm.st, err = leveldb.Open(dir, leveldb.OpenOptions{CreateIfMissing: true, ErrorIfExists: false})
	return cm, err
}

// Close() closes any files or other resources associated with *cm.
// No other methods on cm may be called after Close().
func (cm *ChunkMap) Close() error {
	return cm.st.Close()
}

// AssociateChunkWithLocation() remembers that the specified chunk hash is
// associated with the specified Location.
func (cm *ChunkMap) AssociateChunkWithLocation(ctx *context.T, chunk []byte, loc Location) (err error) {
	// Check of expected lengths explicitly in routines that modify the database.
	if len(loc.Blob) != blobIDLen {
		err = verror.New(errBadBlobIDLen, ctx, cm.dir, len(loc.Blob), blobIDLen)
	} else if len(chunk) != chunkHashLen {
		err = verror.New(errBadChunkHashLen, ctx, cm.dir, len(chunk), chunkHashLen)
	} else {
		var key [maxKeyLen]byte
		var val [maxValLen]byte

		// Put the blob-to-chunk entry first, since it's used
		// to garbage collect the other.
		keyLen := copy(key[:], blobPrefix)
		keyLen += copy(key[keyLen:], loc.Blob)
		binary.BigEndian.PutUint64(key[keyLen:], uint64(loc.Offset))
		keyLen += offsetLen

		valLen := copy(val[:], chunk)
		valLen += binary.PutVarint(val[valLen:], loc.Size)
		err = cm.st.Put(key[:keyLen], val[:valLen])

		if err == nil {
			keyLen = copy(key[:], chunkPrefix)
			keyLen += copy(key[keyLen:], chunk)
			keyLen += copy(key[keyLen:], loc.Blob)

			valLen = binary.PutVarint(val[:], loc.Offset)
			valLen += binary.PutVarint(val[valLen:], loc.Size)

			err = cm.st.Put(key[:keyLen], val[:valLen])
		}
	}

	return err
}

// DeleteBlob() deletes any of the chunk associations previously added with
// AssociateChunkWithLocation(..., chunk, ...).
func (cm *ChunkMap) DeleteBlob(ctx *context.T, blob []byte) (err error) {
	// Check of expected lengths explicitly in routines that modify the database.
	if len(blob) != blobIDLen {
		err = verror.New(errBadBlobIDLen, ctx, cm.dir, len(blob), blobIDLen)
	} else {
		var start [maxKeyLen]byte
		var limit [maxKeyLen]byte

		startLen := copy(start[:], blobPrefix)
		startLen += copy(start[startLen:], blob)

		limitLen := copy(limit[:], start[:startLen])
		limitLen += copy(limit[limitLen:], offsetLimit)

		var keyBuf [maxKeyLen]byte    // buffer for keys returned by stream
		var valBuf [maxValLen]byte    // buffer for values returned by stream
		var deleteKey [maxKeyLen]byte // buffer to construct chunk-to-location keys to delete

		deletePrefixLen := copy(deleteKey[:], chunkPrefix)

		seenAValue := false

		s := cm.st.Scan(start[:startLen], limit[:limitLen])
		for s.Advance() && err == nil {
			seenAValue = true

			key := s.Key(keyBuf[:])
			value := s.Value(valBuf[:])

			if len(value) >= chunkHashLen {
				deleteKeyLen := deletePrefixLen
				deleteKeyLen += copy(deleteKey[deleteKeyLen:], value[:chunkHashLen])
				deleteKeyLen += copy(deleteKey[deleteKeyLen:], blob)
				err = cm.st.Delete(deleteKey[:deleteKeyLen])
			}

			if err == nil {
				// Delete the blob-to-chunk entry last, as it's
				// used to find the chunk-to-location entry.
				err = cm.st.Delete(key)
			}
		}

		if err != nil {
			s.Cancel()
		} else {
			err = s.Err()
			if err == nil && !seenAValue {
				err = verror.New(errNoSuchBlob, ctx, cm.dir, blob)
			}
		}
	}

	return err
}

// LookupChunk() returns a Location for the specified chunk.  Only one Location
// is returned, even if several are available in the database.  If the client
// finds that the Location is not available, perhaps because its blob has
// been deleted, the client should remove the blob from the ChunkMap using
// DeleteBlob(loc.Blob), and try again.  (The client may also wish to
// arrange at some point to call GC() on the blob store.)
func (cm *ChunkMap) LookupChunk(ctx *context.T, chunk []byte) (loc Location, err error) {
	var start [maxKeyLen]byte
	var limit [maxKeyLen]byte

	startLen := copy(start[:], chunkPrefix)
	startLen += copy(start[startLen:], chunk)

	limitLen := copy(limit[:], start[:startLen])
	limitLen += copy(limit[limitLen:], blobLimit)

	var keyBuf [maxKeyLen]byte // buffer for keys returned by stream
	var valBuf [maxValLen]byte // buffer for values returned by stream

	s := cm.st.Scan(start[:startLen], limit[:limitLen])
	if s.Advance() {
		var n int
		key := s.Key(keyBuf[:])
		value := s.Value(valBuf[:])
		loc.Blob = key[len(chunkPrefix)+chunkHashLen:]
		loc.Offset, n = binary.Varint(value)
		if n > 0 {
			loc.Size, n = binary.Varint(value[n:])
		}
		if n <= 0 {
			err = verror.New(errMalformedChunkEntry, ctx, cm.dir, chunk, key, value)
		}
		s.Cancel()
	} else {
		if err == nil {
			err = s.Err()
		}
		if err == nil {
			err = verror.New(errNoSuchChunk, ctx, cm.dir, chunk)
		}
	}

	return loc, err
}

// A BlobStream allows the client to iterate over the chunks in a blob:
//	bs := cm.NewBlobStream(ctx, blob)
//	for bs.Advance() {
//		chunkHash := bs.Value()
//		...process chunkHash...
//	}
//	if bs.Err() != nil {
//		...there was an error...
//	}
type BlobStream struct {
	cm     *ChunkMap
	ctx    *context.T
	stream store.Stream

	keyBuf [maxKeyLen]byte // buffer for keys
	valBuf [maxValLen]byte // buffer for values
	key    []byte          // key for current element
	value  []byte          // value of current element
	loc    Location        // location of current element
	err    error           // error encountered.
	more   bool            // whether stream may be consulted again
}

// NewBlobStream() returns a pointer to a new BlobStream that allows the client
// to enumerate the chunk hashes in a blob, in order.
func (cm *ChunkMap) NewBlobStream(ctx *context.T, blob []byte) *BlobStream {
	var start [maxKeyLen]byte
	var limit [maxKeyLen]byte

	startLen := copy(start[:], blobPrefix)
	startLen += copy(start[startLen:], blob)

	limitLen := copy(limit[:], start[:startLen])
	limitLen += copy(limit[limitLen:], offsetLimit)

	bs := new(BlobStream)
	bs.cm = cm
	bs.ctx = ctx
	bs.stream = cm.st.Scan(start[:startLen], limit[:limitLen])
	bs.more = true

	return bs
}

// Advance() stages an element so the client can retrieve the chunk hash with
// Value(), or its Location with Location().  Advance() returns true iff there
// is an element to retrieve.  The client must call Advance() before calling
// Value() or Location() The client must call Cancel if it does not iterate
// through all elements (i.e. until Advance() returns false).  Advance() may
// block if an element is not immediately available.
func (bs *BlobStream) Advance() (ok bool) {
	if bs.more && bs.err == nil {
		if !bs.stream.Advance() {
			bs.err = bs.stream.Err()
			bs.more = false // no more stream, even if no error
		} else {
			bs.key = bs.stream.Key(bs.keyBuf[:])
			bs.value = bs.stream.Value(bs.valBuf[:])
			ok = (len(bs.value) >= chunkHashLen) &&
				(len(bs.key) == len(blobPrefix)+blobIDLen+offsetLen)
			if ok {
				var n int
				bs.loc.Blob = make([]byte, blobIDLen)
				copy(bs.loc.Blob, bs.key[len(blobPrefix):len(blobPrefix)+blobIDLen])
				bs.loc.Offset = int64(binary.BigEndian.Uint64(bs.key[len(blobPrefix)+blobIDLen:]))
				bs.loc.Size, n = binary.Varint(bs.value[chunkHashLen:])
				ok = (n > 0)
			}
			if !ok {
				bs.err = verror.New(errMalformedBlobEntry, bs.ctx, bs.cm.dir, bs.key, bs.value)
				bs.stream.Cancel()
			}
		}
	}
	return ok
}

// Value() returns the content hash of the chunk staged by
// Advance().  The returned slice may be a sub-slice of buf if buf is large
// enough to hold the entire value.  Otherwise, a newly allocated slice will be
// returned.  It is valid to pass a nil buf.  Value() may panic if Advance()
// returned false or was not called at all.  Value() does not block.
func (bs *BlobStream) Value(buf []byte) (result []byte) {
	if len(buf) < chunkHashLen {
		buf = make([]byte, chunkHashLen)
	}
	copy(buf, bs.value[:chunkHashLen])
	return buf[:chunkHashLen]
}

// Location() returns the Location associated with the chunk staged by
// Advance().  Location() may panic if Advance() returned false or was not
// called at all.  Location() does not block.
func (bs *BlobStream) Location() Location {
	return bs.loc
}

// Err() returns a non-nil error iff the stream encountered any errors.  Err()
// does not block.
func (bs *BlobStream) Err() error {
	return bs.err
}

// Cancel() notifies the stream provider that it can stop producing elements.
// The client must call Cancel() if it does not iterate through all elements
// (i.e. until Advance() returns false).  Cancel() is idempotent and can be
// called concurrently with a goroutine that is iterating via Advance() and
// Value().  Cancel() causes Advance() to subsequently return false.
// Cancel() does not block.
func (bs *BlobStream) Cancel() {
	bs.stream.Cancel()
}
