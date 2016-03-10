// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package server contains utilities for serving a principal using a
// socket-based IPC system.
package server

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"v.io/v23/security"
	"v.io/v23/verror"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/passphrase"
	"v.io/x/ref/services/agent/internal/ipc"
	"v.io/x/ref/services/agent/internal/server"
)

const (
	pkgPath         = "v.io/x/ref/services/agent/server"
	agentSocketName = "agent.sock"
)

var (
	errCantReadPassphrase = verror.Register(pkgPath+".errCantReadPassphrase", verror.NoRetry, "{1:}{2:} failed to read passphrase{:_}")
	errNeedPassphrase     = verror.Register(pkgPath+".errNeedPassphrase", verror.NoRetry, "{1:}{2:} Passphrase required for decrypting principal{:_}")
)

// LoadPrincipal returns the principal persisted in the given credentials
// directory, prompting for a decryption passphrase in case the private key is
// encrypted.
func LoadPrincipal(credentials string) (security.Principal, error) {
	p, err := vsecurity.LoadPersistentPrincipal(credentials, nil)
	if verror.ErrorID(err) == vsecurity.ErrBadPassphrase.ID {
		var pass []byte
		p, pass, err = handlePassphrase(credentials)
		// Zero passhphrase out so it doesn't stay in memory.
		for i := range pass {
			pass[i] = 0
		}
	}
	return p, err
}

// LoadOrCreatePrincipal loads the principal persisted in the given credentials
// directory, prompting for a decryption passphrase in case the private key is
// encrypted.  If the credentials directory is empty, a new principal is created
// with blessing newname, and the user is prompted for an encryption passphrase
// for the private key (unless withPassphrase is false).  Returns the principal,
// passphrase, and any errors encountered.
func LoadOrCreatePrincipal(credentials, newname string, withPassphrase bool) (security.Principal, []byte, error) {
	p, err := vsecurity.LoadPersistentPrincipal(credentials, nil)
	if os.IsNotExist(err) {
		return handleDoesNotExist(credentials, newname, withPassphrase)
	}
	if verror.ErrorID(err) == vsecurity.ErrBadPassphrase.ID {
		if !withPassphrase {
			return nil, nil, verror.New(errNeedPassphrase, nil)
		}
		return handlePassphrase(credentials)
	}
	return p, nil, err
}

// IPCState represents the IPC system serving the principal.
type IPCState interface {
	// Close shuts the IPC system down.
	Close()
	// IdleStartTime returns the time when the IPC system became idle (no
	// connections).  Returns the zero time instant if connections exits.
	IdleStartTime() time.Time
	// NumConnections returns the number of current connections.
	NumConnections() int
}

type ipcState struct {
	*ipc.IPC
}

// NumConnections implements IPCState.NumConnections.
func (s ipcState) NumConnections() int {
	return len(s.IPC.Connections())
}

// Serve serves the given principal using the given socket file, and returns an
// IPCState for the service.
func Serve(p security.Principal, socketPath string) (IPCState, error) {
	var err error
	if socketPath, err = filepath.Abs(socketPath); err != nil {
		return nil, fmt.Errorf("Abs failed: %v", err)
	}
	socketPath = filepath.Clean(socketPath)

	// Start running our server.
	i := ipc.NewIPC()
	if err := server.ServeAgent(i, p); err != nil {
		i.Close()
		return nil, fmt.Errorf("ServeAgent failed: %v", err)
	}
	if err = os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		i.Close()
		return nil, err
	}
	if err := i.Listen(socketPath); err != nil {
		i.Close()
		return nil, fmt.Errorf("Listen failed: %v", err)
	}
	return ipcState{i}, nil
}

// ServeWithKeyManager serves the given principal as well as a key manager using
// the given socket file.  The passhprase is for the key manager.  Returns a
// cleanup function.
func ServeWithKeyManager(p security.Principal, keypath string, passphrase []byte, socketPath string) (func(), error) {
	var err error
	if socketPath, err = filepath.Abs(socketPath); err != nil {
		return nil, fmt.Errorf("Abs failed: %v", err)
	}
	socketPath = filepath.Clean(socketPath)

	// Start running our server.
	i := ipc.NewIPC()
	if err := server.ServeAgent(i, p); err != nil {
		return nil, fmt.Errorf("ServeAgent failed: %v", err)
	}
	if err := server.ServeKeyManager(i, keypath, passphrase); err != nil {
		return nil, fmt.Errorf("ServeKeyManager failed: %v", err)
	}
	if err := i.Listen(socketPath); err != nil {
		i.Close()
		return nil, fmt.Errorf("Listen failed: %v", err)
	}
	return i.Close, nil
}

func handleDoesNotExist(dir, newname string, withPassphrase bool) (security.Principal, []byte, error) {
	fmt.Println("Private key file does not exist. Creating new private key...")
	var pass []byte
	if withPassphrase {
		var err error
		if pass, err = passphrase.Get("Enter passphrase (entering nothing will store unencrypted): "); err != nil {
			return nil, nil, verror.New(errCantReadPassphrase, nil, err)
		}
	}
	p, err := vsecurity.CreatePersistentPrincipal(dir, pass)
	if err != nil {
		return nil, pass, err
	}
	name := newname
	if len(name) == 0 {
		name = "agent_principal"
	}
	vsecurity.InitDefaultBlessings(p, name)
	return p, pass, nil
}

func handlePassphrase(dir string) (security.Principal, []byte, error) {
	pass, err := passphrase.Get(fmt.Sprintf("Passphrase required to decrypt encrypted private key file for credentials in %v.\nEnter passphrase: ", dir))
	if err != nil {
		return nil, nil, verror.New(errCantReadPassphrase, nil, err)
	}
	p, err := vsecurity.LoadPersistentPrincipal(dir, pass)
	return p, pass, err
}
