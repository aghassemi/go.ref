package main

import (
	"log"
	"testing"

	"veyron.io/veyron/veyron2"
	"veyron.io/veyron/veyron2/ipc"
	"veyron.io/veyron/veyron2/naming"
	"veyron.io/veyron/veyron2/security"
	"veyron.io/veyron/veyron2/services/mgmt/binary"
	"veyron.io/veyron/veyron2/services/mgmt/node"
	"veyron.io/veyron/veyron2/vlog"

	"veyron.io/veyron/veyron/profiles"
)

type mockNodeInvoker struct {
	tape *Tape
	t    *testing.T
}

// Mock ListAssociations
type ListAssociationResponse struct {
	na  []node.Association
	err error
}

func (mni *mockNodeInvoker) ListAssociations(ipc.ServerContext) (associations []node.Association, err error) {
	vlog.VI(2).Infof("ListAssociations() was called")

	ir := mni.tape.Record("ListAssociations")
	r := ir.(ListAssociationResponse)
	return r.na, r.err
}

// Mock AssociateAccount
type AddAssociationStimulus struct {
	fun           string
	identityNames []string
	accountName   string
}

func (i *mockNodeInvoker) AssociateAccount(call ipc.ServerContext, identityNames []string, accountName string) error {
	ri := i.tape.Record(AddAssociationStimulus{"AssociateAccount", identityNames, accountName})
	switch r := ri.(type) {
	case nil:
		return nil
	case error:
		return r
	}
	log.Fatalf("AssociateAccount (mock) response %v is of bad type", ri)
	return nil
}

func (i *mockNodeInvoker) Claim(call ipc.ServerContext) error { return nil }
func (*mockNodeInvoker) Describe(ipc.ServerContext) (node.Description, error) {
	return node.Description{}, nil
}
func (*mockNodeInvoker) IsRunnable(_ ipc.ServerContext, description binary.Description) (bool, error) {
	return false, nil
}
func (*mockNodeInvoker) Reset(call ipc.ServerContext, deadline uint64) error { return nil }

// Mock Install
type InstallStimulus struct {
	fun     string
	appName string
}

type InstallResponse struct {
	appId string
	err   error
}

func (mni *mockNodeInvoker) Install(call ipc.ServerContext, appName string) (string, error) {
	ir := mni.tape.Record(InstallStimulus{"Install", appName})
	r := ir.(InstallResponse)
	return r.appId, r.err
}

func (*mockNodeInvoker) Refresh(ipc.ServerContext) error           { return nil }
func (*mockNodeInvoker) Restart(ipc.ServerContext) error           { return nil }
func (*mockNodeInvoker) Resume(ipc.ServerContext) error            { return nil }
func (i *mockNodeInvoker) Revert(call ipc.ServerContext) error     { return nil }
func (*mockNodeInvoker) Start(ipc.ServerContext) ([]string, error) { return []string{}, nil }
func (*mockNodeInvoker) Stop(ipc.ServerContext, uint32) error      { return nil }
func (*mockNodeInvoker) Suspend(ipc.ServerContext) error           { return nil }
func (*mockNodeInvoker) Uninstall(ipc.ServerContext) error         { return nil }
func (i *mockNodeInvoker) Update(ipc.ServerContext) error          { return nil }
func (*mockNodeInvoker) UpdateTo(ipc.ServerContext, string) error  { return nil }

// Mock ACL getting and setting
type GetACLResponse struct {
	acl  security.ACL
	etag string
	err  error
}

type SetACLStimulus struct {
	fun  string
	acl  security.ACL
	etag string
}

func (mni *mockNodeInvoker) SetACL(_ ipc.ServerContext, acl security.ACL, etag string) error {
	ri := mni.tape.Record(SetACLStimulus{"SetACL", acl, etag})
	switch r := ri.(type) {
	case nil:
		return nil
	case error:
		return r
	}
	log.Fatalf("AssociateAccount (mock) response %v is of bad type\n", ri)
	return nil
}

func (mni *mockNodeInvoker) GetACL(ipc.ServerContext) (security.ACL, string, error) {
	ir := mni.tape.Record("GetACL")
	r := ir.(GetACLResponse)
	return r.acl, r.etag, r.err
}

type dispatcher struct {
	tape *Tape
	t    *testing.T
}

func NewDispatcher(t *testing.T, tape *Tape) *dispatcher {
	return &dispatcher{tape: tape, t: t}
}

func (d *dispatcher) Lookup(suffix string) (interface{}, security.Authorizer, error) {
	return node.NodeServer(&mockNodeInvoker{tape: d.tape, t: d.t}), nil, nil
}

func startServer(t *testing.T, r veyron2.Runtime, tape *Tape) (ipc.Server, naming.Endpoint, error) {
	dispatcher := NewDispatcher(t, tape)
	server, err := r.NewServer()
	if err != nil {
		t.Errorf("NewServer failed: %v", err)
		return nil, nil, err
	}
	endpoint, err := server.Listen(profiles.LocalListenSpec)
	if err != nil {
		t.Errorf("Listen failed: %v", err)
		stopServer(t, server)
		return nil, nil, err
	}
	if err := server.ServeDispatcher("", dispatcher); err != nil {
		t.Errorf("ServeDispatcher failed: %v", err)
		stopServer(t, server)
		return nil, nil, err
	}
	return server, endpoint, nil
}

func stopServer(t *testing.T, server ipc.Server) {
	if err := server.Stop(); err != nil {
		t.Errorf("server.Stop failed: %v", err)
	}
}
