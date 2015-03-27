// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"v.io/v23/verror"

	"v.io/x/lib/vlog"

	"v.io/x/ref/lib/exec/consts"
	"v.io/x/ref/lib/timekeeper"
)

const pkgPath = "v.io/x/ref/lib/exec"

var (
	ErrAuthTimeout      = verror.Register(pkgPath+".ErrAuthTimeout", verror.NoRetry, "{1:}{2:} timeout in auth handshake{:_}")
	ErrTimeout          = verror.Register(pkgPath+".ErrTimeout", verror.NoRetry, "{1:}{2:} timeout waiting for child{:_}")
	ErrSecretTooLarge   = verror.Register(pkgPath+".ErrSecretTooLarge", verror.NoRetry, "{1:}{2:} secret is too large{:_}")
	ErrNotUsingProtocol = verror.Register(pkgPath+".ErrNotUsingProtocol", verror.NoRetry, "{1:}{2:} not using parent/child exec protocol{:_}")

	errFailedStatus       = verror.Register(pkgPath+".errFailedStatus", verror.NoRetry, "{1:}{2:} {_}")
	errUnrecognizedStatus = verror.Register(pkgPath+".errUnrecognizedStatus", verror.NoRetry, "{1:}{2:} unrecognised status from subprocess{:_}")
	errUnexpectedType     = verror.Register(pkgPath+".errUnexpectedType", verror.NoRetry, "{1:}{2:} unexpected type {3}{:_}")
	errNoSuchProcess      = verror.Register(pkgPath+".errNoSuchProcess", verror.NoRetry, "{1:}{2:} no such process{:_}")
	errPartialWrite       = verror.Register(pkgPath+".errPartialWrite", verror.NoRetry, "{1:}{2:} partial write{:_}")
)

// A ParentHandle is the Parent process' means of managing a single child.
type ParentHandle struct {
	c           *exec.Cmd
	config      Config
	protocol    bool // true if we're using the parent/child protocol.
	secret      string
	statusRead  *os.File
	statusWrite *os.File
	tk          timekeeper.TimeKeeper
	waitDone    bool
	waitErr     error
	waitLock    sync.Mutex
	callbackPid int
}

// ParentHandleOpt is an option for NewParentHandle.
type ParentHandleOpt interface {
	// ExecParentHandleOpt is a signature 'dummy' method for the
	// interface.
	ExecParentHandleOpt()
}

// ConfigOpt can be used to seed the parent handle with a
// config to be passed to the child.
type ConfigOpt struct {
	Config
}

// ExecParentHandleOpt makes ConfigOpt an instance of
// ParentHandleOpt.
func (ConfigOpt) ExecParentHandleOpt() {}

// SecretOpt can be used to seed the parent handle with a custom secret.
type SecretOpt string

// ExecParentHandleOpt makes SecretOpt an instance of ParentHandleOpt.
func (SecretOpt) ExecParentHandleOpt() {}

// TimeKeeperOpt can be used to seed the parent handle with a custom timekeeper.
type TimeKeeperOpt struct {
	timekeeper.TimeKeeper
}

// ExecParentHandleOpt makes TimeKeeperOpt an instance of ParentHandleOpt.
func (TimeKeeperOpt) ExecParentHandleOpt() {}

// UseExecProtocolOpt can be used to control whether parent/child handshake
// protocol is used. WaitForReady will return immediately with an error if
// this option is set to false. The defaults behaviour is to assume that
// the exec protocol is used. If it is not used, then the Start method
// will not create shared file descriptors to use for the exec protocol.
type UseExecProtocolOpt bool

func (UseExecProtocolOpt) ExecParentHandleOpt() {}

// NewParentHandle creates a ParentHandle for the child process represented by
// an instance of exec.Cmd.
func NewParentHandle(c *exec.Cmd, opts ...ParentHandleOpt) *ParentHandle {
	cfg, secret := NewConfig(), ""
	tk := timekeeper.RealTime()
	protocol := true
	for _, opt := range opts {
		switch v := opt.(type) {
		case ConfigOpt:
			cfg = v
		case SecretOpt:
			secret = string(v)
		case TimeKeeperOpt:
			tk = v
		case UseExecProtocolOpt:
			protocol = bool(v)
		default:
			vlog.Errorf("Unrecognized parent option: %v", v)
		}
	}
	return &ParentHandle{
		c:        c,
		protocol: protocol,
		config:   cfg,
		secret:   secret,
		tk:       tk,
	}
}

