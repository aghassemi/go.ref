package impl

import (
	"time"

	"veyron.io/veyron/veyron2/mgmt"
	"veyron.io/veyron/veyron2/rt"
	"veyron.io/veyron/veyron2/vlog"

	"veyron.io/veyron/veyron/lib/exec"
	"veyron.io/veyron/veyron/services/mgmt/node"
)

// InvokeCallback provides the parent node manager with the given name (which
// is expected to be this node manager's object name).
func InvokeCallback(name string) {
	handle, err := exec.GetChildHandle()
	switch err {
	case nil:
		// Node manager was started by self-update, notify the parent.
		callbackName, err := handle.Config.Get(mgmt.ParentNodeManagerConfigKey)
		if err != nil {
			// Node manager was not started by self-update, return silently.
			return
		}
		nmClient, err := node.BindConfig(callbackName)
		if err != nil {
			vlog.Fatalf("BindNode(%v) failed: %v", callbackName, err)
		}
		ctx, cancel := rt.R().NewContext().WithTimeout(time.Minute)
		defer cancel()
		if err := nmClient.Set(ctx, mgmt.ChildNodeManagerConfigKey, name); err != nil {
			vlog.Fatalf("Set(%v, %v) failed: %v", mgmt.ChildNodeManagerConfigKey, name, err)
		}
	case exec.ErrNoVersion:
	default:
		vlog.Fatalf("GetChildHandle() failed: %v", err)
	}
}
