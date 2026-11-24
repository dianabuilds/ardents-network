package main

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/provisioning"
)

// Each operation owns its root lease. Epoch intake and profile acceptance are
// separate durable transitions; neither an intake receipt nor an inspection
// JSON file substitutes for a live opened Network observer in Admission.
func runNetwork(ctx context.Context, args []string, out io.Writer) int {
	if len(args) == 0 {
		return 2
	}
	switch args[0] {
	case "inspect-closed-profile":
		return runClosedProfileInspection(ctx, args[1:], out)
	case "accept-epoch", "accept-profile", "inspect":
		if ctx == nil || len(args) != 3 {
			return 2
		}
	default:
		return provisioning.RunNetworkArtifacts(ctx, args, out)
	}
	if args[1] != "--config" {
		return 2
	}
	raw, err := readBounded(args[2], 8<<20)
	if err != nil {
		return 2
	}
	input, err := state.DecodeOfflineDeclaration(raw, args[0])
	if err != nil {
		return 2
	}
	result := state.RunOffline(ctx, args[0], input.Config, input.Epoch, input.Inputs, input.Materials, input.Profile)
	code := 1
	switch result.Outcome {
	case "accepted-epoch", "pending-epoch", "accepted-profile", "observed":
		code = 0
	case "canceled":
		code = 130
	}
	return reportNetwork(out, result.Outcome, result.Profile, result.ObservedAt, code)
}

func reportNetwork(out io.Writer, outcome string, profile *network.ProfileFacts, observed time.Time, code int) int {
	if json.NewEncoder(out).Encode(struct {
		Outcome    string                `json:"outcome"`
		Profile    *network.ProfileFacts `json:"profile,omitempty"`
		ObservedAt time.Time             `json:"observed_at"`
	}{outcome, profile, observed}) != nil {
		return 2
	}
	return code
}
