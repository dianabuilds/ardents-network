package duty_test

import (
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"

	duty2 "github.com/dianabuilds/ardents-network/internal/successor/network/duty"
)

func TestNodeParticipationRetainsExclusionAcrossReopenAndBounds(t *testing.T) {
	for _, recordFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "Epoch ends first", true: "record ends first"}[recordFirst], func(t *testing.T) {
			now := time.Unix(1800000000, 0).UTC()
			end := now.Add(time.Minute)
			config := duty2.Config{Root: filepath.Join(t.TempDir(), "participation"), Clock: func() time.Time { return now }, Create: true}
			store, err := duty2.Open(config)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			participation := duty2.NodeParticipation{Identity: [32]byte{1}, Family: "operator-a", Assignment: "rendezvous", EpochUntil: end, RecordUntil: end.Add(time.Hour)}
			if recordFirst {
				participation.EpochUntil, participation.RecordUntil = participation.RecordUntil, participation.EpochUntil
			}
			producer := [32]byte{2}
			for _, phase := range []duty2.NodePhase{duty2.NodePrepared, duty2.NodeQuarantined, duty2.NodeLive} {
				if err := store.RetainNode(producer, participation, phase); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			config.Create = false
			store, err = duty2.Open(config)
			if err != nil {
				t.Fatal(err)
			}
			family := sha256.Sum256([]byte(participation.Family))
			if conflict, err := store.Conflict([32]byte{}, family); err != nil || !conflict {
				t.Fatalf("reopen lost family exclusion: %v", err)
			}
			// An exposure phase belongs to Source, never to Node execution.
			if err := store.RetainNode(producer, participation, duty2.NodePhase("exposed")); err == nil {
				t.Fatal("Node accepted Source exposure phase")
			}
			if conflict, err := store.Conflict(participation.Identity, [32]byte{}); err != nil || !conflict {
				t.Fatalf("invalid replacement removed exclusion: %v", err)
			}
			now = end
			if conflict, err := store.Conflict(participation.Identity, family); err != nil || conflict {
				t.Fatalf("participation outlived its authority: %v", err)
			}
		})
	}
}
