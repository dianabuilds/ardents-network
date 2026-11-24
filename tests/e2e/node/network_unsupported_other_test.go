//go:build !linux

package node_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNetworkNativePreparationRefusesBeforeEffects(t *testing.T) {
	binary := buildNodeCommand(t)
	parent := t.TempDir()
	input := filepath.Join(parent, "public.json")
	plan := map[string]any{
		"schema": "ardents-initial-node-record-plan-v1", "network_id": "01" + strings.Repeat("00", 31),
		"node_id": "02" + strings.Repeat("00", 31), "valid_from": "2033-05-22T05:20:00Z",
		"valid_until": "2033-05-22T06:20:00Z", "family": "family-one", "endpoint": "127.0.0.1:4401",
		"carrier": "ardents-carrier-tcp-tls-v2", "capacity": 3, "public_key": strings.Repeat("07", 32),
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"prepare-initial-node-record", "prepare-initial-epoch", "prepare-closed-profile", "prepare-initial-keys"} {
		t.Run(operation, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "fresh")
			planPath := input
			if operation == "prepare-initial-keys" {
				planPath = filepath.Join(parent, "missing-plan")
			}
			command := exec.CommandContext(t.Context(), binary, "network", operation, planPath, root)
			var output bytes.Buffer
			command.Stdout = &output
			err := command.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || output.Len() != 0 {
				t.Fatalf("unsupported operation: error=%v, output=%q", err, &output)
			}
			if _, err := os.Stat(root); !os.IsNotExist(err) {
				t.Fatal("unsupported operation created output", err)
			}
		})
	}
}

func TestNetworkSigningUnsupportedPlatformRefusesBeforeEffects(t *testing.T) {
	binary := buildNodeCommand(t)
	for _, purpose := range []string{"sign-initial-node-record", "sign-initial-epoch", "sign-closed-profile", "prepare-node-certificate"} {
		t.Run(purpose, func(t *testing.T) {
			parent := t.TempDir()
			outputPath := filepath.Join(parent, "fresh")
			command := exec.CommandContext(t.Context(), binary, "network", purpose,
				filepath.Join(parent, "missing-plan"), filepath.Join(parent, "missing-prepared"),
				"01"+strings.Repeat("00", 31), filepath.Join(parent, "missing-key"), outputPath)
			var output bytes.Buffer
			command.Stdout = &output
			err := command.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || output.Len() != 0 {
				t.Fatalf("unsupported signing: error=%v, output=%q", err, &output)
			}
			if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
				t.Fatal("unsupported signing created output", err)
			}
		})
	}
}
