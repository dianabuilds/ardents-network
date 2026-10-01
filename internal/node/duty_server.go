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
}

func projectRoleInputs(config runtimeConfig) roleInputs {
	current := config.Current
	return roleInputs{
		authority:   nodeAuthority(config),
		currentDuty: func() (state.NodeDuty, error) { return currentFacts(current) },
		now:         config.now, listenOverride: config.ClosedListenOverride,
	}
}

const nativeRouteUnavailableReason = "native Route assignment is not implemented"
const closedRouteUnavailableReason = "closed Route assignment is not locally implemented"

// selectClosedRole preserves the process dispatch order. Admission supplies one
// captured poll time; startup supplies its live clock for each authority check.
func selectClosedRole(source authority.Source, snapshot state.NodeDuty, now func() time.Time) (ardp.Purpose, bool) {
	for _, purpose := range [...]ardp.Purpose{
		ardp.PurposeIssuer,
		ardp.PurposeForwarding,
		ardp.PurposeReachability,
		ardp.PurposeIntroduction,
		ardp.PurposeDataJoin,
	} {
		if _, available := source.Receiver(snapshot, purpose, now()); available {
			return purpose, true
		}
	}
	return 0, false
}

func startDuty(config runtimeConfig, snapshot state.NodeDuty) (*dutyHandle, error) {
	if snapshot.Profile == carrier.ClosedRouteProfile {
		inputs := projectRoleInputs(config)
		purpose, available := selectClosedRole(inputs.authority, snapshot, inputs.now)
		if !available {
			return nil, errors.New(closedRouteUnavailableReason)
		}
		switch purpose {
		case ardp.PurposeIssuer:
			return startClosedIssuer(config.ClosedIssuer, inputs, config.host, snapshot)
		case ardp.PurposeForwarding:
			return startClosedForwarding(config.ClosedForwarding, inputs, snapshot)
		case ardp.PurposeReachability:
			return startClosedResolution(config.ClosedResolution, inputs, config.host, snapshot)
		case ardp.PurposeIntroduction:
			return startClosedIntroduction(config.ClosedIntroduction, inputs, config.host, snapshot)
		case ardp.PurposeDataJoin:
			return startClosedDataJoin(config.ClosedDataJoin, inputs, snapshot)
		}
		return nil, errors.New(closedRouteUnavailableReason)
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
