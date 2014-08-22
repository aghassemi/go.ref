package impl

import (
	"veyron/services/mgmt/repository"

	"veyron/services/mgmt/lib/fs"
	"veyron2/ipc"
	"veyron2/security"
)

// dispatcher holds the state of the application repository dispatcher.
type dispatcher struct {
	store     *fs.Memstore
	auth      security.Authorizer
	storeRoot string
}

// NewDispatcher is the dispatcher factory.
func NewDispatcher(name string, authorizer security.Authorizer) (*dispatcher, error) {
	// TODO(rjkroege@google.com): Use the config service.
	store, err := fs.NewMemstore("")
	if err != nil {
		return nil, err
	}
	return &dispatcher{store: store, storeRoot: name, auth: authorizer}, nil
}

// DISPATCHER INTERFACE IMPLEMENTATION

func (d *dispatcher) Lookup(suffix, method string) (ipc.Invoker, security.Authorizer, error) {
	invoker := ipc.ReflectInvoker(repository.NewServerApplication(NewInvoker(d.store, d.storeRoot, suffix)))
	return invoker, d.auth, nil
}
