// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	"strconv"
	"strings"

	wire "v.io/syncbase/v23/services/syncbase"
	nosqlWire "v.io/syncbase/v23/services/syncbase/nosql"
	pubutil "v.io/syncbase/v23/syncbase/util"
	"v.io/syncbase/x/ref/services/syncbase/server/interfaces"
	"v.io/syncbase/x/ref/services/syncbase/server/util"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
)

type dispatcher struct {
	a interfaces.App
}

var _ rpc.Dispatcher = (*dispatcher)(nil)

func NewDispatcher(a interfaces.App) *dispatcher {
	return &dispatcher{a: a}
}

func (disp *dispatcher) Lookup(suffix string) (interface{}, security.Authorizer, error) {
	suffix = strings.TrimPrefix(suffix, "/")
	parts := strings.Split(suffix, "/")

	if len(parts) == 0 {
		vlog.Fatal("invalid nosql.dispatcher Lookup")
	}

	dParts := strings.Split(parts[0], util.BatchSep)
	dName := dParts[0]

	// Validate all key atoms up front, so that we can avoid doing so in all our
	// method implementations.
	if !pubutil.ValidName(dName) {
		return nil, nil, wire.NewErrInvalidName(nil, suffix)
	}
	for _, s := range parts[1:] {
		if !pubutil.ValidName(s) {
			return nil, nil, wire.NewErrInvalidName(nil, suffix)
		}
	}

	dExists := false
	var d *database
	if dInt, err := disp.a.NoSQLDatabase(nil, nil, dName); err == nil {
		d = dInt.(*database) // panics on failure, as desired
		dExists = true
	} else {
		if verror.ErrorID(err) != verror.ErrNoExist.ID {
			return nil, nil, err
		} else {
			// Database does not exist. Create a short-lived database object to
			// service this request.
			d = &database{
				name: dName,
				a:    disp.a,
			}
		}
	}

	dReq := &databaseReq{database: d}
	if !setBatchFields(dReq, dParts) {
		return nil, nil, wire.NewErrInvalidName(nil, suffix)
	}
	if len(parts) == 1 {
		return nosqlWire.DatabaseServer(dReq), nil, nil
	}

	// All table and row methods require the database to exist. If it doesn't,
	// abort early.
	if !dExists {
		return nil, nil, verror.New(verror.ErrNoExist, nil, d.name)
	}

	// Note, it's possible for the database to be deleted concurrently with
	// downstream handling of this request. Depending on the order in which things
	// execute, the client may not get an error, but in any case ultimately the
	// store will end up in a consistent state.
	tReq := &tableReq{
		name: parts[1],
		d:    dReq,
	}
	if len(parts) == 2 {
		return nosqlWire.TableServer(tReq), nil, nil
	}

	rReq := &rowReq{
		key: parts[2],
		t:   tReq,
	}
	if len(parts) == 3 {
		return nosqlWire.RowServer(rReq), nil, nil
	}

	return nil, nil, verror.NewErrNoExist(nil)
}

// setBatchFields sets the batch-related fields in databaseReq based on the
// value of dParts, the parts of the database name component. It returns false
// if dParts is malformed.
func setBatchFields(d *databaseReq, dParts []string) bool {
	if len(dParts) == 1 {
		return true
	}
	if len(dParts) != 3 {
		return false
	}
	batchId, err := strconv.ParseUint(dParts[2], 0, 64)
	if err != nil {
		return false
	}
	d.batchId = &batchId
	d.mu.Lock()
	defer d.mu.Unlock()
	var ok bool
	switch dParts[1] {
	case "sn":
		d.sn, ok = d.sns[batchId]
	case "tx":
		d.tx, ok = d.txs[batchId]
	default:
		return false
	}
	return ok
}
