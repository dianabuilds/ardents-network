//go:build linux && node_process_native

package node

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/token"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/network/duty"
	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state"
	"github.com/dianabuilds/ardents-network/internal/platform/processguard"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func TestNativeNodeForwardingTCP(t *testing.T) {
	for _, retirement := range []string{"cancel", "protection-loss", "clock-loss", "undercoverage"} {
		t.Run(retirement, func(t *testing.T) { testNativeNodeForwarding(t, retirement) })
	}
}

func testNativeNodeForwarding(t *testing.T, retirement string) {
	binary := processfixture.PinnedCommand(t)
	for _, carrier := range []string{epoch.CarrierClosedTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			processfixture.WaitAdmissionHour(t)
			spec, issuerPlan, inventory := networkinputs.PrepareNetworkGenesis(t, t.TempDir())
			listeners, certificates := addForwardingNodes(t, &spec, carrier, 3)
			var provisioned map[[32]byte][2]string
			if retirement == "certificate" {
				provisioned = ProvisionNodeCertificates(t, &spec, certificates, [32]byte{12}, [32]byte{13}, [32]byte{14}, [32]byte{15})
			}
			bundle, err := networkfixture.BuildClosed(spec)
			if err != nil {
				t.Fatal(err)
			}
			networkinputs.PrepareIssuer(t, &issuerPlan, inventory, spec, bundle.Nodes[0], sha256.Sum256(bundle.Profile))
			clock, closeClock := networkinputs.StartObservedClock(t)
			var clockOnce sync.Once
			stopClock := func() { clockOnce.Do(closeClock) }
			defer stopClock()
			seed := func() state.AuthorityDeclaration {
				return networkinputs.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock)
			}
			authorityPlan := seed()
			config, err := authorityPlan.Config()
			if err != nil {
				t.Fatal(err)
			}
			authority, err := state.OpenRetained(config)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := authority.Close(); err != nil {
					t.Error(err)
				}
			})
			var processes []*processfixture.NativeProcess
			var budgets []string
			var participation []string
			var spendRoots []string
			var initialSpends [][]byte
			for index := byte(12); index < 16; index++ {
				id := [32]byte{index}
				certPath, keyPath := "", ""
				if pair, ok := provisioned[id]; ok {
					certPath, keyPath = pair[0], pair[1]
				} else {
					certPath, keyPath = processfixture.WriteNativeCredential(t, certificates[id])
				}
				budget := processfixture.ProcessBudget(t)
				budgets = append(budgets, budget)
				root := filepath.Join(t.TempDir(), "participation")
				participation = append(participation, root)
				spend := t.TempDir()
				spendRoots = append(spendRoots, spend)
				work := hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}
				if retirement == "undercoverage" {
					work = hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20}
				}
				plan := map[string]any{"network": seed(), "node_id": id, "process_profile": processguard.ForwardingProfile, "participation_root": root, "spend_root": spend, "hosting_root": budget, "certificate": certPath, "private_key": keyPath, "work": work, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
				if err := listeners[id].Close(); err != nil {
					t.Fatal(err)
				}
				processes = append(processes, processfixture.StartFreshRole(t, "forwarding", binary, writeNodePlan(t, plan), index))
				before, err := os.ReadFile(filepath.Join(spend, "closed-token-spends"))
				if err != nil || len(before) != 8+32+32+32+8 {
					t.Fatal("fresh real receiving ledger is not an empty canonical header", err)
				}
				initialSpends = append(initialSpends, before)
			}
			route := map[string]any{
				"entry_root": filepath.Join(t.TempDir(), "entry"), "interior_root": filepath.Join(t.TempDir(), "interior"),
				"hosting_root": processfixture.ProcessBudget(t), "domain": 3, "deadline": time.Now().Add(30 * time.Second).UTC().Truncate(time.Second),
				"work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10},
			}
			holder := map[string]any{"root": t.TempDir(), "network": seed(), "role": admission.AllocationUser, "route": route}
			send, closeHolder := processfixture.OpenAdmissionConsole(t, binary, "holder", writeNodePlan(t, holder))
			fundForwardingConsole(t, send, spec, bundle, issuerPlan, authority.CurrentRuntime)
			opened := send(map[string]string{"operation": "prefix-open"})
			if retirement == "undercoverage" {
				if opened.Outcome == "completed" {
					t.Fatal("undersized original Hosting admitted the Forward allowance")
				}
			} else {
				if opened.Outcome != "completed" {
					t.Fatal("actual Forwarding prefix failed", opened.Outcome, opened.Stage)
				}
				if joined := send(map[string]string{"operation": "prefix-close"}); joined.Outcome != "completed" {
					t.Fatal("actual Forwarding physical join failed", joined.Outcome, joined.Stage)
				}
			}
			closeHolder()
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
				member := bundle.Nodes[index+1]
				journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation[index], Clock: time.Now})
				if err != nil {
					t.Fatal("participation unavailable after known join", err)
				}
				conflict, err := journal.Conflict(member.NodeID, sha256.Sum256([]byte(member.Family)))
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

