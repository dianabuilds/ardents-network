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
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/domain/network/duty"
	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
	networkinputs "github.com/dianabuilds/ardents-network/tests/networkfixture"
)

type receivingStartupRole struct {
	name, profile, historyField, historyMarker, lateFault string
	id, domain, subrole                                   byte
	work                                                  hosting.Traffic
}

func TestNativeDescriptorPartialStartupRefusalsTCP(t *testing.T) {
	testNativeReceivingStartup(t, receivingStartupRole{
		name: "descriptor", profile: processresource.DescriptorProfile, id: 17, domain: 2, subrole: 5,
		historyField: "descriptor_root", historyMarker: ".ardents-reachability-store-v3", lateFault: "foreign-store",
		work: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10},
	})
}

func TestNativeIntroductionPartialStartupRefusalsTCP(t *testing.T) {
	testNativeReceivingStartup(t, receivingStartupRole{
		name: "introduction", profile: processresource.IntroductionProfile, id: 25, domain: 4, subrole: 3,
		historyField: "introduction_root", historyMarker: "closed-introduction-slots", lateFault: "foreign-history",
		work: hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20},
	})
}

func TestNativeJoinPartialStartupRefusalsTCP(t *testing.T) {
	testNativeReceivingStartup(t, receivingStartupRole{
		name: "join", profile: processresource.JoinProfile, id: 20, domain: 2, subrole: 4,
		lateFault: "listener", work: hosting.Traffic{Tx: 32 << 20},
	})
}

func testNativeReceivingStartup(t *testing.T, role receivingStartupRole) {
	t.Helper()
	binary := processfixture.PinnedCommand(t)
	t.Run(epoch.CarrierClosedTCP, func(t *testing.T) {
		for _, fault := range []string{"profile", "unknown-field", "nested-roots", "role", "tls-key", "missing-hosting", role.lateFault, "foreign-spend"} {
			t.Run(fault, func(t *testing.T) {
				testReceivingStartupFault(t, binary, role, fault)
			})
		}
	})
}

