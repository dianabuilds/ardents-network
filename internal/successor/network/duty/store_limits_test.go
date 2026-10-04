package duty_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
)

func TestStoreDistinguishesRecordLimit(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	store, err := duty.Open(duty.Config{
		Root:   filepath.Join(t.TempDir(), "local-roles"),
		Clock:  func() time.Time { return now },
		Create: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	seed := make([]duty.Duty, 0, 64)
	for index := 0; index < 64; index++ {
		seed = append(seed, ordinaryInitiatorDuty(byte(index+1), byte(index+101), now))
	}
	if err := store.Replace([32]byte{1}, seed); err != nil {
		t.Fatalf("seed Replace: %v", err)
	}
	overflow := []duty.Duty{ordinaryInitiatorDuty(90, 91, now)}
	if err := store.Replace([32]byte{2}, overflow); !errors.Is(err, duty.ErrLocalRoleRecordLimit) {
		t.Fatalf("over-record-limit Replace returned %v, want ErrLocalRoleRecordLimit", err)
	}
}

func TestStoreRejectsOversizedReplacementWithRecordLimit(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	store, err := duty.Open(duty.Config{
		Root:   filepath.Join(t.TempDir(), "local-roles"),
		Clock:  func() time.Time { return now },
		Create: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	duties := make([]duty.Duty, 0, 65)
	for index := 0; index < 65; index++ {
		duties = append(duties, ordinaryInitiatorDuty(byte(index+1), byte(index+101), now))
	}
	if err := store.Replace([32]byte{1}, duties); !errors.Is(err, duty.ErrLocalRoleRecordLimit) {
		t.Fatalf("oversized Replace returned %v, want ErrLocalRoleRecordLimit", err)
	}
}

func TestStoreDistinguishesProducerLimit(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	store, err := duty.Open(duty.Config{
		Root:   filepath.Join(t.TempDir(), "local-roles"),
		Clock:  func() time.Time { return now },
		Create: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for index := 1; index <= 16; index++ {
		participation := ordinaryInitiatorDuty(byte(index), byte(index+100), now)
		if err := store.Replace([32]byte{byte(index)}, []duty.Duty{participation}); err != nil {
			t.Fatalf("producer %d Replace: %v", index, err)
		}
	}
	overflow := []duty.Duty{ordinaryInitiatorDuty(90, 91, now)}
	if err := store.Replace([32]byte{17}, overflow); !errors.Is(err, duty.ErrLocalRoleProducerLimit) {
		t.Fatalf("over-producer-limit Replace returned %v, want ErrLocalRoleProducerLimit", err)
	}
}

func ordinaryInitiatorDuty(identity, family byte, now time.Time) duty.Duty {
	return duty.Duty{
		Identity: [32]byte{identity},
		Family:   [32]byte{family},
		Class:    "ordinary-initiator",
		State:    "live",
		NotAfter: now.Add(time.Hour),
	}
}
