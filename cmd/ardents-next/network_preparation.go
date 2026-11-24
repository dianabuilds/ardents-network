package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/node/provisioning"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

func runNetworkPreparation(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) != 3 || !provisioning.IsNetworkPreparation(args[0]) || !pathgrammar.Strict(args[1]) || !pathgrammar.Strict(args[2]) {
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

func runNetworkSigning(ctx context.Context, args []string, out io.Writer) int {
	// operation, public-plan, exact-prepared-artifact, reviewed-sha256,
	// original-private-key, fresh-output-directory. No secret bytes in argv.
	if ctx == nil || len(args) != 6 || !provisioning.IsNetworkSigning(args[0]) {
		return 2
	}
	for _, i := range []int{1, 2, 4, 5} {
		if !pathgrammar.Strict(args[i]) {
			return 2
		}
	}
	if !canonicaldigest.Valid(args[3]) {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	result, err := provisioning.SignNetwork(ctx, args, nil)
	if err != nil {
		return 1
	}
	if result.CheckTime(time.Now()) != nil {
		return 1
	}
	receipt, err := json.Marshal(struct {
		Schema    string `json:"schema"`
		Operation string `json:"operation"`
		Digest    string `json:"signed_sha256"`
		Verified  bool   `json:"signatures_verified"`
		State     bool   `json:"state_accepted"`
		Installed bool   `json:"installed_qualification"`
	}{"ardents-network-signed-v1", args[0], hex.EncodeToString(result.Digest[:]), true, false, false})
	if err != nil || writeNetworkPreparationReceipt(ctx, out, append(receipt, '\n')) != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}
	if result.CheckTime(time.Now()) != nil {
		return 1
	}
	return 0
}

func runInitialKeys(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) != 3 || args[0] != "prepare-initial-keys" {
		return 2
	}
	for _, path := range args[1:] {
		if !pathgrammar.Strict(path) || filepath.Dir(path) == path {
			return 2
		}
	}
	if ctx.Err() != nil {
		return 130
	}
	receipt, err := provisioning.PrepareInitialNetworkKeys(ctx, args[1], args[2])
	if err != nil {
		return 1
	}
	if writeNetworkPreparationReceipt(ctx, out, receipt) != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}
	return 0
}
