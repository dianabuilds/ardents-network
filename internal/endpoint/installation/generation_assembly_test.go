//go:build linux

package installation

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/release"
)

func TestGenerationAssemblyUsesFrozenAuthenticatedBytes(t *testing.T) {
	request := installationRequestFixture(t)
	enrolled := protectedReleaseFixture(t, nil, false)
	verifier, err := release.Open(filepath.Join(t.TempDir(), "floors"))
	if err != nil {
		t.Fatal(err)
	}
	proofs, err := Authenticate(context.Background(), verifier, enrolled)
	closeErr := verifier.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("authenticate: %v / %v", err, closeErr)
	}
	wanted := bytes.Clone(enrolled.ProtectedFiles["ardents-linux-amd64"])
	for _, body := range enrolled.ProtectedFiles {
		for i := range body {
			body[i] = 0
		}
	}
	for i := range enrolled.ProtectedDescriptor {
		enrolled.ProtectedDescriptor[i] = 0
	}
	var roots []rootBinding
	for _, path := range mutableRoots(request.Headless) {
		roots = append(roots, rootBinding{Path: path, Device: 1, Inode: 2})
	}
	files, selected, err := assembleGeneration(request, proofs, 1001, 1001, roots)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 15 || !bytes.Equal(files["ardents-linux-amd64"], wanted) || selected.BindingDigest != digestHex(files["binding.json"]) {
		t.Fatal("assembled generation differs from frozen authorization")
	}
	files["ardents-linux-amd64"][0] = 0
	again, _, err := assembleGeneration(request, proofs, 1001, 1001, roots)
	if err != nil || !bytes.Equal(again["ardents-linux-amd64"], wanted) {
		t.Fatalf("returned bytes mutated authorization: %v", err)
	}
	if _, _, err := assembleGeneration(request, Authorization{}, 1001, 1001, roots); err == nil {
		t.Fatal("zero authorization accepted")
	}
	if _, _, err := assembleGeneration(request, proofs, 0, 1001, roots); err == nil {
		t.Fatal("root account accepted")
	}
}
