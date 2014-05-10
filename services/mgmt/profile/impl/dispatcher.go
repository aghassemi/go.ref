package impl

import (
	"veyron/services/mgmt/profile"

	"veyron2/ipc"
	"veyron2/security"
	"veyron2/storage"
	"veyron2/storage/vstore"
)

// dispatcher holds the state of the profile manager dispatcher.
type dispatcher struct {
	store storage.Store
}

// NewDispatcher is the dispatcher factory.
func NewDispatcher(name string) (*dispatcher, error) {
	store, err := vstore.New(name)
	if err != nil {
		return nil, err
	}
	return &dispatcher{store: store}, nil
}

// DISPATCHER INTERFACE IMPLEMENTATION

func (d *dispatcher) Lookup(suffix string) (ipc.Invoker, security.Authorizer, error) {
	invoker := ipc.ReflectInvoker(profile.NewServerProfile(NewInvoker(d.store, suffix)))
	return invoker, nil, nil
}
