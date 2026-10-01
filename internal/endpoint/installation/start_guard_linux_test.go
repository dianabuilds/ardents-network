//go:build linux

package installation

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestStartGuardRetainsExactIntentAcrossCursorArchival(t *testing.T) {
	if os.Geteuid() != 0 {
		return
	}
	root := replacementTestRoot(t)
	intent := transitionIntent{Request: Request{InstallationRoot: root}, Candidate: selection{GenerationDigest: digestHex([]byte("candidate"))}}
	if err := retainStartGuard(root, intent); err != nil {
		t.Fatal(err)
	}
	wanted, err := canonicalJSON(intent)
	if err != nil {
		t.Fatal(err)
	}
	if err := restoreTransitionIntent(intent); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(root, "journals", intent.Candidate.GenerationDigest)
	if err := os.MkdirAll(journal, 0700); err != nil {
		t.Fatal(err)
	}
	if err := archiveTransitionIntent(root, journal, intent); err != nil {
		t.Fatal(err)
	}
	guard, err := readStartGuard(root)
	if err != nil || !bytes.Equal(guard, wanted) {
		t.Fatal("cursor archival lost guarded recovery provenance", err)
	}
	foreign := intent
	foreign.Candidate.GenerationDigest = digestHex([]byte("foreign"))
	if err := retainStartGuard(root, foreign); err == nil {
		t.Fatal("another transition replaced a retained start guard")
	}
	guard, err = readStartGuard(root)
	if err != nil || !bytes.Equal(guard, wanted) {
		t.Fatal("refusal changed guarded recovery provenance", err)
	}
}
