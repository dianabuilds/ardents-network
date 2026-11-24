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
	"strconv"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

type completionRole struct {
	name, profile, historyField string
	id, domain, subrole         byte
	work                        hosting.Traffic
}

func TestNativeNodeUnavailableCompletionRetainsOriginalProducerTCP(t *testing.T) {
	testNativeUnavailableCompletion(t, completionRole{name: "forwarding", profile: processresource.Profile, id: 12, domain: 3, subrole: 1, work: hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}})
}

func TestNativeDescriptorUnavailableCompletionRetainsOriginalProducerTCP(t *testing.T) {
	testNativeUnavailableCompletion(t, completionRole{name: "descriptor", profile: processresource.DescriptorProfile, id: 17, domain: 2, subrole: 5, historyField: "descriptor_root", work: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
}

func TestNativeIntroductionUnavailableCompletionRetainsOriginalProducerTCP(t *testing.T) {
	testNativeUnavailableCompletion(t, completionRole{name: "introduction", profile: processresource.IntroductionProfile, id: 25, domain: 4, subrole: 3, historyField: "introduction_root", work: hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20}})
}

func TestNativeIssuerUnavailableCompletionRetainsOriginalProducerTCP(t *testing.T) {
	testNativeUnavailableCompletion(t, completionRole{name: "issuer", profile: processresource.IssuerProfile, id: 11, domain: 2, subrole: 6, work: hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}})
}

func TestNativeJoinUnavailableCompletionRetainsOriginalProducerTCP(t *testing.T) {
	testNativeUnavailableCompletion(t, completionRole{name: "join", profile: processresource.JoinProfile, id: 20, domain: 2, subrole: 4, work: hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}})
}

func testNativeUnavailableCompletion(t *testing.T, role completionRole) {
	t.Helper()
	binary := processfixture.PinnedCommand(t)
	t.Run(epoch.CarrierClosedTCP, func(t *testing.T) {
		spec, issuerPlan, inventory := processfixture.PrepareNetworkGenesis(t, t.TempDir())
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = listener.Close() })
		index := 1
		if role.name == "issuer" {
			index = 0
		}
		id := [32]byte{role.id}
		spec.Nodes[index].NodeID = id
		spec.Nodes[index].RoleDomain, spec.Nodes[index].Subrole = role.domain, role.subrole
		spec.Nodes[index].Endpoint = listener.Addr().String()
		bundle, err := networkfixture.BuildClosed(spec)
		if err != nil {
			t.Fatal(err)
		}
		clock, stopClock := processfixture.StartObservedClock(t)
		defer stopClock()
		network := processfixture.SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock)
		config, err := network.Config()
		if err != nil {
			t.Fatal(err)
		}
		owner, err := state.OpenRetained(config)
		if err != nil {
			t.Fatal(err)
		}
		view, viewErr := owner.CurrentRuntime()
		if closeErr := owner.Close(); viewErr != nil || closeErr != nil {
			t.Fatal("read original signed State", viewErr, closeErr)
		}
		member, err := view.Member(id, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}
		retained, err := view.RetainDuty(id, view.ObservedAt())
		if err != nil {
			t.Fatal(err)
		}

		cert, key := processfixture.WriteNativeCredential(t, nativeRoleCertificate(t, spec.Nodes[index].PrivateKey))
		participation := filepath.Join(t.TempDir(), "participation")
		plan := map[string]any{
			"network": network, "node_id": id, "process_profile": role.profile,
			"participation_root": participation, "spend_root": t.TempDir(), "hosting_root": processfixture.ProcessBudget(t),
			"certificate": cert, "private_key": key, "work": role.work,
			"termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 1,
		}
		if role.historyField != "" {
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			plan[role.historyField] = root
		}
		if role.name == "issuer" {
			plan["bootstrap_work"] = hosting.Traffic{Tx: 128 << 10, Rx: 128 << 10}
			prepareNativeIssuer(t, &issuerPlan, inventory, spec, bundle.Nodes[index], sha256.Sum256(bundle.Profile))
			plan["issuance"] = IssuerDeclaration(issuerPlan)
		}
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
		process := processfixture.StartFreshRole(t, role.name, binary, writeNodePlan(t, plan), 91)

		journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation, Clock: time.Now})
		if err != nil {
			t.Fatal(err)
		}
		defer journal.Close()
		if err := exec.CommandContext(t.Context(), "systemctl", "kill", "--kill-whom=main", "--signal=SIGINT", process.Unit).Run(); err != nil {
			t.Fatal(err)
		}
		if role.name != "forwarding" {
			waitForUnavailableCompletion(t, process)
			if role.name != "issuer" {
				config.LocalRoleStateRoot = participation
				if duplicate, err := state.Open(config); err == nil {
					t.Fatal("command returned its original State before physical join", duplicate.Close())
				}
			}
		}
		requireUnavailableCompletion(t, process, role.name)

		conflict, err := journal.Conflict(member.NodeID, member.FamilyID)
		if err != nil || !conflict {
			t.Fatal("unknown completion erased original producer", conflict, err)
		}
		if err := journal.Close(); err != nil {
			t.Fatal(err)
		}
		reopened, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation, Clock: time.Now})
		if err != nil {
			t.Fatal(err)
		}
		conflict, err = reopened.Conflict(member.NodeID, member.FamilyID)
		if closeErr := reopened.Close(); err != nil || closeErr != nil || !conflict {
			t.Fatal("reopen inferred withdrawal from process absence", conflict, err, closeErr)
		}
		fresh, err := duty.OpenNodeProducer(t.Context(), duty.Config{Root: participation, Clock: time.Now}, duty.NodeParticipation{
			Identity: member.NodeID, FamilyID: member.FamilyID, Assignment: member.Assignment,
			EpochUntil: retained.Epoch.ValidUntil, RecordUntil: member.NotAfter(),
		})
		if fresh != nil {
			t.Fatal("replacement producer adopted unresolved original", err, fresh.Close(t.Context()))
		}
		if !errors.Is(err, duty.ErrLocalRoleConflict) {
			t.Fatal("expected the original producer conflict", err)
		}
	})
}

