//go:build linux && node_process_native

package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func launchNativeIssuer(t *testing.T, binary, plan string) *nativeForwardingProcess {
	t.Helper()
	unit := fmt.Sprintf("ardents-issuer-%d.service", os.Getpid())
	p := &nativeForwardingProcess{unit: unit}
	p.command = exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--", binary, "node", "issuer", "--config", plan)
	p.command.Stderr = &p.diagnostic
	var err error
	p.stdout, err = p.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !p.joined {
			_ = exec.Command("systemctl", "stop", unit).Run()
			_ = p.command.Wait()
			_ = p.stdout.Close()
			p.joined = true
		}
		if t.Failed() {
			t.Log("original Issuer retained joined diagnostic", p.diagnostic.String())
		}
	})
	return p
}

func startNativeIssuer(t *testing.T, binary, plan string) *nativeForwardingProcess {
	t.Helper()
	p := launchNativeIssuer(t, binary, plan)
	var event map[string]string
	if err := json.NewDecoder(p.stdout).Decode(&event); err != nil || event["operation"] != "node.issuer" || event["phase"] != "listening" {
		t.Fatal("genuine original Issuer startup refused", event, err, p.diagnostic.String())
	}
	raw, err := exec.Command("systemctl", "show", p.unit, "-p", "MainPID", "--value").Output()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Sscanf(string(raw), "%d", &p.pid); err != nil || p.pid < 2 {
		t.Fatal("actual Issuer MainPID unavailable", err)
	}
	return p
}

func nativeIssuerIssuanceInput(p issuer.Plan) map[string]any {
	b := p.KeyBinding
	return map[string]any{"admission_root": p.AdmissionRoot, "admission_binding": admissionBindingConfig(p.AdmissionBinding), "key_root": p.KeyRoot,
		"key_binding": issuanceBindingInput{Network: fmt.Sprintf("%x", b.Network), Issuer: fmt.Sprintf("%x", b.Issuer), Signer: fmt.Sprintf("%x", b.Signer), Start: b.Start.Format(time.RFC3339), End: b.End.Format(time.RFC3339)}, "result_root": p.ResultRoot}
}

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
	binary := nodeNativeCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			f, sockets, certificates := newRoleRouteFixture(t, carrier, 1, false, true)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := joinProcessClock(t)
			defer closeClock()
			issuerClock, closeIssuerClock := joinProcessClock(t)
			var clockOnce sync.Once
			stopIssuerClock := func() { clockOnce.Do(closeIssuerClock) }
			defer stopIssuerClock()
			seed := func() networkAuthorityPlan { return seedNativeNodeState(t, f, bundle, clock) }
			var peers []*nativeForwardingProcess
			var bootstrapPeers []*joinCommandProcess
			var peerPlans []struct {
				plan  map[string]any
				index byte
			}
			var budgets []string
			var original *nativeForwardingProcess
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
				budget := routeProcessBudget(t)
				budgets = append(budgets, budget)
				plan := map[string]any{"network": seed(), "node_id": id, "spend_root": t.TempDir(), "hosting_root": budget, "certificate": certPath, "private_key": keyPath,
					"work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
				sockets[id]()
				if id == f.profile.IssuerNodeID {
					networkPlan := seed()
					networkPlan.ClockObservationFile = issuerClock
					plan["network"] = networkPlan
					plan["issuance"] = nativeIssuerIssuanceInput(f.plan)
					plan["process_profile"], plan["participation_root"], plan["retirement_wait_ms"] = processresource.IssuerProfile, filepath.Join(t.TempDir(), "participation"), 5000
					plan["work"], plan["bootstrap_work"] = hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, hosting.Traffic{Tx: 128 << 10, Rx: 128 << 10}
					original = startNativeIssuer(t, binary, hostingConfig(t, plan))
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
			plan := routePrefixPlan{EntryRoot: filepath.Join(t.TempDir(), "entry"), InteriorRoot: filepath.Join(t.TempDir(), "interior"), HostingRoot: routeProcessBudget(t), Domain: 1,
				Deadline: time.Now().Add(30 * time.Second).UTC().Truncate(time.Second), Work: hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, Termination: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			budgets = append(budgets, plan.HostingRoot)
			boot, err := startRouteBootstrap(context.Background(), plan, f.authority, holder)
			if err != nil {
				t.Fatal("actual Node bootstrap open", err)
			}
			if err := boot.issue(t.Context(), 2, nil); err != nil {
				_ = boot.close()
				t.Fatal("actual Node bootstrap Forward stock", err, original.diagnostic.String())
			}
			if err := boot.issue(t.Context(), 1, [][32]byte{f.profile.IssuerNodeID, f.profile.IssuerNodeID}); err != nil {
				_ = boot.close()
				t.Fatal("actual Node bootstrap Control stock", err, original.diagnostic.String())
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
				peers = append(peers, startNativeForwarding(t, binary, hostingConfig(t, prepared.plan), prepared.index))
			}
			ordinary, err := startRoutePrefix(t.Context(), plan, f.authority, holder)
			if err != nil {
				t.Fatal("fresh admitted prefix", err)
			}
			if err := ordinary.issue(t.Context(), 2, nil); err != nil {
				_ = ordinary.close()
				t.Fatal("actual ordinary Node issuance", err, original.diagnostic.String())
			}
			if err := ordinary.issue(t.Context(), 3, [][32]byte{f.profile.IssuerNodeID}); err != nil {
				_ = ordinary.close()
				t.Fatal("actual class3 finalization", err, original.diagnostic.String())
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
				if err := exec.Command("systemctl", "set-property", "--runtime", original.unit, "CPUQuota=50%").Run(); err != nil {
					t.Fatal(err)
				}
				original.failedJoin(t, "process")
			case "clock-loss":
				stopIssuerClock()
				if err := os.Remove(issuerClock); err != nil {
					t.Fatal(err)
				}
				original.failedJoin(t, "network")
			case "key-loss":
				material := filepath.Join(f.plan.KeyRoot, "issuer.keys")
				held := filepath.Join(t.TempDir(), "retained-material")
				if err := os.Rename(material, held); err != nil {
					t.Fatal(err)
				}
				original.failedJoin(t, "issuance")
				if err := os.Rename(held, material); err != nil {
					t.Fatal(err)
				}
				checkNativeIssuerRootsReleased(t, f.plan)
			default:
				t.Fatal("unselected native Issuer failure", fault)
			}
			all := append([]*nativeForwardingProcess{original}, peers...)
			signals := []string{"kill", "--kill-whom=main", "--signal=SIGINT"}
			for _, p := range all {
				if !p.joined {
					signals = append(signals, p.unit)
				}
			}
			if len(signals) > 3 {
				if raw, err := exec.Command("systemctl", signals...).CombinedOutput(); err != nil {
					t.Fatal("original caller batch cancellation unavailable", err, string(raw))
				}
				for _, p := range all {
					if !p.joined {
						p.signaled = true
					}
				}
			}
			for _, p := range all {
				p.stop(t, fault != "cancel")
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
