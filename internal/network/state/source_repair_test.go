//go:build !windows

package state

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state/durable"
)

// These are domain-admission and durable-commit tests over real signed State
// and participation. They do not manufacture a successful native death proof;
// the Linux public adapter separately requires the actual original manager.
func sourceRepairFixture(t *testing.T) *networkState {
	t.Helper()
	s, _ := newControlCommitFixture(t, true)
	public, err := hex.DecodeString("c2f38d34dafe402561da5a0a278e8a3255e0fc9c2e58c0209966a589fd07b631")
	if err != nil {
		t.Fatal(err)
	}
	s.config.authorities = map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}
	s.config.threshold = 1
	// Release only physical State custody, simulating the boundary after abrupt
	// invocation loss. The real journal's unresolved producer remains untouched.
	if err := s.storage.Close(); err != nil {
		t.Fatal(err)
	}
	return s
}

func repairHistoryBytes(t *testing.T, roots ...string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				info, err := entry.Info()
				if err != nil {
					return err
				}
				if !info.Mode().IsRegular() {
					files[path] = []byte("non-regular:" + info.Mode().String())
					return nil
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				files[path] = raw
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return files
}

func TestSourceRepairDomainAdmissionRetainsBothLeasesAndHistories(t *testing.T) {
	s := sourceRepairFixture(t)
	before := repairHistoryBytes(t, s.config.root, s.config.localRoles)
	r, err := openSourceRepair(t.Context(), s.config)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := openSourceRepair(t.Context(), s.config); err == nil {
		_ = other.close()
		t.Fatal("concurrent repair obtained State lease")
	}
	// Inspection has made no Claim probe, pointer repair, floor or staging write.
	if after := repairHistoryBytes(t, s.config.root, s.config.localRoles); !reflect.DeepEqual(before, after) {
		t.Fatal("repair admission changed history")
	}
	if err := r.close(); err != nil {
		t.Fatal(err)
	}
	if after := repairHistoryBytes(t, s.config.root, s.config.localRoles); !reflect.DeepEqual(before, after) {
		t.Fatal("closed inspection changed history")
	}
	// Only the domain effect is exercised here, after the independent semantic
	// admission. This is not public/native recovery qualification.
	r, err = openSourceRepair(t.Context(), s.config)
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(r.roles.Remove(sourceProducer("server", s.config.root)), r.close()); err != nil {
		t.Fatal(err)
	}
	for path, raw := range repairHistoryBytes(t, s.config.root) {
		if !bytes.Equal(raw, before[path]) {
			t.Fatal("repair changed signed State or floors", path)
		}
	}
	if err := s.retainSourceServer(); err != nil {
		t.Fatal("new independent Claim refused after domain removal", err)
	}
	if err := s.retainSourceServer(); !errors.Is(err, duty.ErrLocalRoleProducerRetained) {
		t.Fatal("second invocation adopted repaired producer", err)
	}
}

func TestSourceRepairRefusalsPreserveAllOriginalBytes(t *testing.T) {
	for _, failure := range []string{"clean", "other-producer", "mismatched-duty", "missing-participation-pointer", "participation-stage", "participation-fifo", "state-stage", "floor-mismatch", "active-cycle", "pending", "exposure-history", "conflict", "corrupt-signed-generation", "clock"} {
		t.Run(failure, func(t *testing.T) {
			s := sourceRepairFixture(t)
			if failure == "clean" || failure == "other-producer" || failure == "mismatched-duty" {
				roles, err := duty.OpenOperation(t.Context(), duty.Config{Root: s.config.localRoles, Clock: s.config.clock})
				if err != nil {
					t.Fatal(err)
				}
				var change error
				switch failure {
				case "clean":
					change = roles.Remove(sourceProducer("server", s.config.root))
				case "other-producer":
					value := sourceServerDuty(*s.current)
					value.Identity = [32]byte{72}
					value.Family = [32]byte{74}
					change = roles.Claim([32]byte{72}, []duty.Duty{value})
				case "mismatched-duty":
					value := sourceServerDuty(*s.current)
					value.Identity[0] ^= 1
					change = roles.Replace(sourceProducer("server", s.config.root), []duty.Duty{value})
				}
				if err := errors.Join(change, roles.Close()); err != nil {
					t.Fatal(err)
				}
			}
			switch failure {
			case "participation-fifo":
				marker := filepath.Join(s.config.localRoles, ".ardents-local-roles-v1")
				raw, err := os.ReadFile(marker)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(marker); err != nil {
					t.Fatal(err)
				}
				if err := syscall.Mkfifo(marker, 0600); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Remove(marker); _ = os.WriteFile(marker, raw, 0600) })
			case "missing-participation-pointer":
				if err := os.Remove(filepath.Join(s.config.localRoles, "current")); err != nil {
					t.Fatal(err)
				}
			case "participation-stage":
				if err := os.WriteFile(filepath.Join(s.config.localRoles, ".stage-recovery"), []byte("unresolved"), 0600); err != nil {
					t.Fatal(err)
				}
			case "state-stage":
				if err := os.WriteFile(filepath.Join(s.config.root, "distribution", ".current-recovery"), []byte("unresolved"), 0600); err != nil {
					t.Fatal(err)
				}
			case "corrupt-signed-generation":
				if err := os.WriteFile(filepath.Join(s.config.root, "generations", s.current.Snapshot.Generation, "epoch.bin"), []byte("invalid signed bytes"), 0600); err != nil {
					t.Fatal(err)
				}
			case "clock":
				s.config.observe = nil
			case "floor-mismatch", "active-cycle", "pending", "exposure-history", "conflict":
				storage, err := openTestDurableRoot(s.config.root)
				if err != nil {
					t.Fatal(err)
				}
				journal := s.distribution
				switch failure {
				case "floor-mismatch":
					journal.epochFloor++
				case "active-cycle":
					journal.cycleActive = true
				case "pending":
					journal.pendingDigest = s.current.Header.Digest
				case "exposure-history":
					journal.history = [][32]byte{{91}}
				case "conflict":
					journal.conflicting = true
				}
				raw := encodeDistributionState(journal)
				if err := errors.Join(storage.CommitControl(distributionDigest(raw), raw), storage.Close()); err != nil {
					t.Fatal(err)
				}
			}
			before := repairHistoryBytes(t, s.config.root, s.config.localRoles)
			r, err := openSourceRepair(t.Context(), s.config)
			if err == nil {
				_ = r.close()
				t.Fatal("unsafe repair admitted", failure)
			}
			if after := repairHistoryBytes(t, s.config.root, s.config.localRoles); !reflect.DeepEqual(before, after) {
				t.Fatal("refusal changed retained bytes", failure)
			}
		})
	}
}

