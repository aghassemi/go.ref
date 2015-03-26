// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package impl implements the LogFile interface from
// v.io/v23/services/mgmt/logreader, which can be used to allow remote access to
// log files, and the Globbable interface from v.io/v23/services/mounttable to
// find the files in a logs directory.
package impl

import (
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"v.io/v23/rpc"
	"v.io/v23/services/mgmt/logreader"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
)

const pkgPath = "v.io/x/ref/services/mgmt/logreader/impl"

var (
	errOperationFailed = verror.Register(pkgPath+".errOperationFailed", verror.NoRetry, "{1:}{2:} operation failed{:_}")
)

// NewLogFileService returns a new log file server.
func NewLogFileService(root, suffix string) interface{} {
	return logreader.LogFileServer(&logfileService{filepath.Clean(root), suffix})
}

// translateNameToFilename returns the file name that corresponds to the object
// name.
func translateNameToFilename(root, name string) (string, error) {
	name = filepath.Join(strings.Split(name, "/")...)
	p := filepath.Join(root, name)
	// Make sure we're not asked to read a file outside of the root
	// directory. This could happen if suffix contains "../", which get
	// collapsed by filepath.Join().
	if !strings.HasPrefix(p, root) {
		return "", verror.New(errOperationFailed, nil, name)
	}
	return p, nil
}

// logfileService holds the state of a logfile invocation.
type logfileService struct {
	// root is the root directory from which the object names are based.
	root string
	// suffix is the suffix of the current invocation that is assumed to
	// be used as a relative object name to identify a log file.
	suffix string
}

// Size returns the size of the log file, in bytes.
func (i *logfileService) Size(call rpc.ServerCall) (int64, error) {
	vlog.VI(1).Infof("%v.Size()", i.suffix)
	fname, err := translateNameToFilename(i.root, i.suffix)
	if err != nil {
		return 0, err
	}
	fi, err := os.Stat(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, verror.New(verror.ErrNoExist, call.Context(), fname)
		}
		vlog.Errorf("Stat(%v) failed: %v", fname, err)
		return 0, verror.New(errOperationFailed, call.Context(), fname)
	}
	if fi.IsDir() {
		return 0, verror.New(errOperationFailed, call.Context(), fname)
	}
	return fi.Size(), nil
}

// ReadLog returns log entries from the log file.
func (i *logfileService) ReadLog(call logreader.LogFileReadLogServerCall, startpos int64, numEntries int32, follow bool) (int64, error) {
	vlog.VI(1).Infof("%v.ReadLog(%v, %v, %v)", i.suffix, startpos, numEntries, follow)
	fname, err := translateNameToFilename(i.root, i.suffix)
	if err != nil {
		return 0, err
	}
	f, err := os.Open(fname)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, verror.New(verror.ErrNoExist, call.Context(), fname)
		}
		return 0, verror.New(errOperationFailed, call.Context(), fname)
	}
	reader := newFollowReader(call, f, startpos, follow)
	if numEntries == logreader.AllEntries {
		numEntries = int32(math.MaxInt32)
	}
	for n := int32(0); n < numEntries; n++ {
		line, offset, err := reader.readLine()
		if err == io.EOF && n > 0 {
			return reader.tell(), nil
		}
		if err == io.EOF {
			return reader.tell(), verror.NewErrEndOfFile(call.Context())
		}
		if err != nil {
			return reader.tell(), verror.New(errOperationFailed, call.Context(), fname)
		}
		if err := call.SendStream().Send(logreader.LogEntry{Position: offset, Line: line}); err != nil {
			return reader.tell(), err
		}
	}
	return reader.tell(), nil
}

// GlobChildren__ returns the list of files in a directory streamed on a
// channel. The list is empty if the object is a file.
func (i *logfileService) GlobChildren__(call rpc.ServerCall) (<-chan string, error) {
	vlog.VI(1).Infof("%v.GlobChildren__()", i.suffix)
	dirName, err := translateNameToFilename(i.root, i.suffix)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(dirName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, verror.New(verror.ErrNoExist, call.Context(), dirName)
		}
		return nil, verror.New(errOperationFailed, call.Context(), dirName)
	}
	if !stat.IsDir() {
		return nil, nil
	}

	ch := make(chan string)
	go func() {
		defer close(ch)
		f, err := os.Open(dirName)
		if err != nil {
			return
		}
		defer f.Close()
		for {
			fi, err := f.Readdir(100)
			if err != nil {
				return
			}
			for _, file := range fi {
				ch <- file.Name()
			}
		}
	}()
	return ch, nil
}
