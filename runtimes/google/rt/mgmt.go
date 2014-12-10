package rt

import (
	"fmt"
	"time"

	"veyron.io/veyron/veyron2"
	"veyron.io/veyron/veyron2/ipc"
	"veyron.io/veyron/veyron2/mgmt"
	"veyron.io/veyron/veyron2/naming"
	"veyron.io/veyron/veyron2/options"

	"veyron.io/veyron/veyron/lib/exec"
)

// TODO(cnicolaou,caprita): move this all out of the runtime when we
// refactor the profiles/runtime interface.
func (rt *vrt) initMgmt(appCycle veyron2.AppCycle, handle *exec.ChildHandle) (ipc.Server, error) {
	// Do not initialize the mgmt runtime if the process has not
	// been started through the veyron exec library by a device
	// manager.
	if handle == nil {
		return nil, nil
	}
	parentName, err := handle.Config.Get(mgmt.ParentNameConfigKey)
	if err != nil {
		return nil, nil
	}
	listenSpec, err := getListenSpec(handle)
	if err != nil {
		return nil, err
	}
	var serverOpts []ipc.ServerOpt
	parentPeerPattern, err := handle.Config.Get(mgmt.ParentBlessingConfigKey)
	if err == nil && parentPeerPattern != "" {
		// Grab the blessing from our blessing store that the parent
		// told us to use so they can talk to us.
		serverBlessing := rt.Principal().BlessingStore().ForPeer(parentPeerPattern)
		serverOpts = append(serverOpts, options.ServerBlessings{serverBlessing})
	}
	server, err := rt.NewServer(serverOpts...)
	if err != nil {
		return nil, err
	}
	ep, err := server.Listen(*listenSpec)
	if err != nil {
		return nil, err
	}
	if err := server.Serve("", appCycle.Remote(), nil); err != nil {
		server.Stop()
		return nil, err
	}
	err = rt.callbackToParent(parentName, naming.JoinAddressName(ep.String(), ""))
	if err != nil {
		server.Stop()
		return nil, err
	}
	return server, nil
}

func getListenSpec(handle *exec.ChildHandle) (*ipc.ListenSpec, error) {
	protocol, err := handle.Config.Get(mgmt.ProtocolConfigKey)
	if err != nil {
		return nil, err
	}
	if protocol == "" {
		return nil, fmt.Errorf("%v is not set", mgmt.ProtocolConfigKey)
	}

	address, err := handle.Config.Get(mgmt.AddressConfigKey)
	if err != nil {
		return nil, err
	}
	if address == "" {
		return nil, fmt.Errorf("%v is not set", mgmt.AddressConfigKey)
	}
	return &ipc.ListenSpec{Protocol: protocol, Address: address}, nil
}

func (rt *vrt) callbackToParent(parentName, myName string) error {
	ctx, _ := rt.NewContext().WithTimeout(10 * time.Second)
	call, err := rt.Client().StartCall(ctx, parentName, "Set", []interface{}{mgmt.AppCycleManagerConfigKey, myName})
	if err != nil {
		return err
	}
	if ierr := call.Finish(&err); ierr != nil {
		return ierr
	}
	return err
}
