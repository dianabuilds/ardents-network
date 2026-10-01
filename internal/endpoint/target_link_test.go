//go:build linux

package endpoint

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

func TestEndpointTargetFromLinkBindsConfiguredNetwork(t *testing.T) {
	endpoint := &endpoint{network: targetLinkBytes(1)}
	link := targetlink.Link{Network: endpoint.network, Target: targetLinkBytes(33)}
	text, err := targetlink.Encode(link)
	if err != nil {
		t.Fatal(err)
	}
	got, err := endpoint.TargetFromLink(text)
	if err != nil {
		t.Fatal(err)
	}
	if got != link.Target {
		t.Fatalf("Target = %x, want %x", got, link.Target)
	}
}

func TestEndpointTargetFromLinkRejectsAnotherNetwork(t *testing.T) {
	endpoint := &endpoint{network: targetLinkBytes(1)}
	text, err := targetlink.Encode(targetlink.Link{Network: targetLinkBytes(2), Target: targetLinkBytes(33)})
	if err != nil {
		t.Fatal(err)
	}
	if target, err := endpoint.TargetFromLink(text); !errors.Is(err, ErrTargetLinkNetwork) || target != ([32]byte{}) {
		t.Fatalf("TargetFromLink = (%x, %v)", target, err)
	}
}

func TestEndpointTargetFromLinkRetiresAlphaWithoutChangingPersistentFloor(t *testing.T) {
	// ADR-0113 deleted the retained alpha corpus/floor readers, so the
	// retired-floor evidence is synthetic: opaque bytes shaped like the
	// historical marker + corpus-floor pair. No maintained code can read,
	// convert, or delete them; the surviving proof is refusal-before-effects
	// plus byte-for-byte preservation.
	network := targetLinkBytes(1)
	root := alphaPersistentFloorRoot(t)
	markerPath, floorPath := filepath.Join(root, ".ardents-alpha-corpus-floor-v1"), filepath.Join(root, "corpus-floor.bin")
	markerBefore := append([]byte("ardents-alpha-corpus-floor-v1"), 0)
	markerBefore = append(markerBefore, []byte("closed-alpha-1")...)
	floorBefore := bytes.Repeat([]byte{4}, 96)
	if err := os.WriteFile(markerPath, markerBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(floorPath, floorBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	const retiredLink = "ardents-alpha://retired.example"
	for attempt := 0; attempt < 2; attempt++ {
		target, refusal := (&endpoint{network: network}).TargetFromLink(retiredLink)
		if target != ([32]byte{}) || !errors.Is(refusal, ErrAlphaDestinationRetired) {
			t.Fatalf("retired alpha destination attempt %d = (%x, %v)", attempt, target, refusal)
		}
		markerAfter, markerErr := os.ReadFile(markerPath)
		floorAfter, floorErr := os.ReadFile(floorPath)
		if markerErr != nil || floorErr != nil || !bytes.Equal(markerAfter, markerBefore) || !bytes.Equal(floorAfter, floorBefore) {
			t.Fatalf("retired alpha destination attempt %d changed retired floor: marker=%v floor=%v", attempt, markerErr, floorErr)
		}
	}
}

func targetLinkBytes(start byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = start + byte(index)
	}
	return result
}

// alphaPersistentFloorRoot creates the owner-only directory that hosts the
// synthetic retired floor bytes, independent of the test process umask.
func alphaPersistentFloorRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "alpha-floor")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}
