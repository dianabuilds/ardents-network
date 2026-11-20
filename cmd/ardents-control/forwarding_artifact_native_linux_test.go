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

// The former request/plan/fixed-template preparation controls retired with the
// transitional Forwarding orchestration (#604 FJD stage D, ADR-0125):
// prepare-request/prepare-plan/prepare-template return typed before-effect
// refusals. The generic descriptor/inventory preparation below remains the
// public Forwarding artifact surface; the canonical platform unit template is
// produced by `ardents-next platform template`.

// These offline controls prepare public bytes, not a signed/accepted candidate
// or installed execution. The existing checked native profile supplies root and
// original protected ext4 custody; absent prerequisites fail.
func TestForwardingPreparationNativeCanonicalCommand(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command := os.Getenv("ARDENTS_SOURCE_CONTROL_PROGRAM")
	pin := os.Getenv("ARDENTS_SOURCE_CONTROL_PROGRAM_SHA256")
	compiled, err := os.ReadFile(command)
	sum := sha256.Sum256(compiled)
	if err != nil || !filepath.IsAbs(command) || len(pin) != 64 || hex.EncodeToString(sum[:]) != pin {
		t.Fatal("invalid environment: independently pinned canonical Control required", err)
	}
	program, template := []byte("public node program\n"), []byte("public forwarding template\n")
	for name, raw := range map[string][]byte{"program": program, "template": template} {
		if err := os.WriteFile(filepath.Join(parent, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	want := []byte("{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":1,\"invocation\":\"forwarding\",\"files\":{\"ardents-forwarding.service\":\"" + digest(template) + "\",\"ardents-node-linux-amd64\":\"" + digest(program) + "\"}}\n")
	outputDirectory := filepath.Join(parent, "descriptor")
	child := exec.CommandContext(t.Context(), command, "forwarding", "prepare-descriptor", "one", "1", filepath.Join(parent, "program"), filepath.Join(parent, "template"), outputDirectory)
	if output, err := child.CombinedOutput(); err != nil || !bytes.Contains(output, []byte(`"operation":"forwarding.prepare-descriptor"`)) {
		t.Fatal("canonical descriptor command", string(output), err)
	}
	got, err := os.ReadFile(filepath.Join(outputDirectory, "infrastructure-forwarding.json"))
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("independent descriptor bytes", string(got), err)
	}
	distribution := filepath.Join(parent, "distribution")
	if err := os.Mkdir(distribution, 0700); err != nil {
		t.Fatal(err)
	}
	// Deliberately unsigned metadata tests only immutable inventory preparation.
	files := map[string][]byte{"ardents-node-linux-amd64": program, "ardents-forwarding.service": template, "infrastructure-forwarding.json": want,
		"root.json": []byte("{}"), "timestamp.json": []byte("{}"), "snapshot.json": []byte("{}"), "targets.json": []byte("{}")}
	hashes := make(map[string]string, len(files))
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(distribution, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
		hashes[name] = digest(raw)
	}
	wantInventory, err := json.Marshal(struct {
		Schema     string            `json:"schema"`
		Platform   string            `json:"platform"`
		Release    string            `json:"release_identity"`
		Version    int               `json:"release_version"`
		Invocation string            `json:"invocation"`
		Files      map[string]string `json:"files"`
	}{"ardents-infrastructure-enrollment-v1", "linux-amd64", "one", 1, "forwarding", hashes})
	if err != nil {
		t.Fatal(err)
	}
	child = exec.CommandContext(t.Context(), command, "forwarding", "prepare-inventory", distribution, filepath.Join(parent, "inventory"))
	if output, err := child.CombinedOutput(); err != nil || !bytes.Contains(output, []byte(`"operation":"forwarding.prepare-inventory"`)) {
		t.Fatal("canonical inventory command", string(output), err)
	}
	got, err = os.ReadFile(filepath.Join(parent, "inventory", "infrastructure-enrollment.json"))
	if err != nil || !bytes.Equal(got, append(wantInventory, '\n')) {
		t.Fatal("independent inventory bytes", string(got), err)
	}
	// Source cannot consume these Forwarding resources; a foreign extra resource
	// also refuses without creating the requested output.
	for _, fault := range []string{"source-consumer", "extra-resource"} {
		args := []string{"forwarding", "prepare-inventory", distribution, filepath.Join(parent, fault)}
		if fault == "source-consumer" {
			args[0] = "source"
		} else if err := os.WriteFile(filepath.Join(distribution, "ardents-source.service"), []byte("foreign"), 0600); err != nil {
			t.Fatal(err)
		}
		if output, err := exec.CommandContext(t.Context(), command, args...).CombinedOutput(); err == nil {
			t.Fatal("foreign inventory accepted", fault, string(output))
		}
		if _, err := os.Lstat(args[3]); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("refusal created output", err)
		}
	}
}

func TestForwardingPreparationNativeRetainsOriginalFailures(t *testing.T) {
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
			raw, err := prepareForwardingArtifact(ctx, args, func(label string, file *os.File) {
				if label != stage || seen {
					return
				}
				seen = true
				if stage == "written" {
					cancel()
					if err := file.Close(); err != nil {
						t.Error(err)
					}
				} else if err := os.WriteFile(template, []byte("changed"), 0600); err != nil {
					t.Error(err)
				}
			})
			if !seen || err == nil || raw != nil {
				t.Fatal("original failure published", seen, err)
			}
			if stage == "written" && (!errors.Is(err, context.Canceled) || !errors.Is(err, os.ErrClosed)) {
				t.Fatal("original cancellation or physical failure lost", err)
			}
			if err := os.WriteFile(template, []byte("public bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			if raw, err := prepareForwardingArtifact(t.Context(), args, nil); err == nil || raw != nil {
				t.Fatal("partial output adopted", err)
			}
		})
	}
}
