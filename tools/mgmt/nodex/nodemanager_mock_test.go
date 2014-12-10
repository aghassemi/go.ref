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
	"veyron.io/veyron/veyron2/services/security/access"
	"veyron.io/veyron/veyron2/vlog"

	"veyron.io/veyron/veyron/profiles"
)

type mockDeviceInvoker struct {
	tape *Tape
	t    *testing.T
}

// Mock ListAssociations
type ListAssociationResponse struct {
	na  []node.Association
	err error
}

func (mni *mockDeviceInvoker) ListAssociations(ipc.ServerContext) (associations []node.Association, err error) {
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

// simpleCore implements the core of all mock methods that take
// arguments and return error.
func (mni *mockDeviceInvoker) simpleCore(callRecord interface{}, name string) error {
	ri := mni.tape.Record(callRecord)
	switch r := ri.(type) {
	case nil:
		return nil
	case error:
		return r
	}
	log.Fatalf("%s (mock) response %v is of bad type", name, ri)
	return nil
}

func (mni *mockDeviceInvoker) AssociateAccount(call ipc.ServerContext, identityNames []string, accountName string) error {
	return mni.simpleCore(AddAssociationStimulus{"AssociateAccount", identityNames, accountName}, "AssociateAccount")
}

func (mni *mockDeviceInvoker) Claim(call ipc.ServerContext) error {
	return mni.simpleCore("Claim", "Claim")
}

func (*mockDeviceInvoker) Describe(ipc.ServerContext) (node.Description, error) {
	return node.Description{}, nil
}
func (*mockDeviceInvoker) IsRunnable(_ ipc.ServerContext, description binary.Description) (bool, error) {
	return false, nil
}
func (*mockDeviceInvoker) Reset(call ipc.ServerContext, deadline uint64) error { return nil }

// Mock Install
type InstallStimulus struct {
	fun     string
	appName string
}

type InstallResponse struct {
	appId string
	err   error
}

func (mni *mockDeviceInvoker) Install(call ipc.ServerContext, appName string) (string, error) {
	ir := mni.tape.Record(InstallStimulus{"Install", appName})
	r := ir.(InstallResponse)
	return r.appId, r.err
}

func (*mockDeviceInvoker) Refresh(ipc.ServerContext) error { return nil }
func (*mockDeviceInvoker) Restart(ipc.ServerContext) error { return nil }

func (mni *mockDeviceInvoker) Resume(_ ipc.ServerContext) error {
	return mni.simpleCore("Resume", "Resume")
}
func (i *mockDeviceInvoker) Revert(call ipc.ServerContext) error { return nil }

type StartResponse struct {
	appIds []string
	err    error
}

func (mni *mockDeviceInvoker) Start(ipc.ServerContext) ([]string, error) {
	ir := mni.tape.Record("Start")
	r := ir.(StartResponse)
	return r.appIds, r.err
}

type StopStimulus struct {
	fun       string
	timeDelta uint32
}

func (mni *mockDeviceInvoker) Stop(_ ipc.ServerContext, timeDelta uint32) error {
	return mni.simpleCore(StopStimulus{"Stop", timeDelta}, "Stop")
}

func (mni *mockDeviceInvoker) Suspend(_ ipc.ServerContext) error {
	return mni.simpleCore("Suspend", "Suspend")
}
func (*mockDeviceInvoker) Uninstall(ipc.ServerContext) error        { return nil }
func (i *mockDeviceInvoker) Update(ipc.ServerContext) error         { return nil }
func (*mockDeviceInvoker) UpdateTo(ipc.ServerContext, string) error { return nil }

// Mock ACL getting and setting
type GetACLResponse struct {
	acl  access.TaggedACLMap
	etag string
	err  error
}

type SetACLStimulus struct {
	fun  string
	acl  access.TaggedACLMap
	etag string
}

func (mni *mockDeviceInvoker) SetACL(_ ipc.ServerContext, acl access.TaggedACLMap, etag string) error {
	return mni.simpleCore(SetACLStimulus{"SetACL", acl, etag}, "SetACL")
}

func (mni *mockDeviceInvoker) GetACL(ipc.ServerContext) (access.TaggedACLMap, string, error) {
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
	return node.DeviceServer(&mockDeviceInvoker{tape: d.tape, t: d.t}), nil, nil
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
