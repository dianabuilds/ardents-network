//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The actual canonical binary is inspected as ELF only. No Source command,
// Release admission, installed stand or successful semantic owner is fabricated.
func TestSourceCutObserverProfileInspectionNative(t *testing.T) {
	program := os.Getenv("ARDENTS_OBSERVER_INSPECT_PROGRAM")
	if !filepath.IsAbs(program) {
		t.Fatal("actual offline canonical program required")
	}
	raw, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	root := filepath.Join(t.TempDir(), "source")
	zero := strings.Repeat("0", 64) // Format fixture only, not an accepted receipt.
	profile := sourceCutObserverProfile{Schema: "source-original-cut-observer-v1", ProgramSHA256: hex.EncodeToString(sum[:]), PublicReceiptSHA256: zero,
		Arguments: []string{"installation", "install-initial-stopped", filepath.Dir(program), zero, "/testtmp/history", "alpha", "ardents-source-qualification", "/testtmp/request"},
		Journal:   root + ".source-initial-journal", PrefixRecords: 70, LastPhase: "file-born"}
	encoded, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	load := func(t *testing.T, body []byte) (*sourceCutObserverProfile, error) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "profile.json")
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		pin := sha256.Sum256(body)
		return readSourceCutObserverProfile(path, hex.EncodeToString(pin[:]), program, root)
	}
	admitted, err := load(t, encoded)
	if err != nil {
		t.Fatal(err)
	}
	result, err := observeSourceCut(program, root, "", "external-cut", admitted, true, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.InspectionOnly || result.InstalledExecution || result.RootPID != 0 || result.RootExit != -1 || result.Entry == 0 || result.ProgramSHA256 != profile.ProgramSHA256 {
		t.Fatal("offline ELF inspection executed a command or lost identity", result)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatal("inspection touched Source root", err)
	}
	replacement := profile
	replacement.Arguments = []string{"installation", "replace-source", filepath.Dir(program), "/testtmp/history", "alpha", "ardents-source-qualification", filepath.Join(root, "binding.json"), "/testtmp/request"}
	replacement.Journal = root + ".source-replacement-" + strings.Repeat("1", 64) + "-journal"
	replacement.PrefixRecords, replacement.LastPhase = 13, "file-born"
	replacementBytes, err := json.Marshal(replacement)
	if err != nil {
		t.Fatal(err)
	}
	replacementBytes = append(replacementBytes, '\n')
	replacementProfile, err := load(t, replacementBytes)
	if err != nil {
		t.Fatal("actual replacement argv shape refused", err)
	}
	if result, err := observeSourceCut(program, root, "", "external-cut", replacementProfile, true, 2, nil); err != nil || result.InstalledExecution || result.RootPID != 0 {
		t.Fatal("replacement inspection executed command or lost profile", result, err)
	}
	for _, original := range []sourceCutObserverProfile{profile, replacement} {
		t.Run(original.Arguments[1]+"-misplaced-values", func(t *testing.T) {
			pathIndices, envIndex, networkIndex := []int{2, 4, 7}, 5, 6
			if original.Arguments[1] == "replace-source" {
				pathIndices, envIndex, networkIndex = []int{2, 3, 6, 7}, 4, 5
			}
			for _, index := range append(pathIndices, envIndex, networkIndex) {
				changed := original
				changed.Arguments = append([]string(nil), original.Arguments...)
				changed.Arguments[index] = zero
				if index == networkIndex {
					changed.Arguments[index] = "/misplaced/network"
				}
				body, err := json.Marshal(changed)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := load(t, append(body, '\n')); err == nil {
					t.Fatal("misplaced path/pin accepted at", index)
				}
			}
		})
	}
	admitted.ProgramSHA256 = zero
	if _, err := observeSourceCut(program, root, "", "external-cut", admitted, true, 2, nil); err == nil {
		t.Fatal("wrong actual program pin accepted")
	}
	for name, body := range map[string][]byte{
		"unknown-field":    bytes.Replace(encoded, []byte(`"schema":`), []byte(`"unexpected":true,"schema":`), 1),
		"duplicate-field":  bytes.Replace(encoded, []byte(`"schema":`), []byte(`"schema":"source-original-cut-observer-v1","schema":`), 1),
		"start-command":    bytes.Replace(encoded, []byte(`"install-initial-stopped"`), []byte(`"start-source"`), 1),
		"recovery-command": bytes.Replace(encoded, []byte(`"install-initial-stopped"`), []byte(`"recover-source"`), 1),
		"wrong-original":   bytes.Replace(encoded, []byte(`.source-initial-journal`), []byte(`.foreign-journal`), 1),
		"trailing-value":   append(bytes.Clone(encoded), []byte("{}\n")...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := load(t, body); err == nil {
				t.Fatal("noncanonical or out-of-scope profile accepted")
			}
		})
	}
}
