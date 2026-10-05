//go:build linux

package introduction

// These durable controls exercise History with actual exclusive flock leases,
// POSIX root permissions and synced directory replacement, including reopen
// and uncertain writes. They require the selected Linux persistence mechanism;
// an in-memory or portable codec fixture cannot prove retained non-reclaim floors.

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
)

func historyTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	return root
}

func newHistoryFixture(t *testing.T) (*History, *spending.Ledger, string, Binding) {
	t.Helper()
	binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	ledger, err := spending.Open(t.TempDir(), spending.Binding{NetworkID: binding.NetworkID, ProfileDigest: binding.ProfileDigest, ReceiverNodeID: binding.ReceiverNodeID, ReceiverDutyGeneration: binding.ReceiverDutyGeneration})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ledger.Close() })
	fact, err := ledger.TakeFreshRoot()
	if err != nil {
		t.Fatal(err)
	}
	root := historyTestRoot(t)
	history, err := InitializeHistory(root, binding, fact)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = history.Close() })
	return history, ledger, root, binding
}

func TestHistoryCanonicalClaimAndNonReclaimAcrossReopen(t *testing.T) {
	history, ledger, root, binding := newHistoryFixture(t)
	now := time.Unix(1_800_000_000, 0).UTC()
	expiry := now.Add(5 * time.Minute)
	slot := [32]byte{7}
	if err := history.Claim(slot, expiry, now); err != nil {
		t.Fatal(err)
	}
	// Independent canonical oracle, no production encoder or mirrored constants.
	expected := make([]byte, 160)
	copy(expected[0:8], "ARDISL01")
	copy(expected[8:40], binding.NetworkID[:])
	copy(expected[40:72], binding.ProfileDigest[:])
	copy(expected[72:104], binding.ReceiverNodeID[:])
	binary.BigEndian.PutUint64(expected[104:112], 4)
	binary.BigEndian.PutUint64(expected[112:120], 1_800_000_000)
	digest := sha256.Sum256(slot[:])
	copy(expected[120:152], digest[:])
	binary.BigEndian.PutUint64(expected[152:160], 1_800_000_300)
	actual, err := os.ReadFile(filepath.Join(root, "closed-introduction-slots"))
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("snapshot differs from independent canonical bytes", err)
	}
	if bytes.Contains(actual, slot[:]) {
		t.Fatal("persisted raw slot instead of its digest")
	}
	if err := history.Claim(slot, expiry.Add(time.Second), now.Add(time.Second)); err == nil {
		t.Fatal("renewed original slot")
	}
	if _, err := OpenHistory(root, binding); err == nil {
		t.Fatal("opened second live history owner")
	}
	if err := history.Close(); err != nil {
		t.Fatal(err)
	}
	// Route lease is independent: Admission remains open throughout reopen.
	reopened, err := OpenHistory(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Claim(slot, expiry, now.Add(time.Second)); err == nil {
		t.Fatal("reopen reclaimed slot")
	}
	if err := reopened.Claim([32]byte{8}, expiry, now.Add(-time.Second)); err == nil {
		t.Fatal("reopen lowered time floor")
	}
	if err := ledger.Close(); err != nil {
		t.Fatal(err)
	}
	if err := reopened.Claim([32]byte{8}, expiry, now.Add(time.Second)); err != nil {
		t.Fatal("Route history depended on Admission live lease", err)
	}
}