func testReceivingStartupFault(t *testing.T, binary string, role receivingStartupRole, fault string) {
	t.Helper()
	spec, _, _ := networkinputs.PrepareNetworkGenesis(t, t.TempDir())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	spec.Nodes[1].Endpoint = listener.Addr().String()
	spec.Nodes[1].NodeID = [32]byte{role.id}
	spec.Nodes[1].RoleDomain, spec.Nodes[1].Subrole = role.domain, role.subrole
	bundle, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	member, other := bundle.Nodes[1], bundle.Nodes[0]
	clock, stopClock := networkinputs.StartObservedClock(t)
	defer stopClock()
	network := networkinputs.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock)
	cert, key := processfixture.WriteNativeCredential(t, processfixture.RoleCertificate(t, member.PrivateKey))
	participation, spend, budget := filepath.Join(t.TempDir(), "participation"), t.TempDir(), processfixture.ProcessBudget(t)
	plan := map[string]any{
		"network": network, "node_id": member.NodeID, "process_profile": role.profile,
		"participation_root": participation, "spend_root": spend, "hosting_root": budget,
		"certificate": cert, "private_key": key, "work": role.work,
		"termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000,
	}
	var history string
	if role.historyField != "" {
		history = t.TempDir()
		plan[role.historyField] = history
	}
	if role.name == "introduction" {
		if err := os.Chmod(history, 0700); err != nil {
			t.Fatal(err)
		}
	}
	expected := 1
	var foreign string
	switch fault {
	case "profile":
		plan["process_profile"], expected = processresource.Profile, 2
	case "unknown-field":
		plan["accepting"], expected = true, 2
	case "nested-roots":
		if role.historyField == "" {
			plan["spend_root"] = filepath.Join(participation, "nested")
		} else {
			plan[role.historyField] = filepath.Join(spend, "nested")
		}
		expected = 2
	case "role":
		plan["node_id"] = other.NodeID
	case "tls-key":
		plan["certificate"], plan["private_key"] = processfixture.WriteNativeCredential(t, processfixture.RoleCertificate(t, other.PrivateKey))
	case "missing-hosting":
		plan["hosting_root"] = filepath.Join(t.TempDir(), "missing")
	case "foreign-store", "foreign-history":
		foreign = filepath.Join(history, role.historyMarker)
	case "foreign-spend":
		foreign = filepath.Join(spend, "foreign-input")
	}

	foreignBytes := []byte("independent rejected input\n")
	if foreign != "" {
		if err := os.WriteFile(foreign, foreignBytes, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if fault != "listener" {
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
	}
	requireReceivingStartupRefusal(t, binary, role, plan, expected)
	spendBytes, spendErr := os.ReadFile(filepath.Join(spend, "closed-token-spends"))
	if fault == "listener" || fault == "foreign-history" {
		if spendErr != nil || len(spendBytes) != 112 || string(spendBytes[:8]) != "ARDSPN01" {
			t.Fatal("late startup refusal changed the empty receiving ledger", spendErr)
		}
	} else if !os.IsNotExist(spendErr) {
		t.Fatal("earlier refusal created receiving history", spendErr)
	}
	if foreign != "" {
		raw, err := os.ReadFile(foreign)
		if err != nil || !bytes.Equal(raw, foreignBytes) {
			t.Fatal("rejected private input was rewritten", err)
		}
		if err := os.Remove(foreign); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(participation); err == nil {
		journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation, Clock: time.Now})
		if err != nil {
			t.Fatal("partial startup participation not released", err)
		}
		conflict, err := journal.Conflict(member.NodeID, sha256.Sum256([]byte(member.Family)))
		if closeErr := journal.Close(); err != nil || closeErr != nil || conflict {
			t.Fatal("partial startup retained original producer", err, closeErr, conflict)
		}
	} else if !os.IsNotExist(err) {
		t.Fatal("inspect participation after refusal", err)
	}

	delete(plan, "accepting")
	plan["process_profile"], plan["node_id"] = role.profile, member.NodeID
	plan["certificate"], plan["private_key"] = cert, key
	plan["hosting_root"], plan["spend_root"] = budget, spend
	if role.historyField != "" {
		plan[role.historyField] = history
	}
	if fault == "foreign-history" {
		// An existing empty receiving root cannot recreate the original birth fact.
		requireReceivingStartupRefusal(t, binary, role, plan, 1)
		plan["spend_root"], history = t.TempDir(), t.TempDir()
		if err := os.Chmod(history, 0700); err != nil {
			t.Fatal(err)
		}
		plan[role.historyField] = history
	}
	if fault == "listener" {
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
	}
	fresh := processfixture.StartFreshRole(t, role.name, binary, writeNodePlan(t, plan), role.id)
	fresh.Stop(t)
}

func requireReceivingStartupRefusal(t *testing.T, binary string, role receivingStartupRole, plan map[string]any, expected int) {
	t.Helper()
	p := processfixture.LaunchFreshRole(t, role.name, binary, writeNodePlan(t, plan), role.id)
	var event map[string]string
	decodeErr := json.NewDecoder(p.Stdout).Decode(&event)
	err := p.Wait()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != expected {
		t.Fatal("original startup did not refuse", err, p.Diagnostic.String())
	}
	if expected == 2 {
		if decodeErr == nil || len(event) != 0 {
			t.Fatal("invalid declaration reached runtime", event)
		}
		return
	}
	if decodeErr != nil || len(event) != 4 || event["operation"] != "node."+role.name || event["phase"] != "refused" || event["outcome"] != "startup-unavailable" {
		t.Fatal("partial startup lost bounded refusal", event, decodeErr)
	}
	processfixture.RequireStartupFailure(t, event["first_failure"])
}
