//go:build linux && network_signing_native

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNetworkSigningInfrastructurePreparation(t *testing.T) {
	parent := signingNativeParent(t)
	binary, err := os.ReadFile(compiledCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	dir, _, files := infrastructureDistribution(t, binary, nil, 1, nil)
	template := filepath.Join(parent, "template")
	if err := os.WriteFile(template, files["ardents-source.service"], 0600); err != nil {
		t.Fatal(err)
	}
	descriptorOut := filepath.Join(parent, "descriptor")
	cmd := exec.CommandContext(t.Context(), compiledCommand(t), "infrastructure", "prepare-descriptor", "infrastructure-fixture", "1", compiledCommand(t), template, descriptorOut)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual public descriptor %s %v", out, err)
	}
	raw, err := os.ReadFile(filepath.Join(descriptorOut, "infrastructure-source.json"))
	if err != nil || !bytes.Equal(raw, files["infrastructure-source.json"]) {
		t.Fatal("independent descriptor oracle", err)
	}
	if err := os.Remove(filepath.Join(dir, "infrastructure-enrollment.json")); err != nil {
		t.Fatal(err)
	}
	inventoryOut := filepath.Join(parent, "inventory")
	cmd = exec.CommandContext(t.Context(), compiledCommand(t), "infrastructure", "prepare-inventory", dir, inventoryOut)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual public inventory %s %v", out, err)
	}
	prepared, err := os.ReadFile(filepath.Join(inventoryOut, "infrastructure-enrollment.json"))
	if err != nil {
		t.Fatal(err)
	}
	oraclePin := writeInfrastructureOracleInventory(t, dir, files, 1)
	if fmt.Sprintf("%x", sha256.Sum256(prepared)) != oraclePin {
		t.Fatal("independent inventory oracle")
	}
	// The independently prepared pin here is a controlled test input, not an
	// operational independent distribution or permission for first execution.
	cmd = exec.CommandContext(t.Context(), filepath.Join(dir, "ardents-node-linux-amd64"), "infrastructure", "authenticate-initial", dir, oraclePin, filepath.Join(parent, "history"), "h3-test", "new-release-test", infrastructureReference.Format("2006-01-02T15:04:05Z07:00"))
	if out, err := cmd.CombinedOutput(); err != nil || !bytes.Contains(out, []byte("authenticated-source-artifact")) {
		t.Fatalf("prepared actual self and Release pair %s %v", out, err)
	}
}

func TestNetworkSigningInfrastructureOriginalCustodyRefusesMutationAndLateClose(t *testing.T) {
	for _, stage := range []string{"before-file", "written", "commit"} {
		t.Run(stage, func(t *testing.T) {
			parent := signingNativeParent(t)
			program, template := filepath.Join(parent, "program"), filepath.Join(parent, "template")
			if err := os.WriteFile(program, []byte("public program"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(template, []byte("public template"), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{"prepare-descriptor", "one", "1", program, template, filepath.Join(parent, "output")}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			observed := false
			raw, err := executeInfrastructurePreparation(ctx, args, func(label string, f *os.File) {
				if label != stage || observed {
					return
				}
				observed = true
				if stage == "written" {
					cancel()
					if err := f.Close(); err != nil {
						t.Error(err)
					}
					return
				}
				if err := os.WriteFile(template, []byte("changed resource"), 0600); err != nil {
					t.Error(err)
				}
			})
			if !observed || err == nil || raw != nil {
				t.Fatal("original custody loss published", observed, err)
			}
			if stage == "written" && (!strings.Contains(err.Error(), "closed") || !strings.Contains(err.Error(), "canceled")) {
				t.Fatal("independent cancellation/physical Close lost", err)
			}
			// Restored bytes cannot adopt partial original output. A fresh output
			// with independently re-admitted inputs supplies the healthy counterpart.
			if err := os.WriteFile(template, []byte("public template"), 0600); err != nil {
				t.Fatal(err)
			}
			if raw, err := executeInfrastructurePreparation(t.Context(), args, nil); err == nil || raw != nil {
				t.Fatal("partial output adopted")
			}
			args[5] = filepath.Join(parent, "fresh")
			if raw, err := executeInfrastructurePreparation(t.Context(), args, nil); err != nil || len(raw) == 0 {
				t.Fatal("fresh healthy counterpart", err)
			}
		})
	}
}
