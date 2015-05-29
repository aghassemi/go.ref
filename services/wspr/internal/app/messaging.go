// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"bytes"
	"fmt"
	"path/filepath"
	"runtime"

	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/v23/vtrace"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/wspr/internal/lib"
)

const (
	verrorPkgPath = "v.io/x/ref/services/wspr/internal/app"
)

var (
	errUnknownMessageType = verror.Register(verrorPkgPath+".unkownMessage", verror.NoRetry, "{1} {2} Unknown message type {_}")
)

// Incoming message from the javascript client to WSPR.
type MessageType int32

const (
	// Making a vanadium client request, streaming or otherwise.
	VeyronRequestMessage MessageType = 0

	// Serving this  under an object name.
	ServeMessage = 1

	// A response from a service in javascript to a request.
	// from the proxy.
	ServerResponseMessage = 2

	// Sending streaming data, either from a JS client or JS service.
	StreamingValueMessage = 3

	// A response that means the stream is closed by the client.
	StreamCloseMessage = 4

	// A request to get signature of a remote server.
	SignatureRequestMessage = 5

	// A request to stop a server.
	StopServerMessage = 6

	// A request to unlink blessings.  This request means that
	// we can remove the given handle from the handle store.
	UnlinkBlessingsMessage = 8

	// A request to run the lookup function on a dispatcher.
	LookupResponseMessage = 11

	// A request to run the authorizer for an rpc.
	AuthResponseMessage = 12

	// A request to run a namespace client method.
	NamespaceRequestMessage = 13

	// A request to cancel an rpc initiated by the JS.
	CancelMessage = 17

	// A request to add a new name to server.
	AddName = 18

	// A request to remove a name from server.
	RemoveName = 19

	// A request to get the remote blessings of a server.
	RemoteBlessings = 20

	// A response to a caveat validation request.
	CaveatValidationResponse = 21

	// A response to a granter request.
	GranterResponseMessage = 22

	TypeMessage = 23
)

type Message struct {
	// TODO(bprosnitz) Consider changing this ID to a larger value.
	// TODO(bprosnitz) Consider making the ID have positive / negative value
	// depending on whether from/to JS.
	Id int32
	// This contains the json encoded payload.
	Data string

	// Whether it is an rpc request or a serve request.
	Type MessageType
}

// HandleIncomingMessage handles most incoming messages from JS and calls the appropriate handler.
func (c *Controller) HandleIncomingMessage(msg Message, w lib.ClientWriter) {
	// TODO(mattr): Get the proper context information from javascript.
	ctx, _ := vtrace.WithNewTrace(c.Context())

	switch msg.Type {
	case VeyronRequestMessage:
		c.HandleVeyronRequest(ctx, msg.Id, msg.Data, w)
	case CancelMessage:
		go c.HandleVeyronCancellation(msg.Id)
	case StreamingValueMessage:
		// SendOnStream queues up the message to be sent, but doesn't do the send
		// on this goroutine.  We need to queue the messages synchronously so that
		// the order is preserved.
		c.SendOnStream(msg.Id, msg.Data, w)
	case StreamCloseMessage:
		c.CloseStream(msg.Id)

	case ServerResponseMessage:
		go c.HandleServerResponse(msg.Id, msg.Data)
	case LookupResponseMessage:
		go c.HandleLookupResponse(msg.Id, msg.Data)
	case AuthResponseMessage:
		go c.HandleAuthResponse(msg.Id, msg.Data)
	case CaveatValidationResponse:
		go c.HandleCaveatValidationResponse(msg.Id, msg.Data)
	case GranterResponseMessage:
		go c.HandleGranterResponse(msg.Id, msg.Data)
	case TypeMessage:
		// These messages need to be handled in order so they are done in line.
		c.HandleTypeMessage(msg.Data)
	default:
		w.Error(verror.New(errUnknownMessageType, ctx, msg.Type))
	}
}

// ConstructOutgoingMessage constructs a message to send to javascript in a consistent format.
func ConstructOutgoingMessage(messageId int32, messageType lib.ResponseType, data interface{}) (string, error) {
	var buf bytes.Buffer
	if _, err := buf.Write(lib.BinaryEncodeUint(uint64(messageId))); err != nil {
		return "", err
	}
	if _, err := buf.Write(lib.BinaryEncodeUint(uint64(messageType))); err != nil {
		return "", err
	}
	enc := vom.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", buf.Bytes()), nil
}

// FormatAsVerror formats an error as a verror.
// This also logs the error.
func FormatAsVerror(err error) error {
	verr := verror.Convert(verror.ErrUnknown, nil, err)

	// Also log the error but write internal errors at a more severe log level
	var logLevel vlog.Level = 2
	logErr := fmt.Sprintf("%v", verr)

	// Prefix the message with the code locations associated with verr,
	// except the last, which is the Convert() above.  This does nothing if
	// err was not a verror error.
	verrStack := verror.Stack(verr)
	for i := 0; i < len(verrStack)-1; i++ {
		pc := verrStack[i]
		fnc := runtime.FuncForPC(pc)
		file, line := fnc.FileLine(pc)
		logErr = fmt.Sprintf("%s:%d: %s", file, line, logErr)
	}

	// We want to look at the stack three frames up to find where the error actually
	// occurred.  (caller -> websocketErrorResponse/sendError -> generateErrorMessage).
	if _, file, line, ok := runtime.Caller(3); ok {
		logErr = fmt.Sprintf("%s:%d: %s", filepath.Base(file), line, logErr)
	}
	if verror.ErrorID(verr) == verror.ErrInternal.ID {
		logLevel = 2
	}
	vlog.VI(logLevel).Info(logErr)

	return verr
}
