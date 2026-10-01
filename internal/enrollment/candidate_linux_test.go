//go:build linux

package enrollment

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestHeadlessCandidatePreservesInventoryWithoutAnIndependentPin(t *testing.T) {
	root, request := enrolledFixture(t)
	files := map[string][]byte{}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == manifestName {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = body
	}
	for name, body := range map[string][]byte{"ardents-node-linux-amd64": []byte("node artifact"), "ardents-custody-linux-amd64": []byte("custody artifact")} {
		files[name] = body
		if err := os.WriteFile(filepath.Join(root, name), body, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, manifestName), makeManifest(t, files), 0600); err != nil {
		t.Fatal(err)
	}
	candidate, err := ReadHeadlessCandidate(root, request.ExecutablePath, request.ReferenceTime)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(candidate.Inputs.Artifact, files["ardents-linux-amd64"]) || !bytes.Equal(candidate.Inputs.RootBytes, files["1.root.json"]) {
		t.Fatal("untrusted candidate changed its loaded bytes")
	}
	// The independent old pin no longer matches this companion inventory. The
	// initial route refuses it; a Candidate result supplies no replacement pin.
	if _, err := VerifyHeadless(request); err == nil {
		t.Fatal("candidate loading weakened first-install pin acceptance")
	}
	if err := os.WriteFile(request.ExecutablePath, []byte("substituted executable"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadHeadlessCandidate(root, request.ExecutablePath, request.ReferenceTime); err == nil {
		t.Fatal("candidate loader accepted an inconsistent executable")
	}
}
