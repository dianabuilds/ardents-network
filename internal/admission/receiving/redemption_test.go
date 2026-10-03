package receiving

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/spending"
)

func TestRedemptionRefusalReleasesReservationsWithoutSpending(t *testing.T) {
	for _, failure := range []string{"authorization", "window", "capacity"} {
		t.Run(failure, func(t *testing.T) {
			ledger := redemptionLedger(t)
			now := time.Unix(1_800_000_000, 0).UTC()
			raw := bytes.Repeat([]byte{3}, 354)
			cleanup := errors.New("hosting release failed")
			released, reserved, rolledBack := 0, 0, 0
			_, err := Redeem(Redemption{Class: admission.ForwardClass, Token: raw, Deadline: now.Add(time.Minute)}, ledger, func() time.Time { return now }, func() (Approval, error) {
				approval := Approval{Window: now.Truncate(time.Hour), Release: func() error { released++; return cleanup }}
				if failure == "authorization" {
					return approval, errors.New("authorization refused")
				}
				if failure == "window" {
					approval.Window = time.Time{}
				}
				return approval, nil
			}, func() (func(), error) {
				reserved++
				return func() { rolledBack++ }, errors.New("capacity refused")
			})
			if !errors.Is(err, cleanup) || released != 1 {
				t.Fatalf("refusal lost cleanup: %v, releases %d", err, released)
			}
			wantCapacity := 0
			if failure == "capacity" {
				wantCapacity = 1
			}
			if reserved != wantCapacity || rolledBack != wantCapacity {
				t.Fatalf("capacity acquired/rolled back = %d/%d", reserved, rolledBack)
			}
			if err := ledger.Spend(raw, now.Truncate(time.Hour), now); err != nil {
				t.Fatalf("pre-spend refusal consumed the token: %v", err)
			}
		})
	}
}

func TestRedemptionExpiryAfterDurableSpendNeverRefunds(t *testing.T) {
	// Both a fresh channel and a refill cross the same post-durability check.
	for _, initial := range []bool{true, false} {
		t.Run(map[bool]string{true: "initial", false: "refill"}[initial], func(t *testing.T) {
			ledger := redemptionLedger(t)
			now := time.Unix(1_800_000_000, 0).UTC()
			deadline := now.Add(time.Second)
			raw := bytes.Repeat([]byte{4}, 354)
			released, rolledBack, reads := 0, 0, 0
			var reserve func() (func(), error)
			if initial {
				reserve = func() (func(), error) { return func() { rolledBack++ }, nil }
			}
			_, err := Redeem(Redemption{Class: admission.ForwardClass, Token: raw, Deadline: deadline}, ledger, func() time.Time {
				reads++
				if reads == 1 {
					return now
				}
				return deadline
			}, func() (Approval, error) {
				return Approval{Window: now.Truncate(time.Hour), Release: func() error { released++; return nil }}, nil
			}, reserve)
			if err == nil || released != 1 || (initial && rolledBack != 1) || (!initial && rolledBack != 0) {
				t.Fatalf("late expiry = %v, releases %d/%d", err, released, rolledBack)
			}
			if err := ledger.Spend(raw, now.Truncate(time.Hour), deadline); err == nil {
				t.Fatal("late expiry refunded the durable spend")
			}
		})
	}
}

func TestRedemptionTransfersOnlySuccessfulReservation(t *testing.T) {
	ledger := redemptionLedger(t)
	now := time.Unix(1_800_000_000, 0).UTC()
	raw := bytes.Repeat([]byte{5}, 354)
	released, rolledBack := 0, 0
	redeem := func() (Approval, error) {
		return Redeem(Redemption{Class: admission.ControlClass, Token: raw, Deadline: now.Add(time.Second)}, ledger, func() time.Time { return now }, func() (Approval, error) {
			return Approval{Window: now.Truncate(time.Hour), Release: func() error { released++; return nil }}, nil
		}, func() (func(), error) { return func() { rolledBack++ }, nil })
	}
	approved, err := redeem()
	if err != nil || released != 0 || rolledBack != 0 {
		t.Fatalf("success = %v, releases %d/%d", err, released, rolledBack)
	}
	if _, err := redeem(); err == nil || released != 1 || rolledBack != 1 {
		t.Fatalf("duplicate = %v, releases %d/%d", err, released, rolledBack)
	}
	if err := approved.Release(); err != nil || released != 2 {
		t.Fatalf("successful owner release = %v/%d", err, released)
	}
}

func redemptionLedger(t *testing.T) *spending.Ledger {
	t.Helper()
	ledger, err := spending.Open(t.TempDir(), spending.Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := ledger.Close(); err != nil {
			t.Error(err)
		}
	})
	return ledger
}
