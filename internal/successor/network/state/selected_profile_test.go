package state_test

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	state2 "github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

func TestDiagnosticSnapshotUnavailableAfterStateClose(t *testing.T) {
	value := newFixture(t)
	opened, err := state2.Open(state2.Config{
		Root: t.TempDir(), NetworkID: value.networkID,
		Authorities:            map[[32]byte]ed25519.PublicKey{value.authorityID: value.authorityPublic},
		ClosedProfileAuthority: value.authorityPublic,
		Threshold:              1, Now: time.Unix(value.now, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := opened.Current(); err == nil {
		t.Fatal("closed Network State exposed a diagnostic snapshot")
	}
}

func TestStateRejectsUnselectedLiveProfilesBeforeRootEffects(t *testing.T) {
	value := newFixture(t)
	for _, profile := range []string{"h3-role-probe-v1", "ardents-interactive-route-v2", "h3-route-tracer-v1"} {
		t.Run(profile, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "unopened")
			config := fixtureConfig(value, root, time.Unix(value.now, 0))
			config.AcceptedProfile = profile
			if opened, err := state2.Open(config); err == nil {
				_ = opened.Close()
				t.Fatal("unselected live profile was accepted")
			}
			if _, err := os.Stat(root); !os.IsNotExist(err) {
				t.Fatalf("invalid profile created state root: %v", err)
			}
		})
	}
}