// Start starts the child process, sharing a secret with it and
// setting up a communication channel over which to read its status.
func (p *ParentHandle) Start() error {
	// Make sure that there are no instances of the consts.ExecVersionVariable
	// already in the environment (which can happen when a subprocess
	// creates a subprocess etc)
	nenv := make([]string, 0, len(p.c.Env)+1)
	for _, e := range p.c.Env {
		if strings.HasPrefix(e, consts.ExecVersionVariable+"=") {
			continue
		}
		nenv = append(nenv, e)
	}

	if !p.protocol {
		return p.c.Start()
	}

	p.c.Env = append(nenv, consts.ExecVersionVariable+"="+version1)

	// Create anonymous pipe for communicating data between the child
	// and the parent.
	// TODO(caprita): As per ribrdb@, Go's exec does not prune the set
	// of file descriptors passed down to the child process, and hence
	// a child may get access to the files meant for another child.
	// Do we need to ensure only one thread is allowed to create these
	// pipes at any time?
	dataRead, dataWrite, err := os.Pipe()
	if err != nil {
		return err
	}
	defer dataRead.Close()
	defer dataWrite.Close()
	statusRead, statusWrite, err := os.Pipe()
	if err != nil {
		return err
	}
	p.statusRead = statusRead
	p.statusWrite = statusWrite
	// Add the parent-child pipes to cmd.ExtraFiles, offsetting all
	// existing file descriptors accordingly.
	extraFiles := make([]*os.File, len(p.c.ExtraFiles)+2)
	extraFiles[0] = dataRead
	extraFiles[1] = statusWrite
	for i, _ := range p.c.ExtraFiles {
		extraFiles[i+2] = p.c.ExtraFiles[i]
	}
	p.c.ExtraFiles = extraFiles
	// Start the child process.
	if err := p.c.Start(); err != nil {
		p.statusWrite.Close()
		p.statusRead.Close()
		return err
	}
	// Pass data to the child using a pipe.
	serializedConfig, err := p.config.Serialize()
	if err != nil {
		return err
	}
	if err := encodeString(dataWrite, serializedConfig); err != nil {
		p.statusWrite.Close()
		p.statusRead.Close()
		return err
	}
	if err := encodeString(dataWrite, p.secret); err != nil {
		p.statusWrite.Close()
		p.statusRead.Close()
		return err
	}
	return nil
}

// copy is like io.Copy, but it also treats the receipt of the special eofChar
// byte to mean io.EOF.
func copy(w io.Writer, r io.Reader) (err error) {
	buf := make([]byte, 1024)
	for {
		nRead, errRead := r.Read(buf)
		if nRead > 0 {
			if eofCharIndex := bytes.IndexByte(buf[:nRead], eofChar); eofCharIndex != -1 {
				nRead = eofCharIndex
				errRead = io.EOF
			}
			nWrite, errWrite := w.Write(buf[:nRead])
			if errWrite != nil {
				err = errWrite
				break
			}
			if nRead != nWrite {
				err = io.ErrShortWrite
				break
			}
		}
		if errRead == io.EOF {
			break
		}
		if errRead != nil {
			err = errRead
			break
		}
	}
	return
}

func waitForStatus(c chan interface{}, r *os.File) {
	var readBytes bytes.Buffer
	err := copy(&readBytes, r)
	r.Close()
	if err != nil {
		c <- err
	} else {
		c <- readBytes.String()
	}
	close(c)
}

