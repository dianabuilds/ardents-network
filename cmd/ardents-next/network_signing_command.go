package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"time"
)

var errNetworkSigning = errors.New("offline Network signing unavailable")

func isNetworkSigning(operation string) bool {
	return operation == "sign-initial-node-record" || operation == "sign-initial-epoch" || operation == "sign-closed-profile" || operation == "prepare-node-certificate"
}

type networkSigningResult struct {
	digest   [32]byte
	interval networkSigningInterval
}

type networkSigningInterval struct{ reference, from, until time.Time }

func (s networkSigningInterval) checkTime(now time.Time) error {
	if s.reference.Before(now.Add(-5*time.Minute)) || s.reference.After(now.Add(time.Minute)) ||
		s.reference.Before(s.from) || !s.reference.Before(s.until) || now.Before(s.from) || !now.Before(s.until) || s.until.Sub(s.from) > 7*24*time.Hour {
		return errNetworkSigning
	}
	return nil
}

func runNetworkSigning(ctx context.Context, args []string, out io.Writer) int {
	// operation, public-plan, exact-prepared-artifact, reviewed-sha256,
	// original-private-key, fresh-output-directory. No secret bytes in argv.
	if ctx == nil || len(args) != 6 || !isNetworkSigning(args[0]) {
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
	result, err := executeNetworkSigning(ctx, args)
	if err != nil {
		return 1
	}
	if result.interval.checkTime(time.Now()) != nil {
		return 1
	}
	receipt, err := json.Marshal(struct {
		Schema    string `json:"schema"`
		Operation string `json:"operation"`
		Digest    string `json:"signed_sha256"`
		Verified  bool   `json:"signatures_verified"`
		State     bool   `json:"state_accepted"`
		Installed bool   `json:"installed_qualification"`
	}{"ardents-network-signed-v1", args[0], hex.EncodeToString(result.digest[:]), true, false, false})
	if err != nil || writeNetworkPreparationReceipt(ctx, out, append(receipt, '\n')) != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}
	if result.interval.checkTime(time.Now()) != nil {
		return 1
	}
	return 0
}
