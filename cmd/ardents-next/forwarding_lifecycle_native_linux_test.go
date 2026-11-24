//go:build linux && node_process_native

package main

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
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
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// Retained Forwarding lifecycle (#604 FJD stage A): the first installation runs
// the fresh --config composition (creating participation and receiving
// histories), then every subsequent invocation is an independently checked
// --retained-config opening over those existing histories. Proves listening,
// genuine Route prefix forwarding, joined cancellation with producer removal,
// retained relaunch, and crash refusal (a killed invocation leaves the live
// node-duty fence; the next retained opening refuses before listening). The
// repair-retained recovery leg against real manager facts is the separate
// installed platform gate (stage C); here the crash leaves the producer retained
// and the relaunch refuses, exactly as the transient Source lifecycle (1.2).
//
// Honest limits: transient systemd-run units, no installed ardents-forwarding.service,
// no loaded-manager platform check pre-start and no repair-retained manager
// qualification. One fixture operator, not independent participants.

func launchRetainedForwarding(t *testing.T, binary, plan string, index byte) *nodefixture.NativeProcess {
	t.Helper()
	unit := fmt.Sprintf("ardents-forwarding-retained-%d-%d.service", os.Getpid(), index)
	return nodefixture.Launch(t, unit, "retained Forwarding original diagnostic", "",
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--", binary, "node", "forwarding", "--retained-config", plan)
}

func startRetainedForwarding(t *testing.T, binary, plan string, index byte) *nodefixture.NativeProcess {
	t.Helper()
	process := launchRetainedForwarding(t, binary, plan, index)
	var event map[string]string
	if err := json.NewDecoder(process.Stdout).Decode(&event); err != nil || event["operation"] != "node.forwarding" || event["phase"] != "listening" {
		t.Fatal("genuine retained Forwarding startup refused", event, err, process.Diagnostic.String())
	}
	raw, err := exec.Command("systemctl", "show", process.Unit, "-p", "MainPID", "--value").Output()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Sscanf(string(raw), "%d", &process.PID); err != nil || process.PID < 2 {
		t.Fatal("actual retained Forwarding MainPID unavailable", err)
	}
	return process
}

// joinRetainedForwarding signals (or waits for a self-refusing invocation) and
// joins the actual command, returning the exit code and the stdout report.
func joinRetainedForwarding(t *testing.T, p *nodefixture.NativeProcess, signal string) (int, map[string]string) {
	t.Helper()
	if signal != "" {
		if err := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal="+signal, p.Unit).Run(); err != nil {
			t.Fatal("retained Forwarding signal", signal, err)
		}
	}
	stdout, readErr := io.ReadAll(p.Stdout)
	err := p.Command.Wait()
	p.Joined = true
	if readErr != nil {
		t.Fatal("retained Forwarding stdout", readErr)
	}
	code := 0
	var exited *exec.ExitError
	if err != nil {
		if !errors.As(err, &exited) {
			t.Fatal("retained Forwarding wait", err)
		}
		code = exited.ExitCode()
	}
	var event map[string]string
	if raw := bytes.TrimSpace(stdout); len(raw) > 0 {
		if json.Unmarshal(raw, &event) != nil {
			t.Fatal("retained Forwarding stdout report", string(raw))
		}
	}
	return code, event
}

func TestNativeForwardingRetainedLifecycle(t *testing.T) {
	binary := nodefixture.PinnedCommand(t)
	f, sockets, certificates := newRoleRouteFixtureConfigured(t, transport.ClosedCarrierTCP, 3, false, false, nil)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := joinProcessClock(t)
	defer closeClock()
	networkPlan := nodefixture.SeedNetworkState(t, f.spec.NetworkID, [32]byte(f.spec.Authority[32:]), bundle, clock)

	id := [32]byte{12}
	certificate := certificates[id]
	certPath, keyPath := filepath.Join(t.TempDir(), "certificate"), filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0600); err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600); err != nil {
		t.Fatal(err)
	}
	clear(key)
	budget := routeProcessBudget(t)
	participation := filepath.Join(t.TempDir(), "participation")
	spend := t.TempDir()
	plan := map[string]any{"network": networkPlan, "node_id": id, "process_profile": processresource.Profile,
		"participation_root": participation, "spend_root": spend, "hosting_root": budget,
		"certificate": certPath, "private_key": keyPath,
		"work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
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
	// and receiving histories, claims the producer and genuinely forwards a Route
	// prefix before a requested stop joins and removes the producer.
	first := startNativeForwarding(t, binary, planPath, 12)
	producerPresent(true, "first launch")
	first.Stop(t)
	producerPresent(false, "first completion")

	// Retained relaunch over the existing histories: a new independently checked
	// invocation claims the producer again and joins cleanly.
	relaunch := startRetainedForwarding(t, binary, planPath, 12)
	producerPresent(true, "retained relaunch")
	relaunch.Stop(t)
	producerPresent(false, "retained relaunch completion")

	// A killed retained invocation supplies no completion evidence: the live
	// node-duty fence stays retained and the next retained opening refuses before
	// listening. Repair against real manager facts is the installed gate (stage C).
	crash := startRetainedForwarding(t, binary, planPath, 12)
	producerPresent(true, "crash launch")
	code, _ := joinRetainedForwarding(t, crash, "SIGKILL")
	if code == 0 || code == 130 {
		t.Fatal("killed Forwarding reported joined completion", code)
	}
	producerPresent(true, "after crash")
	refused := launchRetainedForwarding(t, binary, planPath, 12)
	code, event := joinRetainedForwarding(t, refused, "")
	if code != 1 || event["phase"] != "refused" {
		t.Fatal("retained producer adopted after crash", code, event, refused.Diagnostic.String())
	}
	producerPresent(true, "after refused relaunch")
}
