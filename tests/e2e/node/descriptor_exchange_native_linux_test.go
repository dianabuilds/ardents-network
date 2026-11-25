//go:build linux && node_process_native

package node

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/reachability"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

func TestNativeDescriptorPublishLookupAndReopenTCP(t *testing.T) {
	nativeDescriptorCommandWorkload(t, "cancel")
}

func TestNativeDescriptorUndercoverageBeforeSpendTCP(t *testing.T) {
	nativeDescriptorCommandWorkload(t, "undercoverage")
}

func TestNativeDescriptorOriginalCommandFailureTCP(t *testing.T) {
	for _, fault := range []string{"protection-loss", "clock-loss", "store-marker-loss", "cpu-pressure"} {
		t.Run(fault, func(t *testing.T) { nativeDescriptorCommandWorkload(t, fault) })
	}
}

func nativeDescriptorCommandWorkload(t *testing.T, fault string) {
	binary := processfixture.PinnedCommand(t)
	for _, carrier := range []string{epoch.CarrierClosedTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			processfixture.WaitAdmissionHour(t)
			spec, issuerPlan, inventory := networkinputs.PrepareNetworkGenesis(t, t.TempDir())
			listeners, certificates := addForwardingNodes(t, &spec, carrier, 1)
			for _, id := range []byte{17, 18} {
				node := spec.Nodes[1]
				node.NodeID, node.RoleDomain, node.Subrole = [32]byte{id}, 2, 5
				node.Endpoint = "127.0.0.1:1"
				if id == 17 {
					listener, err := net.Listen("tcp", "127.0.0.1:0")
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = listener.Close() })
					listeners[node.NodeID] = listener
					node.Endpoint = listener.Addr().String()
				} else {
					node.RoleDomain, node.Subrole = 4, 3
				}
				_, key, err := ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { clear(key) })
				node.PrivateKey = key
				certificates[node.NodeID] = processfixture.RoleCertificate(t, key)
				spec.Nodes = append(spec.Nodes, node)
			}
			bundle, err := networkfixture.BuildClosed(spec)
			if err != nil {
				t.Fatal(err)
			}
			networkinputs.PrepareIssuer(t, &issuerPlan, inventory, spec, bundle.Nodes[0], sha256.Sum256(bundle.Profile))
			clock, closeClock := networkinputs.StartObservedClock(t)
			defer closeClock()
			descriptorClock, closeDescriptorClock := networkinputs.StartObservedClock(t)
			var clockOnce sync.Once
			stopDescriptorClock := func() { clockOnce.Do(closeDescriptorClock) }
			defer stopDescriptorClock()
			seed := func() state.AuthorityDeclaration {
				return networkinputs.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock)
			}
			input := seed()
			config, err := input.Config()
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
			observe := func() (admission.AuthorityFacts, time.Time, error) {
				view, err := authority.CurrentRuntime()
				if err != nil {
					return admission.AuthorityFacts{}, time.Time{}, err
				}
				return funding.NetworkFacts(view)
			}
			facts, now, err := observe()
			if err != nil {
				t.Fatal(err)
			}
			raw, target := processfixture.SignedDescriptor(t, facts, 3, [32]byte{18}, facts.Digest)
			var processes []*processfixture.NativeProcess
			var budgets, participations, spends []string
			var descriptorConfig map[string]any
			descriptorRoot := t.TempDir()
			for _, id := range [][32]byte{{12}, {13}, {14}, {15}, {17}} {
				certPath, keyPath := processfixture.WriteNativeCredential(t, certificates[id])
				budget := processfixture.ProcessBudget(t)
				participation := filepath.Join(t.TempDir(), "participation")
				spend := t.TempDir()
				profile := processresource.Profile
				work := hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}
				if id == [32]byte{17} {
					profile = processresource.DescriptorProfile
					work = hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}
					if fault == "undercoverage" {
						// The fixture provider counts tx+rx: the shared total,
						// rather than each directional maximum, must be one byte short.
						work = hosting.Traffic{Tx: 32 << 10, Rx: (32 << 10) - 1}
					}
				}
				config := map[string]any{"network": seed(), "node_id": id, "process_profile": profile, "participation_root": participation, "spend_root": spend, "hosting_root": budget, "certificate": certPath, "private_key": keyPath, "work": work, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
				if id == [32]byte{17} {
					config["descriptor_root"] = descriptorRoot
					networkPlan := config["network"].(state.AuthorityDeclaration)
					networkPlan.ClockObservationFile = descriptorClock
					config["network"] = networkPlan
					descriptorConfig = config
				}
				if err := listeners[id].Close(); err != nil {
					t.Fatal(err)
				}
				var p *processfixture.NativeProcess
				if id == [32]byte{17} {
					p = processfixture.StartFreshRole(t, "descriptor", binary, writeNodePlan(t, config), id[0])
				} else {
					p = processfixture.StartFreshRole(t, "forwarding", binary, writeNodePlan(t, config), id[0])
				}
				processes = append(processes, p)
				budgets = append(budgets, budget)
				participations = append(participations, participation)
				spends = append(spends, spend)
			}
			resolution := processes[len(processes)-1]
			journal := filepath.Join(spends[len(spends)-1], "closed-token-spends")
			before, err := os.ReadFile(journal)
			if err != nil || len(before) != 8+32+32+32+8 {
				t.Fatal("Descriptor receiving root not genuine empty header", err, len(before))
			}
			budget := processfixture.ProcessBudget(t)
			budgets = append(budgets, budget)
			route := map[string]any{"entry_root": filepath.Join(t.TempDir(), "entry"), "interior_root": filepath.Join(t.TempDir(), "interior"),
				"hosting_root": budget, "domain": 1, "deadline": time.Now().Add(30 * time.Second).UTC().Truncate(time.Second),
				"work": hosting.Traffic{Tx: 2 << 20, Rx: 2 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}}
			holder := map[string]any{"root": t.TempDir(), "network": seed(), "role": admission.AllocationUser, "route": route}
			send, closeHolder := processfixture.OpenAdmissionConsole(t, binary, "holder", writeNodePlan(t, holder))
			fundDescriptorConsole(t, send, spec.Authority, issuerPlan, facts, now, observe)
			if reply := send(map[string]string{"operation": "prefix-open"}); reply.Outcome != "completed" {
				t.Fatal("genuine adjacent Forwarding unavailable", reply.Outcome, reply.Stage)
			}
			publish := func(raw []byte) processfixture.AdmissionReply {
				return send(map[string]any{"operation": "descriptor-publish", "payload": raw})
			}
			lookup := func(target [32]byte) processfixture.AdmissionReply {
				return send(map[string]any{"operation": "descriptor-lookup", "target": target})
			}
			if fault == "undercoverage" {
				if reply := publish(raw); reply.Outcome == "completed" {
					t.Fatal("undersized Descriptor work admitted full Control")
				}
				after, err := os.ReadFile(journal)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatal("physical undercoverage spent Control", err)
				}
				entries, err := os.ReadDir(filepath.Join(descriptorRoot, "records"))
				if err != nil || len(entries) != 0 {
					t.Fatal("physical undercoverage reached Store mutation", err)
				}
				if _, err := os.Stat(fmt.Sprintf("/proc/%d", resolution.PID)); err != nil {
					t.Fatal("request refusal retired the original command", err)
				}
				// A corrected declaration applies only to a fresh original command
				// after joined Stop, retaining the same unspent durable roots.
				resolution.Stop(t, true)
				descriptorConfig["work"] = hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}
				resolution = processfixture.StartFreshRole(t, "descriptor", binary, writeNodePlan(t, descriptorConfig), 17)
				processes[len(processes)-1] = resolution
			}
			if reply := publish(raw); reply.Outcome != "completed" {
				t.Fatal("actual original Descriptor publish/ACK failed", reply.Outcome, reply.Stage)
			}
			assertLookup := func() {
				t.Helper()
				reply := lookup(target)
				if reply.Outcome != "completed" || !bytes.Equal(reply.Proof, raw) {
					t.Fatal("actual original Descriptor exact lookup failed", reply.Outcome, reply.Stage)
				}
			}
			assertLookup()
			initial, err := os.ReadFile(journal)
			if err != nil || len(initial) <= len(before) {
				t.Fatal("publish/lookup did not irreversibly spend genuine Control", err)
			}
			if fault == "cancel" {
				connectOnly, _ := processfixture.SignedDescriptor(t, facts, 2, [32]byte{18}, facts.Digest)
				broken := bytes.Clone(raw)
				broken[len(broken)-1] ^= 1
				foreignIntroduction, _ := processfixture.SignedDescriptor(t, facts, 3, [32]byte{12}, facts.Digest)
				for _, input := range [][]byte{connectOnly, broken, foreignIntroduction} {
					if reply := publish(input); reply.Outcome == "completed" {
						t.Fatal("invalid signed input accepted")
					}
				}
				if reply := lookup([32]byte{99}); reply.Outcome == "completed" {
					t.Fatal("foreign Target accepted")
				}
				assertLookup() // every healthy request refusal leaves the original usable
			}
			after, err := os.ReadFile(journal)
			if err != nil || len(after) < len(initial) || (fault == "cancel" && len(after) <= len(initial)) {
				t.Fatal("refused Descriptor requests refunded admission", err)
			}
			switch fault {
			case "cpu-pressure":
				ExerciseNativeCPUPressure(t, resolution)
				resolution.FailedJoin(t, "process")
				var event map[string]string
				var first struct{ Resource *processresource.FailureFacts }
				if err := json.Unmarshal([]byte(resolution.Diagnostic.String()), &event); err != nil {
					t.Fatal("original CPU outcome framing unavailable", err)
				}
				if err := json.Unmarshal([]byte(event["first_failure"]), &first); err != nil || first.Resource == nil || first.Resource.Cause != "cpu-pressure" || first.Resource.Observation == nil || first.Resource.Observation.CPUPressure < 1 || first.Resource.Observation.CPUWindowStarted.IsZero() {
					t.Fatal("CPU negative lacked actual retained original overload", err, resolution.Diagnostic.String())
				}
			case "protection-loss":
				if err := exec.Command("systemctl", "set-property", "--runtime", resolution.Unit, "CPUQuota=50%").Run(); err != nil {
					t.Fatal(err)
				}
				resolution.FailedJoin(t, "process")
			case "clock-loss":
				stopDescriptorClock()
				if err := os.Remove(descriptorClock); err != nil {
					t.Fatal(err)
				}
				resolution.FailedJoin(t, "network")
			case "store-marker-loss":
				marker := filepath.Join(descriptorRoot, ".ardents-reachability-store-v3")
				original, err := os.ReadFile(marker)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(marker, []byte("foreign marker\n"), 0600); err != nil {
					t.Fatal(err)
				}
				resolution.FailedJoin(t, "reachability")
				if err := os.WriteFile(marker, original, 0600); err != nil {
					t.Fatal(err)
				}
			case "cancel", "undercoverage":
			default:
				t.Fatal("unselected Descriptor command fault", fault)
			}
			if reply := send(map[string]string{"operation": "prefix-close"}); reply.Outcome != "completed" {
				if fault == "cancel" {
					t.Fatal("original holder prefix did not physically join", reply.Outcome, reply.Stage)
				}
				t.Log("retained original holder cleanup after forced failure", reply.Outcome, reply.Stage)
			}
			closeHolder()
			// All original callers stop before any peer is waited, preserving the
			// distinction between deliberate cancellation and manufactured peer loss.
			signals := []string{"kill", "--kill-whom=main", "--signal=SIGINT"}
			for _, p := range processes {
				if !p.Joined {
					signals = append(signals, p.Unit)
				}
			}
			if output, err := exec.Command("systemctl", signals...).CombinedOutput(); err != nil {
				t.Fatal("original caller batch retirement failed", err, string(output))
			}
			for _, p := range processes {
				p.Signaled = true
				p.Stop(t, fault != "cancel")
			}
			if !resolution.Joined {
				t.Fatal("Descriptor manager completion absent")
			}
			fresh, err := reachability.OpenStore(reachability.StoreConfig{Root: descriptorRoot, Network: facts.NetworkID})
			if err != nil {
				t.Fatal("actual Store lease/custody did not reopen after original join", err)
			}
			got, outcome, lookupErr := fresh.Lookup(target, facts.Digest, time.Now())
			if err := fresh.Close(); err != nil || lookupErr != nil || outcome != reachability.Accepted || !bytes.Equal(got, raw) {
				t.Fatal("ACK did not retain exact durable proof", outcome, lookupErr, err)
			}
			if retained, err := os.ReadFile(journal); err != nil || !bytes.Equal(retained, after) {
				t.Fatal("original Close changed irreversible spends", err)
			}
			for index, root := range participations {
				view, err := authority.CurrentRuntime()
				if err != nil {
					t.Fatal(err)
				}
				id := [][32]byte{{12}, {13}, {14}, {15}, {17}}[index]
				member, err := view.Member(id, view.ObservedAt())
				if err != nil {
					t.Fatal(err)
				}
				owner, err := duty.OpenOperation(t.Context(), duty.Config{Root: root, Clock: time.Now})
				if err != nil {
					t.Fatal(err)
				}
				conflict, err := owner.Conflict(member.NodeID, member.FamilyID)
				if closeErr := owner.Close(); err != nil || closeErr != nil || conflict {
					t.Fatal("original producer retained after actual join", err, closeErr, conflict)
				}
			}
			for _, root := range budgets {
				budget, err := hosting.Open(root)
				if err != nil {
					t.Fatal(err)
				}
				observed, err := budget.Observe(t.Context())
				closeErr := budget.Close()
				if err != nil || closeErr != nil || observed.ReservedBytes != 0 {
					t.Fatal("original Hosting returned before or failed after join", err, closeErr, observed.ReservedBytes)
				}
			}
		})
	}
}

