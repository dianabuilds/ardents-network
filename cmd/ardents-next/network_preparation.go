package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/node/provisioning"
)

func runNetworkPreparation(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) != 3 || !provisioning.IsNetworkPreparation(args[0]) || !filepath.IsAbs(args[1]) || filepath.Clean(args[1]) != args[1] || !filepath.IsAbs(args[2]) || filepath.Clean(args[2]) != args[2] {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	prepared, err := provisioning.PrepareNetworkArtifact(ctx, args[0], args[1], args[2])
	if err != nil {
		if errors.Is(err, provisioning.ErrNetworkPreparationOutput) {
			return 1
		}
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	// Grammar preparation is deliberately weaker than signed State acceptance.
	receipt, err := json.Marshal(struct {
		Schema        string `json:"schema"`
		Operation     string `json:"operation"`
		Artifact      string `json:"artifact"`
		SHA256        string `json:"unsigned_sha256"`
		Signed        bool   `json:"signed"`
		StateAccepted bool   `json:"state_accepted"`
		Validation    string `json:"validation"`
	}{"ardents-network-prepared-v1", args[0], prepared.Name, hex.EncodeToString(prepared.Digest[:]), false, false, "public-grammar-only"})
	if err != nil {
		return 1
	}
	if writeNetworkPreparationReceipt(ctx, out, append(receipt, '\n')) != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}
	return 0
}
