//go:build linux && node_process_native

package main

import (
	"crypto/x509"
	"encoding/pem"
	"os"
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

func TestNativeForwardingRetainedLifecycle(t *testing.T) {
	binary := nodefixture.PinnedCommand(t)
	f, sockets, certificates := newRoleRouteFixtureConfigured(t, transport.ClosedCarrierTCP, 3, false, false, nil)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := nodefixture.StartObservedClock(t)
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
	budget := nodefixture.ProcessBudget(t)
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
	relaunch := nodefixture.StartRetainedRole(t, "forwarding", binary, planPath, 12)
	producerPresent(true, "retained relaunch")
	relaunch.Stop(t)
	producerPresent(false, "retained relaunch completion")

	// A killed retained invocation supplies no completion evidence: the live
	// node-duty fence stays retained and the next retained opening refuses before
	// listening. Repair against real manager facts is the installed gate (stage C).
	crash := nodefixture.StartRetainedRole(t, "forwarding", binary, planPath, 12)
	producerPresent(true, "crash launch")
	code, _ := nodefixture.JoinRetainedRole(t, crash, "SIGKILL")
	if code == 0 || code == 130 {
		t.Fatal("killed Forwarding reported joined completion", code)
	}
	producerPresent(true, "after crash")
	refused := nodefixture.LaunchRetainedRole(t, "forwarding", binary, planPath, 12)
	code, event := nodefixture.JoinRetainedRole(t, refused, "")
	if code != 1 || event["phase"] != "refused" {
		t.Fatal("retained producer adopted after crash", code, event, refused.Diagnostic.String())
	}
	producerPresent(true, "after refused relaunch")
}
