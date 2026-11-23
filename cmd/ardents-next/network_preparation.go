package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/node/provisioning"
)

const maximumNetworkPreparationPlan = 8 << 20

var errNetworkPreparation = errors.New("network public preparation unavailable")

func isNetworkPreparation(operation string) bool {
	return operation == "prepare-initial-node-record" || operation == "prepare-initial-epoch" || operation == "prepare-closed-profile"
}

func prepareNetworkPublic(operation string, raw []byte) (string, []byte, error) {
	switch operation {
	case "prepare-initial-node-record":
		_, message, err := provisioning.PrepareInitialNodeRecord(raw)
		return "node-record.signing-input", message, err
	case "prepare-initial-epoch":
		_, message, err := provisioning.PrepareInitialEpoch(raw)
		return "epoch.unsigned", message, err
	case "prepare-closed-profile":
		_, message, err := provisioning.PrepareClosedNetworkProfile(raw)
		return "profile.unsigned", message, err
	default:
		return "", nil, errNetworkPreparation
	}
}

func runNetworkPreparation(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) != 3 || !isNetworkPreparation(args[0]) || !filepath.IsAbs(args[1]) || filepath.Clean(args[1]) != args[1] || !filepath.IsAbs(args[2]) || filepath.Clean(args[2]) != args[2] {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	raw, err := readNetworkPreparation(ctx, args[1])
	if err != nil || len(raw) > maximumNetworkPreparationPlan {
		return 2
	}
	name, message, err := prepareNetworkPublic(args[0], raw)
	if err != nil {
		return 2
	}
	if err = writeNetworkPreparation(ctx, args[2], name, message); err != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}
	digest := sha256.Sum256(message)
	// Grammar preparation is deliberately weaker than signed State acceptance.
	receipt, err := json.Marshal(struct {
		Schema        string `json:"schema"`
		Operation     string `json:"operation"`
		Artifact      string `json:"artifact"`
		SHA256        string `json:"unsigned_sha256"`
		Signed        bool   `json:"signed"`
		StateAccepted bool   `json:"state_accepted"`
		Validation    string `json:"validation"`
	}{"ardents-network-prepared-v1", args[0], name, hex.EncodeToString(digest[:]), false, false, "public-grammar-only"})
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
