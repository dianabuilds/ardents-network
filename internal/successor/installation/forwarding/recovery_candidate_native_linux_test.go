//go:build network_signing_native

package forwarding

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// Actual protected historical reads and complete public byte grammar only.
// These fixtures provide no signed Release, original cut or recovery authority.
func TestForwardingRecoveryPublicationNativeMetadataContinuity(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-forwarding-preparation-native-stand")
	if err != nil || string(marker) != "ardents-forwarding-preparation-native-v1\n" || os.Geteuid() != 0 || os.Getegid() != 0 {
		t.Fatal("invalid disposable root native stand", err)
	}
	b, selected, original := selectedGenerationVector(t)
	fresh := map[string][]byte{}
	for name, raw := range original {
		fresh[name] = bytes.Clone(raw)
	}
	for _, name := range []string{"root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		fresh[name] = append([]byte("advanced "), original[name]...)
		fresh["infrastructure-enrollment.json"] = bytes.ReplaceAll(fresh["infrastructure-enrollment.json"], []byte(digest(original[name])), []byte(digest(fresh[name])))
	}
	newBinding := b
	newBinding.TargetsSHA256 = digest(fresh["targets.json"])
	newBinding.InventorySHA256 = digest(fresh["infrastructure-enrollment.json"])
	newSelected, err := json.Marshal(newBinding)
	if err != nil {
		t.Fatal(err)
	}
	newSelected = append(newSelected, '\n')
	fresh["binding.json"] = bytes.Clone(newSelected)
	if _, err := validateSelectedGeneration(newBinding, newSelected, fresh); err != nil {
		t.Fatal("independent complete advanced public byte vector", err)
	}
	if _, err := validateSelectedGeneration(b, selected, original); err != nil {
		t.Fatal("independent original public byte vector", err)
	}
	fresh["binding.json"] = bytes.Clone(selected)
	parent, err := os.MkdirTemp(os.TempDir(), "frmetadata-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
	})
	for _, name := range []string{"root.json", "timestamp.json", "snapshot.json", "targets.json", "infrastructure-enrollment.json"} {
		file, err := os.OpenFile(filepath.Join(parent, name), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		n, writeErr := file.Write(original[name])
		if err := errors.Join(writeErr, file.Chmod(0444), file.Sync(), file.Close()); err != nil || n != len(original[name]) {
			t.Fatal("original metadata fixture", name, err)
		}
	}
	if err := restoreRecoveryMetadata(t.Context(), parent, fresh); err != nil {
		t.Fatal("actual historical metadata reads", err)
	}
	if _, err := validateSelectedGeneration(b, selected, fresh); err != nil {
		t.Fatal("original generation rejected after current metadata advanced", err)
	}
	for _, name := range forwardingGenerationNames() {
		if !bytes.Equal(fresh[name], original[name]) {
			t.Fatal("original bytes replaced by current metadata", name)
		}
	}
	copyFiles := func() map[string][]byte {
		result := map[string][]byte{}
		for name, raw := range original {
			result[name] = bytes.Clone(raw)
		}
		return result
	}
	inventoryPath := filepath.Join(parent, "infrastructure-enrollment.json")
	if err := os.Remove(inventoryPath); err != nil {
		t.Fatal(err)
	}
	if err := restoreRecoveryMetadata(t.Context(), parent, copyFiles()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("missing original inventory fell back to detached/current bytes", err)
	}
	foreign := []byte("foreign original inventory fixture\n")
	file, err := os.OpenFile(inventoryPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	n, writeErr := file.Write(foreign)
	if err := errors.Join(writeErr, file.Chmod(0444), file.Sync(), file.Close()); err != nil || n != len(foreign) {
		t.Fatal(err)
	}
	changed := copyFiles()
	if err := restoreRecoveryMetadata(t.Context(), parent, changed); err != nil {
		t.Fatal("independent substituted-byte observation", err)
	}
	if _, err := validateSelectedGeneration(b, selected, changed); err == nil {
		t.Fatal("substituted original inventory became generation provenance")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := restoreRecoveryMetadata(ctx, parent, copyFiles()); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled historical read accepted", err)
	}
	for _, name := range []string{"root.json", "timestamp.json", "snapshot.json", "targets.json", "infrastructure-enrollment.json"} {
		body, err := os.ReadFile(filepath.Join(parent, name))
		wanted := original[name]
		if name == "infrastructure-enrollment.json" {
			wanted = foreign
		}
		if err != nil || !bytes.Equal(body, wanted) {
			t.Fatal("historical read rewrote original metadata", name, err)
		}
	}
}

// Independently ordered original artifact groups and real empty/ready leaves.
// Canonical Enrollment bytes plus the original binding are the byte oracle;
// these fixtures grant no original-cut, Release or native continuation right.
func TestForwardingRecoveryPublicationNativeGenerationProposals(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-forwarding-preparation-native-stand")
	if err != nil || string(marker) != "ardents-forwarding-preparation-native-v1\n" || os.Geteuid() != 0 || os.Getegid() != 0 {
		t.Fatal("invalid disposable root native stand", err)
	}
	b, selected, original := selectedGenerationVector(t)
	names := []string{
		"ardents-forwarding.service", "ardents-node-linux-amd64", "binding.json",
		"forwarding-plan.json", "forwarding-request.json", "infrastructure-enrollment.json",
		"infrastructure-forwarding.json", "rendered-forwarding.service", "root.json",
		"snapshot.json", "targets.json", "timestamp.json",
	}
	metadata := map[string]bool{"infrastructure-enrollment.json": true, "root.json": true, "snapshot.json": true, "targets.json": true, "timestamp.json": true}
	for records := uint32(72); records <= 107; records++ {
		faults := []string{"matching", "advanced-proposal", "advanced-pair", "canceled"}
		if records >= 90 {
			faults = append(faults, "missing-ready", "changed-ready")
		}
		for _, fault := range faults {
			t.Run(strconv.Itoa(int(records))+"/"+fault, func(t *testing.T) {
				parent, err := os.MkdirTemp(os.TempDir(), "frgeneration-metadata-")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.RemoveAll(parent); err != nil {
						t.Error(err)
					}
				})
				files := map[string][]byte{}
				for name, raw := range original {
					files[name] = bytes.Clone(raw)
				}
				before := map[string][]byte{}
				identities := map[string]os.FileInfo{}
				for i, name := range names {
					// Count completed triples from the literal original sequence.
					written := int(records)-72 >= (i+1)*3
					emptyBirth := int(records)-72 == i*3+2
					if written && metadata[name] {
						files[name] = append([]byte("fresh detached "), original[name]...)
					}
					if !written && !emptyBirth || fault == "missing-ready" && name == "infrastructure-enrollment.json" {
						continue
					}
					body := []byte{}
					mode := os.FileMode(0600)
					if written {
						body, mode = bytes.Clone(original[name]), 0444
						if name == "ardents-node-linux-amd64" {
							mode = 0555
						}
						if fault == "changed-ready" && name == "infrastructure-enrollment.json" {
							body = append([]byte("foreign original "), body...)
						}
					}
					file, err := os.OpenFile(filepath.Join(parent, name), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
					if err != nil {
						t.Fatal(err)
					}
					n, writeErr := file.Write(body)
					if err := errors.Join(writeErr, file.Chmod(mode), file.Sync(), file.Close()); err != nil || n != len(body) {
						t.Fatal("original prefix file", name, err)
					}
					before[name] = body
					identities[name], err = os.Lstat(filepath.Join(parent, name))
					if err != nil {
						t.Fatal(err)
					}
				}
				if fault == "advanced-proposal" || fault == "advanced-pair" {
					files["timestamp.json"] = append([]byte("advanced missing timestamp "), original["timestamp.json"]...)
					if fault == "advanced-pair" {
						files["infrastructure-enrollment.json"] = bytes.ReplaceAll(files["infrastructure-enrollment.json"], []byte(digest(original["timestamp.json"])), []byte(digest(files["timestamp.json"])))
					}
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				if fault == "canceled" {
					cancel()
				}
				err = restoreRecoveryInitialMetadata(ctx, parent, files, records)
				switch fault {
				case "canceled":
					if !errors.Is(err, context.Canceled) {
						t.Fatal("canceled proposal/read accepted", err)
					}
				case "missing-ready":
					if !errors.Is(err, os.ErrNotExist) {
						t.Fatal("missing original ready inventory used a proposal", err)
					}
				default:
					if err != nil {
						t.Fatal("original ready-only historical observation", err)
					}
					_, validationErr := validateSelectedGeneration(b, selected, files)
					if (validationErr == nil) != (fault == "matching") {
						t.Fatal("proposals escaped original canonical inventory/binding", validationErr)
					}
					if fault == "matching" {
						for name, wanted := range original {
							if !bytes.Equal(files[name], wanted) {
								t.Fatal("original complete byte vector changed", name)
							}
						}
					}
				}
				entries, err := os.ReadDir(parent)
				if err != nil || len(entries) != len(before) {
					t.Fatal("proposal/read created or removed a prefix leaf", err)
				}
				for name, wanted := range before {
					body, readErr := os.ReadFile(filepath.Join(parent, name))
					info, statErr := os.Lstat(filepath.Join(parent, name))
					if readErr != nil || statErr != nil || !bytes.Equal(body, wanted) || !os.SameFile(identities[name], info) || identities[name].Mode() != info.Mode() {
						t.Fatal("proposal/read mutated original prefix custody", name, readErr, statErr)
					}
				}
			})
		}
	}
	for _, count := range []uint32{0, 71, 108, 120} {
		if err := restoreRecoveryInitialMetadata(t.Context(), "/unopened-original-generation", original, count); !errors.Is(err, ErrBinding) {
			t.Fatal("unselected prefix accepted by metadata read mechanism", count, err)
		}
	}
}

// Public byte commitments and actual historical reads only. A matching
// proposal does not supply an original cut or authorize timestamp creation.
func TestForwardingRecoveryPublicationNativeTimestampProposal(t *testing.T) {
	marker, err := os.ReadFile("/etc/ardents-forwarding-preparation-native-stand")
	if err != nil || string(marker) != "ardents-forwarding-preparation-native-v1\n" || os.Geteuid() != 0 || os.Getegid() != 0 {
		t.Fatal("invalid disposable root native stand", err)
	}
	b, selected, original := selectedGenerationVector(t)
	for _, fault := range []string{"matching", "advanced-proposal", "missing-inventory", "changed-inventory", "canceled"} {
		t.Run(fault, func(t *testing.T) {
			parent, err := os.MkdirTemp(os.TempDir(), "frtimestamp-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.RemoveAll(parent); err != nil {
					t.Error(err)
				}
			})
			files := map[string][]byte{}
			for name, raw := range original {
				files[name] = bytes.Clone(raw)
			}
			advanced := append([]byte("different timestamp "), original["timestamp.json"]...)
			if fault == "advanced-proposal" || fault == "changed-inventory" {
				files["timestamp.json"] = bytes.Clone(advanced)
			}
			proposal := bytes.Clone(files["timestamp.json"])
			historical := map[string][]byte{}
			for _, name := range []string{"root.json", "snapshot.json", "targets.json", "infrastructure-enrollment.json"} {
				if fault == "missing-inventory" && name == "infrastructure-enrollment.json" {
					continue
				}
				body := original[name]
				if fault == "changed-inventory" && name == "infrastructure-enrollment.json" {
					body = bytes.ReplaceAll(body, []byte(digest(original["timestamp.json"])), []byte(digest(advanced)))
				}
				historical[name] = bytes.Clone(body)
				file, err := os.OpenFile(filepath.Join(parent, name), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
				if err != nil {
					t.Fatal(err)
				}
				n, writeErr := file.Write(body)
				if err := errors.Join(writeErr, file.Chmod(0444), file.Sync(), file.Close()); err != nil || n != len(body) {
					t.Fatal("original timestamp-input fixture", name, err)
				}
				// Detached current metadata cannot replace the retained original.
				files[name] = append([]byte("advanced detached "), body...)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if fault == "canceled" {
				cancel()
			}
			err = restoreRecoveryInitialMetadata(ctx, parent, files, 107)
			switch fault {
			case "missing-inventory":
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatal("missing original inventory used detached fallback", err)
				}
			case "canceled":
				if !errors.Is(err, context.Canceled) {
					t.Fatal("canceled original read accepted", err)
				}
			default:
				if err != nil {
					t.Fatal("actual four-file historical read", err)
				}
				_, validationErr := validateSelectedGeneration(b, selected, files)
				if (validationErr == nil) != (fault == "matching") {
					t.Fatal("proposal did not require exact original inventory/binding", validationErr)
				}
			}
			if !bytes.Equal(files["timestamp.json"], proposal) {
				t.Fatal("historical read changed proposed timestamp")
			}
			if _, err := os.Lstat(filepath.Join(parent, "timestamp.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("byte proposal created or adopted a timestamp", err)
			}
			for name, wanted := range historical {
				body, err := os.ReadFile(filepath.Join(parent, name))
				if err != nil || !bytes.Equal(body, wanted) {
					t.Fatal("read modified original timestamp inputs", name, err)
				}
			}
		})
	}
}
