//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/endpoint"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

// The v2 declaration supplies only storage targets and pins here. Preparation
// is not an accepting runtime-plan check or installed permission.
func runEndpointHistoryPreparation(ctx context.Context, paths []string, diagnostic io.Writer) int {
	if ctx.Err() != nil {
		return 1
	}
	raw, err := readBounded(paths[0], 16<<10)
	if err != nil {
		return 2
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if uniqueJSON(decoder) != nil {
		return 2
	}
	if _, err := decoder.Token(); err != io.EOF {
		return 2
	}
	var plan endpoint.HeadlessPlan
	if json.Unmarshal(raw, &plan) != nil {
		return 2
	}
	bundleRaw, err := readBounded(paths[1], 1<<20)
	if err != nil {
		return 2
	}
	bundle, err := source.DecodeBundle(bundleRaw)
	if err != nil {
		return 2
	}
	profile, err := readBounded(paths[2], 64<<10)
	if err != nil {
		return 2
	}
	outcome, code := "prepared", 0
	if endpoint.PrepareHistories(ctx, plan, bundle, profile) != nil {
		outcome, code = "unavailable", 1
	}
	if json.NewEncoder(diagnostic).Encode(map[string]string{"phase": "history-preparation", "outcome": outcome}) != nil {
		return 1
	}
	return code
}
