package endpoint

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
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
	now := time.Unix(2_000_500_000, 0).UTC()
	network := targetLinkBytes(1)
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	link, err := alpha.ParseServiceLink("ardents-alpha://retained.example")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: network, Serial: 4,
		Bindings: []alpha.BindingInput{{Link: link, Target: targetLinkBytes(33)}}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}, authority)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(authority.Public().(ed25519.PublicKey), raw)
	if err != nil {
		t.Fatal(err)
	}
	root := alphaPersistentFloorRoot(t)
	initialFloor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: root, Authority: authority.Public().(ed25519.PublicKey),
		Cohort: "closed-alpha-1", Network: network})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := initialFloor.Close(); err != nil {
			t.Errorf("close initial alpha floor: %v", err)
		}
	})
	if err := initialFloor.Observe(corpus); err != nil {
		t.Fatal(err)
	}
	if err := initialFloor.Close(); err != nil {
		t.Fatal(err)
	}
	markerPath, floorPath := filepath.Join(root, ".ardents-alpha-corpus-floor-v1"), filepath.Join(root, "corpus-floor.bin")
	markerBefore, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatal(err)
	}
	floorBefore, err := os.ReadFile(floorPath)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		target, refusal := (&endpoint{network: network}).TargetFromLink(link.String())
		if target != ([32]byte{}) || !errors.Is(refusal, ErrAlphaDestinationRetired) {
			t.Fatalf("retired alpha destination attempt %d = (%x, %v)", attempt, target, refusal)
		}
		markerAfter, markerErr := os.ReadFile(markerPath)
		floorAfter, floorErr := os.ReadFile(floorPath)
		if markerErr != nil || floorErr != nil || !bytes.Equal(markerAfter, markerBefore) || !bytes.Equal(floorAfter, floorBefore) {
			t.Fatalf("retired alpha destination attempt %d changed retained floor: marker=%v floor=%v", attempt, markerErr, floorErr)
		}
	}
	retainedFloor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: root, Authority: authority.Public().(ed25519.PublicKey),
		Cohort: "closed-alpha-1", Network: network})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := retainedFloor.Close(); err != nil {
			t.Errorf("close retained alpha floor: %v", err)
		}
	})
	retained, err := retainedFloor.Current()
	if err != nil || retained.Serial() != 4 {
		t.Fatalf("retained alpha corpus = (%v, %v)", retained, err)
	}
	binding, err := retained.Resolve(link, now)
	if err != nil || binding.Target() != targetLinkBytes(33) {
		t.Fatalf("retained alpha binding = (%+v, %v)", binding, err)
	}
}

func targetLinkBytes(start byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = start + byte(index)
	}
	return result
}
