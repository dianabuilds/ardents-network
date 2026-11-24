//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func runSourcePreparation(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) != 3 || args[0] != "prepare-trust" {
		return 2
	}
	for _, path := range args[1:] {
		if !absoluteAdmissionPath(path) {
			return 2
		}
	}
	if ctx.Err() != nil {
		return 130
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	files, err := endpoint.PrepareSourceTrust(ctx, args[1], args[2], nil)
	if err != nil {
		return 1
	}
	// Local plans include opaque private paths; the receipt contains names and
	// public commitments only, after every original native resource has joined.
	receipt, err := json.Marshal(struct {
		Schema     string            `json:"schema"`
		Files      map[string]string `json:"files"`
		Validation string            `json:"validation"`
	}{"ardents-source-trust-prepared-v1", files, "offline-public-observation"})
	if err != nil || writeNetworkPreparationReceipt(ctx, out, append(receipt, '\n')) != nil || ctx.Err() != nil {
		return 1
	}
	return 0
}

func runSourceStatePreparation(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if ctx == nil || len(args) != 2 {
		return 2
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := state.PrepareSourceStateFiles(ctx, args[0], args[1]); err != nil {
		owner := "state"
		if errors.Is(err, state.ErrSourcePreparationPrincipal) {
			owner = "input"
		}
		return sourceStartupFailure(diagnostic, owner, err, nil)
	}
	const receipt = "{\"operation\":\"source.prepare-state\",\"outcome\":\"state-prepared\"}\n"
	if artifacts.WriteReport(ctx, out, []byte(receipt)) != nil {
		return 1
	}
	return 0
}
