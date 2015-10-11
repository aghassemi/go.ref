// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"time"

	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/flow/message"
	"v.io/v23/security"
	"v.io/v23/verror"
)

type flw struct {
	// These variables are all set during flow construction.
	id         uint64
	dialed     bool
	conn       *Conn
	q          *readq
	bkey, dkey uint64
	noEncrypt  bool
	writeCh    chan struct{}

	// These variables can only be modified by SetDeadlineContext which cannot
	// be called concurrently with other methods on the flow.  Therefore they
	// are not mutex protected.
	ctx    *context.T
	cancel context.CancelFunc

	// NOTE: The remaining variables are actually protected by conn.mu.

	// opened indicates whether the flow has already been opened.  If false
	// we need to send an open flow on the next write.  For accepted flows
	// this will always be true.
	opened bool
	// writing is true if we're in the middle of a write to this flow.
	writing bool
	// released counts tokens already released by the remote end, that is, the number
	// of tokens we are allowed to send.
	released uint64
	// borrowed indicates the number of tokens we have borrowed from the shared pool for
	// sending on newly dialed flows.
	borrowed uint64
	// borrowing indicates whether this flow is using borrowed counters for a newly
	// dialed flow.  This will be set to false after we first receive a
	// release from the remote end.  This is always false for accepted flows.
	borrowing bool

	writerList
}

// Ensure that *flw implements flow.Flow.
var _ flow.Flow = &flw{}

func (c *Conn) newFlowLocked(ctx *context.T, id uint64, bkey, dkey uint64, dialed, preopen bool) *flw {
	f := &flw{
		id:        id,
		dialed:    dialed,
		conn:      c,
		q:         newReadQ(c, id),
		bkey:      bkey,
		dkey:      dkey,
		opened:    preopen,
		borrowing: dialed,
		// It's important that this channel has a non-zero buffer.  Sometimes this
		// flow will be notifying itself, so if there's no buffer a deadlock will
		// occur.
		writeCh: make(chan struct{}, 1),
	}
	f.next, f.prev = f, f
	f.ctx, f.cancel = context.WithCancel(ctx)
	if !f.opened {
		c.unopenedFlows.Add(1)
	}
	c.flows[id] = f
	return f
}

// Implement the writer interface.
func (f *flw) notify()       { f.writeCh <- struct{}{} }
func (f *flw) priority() int { return flowPriority }

// disableEncrytion should not be called concurrently with Write* methods.
func (f *flw) disableEncryption() {
	f.noEncrypt = false
}

// Implement io.Reader.
// Read and ReadMsg should not be called concurrently with themselves
// or each other.
func (f *flw) Read(p []byte) (n int, err error) {
	if err = f.checkBlessings(); err != nil {
		return
	}
	f.markUsed()
	if n, err = f.q.read(f.ctx, p); err != nil {
		f.close(f.ctx, err)
	}
	return
}

// ReadMsg is like read, but it reads bytes in chunks.  Depending on the
// implementation the batch boundaries might or might not be significant.
// Read and ReadMsg should not be called concurrently with themselves
// or each other.
func (f *flw) ReadMsg() (buf []byte, err error) {
	if err = f.checkBlessings(); err != nil {
		return
	}
	f.markUsed()
	// TODO(mattr): Currently we only ever release counters when some flow
	// reads.  We may need to do it more or less often.  Currently
	// we'll send counters whenever a new flow is opened.
	if buf, err = f.q.get(f.ctx); err != nil {
		f.close(f.ctx, err)
	}
	return
}

// Implement io.Writer.
// Write, WriteMsg, and WriteMsgAndClose should not be called concurrently
// with themselves or each other.
func (f *flw) Write(p []byte) (n int, err error) {
	return f.WriteMsg(p)
}

// tokensLocked returns the number of tokens this flow can send right now.
// It is bounded by the channel mtu, the released counters, and possibly
// the number of shared counters for the conn if we are sending on a just
// dialed flow.
func (f *flw) tokensLocked() (int, func(int)) {
	max := uint64(mtu)
	if f.borrowing {
		if f.conn.lshared < max {
			max = f.conn.lshared
		}
		return int(max), func(used int) {
			f.conn.lshared -= uint64(used)
			f.borrowed += uint64(used)
		}
	}
	if f.released < max {
		max = f.released
	}
	return int(max), func(used int) { f.released -= uint64(used) }
}

// releaseLocked releases some counters from a remote reader to the local
// writer.  This allows the writer to then write more data to the wire.
func (f *flw) releaseLocked(tokens uint64) {
	f.borrowing = false
	if f.borrowed > 0 {
		n := tokens
		if f.borrowed < tokens {
			n = f.borrowed
		}
		tokens -= n
		f.borrowed -= n
		f.conn.lshared += n
	}
	f.released += tokens
	if f.writing {
		f.conn.activateWriterLocked(f)
	}
}

