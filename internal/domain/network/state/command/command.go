package command

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

const maximumDeclarationBytes = 8 << 20

// Run adapts one local Network intake or inspection to the original State.
func Run(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) != 3 {
		return 2
	}
	if args[1] != "--config" {
		return 2
	}
	raw, err := artifacts.ReadBoundedFile(args[2], maximumDeclarationBytes)
	if err != nil {
		return 2
	}
	input, err := decodeDeclaration(raw, args[0])
	if err != nil {
		return 2
	}
	result := runOffline(ctx, args[0], input)
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
