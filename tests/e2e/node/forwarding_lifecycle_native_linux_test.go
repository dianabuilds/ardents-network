//go:build linux && node_process_native

package node

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// Retained Forwarding lifecycle (#604 FJD stage A): the first installation runs
// the fresh --config composition (creating participation and receiving
// histories), then every subsequent invocation is an independently checked
// --retained-config opening over those existing histories. Proves listening,
// joined cancellation with producer removal,
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
	binary := PinnedCommand(t)
	spec, _, _ := PrepareNetworkGenesis(t, t.TempDir())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	spec.Nodes[1].Endpoint = listener.Addr().String()
	bundle, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	clock, closeClock := StartObservedClock(t)
	defer closeClock()
	networkPlan := SeedNetworkState(t, spec.NetworkID, [32]byte(spec.Authority[32:]), bundle, clock)

	member := bundle.Nodes[1]
	key := member.PrivateKey
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	leaf, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath := WriteNativeCredential(t, tls.Certificate{Certificate: [][]byte{leaf}, PrivateKey: key})
	budget := ProcessBudget(t)
	participation := filepath.Join(t.TempDir(), "participation")
	spend := t.TempDir()
	plan := map[string]any{"network": networkPlan, "node_id": member.NodeID, "process_profile": processresource.Profile,
		"participation_root": participation, "spend_root": spend, "hosting_root": budget,
		"certificate": certPath, "private_key": keyPath,
		"work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(planPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
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
	first := StartForwarding(t, binary, planPath, 12)
	producerPresent(true, "first launch")
	first.Stop(t)
	producerPresent(false, "first completion")

	// Retained relaunch over the existing histories: a new independently checked
	// invocation claims the producer again and joins cleanly.
	relaunch := StartRetainedRole(t, "forwarding", binary, planPath, 12)
	producerPresent(true, "retained relaunch")
	relaunch.Stop(t)
	producerPresent(false, "retained relaunch completion")

	// A killed retained invocation supplies no completion evidence: the live
	// node-duty fence stays retained and the next retained opening refuses before
	// listening. Repair against real manager facts is the installed gate (stage C).
	crash := StartRetainedRole(t, "forwarding", binary, planPath, 12)
	producerPresent(true, "crash launch")
	code, _ := JoinRetainedRole(t, crash, "SIGKILL")
	if code == 0 || code == 130 {
		t.Fatal("killed Forwarding reported joined completion", code)
	}
	producerPresent(true, "after crash")
	refused := LaunchRetainedRole(t, "forwarding", binary, planPath, 12)
	code, event := JoinRetainedRole(t, refused, "")
	if code != 1 || event["phase"] != "refused" {
		t.Fatal("retained producer adopted after crash", code, event, refused.Diagnostic.String())
	}
	producerPresent(true, "after refused relaunch")
}