func (f *flw) writeMsg(alsoClose bool, parts ...[]byte) (sent int, err error) {
	if err = f.checkBlessings(); err != nil {
		return 0, err
	}
	select {
	// Catch cancellations early.  If we caught a cancel when waiting
	// our turn below its possible that we were notified simultaneously.
	// Then the notify channel will be full and we would deadlock
	// notifying ourselves.
	case <-f.ctx.Done():
		f.close(f.ctx, f.ctx.Err())
		return 0, f.ctx.Err()
	default:
	}
	size, sent, tosend := 0, 0, make([][]byte, len(parts))
	f.conn.mu.Lock()
	f.markUsedLocked()
	f.writing = true
	f.conn.activateWriterLocked(f)
	for err == nil && len(parts) > 0 {
		f.conn.notifyNextWriterLocked(f)

		// Wait for our turn.
		f.conn.mu.Unlock()
		select {
		case <-f.ctx.Done():
			err = f.ctx.Err()
		case <-f.writeCh:
		}

		// It's our turn, we lock to learn the current state of our buffer tokens.
		f.conn.mu.Lock()
		if err != nil {
			break
		}
		opened := f.opened
		tokens, deduct := f.tokensLocked()
		if tokens == 0 {
			// Oops, we really don't have data to send, probably because we've exhausted
			// the remote buffer.  deactivate ourselves but keep trying.
			f.conn.deactivateWriterLocked(f)
			continue
		}
		parts, tosend, size = popFront(parts, tosend[:0], tokens)
		deduct(size)
		f.conn.mu.Unlock()

		// Actually write to the wire.  This is also where encryption
		// happens, so this part can be slow.
		d := &message.Data{ID: f.id, Payload: tosend}
		if alsoClose && len(parts) == 0 {
			d.Flags |= message.CloseFlag
		}
		if f.noEncrypt {
			d.Flags |= message.DisableEncryptionFlag
		}
		if opened {
			err = f.conn.mp.writeMsg(f.ctx, d)
		} else {
			err = f.conn.mp.writeMsg(f.ctx, &message.OpenFlow{
				ID:              f.id,
				InitialCounters: DefaultBytesBufferedPerFlow,
				BlessingsKey:    f.bkey,
				DischargeKey:    f.dkey,
				Flags:           d.Flags,
				Payload:         d.Payload,
			})
			f.conn.unopenedFlows.Done()
		}
		sent += size

		// The top of the loop expects to be locked, so lock here and update
		// opened.  Note that since we've definitely sent a message now opened is surely
		// true.
		f.conn.mu.Lock()
		f.opened = true
	}
	f.writing = false
	f.conn.deactivateWriterLocked(f)
	f.conn.notifyNextWriterLocked(f)
	f.conn.mu.Unlock()

	if alsoClose || err != nil {
		f.close(f.ctx, err)
	}
	return sent, err
}

// WriteMsg is like Write, but allows writing more than one buffer at a time.
// The data in each buffer is written sequentially onto the flow.  Returns the
// number of bytes written.  WriteMsg must return a non-nil error if it writes
// less than the total number of bytes from all buffers.
// Write, WriteMsg, and WriteMsgAndClose should not be called concurrently
// with themselves or each other.
func (f *flw) WriteMsg(parts ...[]byte) (int, error) {
	return f.writeMsg(false, parts...)
}

// WriteMsgAndClose performs WriteMsg and then closes the flow.
// Write, WriteMsg, and WriteMsgAndClose should not be called concurrently
// with themselves or each other.
func (f *flw) WriteMsgAndClose(parts ...[]byte) (int, error) {
	return f.writeMsg(true, parts...)
}

func (f *flw) checkBlessings() error {
	var err error
	if !f.dialed && f.bkey != 0 {
		_, _, err = f.conn.blessingsFlow.getRemote(f.ctx, f.bkey, f.dkey)
	}
	return err
}

// SetContext sets the context associated with the flow.  Typically this is
// used to set state that is only available after the flow is connected, such
// as a more restricted flow timeout, or the language of the request.
// Calling SetContext may invalidate values previously returned from Closed.
//
// The flow.Manager associated with ctx must be the same flow.Manager that the
// flow was dialed or accepted from, otherwise an error is returned.
// TODO(mattr): enforce this restriction.
//
// TODO(mattr): update v23/flow documentation.
// SetContext may not be called concurrently with other methods.
func (f *flw) SetDeadlineContext(ctx *context.T, deadline time.Time) *context.T {
	if f.cancel != nil {
		f.cancel()
	}
	if !deadline.IsZero() {
		f.ctx, f.cancel = context.WithDeadline(ctx, deadline)
	} else {
		f.ctx, f.cancel = context.WithCancel(ctx)
	}
	return f.ctx
}

