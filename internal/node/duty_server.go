package node

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// dutyHandle is the bounded supervision surface of one selected role.
type dutyHandle struct {
	Done    <-chan error
	Joined  <-chan struct{}
	Protect func(bool)
	Usage   func() (uint64, uint64, uint64)
	Stop    func()
	Drain   func(context.Context) error
}

// roleInputs projects the process-owned dependencies needed to start one
// selected network role. Role adapters do not receive the entire process
// configuration or its pressure and event state.
type roleInputs struct {
	authority      authority.Source
	currentDuty    func() (state.NodeDuty, error)
	now            func() time.Time
	listenOverride string
	host           closedHostingHandle
}

func projectRoleInputs(config runtimeConfig) roleInputs {
	current := config.Current
	return roleInputs{
		authority:   nodeAuthority(config),
		currentDuty: func() (state.NodeDuty, error) { return currentFacts(current) },
		now:         config.now, listenOverride: config.ClosedListenOverride, host: config.host,
	}
}

const nativeRouteUnavailableReason = "native Route assignment is not implemented"

func startDuty(config runtimeConfig, snapshot state.NodeDuty) (*dutyHandle, error) {
	if snapshot.Profile == carrier.ClosedRouteProfile {
		inputs := projectRoleInputs(config)
		if _, available := inputs.authority.Receiver(snapshot, ardp.PurposeIssuer, inputs.now()); available {
			return startClosedIssuer(config.ClosedIssuer, inputs, snapshot)
		}
		if _, available := inputs.authority.Receiver(snapshot, ardp.PurposeForwarding, inputs.now()); available {
			return startClosedForwarding(config.ClosedForwarding, inputs, snapshot)
		}
		if _, available := inputs.authority.Receiver(snapshot, ardp.PurposeReachability, inputs.now()); available {
			return startClosedResolution(config.ClosedResolution, inputs, snapshot)
		}
		if _, available := inputs.authority.Receiver(snapshot, ardp.PurposeIntroduction, inputs.now()); available {
			return startClosedIntroduction(config.ClosedIntroduction, inputs, snapshot)
		}
		if _, available := inputs.authority.Receiver(snapshot, ardp.PurposeDataJoin, inputs.now()); available {
			return startClosedDataJoin(config.ClosedDataJoin, inputs, snapshot)
		}
		return nil, errors.New("closed Route assignment is not locally implemented")
	}
	if snapshot.Profile == route.Profile {
		return nil, errors.New(nativeRouteUnavailableReason)
	}
	selected, err := config.probe.Start(newProbeDuty(snapshot))
	if err != nil {
		return nil, err
	}
	return &dutyHandle{Done: selected.Done, Protect: selected.Protect, Usage: selected.Usage,
		Stop: selected.Stop, Drain: selected.Drain}, nil
}
