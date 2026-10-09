package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/publication/durable"
)

// Preparation reconciles only durable public history. A ready result is never
// emitted by this operation: Instance and live qualified Publisher are separate.
func runPublication(ctx context.Context, args []string, out io.Writer) int {
	if len(args) != 4 || args[0] != "prepare-root" {
		return enrollmentReport(out, "publication-invalid", 2)
	}
	targetRaw, err := admissionHex(args[2], 32)
	if err != nil {
		return enrollmentReport(out, "publication-invalid", 2)
	}
	networkRaw, err := admissionHex(args[3], 32)
	if err != nil {
		return enrollmentReport(out, "publication-invalid", 2)
	}
	var target, network [32]byte
	copy(target[:], targetRaw)
	copy(network[:], networkRaw)
	root, err := durable.Open(ctx, durable.Config{Root: args[1], Target: target, Network: network})
	if err != nil {
		return enrollmentReport(out, "publication-unavailable", 1)
	}
	floor, floorErr := root.Floor(ctx)
	if closeErr := root.Close(); floorErr != nil || closeErr != nil || ctx.Err() != nil {
		return enrollmentReport(out, "publication-unavailable", 1)
	}
	if json.NewEncoder(out).Encode(struct {
		Outcome string `json:"outcome"`
		Floor   uint64 `json:"floor"`
	}{"publication-root-prepared", floor}) != nil {
		return 2
	}
	return 0
}
