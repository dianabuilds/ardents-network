//go:build !linux

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNetworkPreparationUnsupportedNativeEffectsRefused(t *testing.T) {
	for _, operation := range []string{"prepare-initial-node-record", "prepare-initial-epoch", "prepare-closed-profile"} {
		parent := t.TempDir()
		input := filepath.Join(parent, "public.json")
		root := filepath.Join(parent, "fresh")
		if err := os.WriteFile(input, preparationJSON(t, preparationRecordPlan(bytes.Repeat([]byte{7}, 32))), 0600); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		if runNetworkPreparation(t.Context(), []string{operation, input, root}, &out) == 0 || out.Len() != 0 {
			t.Fatal("unsupported native environment reported success")
		}
		if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("unsupported native effect", err)
		}
	}
}
