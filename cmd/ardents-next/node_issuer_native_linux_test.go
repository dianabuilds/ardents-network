//go:build linux && node_process_native

package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Actual hash-pinned Node Issuer in its sole native leaf, genuine signed State,
// prepared RSA/quota/results and Stock. Neighboring native Forwarding Nodes use
// their required profile and full Hosting coverage; no successful response is supplied.
func TestNativeIssuerBootstrapOrdinaryAndFreshSpendTCP(t *testing.T) {
	testNativeIssuerWorkload(t, "cancel")
}

func TestNativeIssuerOriginalFailureTCP(t *testing.T) {
	for _, fault := range []string{"protection-loss", "clock-loss", "key-loss"} {
		t.Run(fault, func(t *testing.T) { testNativeIssuerWorkload(t, fault) })
	}
}

func testNativeIssuerWorkload(t *testing.T, fault string) {
	t.Helper()
	binary := processfixture.PinnedCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			f, sockets, certificates := newRoleRouteFixture(t, carrier, 1, false, true)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := processfixture.StartObservedClock(t)
			defer closeClock()
			issuerClock, closeIssuerClock := processfixture.StartObservedClock(t)
			var clockOnce sync.Once
			stopIssuerClock := func() { clockOnce.Do(closeIssuerClock) }
			defer stopIssuerClock()
			seed := func() state.AuthorityDeclaration {
				return processfixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), bundle, clock)
			}
			var peers []*processfixture.NativeProcess
			var bootstrapPeers []*joinCommandProcess
			var peerPlans []struct {
				plan  map[string]any
				index byte
			}
			var budgets []string
			var original *processfixture.NativeProcess
			for _, id := range [][32]byte{{12}, {13}, {14}, {15}, f.profile.IssuerNodeID} {
				certificate := certificates[id]
				certPath, keyPath := filepath.Join(t.TempDir(), "certificate"), filepath.Join(t.TempDir(), "key")
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
				budget := processfixture.ProcessBudget(t)
				budgets = append(budgets, budget)
				plan := map[string]any{"network": seed(), "node_id": id, "spend_root": t.TempDir(), "hosting_root": budget, "certificate": certPath, "private_key": keyPath,
					"work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
				sockets[id]()
				if id == f.profile.IssuerNodeID {
					networkPlan := seed()
					networkPlan.ClockObservationFile = issuerClock
					plan["network"] = networkPlan
					plan["issuance"] = nodefixture.IssuerDeclaration(f.plan)
					plan["process_profile"], plan["participation_root"], plan["retirement_wait_ms"] = processresource.IssuerProfile, filepath.Join(t.TempDir(), "participation"), 5000
					plan["work"], plan["bootstrap_work"] = hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, hosting.Traffic{Tx: 128 << 10, Rx: 128 << 10}
					original = processfixture.StartFreshRole(t, "issuer", binary, hostingConfig(t, plan), 0)
					continue
				}
				p := startJoinCommandProcess(t, t.Context(), binary, "route", "receive", "--config", hostingConfig(t, plan))
				t.Cleanup(func() {
					if !p.waited {
						_ = p.command.Process.Kill()
						_ = p.wait()
					}
				})
				var ready map[string]string
				if err := p.decode.Decode(&ready); err != nil || ready["phase"] != "listening" {
					t.Fatal("actual bootstrap Route peer unavailable", err, p.diagnostic.String())
				}
				bootstrapPeers = append(bootstrapPeers, p)
				peerPlans = append(peerPlans, struct {
					plan  map[string]any
					index byte
				}{plan, id[0]})
			}
			holder := routePermissionStock(t, f, admission.AllocationUser, [3]uint32{2, 4, 1})
			plan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: processfixture.ProcessBudget(t), Domain: 1,
				Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			budgets = append(budgets, plan.HostingRoot)
			boot, err := startRouteBootstrap(context.Background(), plan, f.authority, holder)
			if err != nil {
				t.Fatal("actual Node bootstrap open", err)
			}
			if err := boot.issue(t.Context(), 2, nil); err != nil {
				_ = boot.close()
				t.Fatal("actual Node bootstrap Forward stock", err, original.Diagnostic.String())
			}
			if err := boot.issue(t.Context(), 1, [][32]byte{f.profile.IssuerNodeID, f.profile.IssuerNodeID}); err != nil {
				_ = boot.close()
				t.Fatal("actual Node bootstrap Control stock", err, original.Diagnostic.String())
			}
			if err := boot.close(); err != nil {
				t.Fatal("bootstrap physical retirement", err)
			}
			// Existing Route consumers own bootstrap, which the selected Forwarding
			// Node prerequisite does not compose. Join those genuine peers before
			// native Forwarding opens their same retained State/spend/Hosting roots.
			// The original Issuer and its key/quota/results remain live throughout.
			for _, p := range bootstrapPeers {
				p.interrupt(t)
			}
			for _, p := range bootstrapPeers {
				err := p.wait()
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 130 {
					t.Fatal("bootstrap peer did not physically join", err, p.diagnostic.String())
				}
			}
			for _, prepared := range peerPlans {
				prepared.plan["process_profile"], prepared.plan["participation_root"], prepared.plan["retirement_wait_ms"] = processresource.Profile, filepath.Join(t.TempDir(), "participation"), 5000
				prepared.plan["work"] = hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}
				peers = append(peers, processfixture.StartFreshRole(t, "forwarding", binary, hostingConfig(t, prepared.plan), prepared.index))
			}
			ordinary, err := startRoutePrefix(t.Context(), plan, f.authority, holder)
			if err != nil {
				t.Fatal("fresh admitted prefix", err)
			}
			if err := ordinary.issue(t.Context(), 2, nil); err != nil {
				_ = ordinary.close()
				t.Fatal("actual ordinary Node issuance", err, original.Diagnostic.String())
			}
			if err := ordinary.issue(t.Context(), 3, [][32]byte{f.profile.IssuerNodeID}); err != nil {
				_ = ordinary.close()
				t.Fatal("actual class3 finalization", err, original.Diagnostic.String())
			}
			if err := ordinary.close(); err != nil {
				t.Fatal("ordinary physical join", err)
			}
			// Initial Forward tokens were spent. This fresh Prefix must spend the
			// genuinely finalized ordinary result, not a fixture/offline token.
			fresh, err := startRoutePrefix(t.Context(), plan, f.authority, holder)
			if err != nil {
				t.Fatal("freshly issued token spend", err)
			}
			if err := fresh.close(); err != nil {
				t.Fatal("fresh spend join", err)
			}
			switch fault {
			case "cancel":
				// All original callers are canceled together below, before joins.
			case "protection-loss":
				if err := exec.Command("systemctl", "set-property", "--runtime", original.Unit, "CPUQuota=50%").Run(); err != nil {
					t.Fatal(err)
				}
				original.FailedJoin(t, "process")
			case "clock-loss":
				stopIssuerClock()
				if err := os.Remove(issuerClock); err != nil {
					t.Fatal(err)
				}
				original.FailedJoin(t, "network")
			case "key-loss":
				material := filepath.Join(f.plan.KeyRoot, "issuer.keys")
				held := filepath.Join(t.TempDir(), "retained-material")
				if err := os.Rename(material, held); err != nil {
					t.Fatal(err)
				}
				original.FailedJoin(t, "issuance")
				if err := os.Rename(held, material); err != nil {
					t.Fatal(err)
				}
				nodefixture.RequireIssuerRootsReleased(t, f.plan)
			default:
				t.Fatal("unselected native Issuer failure", fault)
			}
			all := append([]*processfixture.NativeProcess{original}, peers...)
			signals := []string{"kill", "--kill-whom=main", "--signal=SIGINT"}
			for _, p := range all {
				if !p.Joined {
					signals = append(signals, p.Unit)
				}
			}
			if len(signals) > 3 {
				if raw, err := exec.Command("systemctl", signals...).CombinedOutput(); err != nil {
					t.Fatal("original caller batch cancellation unavailable", err, string(raw))
				}
				for _, p := range all {
					if !p.Joined {
						p.Signaled = true
					}
				}
			}
			for _, p := range all {
				p.Stop(t, fault != "cancel")
			}
			for _, root := range budgets {
				b, err := hosting.Open(root)
				if err != nil {
					t.Fatal(err)
				}
				observed, err := b.Observe(t.Context())
				closeErr := b.Close()
				if err != nil || closeErr != nil || observed.ReservedBytes != 0 {
					t.Fatal("joined native Issuer/Route retains Hosting", err, closeErr, observed.ReservedBytes)
				}
			}
		})
	}
}
