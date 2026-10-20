package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
)

var errInitialKeys = errors.New("initial Network key preparation unavailable")

type initialKeyPublic struct {
	Purpose string `json:"purpose"`
	Index   *int   `json:"index,omitempty"`
	File    string `json:"file"`
	Public  string `json:"ed25519_public_hex"`
	Digest  string `json:"public_sha256"`
}

type initialKeyReceipt struct {
	Schema      string             `json:"schema"`
	Operation   string             `json:"operation"`
	Keys        []initialKeyPublic `json:"keys"`
	Signed      bool               `json:"signed"`
	State       bool               `json:"state_accepted"`
	Installed   bool               `json:"installed_qualification"`
	Independent bool               `json:"independent_custody"`
}

func runInitialKeys(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) != 3 || args[0] != "prepare-initial-keys" {
		return 2
	}
	for _, path := range args[1:] {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Dir(path) == path {
			return 2
		}
	}
	if ctx.Err() != nil {
		return 130
	}
	receipt, err := executeInitialKeys(ctx, args[1], args[2])
	if err != nil {
		return 1
	}
	raw, err := json.Marshal(receipt)
	if err != nil || writeNetworkPreparationReceipt(ctx, out, append(raw, '\n')) != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}
	return 0
}
