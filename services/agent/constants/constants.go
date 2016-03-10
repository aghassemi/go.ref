// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(caprita): Move to internal.

// Package constants holds constants shared by client and server.
package constants

import "path/filepath"

const (
	agentDir   = "agent"
	socketFile = "sock"
	ServingMsg = "serving"
)

// SocketPath returns the location where the agent generates the socket file.
func SocketPath(credsDir string) string {
	return filepath.Join(AgentDir(credsDir), socketFile)
}

// AgentDir returns the directory where the agent keeps its state.
func AgentDir(credsDir string) string {
	return filepath.Join(credsDir, agentDir)
}
