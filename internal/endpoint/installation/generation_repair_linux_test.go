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

func TestRecoverySnapshotRequiresFreshMatchingProofsAndExactBoundPlans(t *testing.T) {
	enrolled := protectedReleaseFixture(t, nil, false)
	verifier, err := release.Open(filepath.Join(t.TempDir(), "floors"))
	if err != nil {
		t.Fatal(err)
	}
	defer verifier.Close()
	authorization, err := Authenticate(context.Background(), verifier, enrolled)
	if err != nil {
		t.Fatal(err)
	}
	request := installationRequestFixture(t)
	var roots []rootBinding
	for _, path := range mutableRoots(request.Headless) {
		roots = append(roots, rootBinding{Path: path, Device: 1, Inode: 2})
	}
	files, selected, err := assembleGeneration(request, authorization, 1001, 1001, roots)
	if err != nil {
		t.Fatal(err)
	}
	var binding localBinding
	if err := decodeCanonical(files["binding.json"], 64<<10, &binding); err != nil {
		t.Fatal(err)
	}
	intent := transitionIntent{Request: request, Candidate: selected, CandidateBinding: binding}
	fresh, err := Authenticate(context.Background(), verifier, enrolled)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := recoverGenerationSnapshot(intent, fresh)
	if err != nil {
		t.Fatal(err)
	}
	for name, original := range files {
		if !bytes.Equal(original, snapshot[name]) {
			t.Fatalf("recovery changed bound generation file %s", name)
		}
	}
	if _, err := recoverGenerationSnapshot(intent, Authorization{}); err == nil {
		t.Fatal("stored binding recreated Release authorization")
	}
	intent.Request.Headless.ApplicationSocket += "-foreign"
	if _, err := recoverGenerationSnapshot(intent, fresh); err == nil {
		t.Fatal("recovery accepted an unbound plan")
	}
}

func TestGenerationRepairRefusesUnknownBytesAndForeignDirectory(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	base := replacementTestRoot(t)
	path := filepath.Join(base, "program")
	wanted := []byte("fresh authorized complete bytes")
	if err := writeExclusiveGenerationFile(path, wanted[:5], 0600, 0); err != nil {
		t.Fatal(err)
	}
	if err := repairGenerationFile(path, wanted, 0555, 1001); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if !bytes.Equal(body, wanted) {
		t.Fatal("owned prefix was not completed")
	}
	if err := os.WriteFile(path, []byte("foreign"), 0555); err != nil {
		t.Fatal(err)
	}
	if err := repairGenerationFile(path, wanted, 0555, 1001); err == nil {
		t.Fatal("generation repair overwrote foreign bytes")
	}
	selected := selection{GenerationDigest: digestHex(wanted), BindingDigest: digestHex([]byte("binding"))}
	directory, journal := filepath.Join(base, "generation"), filepath.Join(base, "journal")
	for _, name := range []string{directory, journal} {
		if err := os.Mkdir(name, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := recordGenerationDirectory(journal, directory, selected); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(directory, directory+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := verifyGenerationDirectory(journal, directory, selected); err == nil {
		t.Fatal("generation journal adopted another directory inode")
	}
}
