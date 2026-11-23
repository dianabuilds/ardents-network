package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node/provisioning"
)

func runNetworkSigning(ctx context.Context, args []string, out io.Writer) int {
	// operation, public-plan, exact-prepared-artifact, reviewed-sha256,
	// original-private-key, fresh-output-directory. No secret bytes in argv.
	if ctx == nil || len(args) != 6 || !provisioning.IsNetworkSigning(args[0]) {
		return 2
	}
	for _, i := range []int{1, 2, 4, 5} {
		if !filepath.IsAbs(args[i]) || filepath.Clean(args[i]) != args[i] {
			return 2
		}
	}
	if _, err := admissionHex(args[3], 32); err != nil {
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
