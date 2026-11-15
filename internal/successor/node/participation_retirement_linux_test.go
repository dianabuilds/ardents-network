package node

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
)

func TestParticipationRetirementKeepsConflictOnCleanupFailure(t *testing.T) {
	for _, scenario := range []struct {
		name           string
		first, cleanup error
		conflict       bool
	}{
		{"joined", nil, nil, false},
		{"canceled", context.Canceled, nil, false},
		{"runtime failure", errors.New("runtime failure"), nil, true},
		{"wrapped cancellation", fmt.Errorf("physical failure: %w", context.Canceled), nil, true},
		{"cleanup failure", nil, errors.New("physical owner failed to close"), true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
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
			if err := releaseParticipation(producer, scenario.first, scenario.cleanup); err != nil {
				t.Fatal(err)
			}
			config.Create = false
			store, err := duty.OpenOperation(t.Context(), config)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			conflict, err := store.Conflict(facts.Identity, facts.FamilyID)
			if err != nil || conflict != scenario.conflict {
				t.Fatalf("conflict=%v, expected=%v: %v", conflict, scenario.conflict, err)
			}
		})
	}
}

func TestFailedPreparedParticipationStillExpires(t *testing.T) {
	now := time.Unix(1800000000, 0).UTC()
	config := duty.Config{Root: filepath.Join(t.TempDir(), "participation"), Clock: func() time.Time { return now }, Create: true}
	facts := duty.NodeParticipation{Identity: [32]byte{1}, FamilyID: [32]byte{2}, Assignment: "rendezvous", EpochUntil: now.Add(time.Minute), RecordUntil: now.Add(2 * time.Minute)}
	producer, err := duty.OpenNodeProducer(t.Context(), config, facts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = producer.Close(context.Background()) })
	if err := releaseParticipation(producer, nil, errors.New("cleanup failed before live")); err != nil {
		t.Fatal(err)
	}
	now = now.Add(3 * time.Minute)
	config.Create = false
	store, err := duty.OpenOperation(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	conflict, err := store.Conflict(facts.Identity, facts.FamilyID)
	if err != nil || conflict {
		t.Fatalf("prepared participation should retain its original expiry: conflict=%v, error=%v", conflict, err)
	}
}