// LocalBlessings returns the blessings presented by the local end of the flow
// during authentication.
func (f *flw) LocalBlessings() security.Blessings {
	if f.dialed {
		blessings, _ := f.conn.blessingsFlow.getLocal(f.ctx, f.bkey, 0)
		return blessings
	}
	return f.conn.lBlessings
}

// RemoteBlessings returns the blessings presented by the remote end of the
// flow during authentication.
func (f *flw) RemoteBlessings() security.Blessings {
	var blessings security.Blessings
	var err error
	if !f.dialed {
		blessings, _, err = f.conn.blessingsFlow.getRemote(f.ctx, f.bkey, 0)
	} else {
		blessings, _, err = f.conn.blessingsFlow.getLatestRemote(f.ctx, f.conn.rBKey)
	}
	if err != nil {
		f.conn.Close(f.ctx, err)
	}
	return blessings
}

// LocalDischarges returns the discharges presented by the local end of the
// flow during authentication.
//
// Discharges are organized in a map keyed by the discharge-identifier.
func (f *flw) LocalDischarges() map[string]security.Discharge {
	if f.dialed {
		_, discharges := f.conn.blessingsFlow.getLocal(f.ctx, f.bkey, f.dkey)
		return discharges
	}
	return f.conn.blessingsFlow.getLatestLocal(f.ctx, f.conn.lBlessings)
}

// RemoteDischarges returns the discharges presented by the remote end of the
// flow during authentication.
//
// Discharges are organized in a map keyed by the discharge-identifier.
func (f *flw) RemoteDischarges() map[string]security.Discharge {
	var discharges map[string]security.Discharge
	var err error
	if !f.dialed {
		_, discharges, err = f.conn.blessingsFlow.getRemote(f.ctx, f.bkey, f.dkey)
	} else {
		_, discharges, err = f.conn.blessingsFlow.getLatestRemote(f.ctx, f.conn.rBKey)
	}
	if err != nil {
		f.conn.Close(f.ctx, err)
	}
	return discharges
}

// Conn returns the connection the flow is multiplexed on.
func (f *flw) Conn() flow.ManagedConn {
	return f.conn
}

// Closed returns a channel that remains open until the flow has been closed remotely
// or the context attached to the flow has been canceled.
//
// Note that after the returned channel is closed starting new writes will result
// in an error, but reads of previously queued data are still possible.  No
// new data will be queued.
// TODO(mattr): update v23/flow docs.
func (f *flw) Closed() <-chan struct{} {
	return f.ctx.Done()
}

func (f *flw) close(ctx *context.T, err error) {
	if f.q.close(ctx) {
		eid := verror.ErrorID(err)
		f.cancel()
		// After cancel has been called no new writes will begin for this
		// flow.  There may be a write in progress, but it must finish
		// before another writer gets to use the channel.  Therefore we
		// can simply use sendMessageLocked to send the close flow
		// message.
		f.conn.mu.Lock()
		delete(f.conn.flows, f.id)
		connClosing := f.conn.status == Closing
		var serr error
		if !f.opened {
			// Closing a flow that was never opened.
			f.conn.unopenedFlows.Done()
		} else if eid != ErrFlowClosedRemotely.ID && !connClosing {
			// Note: If the conn is closing there is no point in trying to
			// send the flow close message as it will fail.  This is racy
			// with the connection closing, but there are no ill-effects
			// other than spamming the logs a little so it's OK.
			serr = f.conn.sendMessageLocked(ctx, false, expressPriority, &message.Data{
				ID:    f.id,
				Flags: message.CloseFlag,
			})
		}
		f.conn.mu.Unlock()
		if serr != nil {
			ctx.Errorf("Could not send close flow message: %v", err)
		}
	}
}

// Close marks the flow as closed. After Close is called, new data cannot be
// written on the flow. Reads of already queued data are still possible.
func (f *flw) Close() error {
	f.close(f.ctx, nil)
	return nil
}

func (f *flw) markUsed() {
	if f.id >= reservedFlows {
		f.conn.markUsed()
	}
}

func (f *flw) markUsedLocked() {
	if f.id >= reservedFlows {
		f.conn.markUsedLocked()
	}
}

// popFront removes the first num bytes from in and appends them to out
// returning in, out, and the actual number of bytes appended.
func popFront(in, out [][]byte, num int) ([][]byte, [][]byte, int) {
	i, sofar := 0, 0
	for i < len(in) && sofar < num {
		i, sofar = i+1, sofar+len(in[i])
	}
	out = append(out, in[:i]...)
	if excess := sofar - num; excess > 0 {
		i, sofar = i-1, num
		keep := len(out[i]) - excess
		in[i], out[i] = in[i][keep:], out[i][:keep]
	}
	return in[i:], out, sofar
}
