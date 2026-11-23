//go:build !windows

package state

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
)

// The four node-duty participation repairs are one shared mechanism bound to a
// role descriptor (W4, #711; audit AR-N-01/E1-5). These tests prove the
// parameterization preserves each role's outcomes: the same causal mutations
// produce the same refusal class and the same retained bytes for every role,
// each role returns its own sentinel (never a sibling's), and the positive
// admission/removal path behaves identically. The per-role refusal inventories
// in the role test files remain the detailed causal coverage; this file is the
// cross-role equivalence oracle over the shared entry point.

var nodeDutyRepairRoles = []struct {
	name    string
	display string
	role    nodeDutyRepairRole
	fixture func(*testing.T) *State
}{
	{"forwarding", "Forwarding", forwardingRepairRole, forwardingRepairFixture},
	{"introduction", "Introduction", introductionRepairRole, introductionRepairFixture},
	{"join", "JOIN", joinRepairRole, joinRepairFixture},
	{"descriptor", "Descriptor", descriptorRepairRole, descriptorRepairFixture},
}

func TestNodeDutyRepairSharedMechanismPositiveEquivalence(t *testing.T) {
	for _, entry := range nodeDutyRepairRoles {
		t.Run(entry.name, func(t *testing.T) {
			s := entry.fixture(t)
			before := repairHistoryBytes(t, s.config.root, s.config.localRoles)
			r, err := openNodeDutyRepair(t.Context(), s.config, entry.role)
			if err != nil {
				t.Fatal(err)
			}
			if r.producer == [32]byte{} {
				t.Fatal("shared mechanism discovered no producer")
			}
			if other, err := openNodeDutyRepair(t.Context(), s.config, entry.role); err == nil {
				_ = other.close()
				t.Fatal("concurrent repair obtained State lease")
			}
			if after := repairHistoryBytes(t, s.config.root, s.config.localRoles); !reflect.DeepEqual(before, after) {
				t.Fatal("repair admission changed history")
			}
			if err := errors.Join(r.roles.Remove(r.producer), r.close()); err != nil {
				t.Fatal(err)
			}
			for path, raw := range repairHistoryBytes(t, s.config.root) {
				if !bytes.Equal(raw, before[path]) {
					t.Fatal("repair changed signed State or floors", path)
				}
			}
			// A new independent node-duty claim is admitted after the domain
			// removal, and a second claim is refused (the fence is exclusive again).
			now := s.config.clock()
			facts := duty.NodeParticipation{Identity: s.current.Snapshot.NodeID, Family: s.current.Snapshot.DeclaredFamily,
				Assignment: entry.name, EpochUntil: now.Add(time.Hour), RecordUntil: now.Add(time.Hour)}
			fresh, err := duty.OpenNodeProducer(t.Context(), duty.Config{Root: s.config.localRoles, Clock: s.config.clock}, facts)
			if err != nil {
				t.Fatal("new independent node-duty claim refused after domain removal", err)
			}
			if err := fresh.Live(); err != nil {
				t.Fatal(err)
			}
			if again, err := duty.OpenNodeProducer(t.Context(), duty.Config{Root: s.config.localRoles, Clock: s.config.clock}, facts); err == nil {
				_ = again.Close(t.Context())
				t.Fatal("second invocation adopted repaired producer")
			}
			if err := fresh.Close(t.Context()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNodeDutyRepairSharedMechanismRefusalEquivalence(t *testing.T) {
	for _, failure := range []string{"clean", "other-producer", "mismatched-duty", "active-cycle", "pending", "exposure-history", "expired", "clock"} {
		t.Run(failure, func(t *testing.T) {
			var messages []string
			for _, entry := range nodeDutyRepairRoles {
				s := entry.fixture(t)
				switch failure {
				case "clean", "other-producer", "mismatched-duty":
					roles, err := duty.OpenOperation(t.Context(), duty.Config{Root: s.config.localRoles, Clock: s.config.clock})
					if err != nil {
						t.Fatal(err)
					}
					producer, obligation, err := roles.CheckNodeProducer()
					if err != nil {
						t.Fatal(err)
					}
					var change error
					switch failure {
					case "clean":
						change = roles.Remove(producer)
					case "other-producer":
						value := obligation
						value.Identity = [32]byte{72}
						value.Family = [32]byte{74}
						change = roles.Claim([32]byte{72}, []duty.Duty{value})
					case "mismatched-duty":
						value := obligation
						value.Identity[0] ^= 1
						change = roles.Replace(producer, []duty.Duty{value})
					}
					if err := errors.Join(change, roles.Close()); err != nil {
						t.Fatal(err)
					}
				case "active-cycle", "pending", "exposure-history":
					storage, err := openTestDurableRoot(s.config.root)
					if err != nil {
						t.Fatal(err)
					}
					journal := s.distribution
					switch failure {
					case "active-cycle":
						journal.cycleActive = true
					case "pending":
						journal.pendingDigest = s.current.Header.Digest
					case "exposure-history":
						journal.history = [][32]byte{{91}}
					}
					raw := encodeDistributionState(journal)
					if err := errors.Join(storage.CommitControl(distributionDigest(raw), raw), storage.Close()); err != nil {
						t.Fatal(err)
					}
				case "expired":
					expired := s.current.Snapshot.ValidUntil.Add(time.Second)
					s.config.clock = func() time.Time { return expired }
					s.config.observe = func() time.Time { return expired }
				case "clock":
					s.config.observe = nil
				}
				before := repairHistoryBytes(t, s.config.root, s.config.localRoles)
				r, err := openNodeDutyRepair(t.Context(), s.config, entry.role)
				if err == nil {
					_ = r.close()
					t.Fatal("unsafe repair admitted", failure, entry.name)
				}
				// Active-cycle and pending refuse inside readRetainedHistory's
				// explicit-recovery class before any role sentinel is joined; that
				// shared refusal grammar must stay sentinel-free for every role.
				if failure == "active-cycle" || failure == "pending" {
					for _, any := range nodeDutyRepairRoles {
						if errors.Is(err, any.role.unavailable) {
							t.Fatal("explicit-recovery refusal carries a role sentinel", failure, entry.name, err)
						}
					}
				} else {
					if !errors.Is(err, entry.role.unavailable) {
						t.Fatal("refusal lost the role sentinel", failure, entry.name, err)
					}
					for _, sibling := range nodeDutyRepairRoles {
						if sibling.name != entry.name && errors.Is(err, sibling.role.unavailable) {
							t.Fatal("refusal carries a sibling role sentinel", failure, entry.name, sibling.name)
						}
					}
				}
				if after := repairHistoryBytes(t, s.config.root, s.config.localRoles); !reflect.DeepEqual(before, after) {
					t.Fatal("refusal changed retained bytes", failure, entry.name)
				}
				messages = append(messages, strings.ReplaceAll(err.Error(), "retained "+entry.display+" producer", "retained ROLE producer"))
			}
			// Identical causal refusal grammar across roles: the only permitted
			// difference is the role name inside the shared sentinel text.
			for i, message := range messages[1:] {
				if message != messages[0] {
					t.Fatalf("refusal grammar diverged for %s: %q vs %q", nodeDutyRepairRoles[i+1].name, message, messages[0])
				}
			}
		})
	}
}

func TestNodeDutyRepairSharedMechanismLeaseAndCancellationEquivalence(t *testing.T) {
	for _, entry := range nodeDutyRepairRoles {
		t.Run(entry.name, func(t *testing.T) {
			s := entry.fixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			before := repairHistoryBytes(t, s.config.root, s.config.localRoles)
			r, err := openNodeDutyRepair(ctx, s.config, entry.role)
			if err == nil {
				_ = r.close()
				t.Fatal("cancelled context admitted")
			}
			if !errors.Is(err, context.Canceled) || !errors.Is(err, entry.role.unavailable) {
				t.Fatal("cancellation lost original join", err)
			}
			if after := repairHistoryBytes(t, s.config.root, s.config.localRoles); !reflect.DeepEqual(before, after) {
				t.Fatal("refusal changed bytes")
			}
			r, err = openNodeDutyRepair(t.Context(), s.config, entry.role)
			if err != nil {
				t.Fatal("refusal leaked original leases", err)
			}
			if err := r.close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
