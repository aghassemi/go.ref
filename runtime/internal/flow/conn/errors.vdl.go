// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Source: errors.vdl

package conn

import (
	// VDL system imports
	"v.io/v23/context"
	"v.io/v23/i18n"
	"v.io/v23/verror"
)

var (
	ErrInvalidMsg        = verror.Register("v.io/x/ref/runtime/internal/flow/conn.InvalidMsg", verror.NoRetry, "{1:}{2:} message of type{:3} and size{:4} failed decoding at field{:5}.")
	ErrInvalidControlMsg = verror.Register("v.io/x/ref/runtime/internal/flow/conn.InvalidControlMsg", verror.NoRetry, "{1:}{2:} control message of cmd{:3} and size{:4} failed decoding at field{:5}.")
	ErrUnknownMsg        = verror.Register("v.io/x/ref/runtime/internal/flow/conn.UnknownMsg", verror.NoRetry, "{1:}{2:} unknown message type{:3}.")
	ErrUnknownControlMsg = verror.Register("v.io/x/ref/runtime/internal/flow/conn.UnknownControlMsg", verror.NoRetry, "{1:}{2:} unknown control command{:3}.")
	ErrUnexpectedMsg     = verror.Register("v.io/x/ref/runtime/internal/flow/conn.UnexpectedMsg", verror.NoRetry, "{1:}{2:} unexpected message type{:3}.")
	ErrConnectionClosed  = verror.Register("v.io/x/ref/runtime/internal/flow/conn.ConnectionClosed", verror.NoRetry, "{1:}{2:} connection closed.")
	ErrSend              = verror.Register("v.io/x/ref/runtime/internal/flow/conn.Send", verror.NoRetry, "{1:}{2:} failure sending {3} message to {4}{:5}.")
	ErrRecv              = verror.Register("v.io/x/ref/runtime/internal/flow/conn.Recv", verror.NoRetry, "{1:}{2:} error reading from {3}{:4}")
	ErrCacheClosed       = verror.Register("v.io/x/ref/runtime/internal/flow/conn.CacheClosed", verror.NoRetry, "{1:}{2:} cache is closed")
	ErrCounterOverflow   = verror.Register("v.io/x/ref/runtime/internal/flow/conn.CounterOverflow", verror.NoRetry, "{1:}{2:} A remote process has sent more data than allowed.")
)

func init() {
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrInvalidMsg.ID), "{1:}{2:} message of type{:3} and size{:4} failed decoding at field{:5}.")
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrInvalidControlMsg.ID), "{1:}{2:} control message of cmd{:3} and size{:4} failed decoding at field{:5}.")
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrUnknownMsg.ID), "{1:}{2:} unknown message type{:3}.")
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrUnknownControlMsg.ID), "{1:}{2:} unknown control command{:3}.")
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrUnexpectedMsg.ID), "{1:}{2:} unexpected message type{:3}.")
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrConnectionClosed.ID), "{1:}{2:} connection closed.")
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrSend.ID), "{1:}{2:} failure sending {3} message to {4}{:5}.")
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrRecv.ID), "{1:}{2:} error reading from {3}{:4}")
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrCacheClosed.ID), "{1:}{2:} cache is closed")
	i18n.Cat().SetWithBase(i18n.LangID("en"), i18n.MsgID(ErrCounterOverflow.ID), "{1:}{2:} A remote process has sent more data than allowed.")
}

// NewErrInvalidMsg returns an error with the ErrInvalidMsg ID.
func NewErrInvalidMsg(ctx *context.T, typ byte, size int64, field int64) error {
	return verror.New(ErrInvalidMsg, ctx, typ, size, field)
}

// NewErrInvalidControlMsg returns an error with the ErrInvalidControlMsg ID.
func NewErrInvalidControlMsg(ctx *context.T, cmd byte, size int64, field int64) error {
	return verror.New(ErrInvalidControlMsg, ctx, cmd, size, field)
}

// NewErrUnknownMsg returns an error with the ErrUnknownMsg ID.
func NewErrUnknownMsg(ctx *context.T, typ byte) error {
	return verror.New(ErrUnknownMsg, ctx, typ)
}

// NewErrUnknownControlMsg returns an error with the ErrUnknownControlMsg ID.
func NewErrUnknownControlMsg(ctx *context.T, cmd byte) error {
	return verror.New(ErrUnknownControlMsg, ctx, cmd)
}

// NewErrUnexpectedMsg returns an error with the ErrUnexpectedMsg ID.
func NewErrUnexpectedMsg(ctx *context.T, typ string) error {
	return verror.New(ErrUnexpectedMsg, ctx, typ)
}

// NewErrConnectionClosed returns an error with the ErrConnectionClosed ID.
func NewErrConnectionClosed(ctx *context.T) error {
	return verror.New(ErrConnectionClosed, ctx)
}

// NewErrSend returns an error with the ErrSend ID.
func NewErrSend(ctx *context.T, typ string, dest string, err error) error {
	return verror.New(ErrSend, ctx, typ, dest, err)
}

// NewErrRecv returns an error with the ErrRecv ID.
func NewErrRecv(ctx *context.T, src string, err error) error {
	return verror.New(ErrRecv, ctx, src, err)
}

// NewErrCacheClosed returns an error with the ErrCacheClosed ID.
func NewErrCacheClosed(ctx *context.T) error {
	return verror.New(ErrCacheClosed, ctx)
}

// NewErrCounterOverflow returns an error with the ErrCounterOverflow ID.
func NewErrCounterOverflow(ctx *context.T) error {
	return verror.New(ErrCounterOverflow, ctx)
}