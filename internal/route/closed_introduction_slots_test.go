package route

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClosedIntroductionSlotsSurviveLeaseTransferAndExpire(t *testing.T) {
	root := t.TempDir()
	binding := ClosedSpendBinding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	open := func() (*ClosedSpendLedger, *ClosedIntroductionSlots) {
		t.Helper()
		ledger, err := OpenClosedSpendLedger(root, binding)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := ledger.Close(); err != nil {
				t.Error(err)
			}
		})
		slots, err := ledger.IntroductionSlots()
		if err != nil {
			t.Fatal(err)
		}
		return ledger, slots
	}
	now := time.Unix(1800000000, 0).UTC()
	slot := [32]byte{12}
	ledger, slots := open()
	if err := slots.Claim(slot, now.Add(time.Minute), now); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(slots.path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, slot[:]) {
		t.Fatal("persisted the raw slot identifier")
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	if err := slots.Claim([32]byte{13}, now.Add(time.Minute), now); err == nil {
		t.Fatal("released slot owner persisted a claim")
	}
	_, reopened := open()
	if err := reopened.Claim(slot, now.Add(2*time.Minute), now.Add(time.Second)); err == nil {
		t.Fatal("restart reset slot replay floor")
	}
	if err := reopened.Claim(slot, now.Add(2*time.Minute), now.Add(time.Minute)); err != nil {
		t.Fatalf("expired floor retained slot: %v", err)
	}
}

func TestClosedIntroductionSlotsCannotOpenAfterLeaseRelease(t *testing.T) {
	ledger, err := OpenClosedSpendLedger(t.TempDir(), ClosedSpendBinding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.IntroductionSlots(); err == nil {
		t.Fatal("opened slot floor without the process lease")
	}
	now := time.Now().UTC().Truncate(time.Hour)
	if err := ledger.Spend(make([]byte, 354), now, now); err == nil {
		t.Fatal("spent token without the process lease")
	}
}

func TestClosedIntroductionSlotsRefuseDamagedFloor(t *testing.T) {
	for _, damage := range []string{"missing", "truncated", "foreign-binding"} {
		t.Run(damage, func(t *testing.T) {
			root := t.TempDir()
			binding := ClosedSpendBinding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
			ledger, err := OpenClosedSpendLedger(root, binding)
			if err != nil {
				t.Fatal(err)
			}
			slots, err := ledger.IntroductionSlots()
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC().Truncate(time.Second)
			if err := ledger.Spend(make([]byte, 354), now.Truncate(time.Hour), now); err != nil {
				t.Fatal(err)
			}
			if err := slots.Claim([32]byte{8}, now.Add(time.Minute), now); err != nil {
				t.Fatal(err)
			}
			if err := ledger.Close(); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(slots.path)
			if err != nil {
				t.Fatal(err)
			}
			switch damage {
			case "missing":
				err = os.Remove(slots.path)
			case "truncated":
				err = os.WriteFile(slots.path, raw[:len(raw)-1], 0600)
			case "foreign-binding":
				raw[8]++
				err = os.WriteFile(slots.path, raw, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			reopened, err := OpenClosedSpendLedger(root, binding)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := reopened.Close(); err != nil {
					t.Error(err)
				}
			}()
			if _, err := reopened.IntroductionSlots(); err == nil {
				t.Fatal("damaged slot floor accepted")
			}
		})
	}
}

func TestClosedIntroductionSlotsPersistenceFailureStopsClaims(t *testing.T) {
	root := t.TempDir()
	ledger, err := OpenClosedSpendLedger(root, ClosedSpendBinding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ledger.Close(); err != nil {
			t.Error(err)
		}
	}()
	slots, err := ledger.IntroductionSlots()
	if err != nil {
		t.Fatal(err)
	}
	original := slots.path + ".retained"
	if err := os.Rename(slots.path, original); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(slots.path, 0700); err != nil {
		t.Fatal(err)
	}
	obstruction := filepath.Join(slots.path, "obstruction")
	if err := os.WriteFile(obstruction, []byte("prevent replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	if err := slots.Claim([32]byte{1}, now.Add(time.Minute), now); err == nil {
		t.Fatal("acknowledged failed durable claim")
	}
	if err := os.Remove(obstruction); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(slots.path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(original, slots.path); err != nil {
		t.Fatal(err)
	}
	if err := slots.Claim([32]byte{2}, now.Add(time.Minute), now); err == nil {
		t.Fatal("persistence failure revived without reopening")
	}
}

func TestClosedIntroductionSlotsPruningRetainsTimeFloor(t *testing.T) {
	root := t.TempDir()
	binding := ClosedSpendBinding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	ledger, err := OpenClosedSpendLedger(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	slots, err := ledger.IntroductionSlots()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1800000000, 0).UTC()
	expiry := now.Add(time.Minute)
	if err := slots.Claim([32]byte{1}, expiry, now); err != nil {
		t.Fatal(err)
	}
	if err := slots.Claim([32]byte{2}, expiry.Add(time.Minute), expiry); err != nil {
		t.Fatal(err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenClosedSpendLedger(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	}()
	floor, err := reopened.IntroductionSlots()
	if err != nil {
		t.Fatal(err)
	}
	if err := floor.Claim([32]byte{1}, expiry, expiry.Add(-time.Second)); err == nil {
		t.Fatal("clock rollback revived a pruned slot")
	}
	if err := floor.Claim([32]byte{3}, expiry.Add(time.Minute), expiry); err != nil {
		t.Fatal(err)
	}
}
