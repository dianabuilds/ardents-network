//go:build linux

package installation

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/release"
)

func TestGenerationWriterExclusiveAndUnselected(t *testing.T) {
	if os.Geteuid() != 0 {
		if _, err := writeGeneration(context.Background(), "", Authorization{}, Request{}, 1, 1, nil); err == nil {
			t.Fatal("non-root writer accepted")
		}
		return // This branch verifies refusal only, never filesystem qualification.
	}
	base, err := os.MkdirTemp("/root", "ardents-generation-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(base); err != nil {
			t.Error(err)
		}
	})
	request := installationRequestFixture(t)
	request.InstallationRoot = filepath.Join(base, "installation")
	if err := os.MkdirAll(filepath.Join(request.InstallationRoot, "generations"), 0750); err != nil {
		t.Fatal(err)
	}
	enrolled := protectedReleaseFixture(t, nil, false)
	verifier, err := release.Open(filepath.Join(base, "floors"))
	if err != nil {
		t.Fatal(err)
	}
	proofs, err := Authenticate(context.Background(), verifier, enrolled)
	closeErr := verifier.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("authenticate: %v / %v", err, closeErr)
	}
	var roots []rootBinding
	for _, path := range mutableRoots(request.Headless) {
		roots = append(roots, rootBinding{Path: path, Device: 1, Inode: 2})
	}
	lease, err := acquireInstallationLease(request.InstallationRoot)
	if err != nil {
		t.Fatal(err)
	}
	_, busyErr := writeGeneration(context.Background(), request.InstallationRoot, proofs, request, 1001, 1001, roots)
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if busyErr == nil {
		t.Fatal("concurrent root writer accepted")
	}
	if _, err := os.Lstat(filepath.Join(request.InstallationRoot, "journals")); !os.IsNotExist(err) {
		t.Fatal("busy writer created journal")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := writeGeneration(cancelled, request.InstallationRoot, proofs, request, 1001, 1001, roots); err == nil {
		t.Fatal("cancelled write accepted")
	}
	entries, err := os.ReadDir(filepath.Join(request.InstallationRoot, "generations"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("cancelled write created files: %v", err)
	}
	selected, err := writeGeneration(context.Background(), request.InstallationRoot, proofs, request, 1001, 1001, roots)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(request.InstallationRoot, "generations", selected.GenerationDigest)
	entries, err = os.ReadDir(directory)
	if err != nil || len(entries) != 15 {
		t.Fatalf("incomplete generation: %v", err)
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		wanted := os.FileMode(0640)
		if entry.Name() == "ardents-linux-amd64" || entry.Name() == "ardents-text-linux-amd64" {
			wanted = 0555
		}
		if info.Mode().Perm() != wanted {
			t.Fatalf("incorrect mode for %s", entry.Name())
		}
	}
	if _, err := writeGeneration(context.Background(), request.InstallationRoot, proofs, request, 1001, 1001, roots); err == nil {
		t.Fatal("existing generation overwritten")
	}
	if _, err := os.Lstat(filepath.Join(request.InstallationRoot, "selection.json")); !os.IsNotExist(err) {
		t.Fatal("writer selected generation")
	}
	journal := filepath.Join(request.InstallationRoot, "journals", selected.GenerationDigest)
	if err := requireStagedJournal(journal, selected); err != nil {
		t.Fatal(err)
	}
	wrongSelection := selected
	wrongSelection.BindingDigest = digestHex([]byte("different binding"))
	if err := requireStagedJournal(journal, wrongSelection); err == nil {
		t.Fatal("journal accepted another binding")
	}
	record, err := os.ReadFile(filepath.Join(journal, "0002.json"))
	if err != nil || !bytes.Contains(record, []byte(`"phase":"generation-staged"`)) {
		t.Fatalf("missing staged journal: %v", err)
	}
	info, err := os.Stat(filepath.Join(journal, "0002.json"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("journal access: %v", err)
	}
	request.InstallationRoot = filepath.Join(base, "conflicting-installation")
	conflict := filepath.Join(request.InstallationRoot, "generations", selected.GenerationDigest)
	if err := os.MkdirAll(conflict, 0750); err != nil {
		t.Fatal(err)
	}
	_, original := writeGeneration(context.Background(), request.InstallationRoot, proofs, request, 1001, 1001, roots)
	if original == nil {
		t.Fatal("foreign candidate accepted")
	}
	record, err = os.ReadFile(filepath.Join(request.InstallationRoot, "journals", selected.GenerationDigest, "0002.json"))
	var failed transitionRecord
	if err != nil {
		t.Fatal(err)
	}
	if err := decodeCanonical(record, 64<<10, &failed); err != nil {
		t.Fatal(err)
	}
	if failed.Phase != "generation-write-failed" || failed.OriginalError != original.Error() {
		t.Fatal("original failure not retained")
	}
	if _, err := writeGeneration(context.Background(), request.InstallationRoot, proofs, request, 1001, 1001, roots); err == nil {
		t.Fatal("unfinished transition silently retried")
	}
	retained, err := os.ReadFile(filepath.Join(request.InstallationRoot, "journals", selected.GenerationDigest, "0002.json"))
	if err != nil || !bytes.Equal(retained, record) {
		t.Fatalf("prior failure changed: %v", err)
	}
}
