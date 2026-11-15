package node

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
)

func TestParticipationRetirementKeepsConflictOnCleanupFailure(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "joined", true: "failed"}[failed], func(t *testing.T) {
			now := time.Unix(1800000000, 0).UTC()
			config := duty.Config{Root: filepath.Join(t.TempDir(), "participation"), Clock: func() time.Time { return now }, Create: true}
			facts := duty.NodeParticipation{Identity: [32]byte{1}, FamilyID: [32]byte{2}, Assignment: "rendezvous", EpochUntil: now.Add(time.Minute), RecordUntil: now.Add(2 * time.Minute)}
			producer, err := duty.OpenNodeProducer(t.Context(), config, facts)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = producer.Close(context.Background()) })
			if err := producer.Live(); err != nil {
				t.Fatal(err)
			}
			var cleanup error
			if failed {
				cleanup = errors.New("physical owner failed to close")
			}
			if err := releaseParticipation(producer, cleanup); err != nil {
				t.Fatal(err)
			}
			config.Create = false
			store, err := duty.OpenOperation(t.Context(), config)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			conflict, err := store.Conflict(facts.Identity, facts.FamilyID)
			if err != nil || conflict != failed {
				t.Fatalf("conflict=%v, cleanup failed=%v: %v", conflict, failed, err)
			}
		})
	}
}