// WaitForReady will wait for the child process to become ready.
func (p *ParentHandle) WaitForReady(timeout time.Duration) error {
	if !p.protocol {
		return verror.New(ErrNotUsingProtocol, nil)
	}
	// An invariant of WaitForReady is that both statusWrite and statusRead
	// get closed before WaitForStatus returns (statusRead gets closed by
	// waitForStatus).
	defer p.statusWrite.Close()
	c := make(chan interface{}, 1)
	go waitForStatus(c, p.statusRead)
	// TODO(caprita): This can be simplified further by doing the reading
	// from the status pipe here, and instead moving the timeout listener to
	// a separate goroutine.
	select {
	case msg := <-c:
		switch m := msg.(type) {
		case error:
			return m
		case string:
			if strings.HasPrefix(m, readyStatus) {
				pid, err := strconv.Atoi(m[len(readyStatus):])
				if err != nil {
					return err
				}
				p.callbackPid = pid
				return nil
			}
			if strings.HasPrefix(m, failedStatus) {
				return verror.New(errFailedStatus, nil, strings.TrimPrefix(m, failedStatus))
			}
			return verror.New(errUnrecognizedStatus, nil, m)
		default:
			return verror.New(errUnexpectedType, nil, fmt.Sprintf("%T", m))
		}
	case <-p.tk.After(timeout):
		vlog.Errorf("Timed out waiting for child status")
		// By writing the special eofChar byte to the pipe, we ensure
		// that waitForStatus returns: the copy function treats eofChar
		// to indicate end of read input.  Note, copy could have
		// finished for other reasons already (receipt of eofChar from
		// the child process).  Note, closing the pipe from the child
		// (explicitly or due to crash) would NOT cause copy to read
		// io.EOF, since we keep the statusWrite open in the parent.
		// Hence, a child crash will eventually trigger this timeout.
		p.statusWrite.Write([]byte{eofChar})
		// Before returning, waitForStatus will close r, and then close
		// c.  Waiting on c ensures that r.Close() in waitForStatus
		// already executed.
		<-c
		return verror.New(ErrTimeout, nil)
	}
	panic("unreachable")
}

// wait performs the Wait on the underlying command under lock, and only once
// (subsequent wait calls block until the Wait is finished).  It's ok to call
// wait multiple times, and in parallel.  The error from the initial Wait is
// cached and returned for all subsequent calls.
func (p *ParentHandle) wait() error {
	p.waitLock.Lock()
	defer p.waitLock.Unlock()
	if p.waitDone {
		return p.waitErr
	}
	p.waitErr = p.c.Wait()
	p.waitDone = true
	return p.waitErr
}

// Wait will wait for the child process to terminate of its own accord.
// It returns nil if the process exited cleanly with an exit status of 0,
// any other exit code or error will result in an appropriate error return
func (p *ParentHandle) Wait(timeout time.Duration) error {
	c := make(chan error, 1)
	go func() {
		c <- p.wait()
		close(c)
	}()
	// If timeout is zero time.After will panic; we handle zero specially
	// to mean infinite timeout.
	if timeout > 0 {
		select {
		case <-p.tk.After(timeout):
			return verror.New(ErrTimeout, nil)
		case err := <-c:
			return err
		}
	} else {
		return <-c
	}
	panic("unreachable")
}

// Pid returns the pid of the child, 0 if the child process doesn't exist
func (p *ParentHandle) Pid() int {
	if p.c.Process != nil {
		return p.c.Process.Pid
	}
	return 0
}

// ChildPid returns the pid of a child process as reported by its status
// callback.
func (p *ParentHandle) ChildPid() int {
	return p.callbackPid
}

// Exists returns true if the child process exists and can be signal'ed
func (p *ParentHandle) Exists() bool {
	if p.c.Process != nil {
		return syscall.Kill(p.c.Process.Pid, 0) == nil
	}
	return false
}

// Kill kills the child process.
func (p *ParentHandle) Kill() error {
	if p.c.Process == nil {
		return verror.New(errNoSuchProcess, nil)
	}
	return p.c.Process.Kill()
}

// Signal sends the given signal to the child process.
func (p *ParentHandle) Signal(sig syscall.Signal) error {
	if p.c.Process == nil {
		return verror.New(errNoSuchProcess, nil)
	}
	return syscall.Kill(p.c.Process.Pid, sig)
}

// Clean will clean up state, including killing the child process.
func (p *ParentHandle) Clean() error {
	if err := p.Kill(); err != nil {
		return err
	}
	return p.wait()
}

func encodeString(w io.Writer, data string) error {
	l := len(data)
	if err := binary.Write(w, binary.BigEndian, int64(l)); err != nil {
		return err
	}
	if n, err := w.Write([]byte(data)); err != nil || n != l {
		if err != nil {
			return err
		} else {
			return verror.New(errPartialWrite, nil)
		}
	}
	return nil
}
