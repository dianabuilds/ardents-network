package command

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
)

// offlineResult is a detached receipt from a joined Network State operation.
type offlineResult struct {
	Outcome    string
	Profile    *network.ProfileFacts
	ObservedAt time.Time
}

// runOffline opens the appropriate State, performs one offline intake or
// observation and joins the original owner before publishing its receipt.
func runOffline(ctx context.Context, operation string, input offlineDeclaration) offlineResult {
	if ctx == nil {
		return offlineResult{Outcome: "unavailable"}
	}
	if operation != "accept-epoch" && operation != "accept-profile" && operation != "inspect" {
		return offlineResult{Outcome: "unavailable"}
	}
	openState := state.Open
	if operation == "inspect" {
		openState = state.OpenRetained
	}
	// The original caller must still be live before Open can create history.
	if ctx.Err() != nil {
		return offlineResult{Outcome: "canceled"}
	}
	owner, err := openState(input.Config)
	if err != nil {
		return offlineResult{Outcome: "owner-unavailable"}
	}
	result := offlineResult{Outcome: "unavailable"}
	switch operation {
	case "accept-epoch":
		var accepted state.OfflineAcceptance
		accepted, err = owner.AcceptEpoch(ctx, input.Epoch, input.Inputs, input.Materials)
		if err == nil {
			result.Outcome = "accepted-epoch"
			if accepted.Pending {
				result.Outcome = "pending-epoch"
			}
		}
	case "accept-profile":
		if ctx.Err() == nil {
			_, err = owner.AcceptClosedProfile(input.Profile)
		} else {
			err = ctx.Err()
		}
		if err == nil {
			result.Outcome = "accepted-profile"
		}
	case "inspect":
		var view network.RuntimeView
		view, err = owner.CurrentRuntime()
		if err == nil {
			value := view.Profile()
			result.Profile, result.ObservedAt, result.Outcome = &value, view.ObservedAt(), "observed"
		}
	}
	if errors.Join(err, ctx.Err(), owner.Close()) != nil {
		result = offlineResult{Outcome: "unavailable"}
		if ctx.Err() != nil {
			result.Outcome = "canceled"
		}
	}
	return result
}