func fundDescriptorConsole(t *testing.T, send func(any) processfixture.AdmissionReply, authority ed25519.PrivateKey, plan issuer.Plan,
	facts admission.AuthorityFacts, now time.Time, observe func() (admission.AuthorityFacts, time.Time, error)) {
	t.Helper()
	request := send(map[string]any{"operation": "request", "maxima": [3]uint32{8, 4, 0}})
	if request.Outcome != "completed" {
		t.Fatal("holder request", request.Outcome)
	}
	raw := processfixture.FixturePermission(t, request.Request, facts, authority, now)
	if imported := send(map[string]any{"operation": "import", "digest": request.Digest, "payload": raw}); imported.Outcome != "completed" {
		t.Fatal("holder import", imported.Outcome)
	}
	challenge := func(id, class uint8) token.ClosedTokenContext {
		return token.ClosedTokenContext{NetworkID: facts.NetworkID, ProfileDigest: facts.Digest, IssuerNodeID: facts.IssuerNodeID,
			ReceiverNodeID: [32]byte{id}, ReceiverDutyGeneration: 9, Class: class, WindowStart: now.UTC().Truncate(time.Hour)}
	}
	var forwards, controls []token.ClosedTokenContext
	for id := byte(12); id < 16; id++ {
		forwards = append(forwards, challenge(id, 2))
	}
	for range 8 {
		controls = append(controls, challenge(17, 1))
	}
	for i, challenges := range [][]token.ClosedTokenContext{forwards, controls} {
		intent := stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: [32]byte{byte(41 + i)}, ProfileDigest: facts.Digest}, Bootstrap: true, Deadline: facts.NotAfter}
		batch := send(map[string]any{"operation": "begin", "intent": intent})
		if batch.Outcome != "completed" {
			t.Fatal("holder begin", batch.Outcome, batch.Stage)
		}
		issued := issuer.IssueCurrent(t.Context(), plan, batch.Request, quota.Bootstrap, observe)
		if issued.Outcome != "issued-offline" {
			t.Fatal("issuer refused genuine handle", issued.Outcome)
		}
		if completed := send(map[string]any{"operation": "complete", "payload": issued.Response}); completed.Outcome != "completed" {
			t.Fatal("holder completion", completed.Outcome)
		}
	}
}