func fundForwardingConsole(t *testing.T, send func(any) processfixture.AdmissionReply, spec networkfixture.ClosedSpec, bundle networkfixture.Closed, plan issuer.Plan, read func() (network.RuntimeView, error)) {
	t.Helper()
	observe := func() (admission.AuthorityFacts, time.Time, error) {
		view, err := read()
		if err != nil {
			return admission.AuthorityFacts{}, time.Time{}, err
		}
		return funding.NetworkFacts(view)
	}
	facts, now, err := observe()
	if err != nil {
		t.Fatal(err)
	}
	request := send(map[string]any{"operation": "request", "maxima": [3]uint32{0, 4, 0}})
	if request.Outcome != "completed" {
		t.Fatal("holder request", request.Outcome)
	}
	raw := processfixture.FixturePermission(t, request.Request, facts, spec.Authority, now)
	if reply := send(map[string]any{"operation": "import", "digest": request.Digest, "payload": raw}); reply.Outcome != "completed" {
		t.Fatal("holder import", reply.Outcome)
	}
	var challenges []token.ClosedTokenContext
	for _, member := range bundle.Nodes[1:] {
		challenges = append(challenges, token.ClosedTokenContext{
			NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
			ReceiverNodeID: member.NodeID, ReceiverDutyGeneration: member.Generation,
			Class: uint8(admission.ForwardClass), WindowStart: now.UTC().Truncate(time.Hour),
		})
	}
	intent := stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{31}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter}
	batch := send(map[string]any{"operation": "begin", "intent": intent})
	if batch.Outcome != "completed" {
		t.Fatal("holder begin", batch.Outcome, batch.Stage)
	}
	issued := issuer.IssueCurrent(t.Context(), plan, batch.Request, quota.Bootstrap, observe)
	if issued.Outcome != "issued-offline" {
		t.Fatal("issuer refused genuine handle", issued.Outcome)
	}
	if reply := send(map[string]any{"operation": "complete", "payload": issued.Response}); reply.Outcome != "completed" {
		t.Fatal("holder completion", reply.Outcome)
	}
}

func addForwardingNodes(t *testing.T, spec *networkfixture.ClosedSpec, carrier string, domain byte) (map[[32]byte]net.Listener, map[[32]byte]tls.Certificate) {
	t.Helper()
	spec.Nodes = spec.Nodes[:1]
	listeners := make(map[[32]byte]net.Listener)
	certificates := make(map[[32]byte]tls.Certificate)
	for i := byte(12); i < 16; i++ {
		id := [32]byte{i}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = listener.Close() })
		listeners[id] = listener
		_, key, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { clear(key) })
		subrole := byte(1)
		if i >= 14 {
			subrole = 2
		}
		spec.Nodes = append(spec.Nodes, networkfixture.ClosedNode{
			RecordSpec: networkfixture.RecordSpec{NodeID: id, Generation: 9, ValidFrom: spec.NotBefore,
				ValidUntil: spec.NotAfter, Endpoint: listener.Addr().String(), Carrier: carrier,
				Capability: 2, Capacity: 1, PrivateKey: key},
			RoleDomain: domain, Subrole: subrole,
		})
		certificates[id] = processfixture.RoleCertificate(t, key)
	}
	return listeners, certificates
}
