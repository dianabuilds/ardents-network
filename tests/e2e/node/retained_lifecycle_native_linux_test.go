//go:build linux && node_process_native

package node

import (
	"crypto/sha256"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/reachability"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func TestNativeForwardingRetainedLifecycle(t *testing.T) {
	testNativeRetainedLifecycle(t, "forwarding", processresource.Profile, 12, 3, 1, hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20})
}

func TestNativeDescriptorRetainedLifecycle(t *testing.T) {
	testNativeRetainedLifecycle(t, "descriptor", processresource.DescriptorProfile, 17, 2, 5, hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10})
}

func TestNativeJoinRetainedLifecycle(t *testing.T) {
	testNativeRetainedLifecycle(t, "join", processresource.JoinProfile, 20, 2, 4, hosting.Traffic{Tx: 32 << 20})
}

// The transient native process proves listening, joined cancellation and the
// durable refusal after a killed original process. Installed manager repair
// and live Route traffic each have their own native profiles.
func testNativeRetainedLifecycle(t *testing.T, role, profile string, id, domain, subrole byte, work hosting.Traffic) {
	t.Helper()
	binary := processfixture.PinnedCommand(t)
	spec, _, _ := processfixture.PrepareNetworkGenesis(t, t.TempDir())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	spec.Nodes[1].Endpoint = listener.Addr().String()
	spec.Nodes[1].NodeID = [32]byte{id}
	spec.Nodes[1].RoleDomain = domain
	spec.Nodes[1].Subrole = subrole
	bundle, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := processfixture.StartObservedClock(t)
	defer closeClock()
	networkPlan := processfixture.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock)

	member := bundle.Nodes[1]
	key := member.PrivateKey
	certPath, keyPath := processfixture.WriteNativeCredential(t, processfixture.RoleCertificate(t, key))
	budget := processfixture.ProcessBudget(t)
	participation := filepath.Join(t.TempDir(), "participation")
	spend := t.TempDir()
	plan := map[string]any{"network": networkPlan, "node_id": member.NodeID, "process_profile": profile,
		"participation_root": participation, "spend_root": spend, "hosting_root": budget,
		"certificate": certPath, "private_key": keyPath,
		"work": work, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
	var storeConfig reachability.StoreConfig
	if role == "descriptor" {
		storeConfig = reachability.StoreConfig{Root: t.TempDir(), Network: networkPlan.NetworkID}
		plan["descriptor_root"] = storeConfig.Root
		store, err := reachability.OpenStore(storeConfig)
		if err != nil {
			t.Fatal("Store root provisioning refused", err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}
	planPath := writeNodePlan(t, plan)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	producerPresent := func(want bool, phase string) {
		t.Helper()
		journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation, Clock: time.Now})
		if err != nil {
			t.Fatal(phase, "participation unavailable", err)
		}
		conflict, err := journal.Conflict(member.NodeID, sha256.Sum256([]byte(member.Family)))
		closeErr := journal.Close()
		if err != nil || closeErr != nil || conflict != want {
			t.Fatal(phase, "node-duty producer presence", conflict, want, err, closeErr)
		}
	}

	// First installation: the fresh --config composition creates the participation
	// and receiving histories and claims the producer before a requested stop
	// joins and removes the producer.
	first := processfixture.StartFreshRole(t, role, binary, planPath, id)
	producerPresent(true, "first launch")
	first.Stop(t)
	producerPresent(false, "first completion")

	// Retained relaunch over the existing histories: a new independently checked
	// invocation claims the producer again and joins cleanly.
	relaunch := processfixture.StartRetainedRole(t, role, binary, planPath, id)
	producerPresent(true, "retained relaunch")
	relaunch.Stop(t)
	producerPresent(false, "retained relaunch completion")

	// A killed retained invocation supplies no completion evidence: the live
	// node-duty fence stays retained and the next retained opening refuses before
	// listening. Repair against real manager facts is the installed gate (stage C).
	crash := processfixture.StartRetainedRole(t, role, binary, planPath, id)
	producerPresent(true, "crash launch")
	code, _ := processfixture.JoinRetainedRole(t, crash, "SIGKILL")
	if code == 0 || code == 130 {
		t.Fatal("killed role reported joined completion", role, code)
	}
	producerPresent(true, "after crash")
	if role == "descriptor" {
		// The process died with no publication in flight. The original Store
		// must reopen without an uncertain stage or a live lease.
		store, err := reachability.OpenRetainedStore(storeConfig)
		if err != nil {
			t.Fatal("retained Store refused after crash without uncertain stage", err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}
	refused := processfixture.LaunchRetainedRole(t, role, binary, planPath, id)
	code, event := processfixture.JoinRetainedRole(t, refused, "")
	if code != 1 || event["phase"] != "refused" {
		t.Fatal("retained producer adopted after crash", code, event, refused.Diagnostic.String())
	}
	producerPresent(true, "after refused relaunch")
}

func writeNodePlan(t testing.TB, plan map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(planPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return planPath
}
