package discharger

import (
	"fmt"
	"time"

	services "veyron.io/veyron/veyron/services/security"
	"veyron.io/veyron/veyron2/ipc"
	"veyron.io/veyron/veyron2/security"
	"veyron.io/veyron/veyron2/vdl/vdlutil"
)

// dischargerd issues discharges for all caveats present in the current
// namespace with no additional caveats iff the caveat is valid.
type dischargerd struct {
	id security.PrivateID
}

// TODO(andreser,ataly): make it easier for third party public key caveats to specify the caveats on their discharges

func (d dischargerd) Discharge(ctx ipc.ServerContext, caveatAny vdlutil.Any, _ security.DischargeImpetus) (vdlutil.Any, error) {
	caveat, ok := caveatAny.(security.ThirdPartyCaveat)
	if !ok {
		return nil, fmt.Errorf("type %T does not implement security.ThirdPartyCaveat", caveatAny)
	}
	if err := caveat.Dischargeable(ctx); err != nil {
		return nil, fmt.Errorf("third-party caveat %v cannot be discharged for this context: %v", caveat, err)
	}
	return d.id.MintDischarge(caveat, ctx, time.Minute, nil)
}

// NewDischarger returns a discharger service implementation that grants discharges using id.MintDischarge.
func NewDischarger(id security.PrivateID) services.DischargerService {
	return dischargerd{id}
}
