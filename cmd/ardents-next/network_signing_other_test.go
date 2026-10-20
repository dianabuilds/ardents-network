//go:build !linux

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNetworkSigningUnsupportedPlatformRefusesBeforeEffects(t *testing.T) {
	for _, purpose := range []string{"sign-initial-node-record", "sign-initial-epoch", "sign-closed-profile", "prepare-node-certificate"} {
		parent := t.TempDir()
		output := filepath.Join(parent, "fresh")
		var receipt bytes.Buffer
		if code := runNetworkSigning(t.Context(), []string{purpose, filepath.Join(parent, "missing-plan"), filepath.Join(parent, "missing-prepared"), preparationIdentifier(1), filepath.Join(parent, "missing-key"), output}, &receipt); code == 0 || receipt.Len() != 0 {
			t.Fatal("unsupported native signing accepted")
		}
		if _, err := os.Stat(output); !os.IsNotExist(err) {
			t.Fatal("unsupported platform created output", err)
		}
	}
}