func TestHistoryPruningRetainsFloorAndOriginalExpiry(t *testing.T) {
	history, _, root, binding := newHistoryFixture(t)
	now := time.Unix(1_800_000_000, 0).UTC()
	if err := history.Claim([32]byte{1}, now.Add(time.Second), now); err != nil {
		t.Fatal(err)
	}
	later := now.Add(2 * time.Second)
	if err := history.Claim([32]byte{2}, later.Add(time.Minute), later); err != nil {
		t.Fatal(err)
	}
	if len(history.entries) != 1 {
		t.Fatal("expired entry not pruned")
	}
	if err := history.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenHistory(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Claim([32]byte{1}, now.Add(time.Minute), now); err == nil {
		t.Fatal("pruning revived earlier time")
	}
}

func TestHistoryLossDamageAndRebindingNeverInitializeAgain(t *testing.T) {
	for _, fault := range []string{"snapshot missing", "lease missing", "entire root", "foreign binding", "corrupt", "foreign member"} {
		t.Run(fault, func(t *testing.T) {
			history, ledger, root, binding := newHistoryFixture(t)
			now := time.Unix(1_800_000_000, 0).UTC()
			if err := history.Claim([32]byte{7}, now.Add(time.Minute), now); err != nil {
				t.Fatal(err)
			}
			if err := history.Close(); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "snapshot missing":
				if err := os.Remove(filepath.Join(root, slotHistoryName)); err != nil {
					t.Fatal(err)
				}
			case "lease missing":
				if err := os.Remove(filepath.Join(root, slotLeaseName)); err != nil {
					t.Fatal(err)
				}
			case "entire root":
				// Only the two explicitly known files in the fixture's private root.
				for _, name := range []string{slotHistoryName, slotLeaseName} {
					if err := os.Remove(filepath.Join(root, name)); err != nil {
						t.Fatal(err)
					}
				}
			case "foreign binding":
				binding.ReceiverDutyGeneration++
			case "corrupt":
				if err := os.WriteFile(filepath.Join(root, slotHistoryName), []byte("damaged"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "foreign member":
				if err := os.WriteFile(filepath.Join(root, "unexpected"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := OpenHistory(root, binding); err == nil {
				t.Fatal("retained damage reopened")
			}
			if _, err := ledger.TakeFreshRoot(); err == nil {
				t.Fatal("retained Admission reset Route loss")
			}
			if _, err := InitializeHistory(root, binding, nil); err == nil {
				t.Fatal("initialized without genuine fresh fact")
			}
		})
	}
}

func TestHistoryUncertainReplaceTerminalizesWithoutErasingClaim(t *testing.T) {
	history, _, root, binding := newHistoryFixture(t)
	now := time.Unix(1_800_000_000, 0).UTC()
	failure := errors.New("directory sync unavailable after rename")
	history.store.(*historyFiles).replace = func(path string, raw []byte) error {
		if err := replaceHistory(path, raw); err != nil {
			return err
		}
		return failure
	}
	if err := history.Claim([32]byte{7}, now.Add(time.Minute), now); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if err := history.Claim([32]byte{8}, now.Add(time.Minute), now); !errors.Is(err, failure) {
		t.Fatal("uncertain owner continued", err)
	}
	first := history.Close()
	if !errors.Is(first, failure) || history.Close() != first {
		t.Fatal("Close lost retained terminal result")
	}
	reopened, err := OpenHistory(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.Claim([32]byte{7}, now.Add(time.Minute), now.Add(time.Second)); err == nil {
		t.Fatal("uncertain committed claim refunded")
	}
}

func TestHistoryInitializationChecksFreshOwnerAfterDurability(t *testing.T) {
	for _, cause := range []string{"admission attempt", "close", "sync failure"} {
		t.Run(cause, func(t *testing.T) {
			binding := Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
			ledger, err := spending.Open(t.TempDir(), spending.Binding{NetworkID: binding.NetworkID, ProfileDigest: binding.ProfileDigest, ReceiverNodeID: binding.ReceiverNodeID, ReceiverDutyGeneration: 4})
			if err != nil {
				t.Fatal(err)
			}
			defer ledger.Close()
			fact, err := ledger.TakeFreshRoot()
			if err != nil {
				t.Fatal(err)
			}
			root := historyTestRoot(t)
			failure := errors.New("initial sync unavailable")
			synchronized := false
			history, err := openHistory(root, binding, fact, func(path string) error {
				if err := syncHistory(path); err != nil {
					return err
				}
				synchronized = true
				switch cause {
				case "admission attempt":
					ledger.InvalidateFreshRoot()
				case "close":
					return ledger.Close()
				case "sync failure":
					return failure
				}
				return nil
			})
			if !synchronized || history != nil || err == nil {
				t.Fatal("late/uncertain initialization published usable owner")
			}
			if cause == "sync failure" && !errors.Is(err, failure) {
				t.Fatal("uncertain sync cause lost", err)
			}
			if _, err := InitializeHistory(historyTestRoot(t), binding, fact); err == nil {
				t.Fatal("reused interrupted initialization fact")
			}
			if _, err := ledger.TakeFreshRoot(); err == nil {
				t.Fatal("reconstructed interrupted fact")
			}
		})
	}
}
