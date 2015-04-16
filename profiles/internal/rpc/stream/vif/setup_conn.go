// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vif

import (
	"io"

	"v.io/v23/verror"

	"v.io/x/ref/profiles/internal/lib/iobuf"
	"v.io/x/ref/profiles/internal/rpc/stream"
	"v.io/x/ref/profiles/internal/rpc/stream/crypto"
	"v.io/x/ref/profiles/internal/rpc/stream/message"
)

// setupConn writes the data to the net.Conn using SetupStream messages.
type setupConn struct {
	writer  io.Writer
	reader  *iobuf.Reader
	cipher  crypto.ControlCipher
	rbuffer []byte // read buffer
}

var _ io.ReadWriteCloser = (*setupConn)(nil)

const maxFrameSize = 8192

func newSetupConn(writer io.Writer, reader *iobuf.Reader, c crypto.ControlCipher) *setupConn {
	return &setupConn{writer: writer, reader: reader, cipher: c}
}

// Read implements the method from net.Conn.
func (s *setupConn) Read(buf []byte) (int, error) {
	for len(s.rbuffer) == 0 {
		msg, err := message.ReadFrom(s.reader, s.cipher)
		if err != nil {
			return 0, err
		}
		emsg, ok := msg.(*message.SetupStream)
		if !ok {
			return 0, verror.New(stream.ErrSecurity, nil, verror.New(errVersionNegotiationFailed, nil))
		}
		s.rbuffer = emsg.Data
	}
	n := copy(buf, s.rbuffer)
	s.rbuffer = s.rbuffer[n:]
	return n, nil
}

// Write implements the method from net.Conn.
func (s *setupConn) Write(buf []byte) (int, error) {
	amount := 0
	for len(buf) > 0 {
		n := len(buf)
		if n > maxFrameSize {
			n = maxFrameSize
		}
		emsg := message.SetupStream{Data: buf[:n]}
		if err := message.WriteTo(s.writer, &emsg, s.cipher); err != nil {
			return 0, err
		}
		buf = buf[n:]
		amount += n
	}
	return amount, nil
}

// Close does nothing.
func (s *setupConn) Close() error { return nil }
