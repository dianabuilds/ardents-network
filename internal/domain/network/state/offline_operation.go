package state

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network"
)

// OfflineResult is a detached receipt from a joined Network State operation.
type OfflineResult struct {
	Outcome    string
	Profile    *network.ProfileFacts
	ObservedAt time.Time
}

// RunOffline opens the appropriate State, performs one offline intake or
// observation and joins the original owner before publishing its receipt.
func RunOffline(ctx context.Context, operation string, config Config, epoch []byte, inputs, materials [][]byte, profile []byte) OfflineResult {
	if ctx == nil {
		return OfflineResult{Outcome: "unavailable"}
	}
	if operation != "accept-epoch" && operation != "accept-profile" && operation != "inspect" {
		return OfflineResult{Outcome: "unavailable"}
	}
	openState := Open
	if operation == "inspect" {
		openState = OpenRetained
	}
	owner, err := openState(config)
	if err != nil {
		return OfflineResult{Outcome: "owner-unavailable"}
	}
	result := OfflineResult{Outcome: "unavailable"}
	switch operation {
	case "accept-epoch":
		var accepted OfflineAcceptance
		accepted, err = owner.AcceptEpoch(ctx, epoch, inputs, materials)
		if err == nil {
			result.Outcome = "accepted-epoch"
			if accepted.Pending {
				result.Outcome = "pending-epoch"
			}
		}
	case "accept-profile":
		if ctx.Err() == nil {
			_, err = owner.AcceptClosedProfile(profile)
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
		result = OfflineResult{Outcome: "unavailable"}
		if ctx.Err() != nil {
			result.Outcome = "canceled"
		}
	}
	return result
}
