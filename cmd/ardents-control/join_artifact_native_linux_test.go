//go:build linux && network_signing_native

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// This profile proves offline byte/output custody, not signatures, JOIN startup
// or runtime. Public fixture metadata intentionally supplies no Release proof.
func TestJoinPreparationNativeCanonicalBytesAndInventory(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command := os.Getenv("ARDENTS_JOIN_CONTROL_PROGRAM")
	pin := os.Getenv("ARDENTS_JOIN_CONTROL_PROGRAM_SHA256")
	compiled, err := os.ReadFile(command)
	actualPin := sha256.Sum256(compiled)
	if err != nil || !filepath.IsAbs(command) || len(pin) != 64 || hex.EncodeToString(actualPin[:]) != pin {
		t.Fatal("invalid environment: independently pinned canonical Control required", err)
	}
	program, template := []byte("public JOIN program\n"), []byte("public JOIN template\n")
	for name, raw := range map[string][]byte{"program": program, "template": template} {
		if err := os.WriteFile(filepath.Join(parent, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	expected := []byte("{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":1,\"invocation\":\"join\",\"files\":{\"ardents-join.service\":\"" + digest(template) + "\",\"ardents-node-linux-amd64\":\"" + digest(program) + "\"}}\n")
	call := func(args ...string) []byte {
		t.Helper()
		output, err := exec.CommandContext(t.Context(), command, append([]string{"join"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal("actual canonical JOIN preparation", string(output), err)
		}
		return output
	}
	out := filepath.Join(parent, "descriptor")
	if output := call("prepare-descriptor", "one", "1", filepath.Join(parent, "program"), filepath.Join(parent, "template"), out); !bytes.Contains(output, []byte(`"operation":"join.prepare-descriptor"`)) {
		t.Fatal("JOIN descriptor receipt", string(output))
	}
	onDisk, err := os.ReadFile(filepath.Join(out, "infrastructure-join.json"))
	if err != nil || !bytes.Equal(onDisk, expected) {
		t.Fatal("independent durable JOIN descriptor", string(onDisk), err)
	}
	distribution := filepath.Join(parent, "distribution")
	if err := os.Mkdir(distribution, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"ardents-node-linux-amd64": program, "ardents-join.service": template, "infrastructure-join.json": expected,
		"root.json": []byte("{}"), "timestamp.json": []byte("{}"), "snapshot.json": []byte("{}"), "targets.json": []byte("{}")}
	hashes := make(map[string]string, len(files))
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(distribution, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
		hashes[name] = digest(raw)
	}
	expectedInventory, err := json.Marshal(struct {
		Schema     string            `json:"schema"`
		Platform   string            `json:"platform"`
		Release    string            `json:"release_identity"`
		Version    int               `json:"release_version"`
		Invocation string            `json:"invocation"`
		Files      map[string]string `json:"files"`
	}{"ardents-infrastructure-enrollment-v1", "linux-amd64", "one", 1, "join", hashes})
	if err != nil {
		t.Fatal(err)
	}
	if output := call("prepare-inventory", distribution, filepath.Join(parent, "inventory")); !bytes.Contains(output, []byte(`"operation":"join.prepare-inventory"`)) {
		t.Fatal("JOIN inventory receipt", string(output))
	}
	actual, err := os.ReadFile(filepath.Join(parent, "inventory", "infrastructure-enrollment.json"))
	if err != nil || !bytes.Equal(actual, append(expectedInventory, '\n')) {
		t.Fatal("independent durable JOIN inventory", string(actual), err)
	}
	for _, fault := range []string{"extra", "foreign-unit", "changed-program", "foreign-role"} {
		t.Run(fault, func(t *testing.T) {
			candidate := filepath.Join(parent, fault)
			if err := os.Mkdir(candidate, 0700); err != nil {
				t.Fatal(err)
			}
			for name, raw := range files {
				if fault == "foreign-unit" && name == "ardents-join.service" {
					name = "ardents-forwarding.service"
				}
				if fault == "changed-program" && name == "ardents-node-linux-amd64" {
					raw = []byte("changed")
				}
				if fault == "foreign-role" && name == "infrastructure-join.json" {
					raw = bytes.Replace(raw, []byte(`"join"`), []byte(`"forwarding"`), 1)
				}
				if err := os.WriteFile(filepath.Join(candidate, name), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if fault == "extra" {
				if err := os.WriteFile(filepath.Join(candidate, "extra"), []byte("extra"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			destination := filepath.Join(parent, fault+"-output")
			if output, err := exec.CommandContext(t.Context(), command, "join", "prepare-inventory", candidate, destination).CombinedOutput(); err == nil || bytes.Contains(output, []byte(`"sha256"`)) {
				t.Fatal("invalid JOIN inventory published", string(output), err)
			}
			if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("refusal mutated output", err)
			}
		})
	}
}

func TestJoinPreparationNativeRetainsFailureAndRejectsAdoption(t *testing.T) {
	for _, stage := range []string{"before-file", "written", "commit"} {
		t.Run(stage, func(t *testing.T) {
			parent := sourceArtifactNativeParent(t)
			program, template := filepath.Join(parent, "program"), filepath.Join(parent, "template")
			for _, path := range []string{program, template} {
				if err := os.WriteFile(path, []byte("public bytes"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			args := []string{"prepare-descriptor", "one", "1", program, template, filepath.Join(parent, "output")}
			seen := false
			raw, err := prepareJoinArtifact(ctx, args, func(label string, file *os.File) {
				if label != stage || seen {
					return
				}
				seen = true
				if stage == "written" {
					cancel()
					if err := file.Close(); err != nil {
						t.Error(err)
					}
					return
				}
				if err := os.WriteFile(template, []byte("changed original"), 0600); err != nil {
					t.Error(err)
				}
			})
			if !seen || err == nil || raw != nil {
				t.Fatal("original failure published", seen, err)
			}
			if stage == "written" && (!errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed)) {
				t.Fatal("original cancellation/physical failure lost", err)
			}
			if err := os.WriteFile(template, []byte("public bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			if raw, err := prepareJoinArtifact(t.Context(), args, nil); err == nil || raw != nil {
				t.Fatal("partial output adopted", err)
			}
			args[5] = filepath.Join(parent, "fresh")
			if raw, err := prepareJoinArtifact(t.Context(), args, nil); err != nil || len(raw) == 0 {
				t.Fatal("fresh positive counterpart", err)
			}
		})
	}
}
