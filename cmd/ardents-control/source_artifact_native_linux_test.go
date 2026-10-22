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
	"syscall"
	"testing"
)

// These are offline native output controls, never Release authentication or
// installed Source execution. Root, protected ancestors and actual ext4 are
// required by this explicit native profile; missing prerequisites fail.
func sourceArtifactNativeParent(t *testing.T) string {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: native preparation requires root")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs(parent, &fs); err != nil || fs.Type != 0xef53 {
		t.Fatal("invalid environment: actual ext4 required", err)
	}
	for path := parent; ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 || info.Sys().(*syscall.Stat_t).Uid != 0 {
			t.Fatal("invalid environment: protected original ancestor required", path, err)
		}
		if path == "/" {
			break
		}
	}
	return parent
}

func TestSourcePreparationNativeArtifactBytesAndInventory(t *testing.T) {
	parent := sourceArtifactNativeParent(t)
	command := os.Getenv("ARDENTS_SOURCE_CONTROL_PROGRAM")
	pin := os.Getenv("ARDENTS_SOURCE_CONTROL_PROGRAM_SHA256")
	compiled, err := os.ReadFile(command)
	actualPin := sha256.Sum256(compiled)
	if err != nil || !filepath.IsAbs(command) || len(pin) != 64 || hex.EncodeToString(actualPin[:]) != pin {
		t.Fatal("invalid environment: original independently pinned canonical Control required", err)
	}
	program, template := []byte("public program bytes\n"), []byte("public template bytes\n")
	for name, raw := range map[string][]byte{"program": program, "template": template} {
		if err := os.WriteFile(filepath.Join(parent, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	digest := func(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
	expected := []byte("{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"one\",\"release_version\":1,\"invocation\":\"source\",\"files\":{\"ardents-node-linux-amd64\":\"" + digest(program) + "\",\"ardents-source.service\":\"" + digest(template) + "\"}}\n")
	out := filepath.Join(parent, "descriptor")
	child := exec.CommandContext(t.Context(), command, "source", "prepare-descriptor", "one", "1", filepath.Join(parent, "program"), filepath.Join(parent, "template"), out)
	if output, err := child.CombinedOutput(); err != nil || !bytes.Contains(output, []byte(`"operation":"source.prepare-descriptor"`)) {
		t.Fatal("actual canonical descriptor command", string(output), err)
	}
	onDisk, err := os.ReadFile(filepath.Join(out, "infrastructure-source.json"))
	if err != nil || !bytes.Equal(onDisk, expected) {
		t.Fatal("durable descriptor", err)
	}
	distribution := filepath.Join(parent, "distribution")
	if err := os.Mkdir(distribution, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"ardents-node-linux-amd64": program, "ardents-source.service": template, "infrastructure-source.json": expected,
		"root.json": []byte("{}"), "timestamp.json": []byte("{}"), "snapshot.json": []byte("{}"), "targets.json": []byte("{}")}
	// Deliberately unsigned public fixture metadata: inventory is only byte
	// preparation. It must never be described as a verified Release artifact.
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
	}{"ardents-infrastructure-enrollment-v1", "linux-amd64", "one", 1, "source", hashes})
	if err != nil {
		t.Fatal(err)
	}
	expectedInventory = append(expectedInventory, '\n')
	child = exec.CommandContext(t.Context(), command, "source", "prepare-inventory", distribution, filepath.Join(parent, "inventory"))
	if output, err := child.CombinedOutput(); err != nil || !bytes.Contains(output, []byte(`"operation":"source.prepare-inventory"`)) {
		t.Fatal("actual canonical inventory command", string(output), err)
	}
	actual, err := os.ReadFile(filepath.Join(parent, "inventory", "infrastructure-enrollment.json"))
	if err != nil || !bytes.Equal(actual, expectedInventory) {
		t.Fatal("independent inventory bytes", string(actual), err)
	}
}

func TestSourcePreparationNativeRetainsFailureAndRejectsAdoption(t *testing.T) {
	for _, stage := range []string{"before-file", "written", "commit"} {
		t.Run(stage, func(t *testing.T) {
			parent := sourceArtifactNativeParent(t)
			program, template := filepath.Join(parent, "program"), filepath.Join(parent, "template")
			if err := os.WriteFile(program, []byte("public program"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(template, []byte("public template"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			args := []string{"prepare-descriptor", "one", "1", program, template, filepath.Join(parent, "output")}
			seen := false
			raw, err := prepareSourceArtifact(ctx, args, func(label string, file *os.File) {
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
			if err := os.WriteFile(template, []byte("public template"), 0600); err != nil {
				t.Fatal(err)
			}
			if raw, err := prepareSourceArtifact(t.Context(), args, nil); err == nil || raw != nil {
				t.Fatal("partial output adopted", err)
			}
			args[5] = filepath.Join(parent, "fresh")
			if raw, err := prepareSourceArtifact(t.Context(), args, nil); err != nil || len(raw) == 0 {
				t.Fatal("healthy fresh counterpart", err)
			}
		})
	}
}

func TestSourcePreparationNativeOutputSubstitutionRefuses(t *testing.T) {
	for _, fault := range []string{"file-replacement", "hardlink", "directory-replacement", "late-extra", "closed-sync"} {
		t.Run(fault, func(t *testing.T) {
			parent := sourceArtifactNativeParent(t)
			out := filepath.Join(parent, "output")
			seen := false
			err := persistSourceArtifacts(t.Context(), out, []sourceArtifactFile{{"public", []byte("public bytes")}}, func() error { return nil }, func(stage string, file *os.File) {
				if seen || stage != "file-created" && stage != "before-directory-sync" {
					return
				}
				if (fault == "late-extra" || fault == "closed-sync") != (stage == "before-directory-sync") {
					return
				}
				seen = true
				var err error
				switch fault {
				case "file-replacement":
					err = os.Rename(filepath.Join(out, "public"), filepath.Join(parent, "retained"))
					if err == nil {
						err = os.WriteFile(filepath.Join(out, "public"), []byte("public bytes"), 0600)
					}
				case "hardlink":
					err = os.Link(filepath.Join(out, "public"), filepath.Join(parent, "alias"))
				case "directory-replacement":
					err = os.Rename(out, out+".retained")
					if err == nil {
						err = os.Mkdir(out, 0700)
					}
				case "late-extra":
					err = os.WriteFile(filepath.Join(out, "foreign"), []byte("keep"), 0600)
				case "closed-sync":
					err = file.Close()
				}
				if err != nil {
					t.Error(err)
				}
			})
			if !seen || err == nil {
				t.Fatal("substitution published", seen, err)
			}
		})
	}
}
