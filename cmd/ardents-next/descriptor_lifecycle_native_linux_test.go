//go:build linux && node_process_native

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/reachability"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// Retained Descriptor lifecycle (#604 FJD stage A): the first installation runs
// the fresh --config composition (creating the participation and receiving
// histories and the Store root), then every subsequent invocation is an
// independently checked --retained-config opening over those existing histories.
// Proves listening, joined cancellation with producer removal, retained
// relaunch, and crash refusal (a killed invocation leaves the live node-duty
// fence; the next retained opening refuses before listening). The repair-retained
// recovery leg against real manager facts — including the bounded Store
// uncertain-stage removal after a crash mid-Publish — is the separate installed
// platform gate (stage C); here the crash leaves the producer retained and the
// relaunch refuses, exactly as the retained Forwarding/JOIN lifecycles.
//
// The lifecycle legs are single-unit: one Descriptor member (id 17, domain 2,
// subrole 5) owns its own participation root and its own sole node-duty fence,
// plus its own Store root. The readiness oracle is the authenticated listener
// (the listening event), NOT publication readiness; genuine publish/lookup over
// the Route prefix is proven by TestNativeDescriptorPublishLookupAndReopenTCP.
//
// Honest limits: transient systemd-run units, no installed
// ardents-descriptor.service, no loaded-manager platform check pre-start and no
// repair-retained manager qualification. One fixture operator, not independent
// participants.
func launchRetainedDescriptor(t *testing.T, binary, plan string, index byte) *nodefixture.NativeProcess {
	t.Helper()
	unit := fmt.Sprintf("ardents-descriptor-retained-%d-%d.service", os.Getpid(), index)
	return nodefixture.Launch(t, unit, "retained Descriptor original diagnostic", "",
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--", binary, "node", "descriptor", "--retained-config", plan)
}

func startRetainedDescriptor(t *testing.T, binary, plan string, index byte) *nodefixture.NativeProcess {
	t.Helper()
	process := launchRetainedDescriptor(t, binary, plan, index)
	var event map[string]string
	if err := json.NewDecoder(process.Stdout).Decode(&event); err != nil || event["operation"] != "node.descriptor" || event["phase"] != "listening" {
		t.Fatal("genuine retained Descriptor startup refused", event, err, process.Diagnostic.String())
	}
	raw, err := exec.Command("systemctl", "show", process.Unit, "-p", "MainPID", "--value").Output()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Sscanf(string(raw), "%d", &process.PID); err != nil || process.PID < 2 {
		t.Fatal("actual retained Descriptor MainPID unavailable", err)
	}
	return process
}

// joinRetainedDescriptor signals (or waits for a self-refusing invocation) and
// joins the actual command, returning the exit code and the stdout report.
func joinRetainedDescriptor(t *testing.T, p *nodefixture.NativeProcess, signal string) (int, map[string]string) {
	t.Helper()
	if signal != "" {
		if err := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal="+signal, p.Unit).Run(); err != nil {
			t.Fatal("retained Descriptor signal", signal, err)
		}
	}
	stdout, readErr := io.ReadAll(p.Stdout)
	err := p.Command.Wait()
	p.Joined = true
	if readErr != nil {
		t.Fatal("retained Descriptor stdout", readErr)
	}
	code := 0
	var exited *exec.ExitError
	if err != nil {
		if !errors.As(err, &exited) {
			t.Fatal("retained Descriptor wait", err)
		}
		code = exited.ExitCode()
	}
	var event map[string]string
	if raw := bytes.TrimSpace(stdout); len(raw) > 0 {
		if json.Unmarshal(raw, &event) != nil {
			t.Fatal("retained Descriptor stdout report", string(raw))
		}
	}
	return code, event
}

func TestNativeDescriptorRetainedLifecycle(t *testing.T) {
	binary := nodefixture.PinnedCommand(t)
	f, sockets, certificates := descriptorRouteFixtureConfigured(t, transport.ClosedCarrierTCP, nil)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := nodefixture.StartObservedClock(t)
	defer closeClock()
	networkPlan := nodefixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), bundle, clock)

	id := [32]byte{17}
	certPath, keyPath := nodefixture.WriteNativeCredential(t, certificates[id])
	budget := routeProcessBudget(t)
	participation := filepath.Join(t.TempDir(), "participation")
	spend := t.TempDir()
	storeRoot := t.TempDir()
	// Provision the Store root through the genuine fresh opener, then release
	// it; the first --config launch and every retained launch reopen this same
	// existing root.
	provisioned, err := reachability.OpenStore(reachability.StoreConfig{Root: storeRoot, Network: networkPlan.NetworkID})
	if err != nil {
		t.Fatal("Store root provisioning refused", err)
	}
	if err := provisioned.Close(); err != nil {
		t.Fatal(err)
	}
	plan := map[string]any{"network": networkPlan, "node_id": id, "process_profile": processresource.DescriptorProfile,
		"participation_root": participation, "spend_root": spend, "hosting_root": budget, "descriptor_root": storeRoot,
		"certificate": certPath, "private_key": keyPath,
		"work": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
	planPath := hostingConfig(t, plan)
	sockets[id]()

	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	member, err := view.Member(id, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	producerPresent := func(want bool, phase string) {
		t.Helper()
		journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation, Clock: time.Now})
		if err != nil {
			t.Fatal(phase, "participation unavailable", err)
		}
		conflict, err := journal.Conflict(member.NodeID, member.FamilyID)
		closeErr := journal.Close()
		if err != nil || closeErr != nil || conflict != want {
			t.Fatal(phase, "node-duty producer presence", conflict, want, err, closeErr)
		}
	}

	// First installation: the fresh --config composition creates the participation
	// and receiving histories, claims the producer and listens before a requested
	// stop joins and removes the producer.
	first := startNativeDescriptor(t, binary, planPath, id[0])
	producerPresent(true, "first launch")
	first.Stop(t)
	producerPresent(false, "first completion")

	// Retained relaunch over the existing histories: a new independently checked
	// invocation claims the producer again and joins cleanly.
	relaunch := startRetainedDescriptor(t, binary, planPath, id[0])
	producerPresent(true, "retained relaunch")
	relaunch.Stop(t)
	producerPresent(false, "retained relaunch completion")

	// A killed retained invocation supplies no completion evidence: the live
	// node-duty fence stays retained and the next retained opening refuses before
	// listening. Repair against real manager facts is the installed gate (stage C).
	crash := startRetainedDescriptor(t, binary, planPath, id[0])
	producerPresent(true, "crash launch")
	code, _ := joinRetainedDescriptor(t, crash, "SIGKILL")
	if code == 0 || code == 130 {
		t.Fatal("killed Descriptor reported joined completion", code)
	}
	producerPresent(true, "after crash")
	// The Store flock lease died with the killed process; no publication was in
	// flight, so the retained Store history reopens without an uncertain stage.
	// The causal mid-Publish stage-recovery leg is the installed gate (stage C).
	storeConfig := reachability.StoreConfig{Root: storeRoot, Network: networkPlan.NetworkID}
	reopened, err := reachability.OpenRetainedStore(storeConfig)
	if err != nil {
		t.Fatal("retained Store refused after crash without uncertain stage", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	refused := launchRetainedDescriptor(t, binary, planPath, id[0])
	code, event := joinRetainedDescriptor(t, refused, "")
	if code != 1 || event["phase"] != "refused" {
		t.Fatal("retained producer adopted after crash", code, event, refused.Diagnostic.String())
	}
	producerPresent(true, "after refused relaunch")
}
