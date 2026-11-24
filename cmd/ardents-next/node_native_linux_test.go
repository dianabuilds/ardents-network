//go:build linux && node_process_native

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func TestNativeNodeForwardingTCP(t *testing.T) {
	for _, retirement := range []string{"cancel", "protection-loss", "clock-loss", "undercoverage"} {
		t.Run(retirement, func(t *testing.T) { testNativeNodeForwarding(t, retirement) })
	}
}

func testNativeNodeForwarding(t *testing.T, retirement string) {
	binary := nodefixture.PinnedCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			var provisioned map[[32]byte][2]string
			f, sockets, certificates := newRoleRouteFixtureConfigured(t, carrier, 3, false, false, func(f *networkAdmissionFixture, _ map[[32]byte]func(), certificates map[[32]byte]tls.Certificate) {
				if retirement == "certificate" {
					provisioned = nodefixture.ProvisionNodeCertificates(t, &f.spec, certificates, [32]byte{12}, [32]byte{13}, [32]byte{14}, [32]byte{15})
				}
			})
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := nodefixture.StartObservedClock(t)
			var clockOnce sync.Once
			stopClock := func() { clockOnce.Do(closeClock) }
			defer stopClock()
			seed := func() state.AuthorityDeclaration {
				return nodefixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), bundle, clock)
			}
			var processes []*nodefixture.NativeProcess
			var budgets []string
			var participation []string
			var spendRoots []string
			var initialSpends [][]byte
			for index := byte(12); index < 16; index++ {
				id := [32]byte{index}
				certificate := certificates[id]
				certPath, keyPath := filepath.Join(t.TempDir(), "certificate"), filepath.Join(t.TempDir(), "key")
				if pair, ok := provisioned[id]; ok {
					certPath, keyPath = pair[0], pair[1]
				} else {
					if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0600); err != nil {
						t.Fatal(err)
					}
					key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
					if err != nil {
						t.Fatal(err)
					}
					encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
					if err := os.WriteFile(keyPath, encoded, 0600); err != nil {
						t.Fatal(err)
					}
					clear(key)
					clear(encoded)
				}
				budget := nodefixture.ProcessBudget(t)
				budgets = append(budgets, budget)
				root := filepath.Join(t.TempDir(), "participation")
				participation = append(participation, root)
				spend := t.TempDir()
				spendRoots = append(spendRoots, spend)
				work := hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}
				if retirement == "undercoverage" {
					work = hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20}
				}
				plan := map[string]any{"network": seed(), "node_id": id, "process_profile": processresource.Profile, "participation_root": root, "spend_root": spend, "hosting_root": budget, "certificate": certPath, "private_key": keyPath, "work": work, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
				sockets[id]()
				processes = append(processes, nodefixture.StartFreshRole(t, "forwarding", binary, hostingConfig(t, plan), index))
				before, err := os.ReadFile(filepath.Join(spend, "closed-token-spends"))
				if err != nil || len(before) != 8+32+32+32+8 {
					t.Fatal("fresh real receiving ledger is not an empty canonical header", err)
				}
				initialSpends = append(initialSpends, before)
			}
			holder := routeStock(t, f)
			plan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: nodefixture.ProcessBudget(t), Domain: 3, Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			prefix, err := startRoutePrefix(context.Background(), plan, f.authority, holder)
			if retirement == "undercoverage" {
				if err == nil {
					_ = prefix.close()
					t.Fatal("undersized actual Hosting admitted the complete Forward allowance")
				}
			} else if err != nil {
				t.Fatal("actual Forwarding prefix failed", err)
			} else if err := prefix.close(); err != nil {
				t.Fatal("actual Forwarding physical join failed", err)
			}
			switch retirement {
			case "protection-loss":
				if err := exec.Command("systemctl", "set-property", "--runtime", processes[0].Unit, "CPUQuota=50%").Run(); err != nil {
					t.Fatal(err)
				}
				processes[0].FailedJoin(t, "process")
			case "clock-loss":
				stopClock()
				if err := os.Remove(clock); err != nil {
					t.Fatal(err)
				}
				for _, process := range processes {
					process.FailedJoin(t, "network")
				}
			}
			// Retire every original caller before waiting for any peer's join.
			// Sequential stop-and-wait would manufacture peer loss in still-live
			// invocations and mix that failure with the cancellation control.
			signals := []string{"kill", "--kill-whom=main", "--signal=SIGINT"}
			for _, process := range processes {
				if !process.Joined {
					signals = append(signals, process.Unit)
				}
			}
			if len(signals) > 3 {
				if raw, err := exec.Command("systemctl", signals...).CombinedOutput(); err != nil {
					for _, process := range processes {
						t.Log("original unit terminal evidence", process.Unit, process.Diagnostic.String())
					}
					t.Fatal("original caller batch cancellation unavailable", err, string(raw))
				}
				for _, process := range processes {
					process.Signaled = true
				}
			}
			for index, process := range processes {
				// A forced protection loss also closes its real peer channels;
				// adjacent invocations retain any resulting Route cleanup failure.
				process.Stop(t, retirement == "undercoverage" || retirement == "protection-loss")
				if retirement == "undercoverage" {
					after, err := os.ReadFile(filepath.Join(spendRoots[index], "closed-token-spends"))
					if err != nil || !bytes.Equal(initialSpends[index], after) {
						t.Fatal("pre-spend physical coverage refusal changed real durable spend", err)
					}
				}
				view, err := f.current()
				if err != nil {
					t.Fatal(err)
				}
				member, err := view.Member([32]byte{byte(12 + index)}, view.ObservedAt())
				if err != nil {
					t.Fatal(err)
				}
				journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation[index], Clock: time.Now})
				if err != nil {
					t.Fatal("participation unavailable after known join", err)
				}
				conflict, err := journal.Conflict(member.NodeID, member.FamilyID)
				if closeErr := journal.Close(); err != nil || closeErr != nil || conflict {
					t.Fatal("joined original producer still conflicts", err, closeErr, conflict)
				}
			}
			for _, root := range budgets {
				budget, err := hosting.Open(root)
				if err != nil {
					t.Fatal(err)
				}
				observation, err := budget.Observe(t.Context())
				closeErr := budget.Close()
				if err != nil || closeErr != nil || observation.ReservedBytes != 0 {
					t.Fatal("joined original Node retained or invented capacity", err, closeErr, observation.ReservedBytes)
				}
			}
		})
	}
}

func TestNativeNodeProvisionedCertificateTCP(t *testing.T) {
	testNativeNodeForwarding(t, "certificate")
}
