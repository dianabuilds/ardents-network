//go:build linux && node_process_native

package node

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
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
	for _, carrier := range []string{epoch.CarrierClosedTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			processfixture.WaitAdmissionHour(t)
			spec, issuerPlan, inventory := networkinputs.PrepareNetworkGenesis(t, t.TempDir())
			listeners, certificates := addForwardingNodes(t, &spec, carrier, 1)
			issuerID := spec.Nodes[0].NodeID
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = listener.Close() })
			spec.Nodes[0].Endpoint = listener.Addr().String()
			listeners[issuerID] = listener
			certificates[issuerID] = processfixture.RoleCertificate(t, spec.Nodes[0].PrivateKey)
			bundle, err := networkfixture.BuildClosed(spec)
			if err != nil {
				t.Fatal(err)
			}
			networkinputs.PrepareIssuer(t, &issuerPlan, inventory, spec, bundle.Nodes[0], sha256.Sum256(bundle.Profile))
			clock, closeClock := networkinputs.StartObservedClock(t)
			defer closeClock()
			issuerClock, closeIssuerClock := networkinputs.StartObservedClock(t)
			var clockOnce sync.Once
			stopIssuerClock := func() { clockOnce.Do(closeIssuerClock) }
			defer stopIssuerClock()
			seed := func() state.AuthorityDeclaration {
				return networkinputs.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock)
			}
			var peers []*processfixture.NativeProcess
			var bootstrapPeers []*bootstrapPeer
			var peerPlans []struct {
				plan  map[string]any
				index byte
			}
			var budgets []string
			var original *processfixture.NativeProcess
			for _, id := range [][32]byte{{12}, {13}, {14}, {15}, issuerID} {
				certPath, keyPath := processfixture.WriteNativeCredential(t, certificates[id])
				budget := processfixture.ProcessBudget(t)
				budgets = append(budgets, budget)
				plan := map[string]any{"network": seed(), "node_id": id, "spend_root": t.TempDir(), "hosting_root": budget, "certificate": certPath, "private_key": keyPath,
					"work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
				if err := listeners[id].Close(); err != nil {
					t.Fatal(err)
				}
				if id == issuerID {
					networkPlan := seed()
					networkPlan.ClockObservationFile = issuerClock
					plan["network"] = networkPlan
					plan["issuance"] = IssuerDeclaration(issuerPlan)
					plan["process_profile"], plan["participation_root"], plan["retirement_wait_ms"] = processresource.IssuerProfile, filepath.Join(t.TempDir(), "participation"), 5000
					plan["work"], plan["bootstrap_work"] = hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, hosting.Traffic{Tx: 128 << 10, Rx: 128 << 10}
					original = processfixture.StartFreshRole(t, "issuer", binary, writeNodePlan(t, plan), 0)
					continue
				}
				p := startBootstrapPeer(t, binary, writeNodePlan(t, plan))
				bootstrapPeers = append(bootstrapPeers, p)
				peerPlans = append(peerPlans, struct {
					plan  map[string]any
					index byte
				}{plan, id[0]})
			}
			networkPlan := seed()
			config, err := networkPlan.Config()
			if err != nil {
				t.Fatal(err)
			}
			authority, err := state.OpenRetained(config)
			if err != nil {
				t.Fatal(err)
			}
			view, viewErr := authority.CurrentRuntime()
			if closeErr := authority.Close(); viewErr != nil || closeErr != nil {
				t.Fatal("read signed holder Authority", viewErr, closeErr)
			}
			facts, now, err := funding.NetworkFacts(view)
			if err != nil {
				t.Fatal(err)
			}
			budget := processfixture.ProcessBudget(t)
			budgets = append(budgets, budget)
			route := map[string]any{"entry_root": filepath.Join(t.TempDir(), "entry"), "interior_root": filepath.Join(t.TempDir(), "interior"),
				"hosting_root": budget, "domain": 1, "deadline": time.Now().Add(30 * time.Second).UTC().Truncate(time.Second),
				"work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			holder := map[string]any{"root": t.TempDir(), "network": networkPlan, "role": admission.AllocationUser, "route": route}
			send, closeHolder := processfixture.OpenAdmissionConsole(t, binary, "holder", writeNodePlan(t, holder))
			request := send(map[string]any{"operation": "request", "maxima": [3]uint32{2, 4, 1}})
			if request.Outcome != "completed" {
				t.Fatal("holder request", request.Outcome)
			}
			permission := processfixture.FixturePermission(t, request.Request, facts, spec.Authority, now)
			run := func(op string, args map[string]any) {
				t.Helper()
				if args == nil {
					args = make(map[string]any)
				}
				args["operation"] = op
				if reply := send(args); reply.Outcome != "completed" {
					t.Fatal("original holder operation refused", op, reply.Outcome, reply.Stage, original.Diagnostic.String())
				}
			}
			run("import", map[string]any{"digest": request.Digest, "payload": permission})
			run("bootstrap-open", nil)
			run("issuer-issue", map[string]any{"class": 2})
			run("issuer-issue", map[string]any{"class": 1, "receivers": [][32]byte{issuerID, issuerID}})
			run("bootstrap-close", nil)
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
				peers = append(peers, processfixture.StartFreshRole(t, "forwarding", binary, writeNodePlan(t, prepared.plan), prepared.index))
			}
			run("prefix-open", nil)
			run("issuer-issue", map[string]any{"class": 2})
			run("issuer-issue", map[string]any{"class": 3, "receivers": [][32]byte{issuerID}})
			run("prefix-close", nil)
			// This prefix spends the genuinely finalized ordinary tokens.
			run("prefix-open", nil)
			run("prefix-close", nil)
			closeHolder()
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
				material := filepath.Join(issuerPlan.KeyRoot, "issuer.keys")
				held := filepath.Join(t.TempDir(), "retained-material")
				if err := os.Rename(material, held); err != nil {
					t.Fatal(err)
				}
				original.FailedJoin(t, "issuance")
				if err := os.Rename(held, material); err != nil {
					t.Fatal(err)
				}
				processfixture.RequireIssuerRootsReleased(t, issuerPlan)
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

type bootstrapPeer struct {
	command    *exec.Cmd
	diagnostic bytes.Buffer
	waited     bool
}

func startBootstrapPeer(t *testing.T, binary, plan string) *bootstrapPeer {
	t.Helper()
	p := &bootstrapPeer{command: exec.CommandContext(t.Context(), binary, "route", "receive", "--config", plan)}
	out, err := p.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	p.command.Stderr = &p.diagnostic
	if err := p.command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !p.waited {
			_ = p.command.Process.Kill()
			_ = p.wait()
		}
		if t.Failed() {
			t.Log("bootstrap peer diagnostics", p.diagnostic.String())
		}
	})
	var event map[string]string
	if err := json.NewDecoder(out).Decode(&event); err != nil || event["operation"] != "route.receive" || event["phase"] != "listening" {
		t.Fatal("original bootstrap peer did not listen", err, event)
	}
	return p
}

func (p *bootstrapPeer) interrupt(t *testing.T) {
	t.Helper()
	if err := p.command.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Error("original bootstrap peer interrupt", err)
	}
}

func (p *bootstrapPeer) wait() error {
	err := p.command.Wait()
	p.waited = true
	return err
}
