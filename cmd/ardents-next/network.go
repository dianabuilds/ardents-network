package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// Each operation owns its root lease. Epoch intake and profile acceptance are
// separate durable transitions; neither an intake receipt nor an inspection
// JSON file substitutes for a live opened Network observer in Admission.
func runNetwork(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) != 3 || (args[0] != "accept-epoch" && args[0] != "accept-profile" && args[0] != "inspect") {
		return 2
	}
	var input struct {
		Network   *networkAuthorityPlan `json:"network"`
		Epoch     []byte                `json:"epoch,omitempty"`
		Inputs    [][]byte              `json:"inputs,omitempty"`
		Materials [][]byte              `json:"materials,omitempty"`
		Profile   []byte                `json:"profile,omitempty"`
	}
	if admissionConfigBounded(args[1:], &input, 8<<20) != nil {
		return 2
	}
	if (args[0] != "accept-epoch" && (len(input.Epoch) != 0 || len(input.Inputs) != 0 || len(input.Materials) != 0)) ||
		(args[0] != "accept-profile" && len(input.Profile) != 0) {
		return 2
	}
	config, err := networkStateConfig(input.Network)
	if err != nil {
		return 2
	}
	owner, err := state.Open(config)
	if err != nil {
		return reportNetwork(out, "owner-unavailable", nil, time.Time{}, 1)
	}
	outcome, code := "unavailable", 1
	var facts *network.ProfileFacts
	var observed time.Time
	switch args[0] {
	case "accept-epoch":
		_, err = owner.Accept(ctx, input.Epoch, input.Inputs, input.Materials)
		if err == nil {
			outcome, code = "accepted-epoch", 0
		}
	case "accept-profile":
		if ctx.Err() == nil {
			_, err = owner.AcceptClosedProfile(input.Profile)
		} else {
			err = ctx.Err()
		}
		if err == nil {
			outcome, code = "accepted-profile", 0
		}
	case "inspect":
		var view network.RuntimeView
		view, err = owner.CurrentRuntime()
		if err == nil {
			value := view.Profile()
			facts = &value
			observed = view.ObservedAt()
			outcome, code = "observed", 0
		}
	}
	// Cancellation or failed closure suppresses a successful command receipt.
	if errors.Join(err, ctx.Err(), owner.Close()) != nil {
		facts, observed, outcome, code = nil, time.Time{}, "unavailable", 1
		if ctx.Err() != nil {
			outcome, code = "canceled", 130
		}
	}
	return reportNetwork(out, outcome, facts, observed, code)
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