func TestSourceRepairLeaseAndCancellationRefusalsJoinOriginalAcquisitions(t *testing.T) {
	for _, held := range []string{"state", "participation", "cancellation"} {
		t.Run(held, func(t *testing.T) {
			s := sourceRepairFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var closeHeld func() error
			switch held {
			case "state":
				owner, err := durable.OpenRetainedReader(s.config.root, storageLimits())
				if err != nil {
					t.Fatal(err)
				}
				closeHeld = owner.Close
			case "participation":
				owner, err := duty.OpenRetainedOperation(ctx, duty.Config{Root: s.config.localRoles, Clock: s.config.clock})
				if err != nil {
					t.Fatal(err)
				}
				closeHeld = owner.Close
			case "cancellation":
				cancel()
			}
			before := repairHistoryBytes(t, s.config.root, s.config.localRoles)
			r, err := openSourceRepair(ctx, s.config)
			if err == nil {
				_ = r.close()
				t.Fatal("unavailable original admitted")
			}
			if held == "cancellation" && !errors.Is(err, context.Canceled) {
				t.Fatal("original cancellation lost", err)
			}
			if closeHeld != nil {
				if err := closeHeld(); err != nil {
					t.Fatal(err)
				}
			}
			if after := repairHistoryBytes(t, s.config.root, s.config.localRoles); !reflect.DeepEqual(before, after) {
				t.Fatal("refusal changed bytes")
			}
			// A failed second acquisition must physically release the first lease.
			r, err = openSourceRepair(t.Context(), s.config)
			if err != nil {
				t.Fatal("refusal leaked original leases", err)
			}
			if err := r.close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