func waitForUnavailableCompletion(t *testing.T, process *processfixture.NativeProcess) {
	t.Helper()
	timeout := time.NewTimer(500 * time.Millisecond)
	defer timeout.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for !bytes.Contains([]byte(process.Diagnostic.String()), []byte("completion-unavailable")) {
		select {
		case <-timeout.C:
			t.Fatal("bounded unavailable report absent", process.Diagnostic.String())
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		case <-ticker.C:
		}
	}
	if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(process.PID))); err != nil {
		t.Fatal("command exited before original producer join", err)
	}
}

func requireUnavailableCompletion(t *testing.T, process *processfixture.NativeProcess, role string) {
	t.Helper()
	waitErr := process.Wait()
	var exit *exec.ExitError
	decoder := json.NewDecoder(bytes.NewBufferString(process.Diagnostic.String()))
	var unavailable, terminal map[string]string
	if !errors.As(waitErr, &exit) || exit.ExitCode() != 1 {
		t.Fatal("bounded failure replaced by success", waitErr, process.Diagnostic.String())
	}
	if err := decoder.Decode(&unavailable); err != nil || unavailable["operation"] != "node."+role || unavailable["phase"] != "retiring" || unavailable["outcome"] != "completion-unavailable" {
		t.Fatal("unavailable projection changed", unavailable, err)
	}
	if role == "forwarding" {
		var first struct{ Kind, Phase, Cause string }
		if err := json.Unmarshal([]byte(unavailable["first_failure"]), &first); err != nil || first.Kind != "logical" || first.Phase != "serving" || first.Cause != "canceled" {
			t.Fatal("bounded unknown completion lost known original caller cause", err, process.Diagnostic.String())
		}
	} else if err := decoder.Decode(&terminal); err != nil || terminal["phase"] != "joined" || terminal["outcome"] != "failed" || terminal["cleanup_owner"] != "participation" {
		t.Fatal("failed producer cleanup reported withdrawal", terminal, err, process.Diagnostic.String())
	}
	if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(process.PID))); !os.IsNotExist(err) {
		t.Fatal("original process remains after manager join", err)
	}
}
