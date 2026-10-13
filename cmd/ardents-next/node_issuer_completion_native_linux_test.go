//go:build linux && node_process_native

package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// The actual original producer's cleanup encounters a genuinely occupied
// journal lease. The command's one-millisecond wait must remain unavailable,
// even after the manager proves process termination. Neither process absence
// nor a new caller may erase or adopt the unresolved producer.
func TestNativeIssuerUnavailableCompletionRetainsOriginalProducerBothCarriers(t *testing.T) {
	binary := nodeNativeCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			f, sockets, certificates := newRoleRouteFixture(t, carrier, 1, false, true)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := joinProcessClock(t)
			defer closeClock()
			certificate := certificates[[32]byte{11}]
			certPath, keyPath := filepath.Join(t.TempDir(), "certificate"), filepath.Join(t.TempDir(), "key")
			if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}), 0600); err != nil {
				t.Fatal(err)
			}
			key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
			if err != nil {
				t.Fatal(err)
			}
			encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
			if err := os.WriteFile(keyPath, encoded, 0600); err != nil {
				t.Fatal(err)
			}
			clear(key)
			clear(encoded)
			participation := filepath.Join(t.TempDir(), "participation")
			plan := map[string]any{"network": seedNativeNodeState(t, f, bundle, clock), "node_id": [32]byte{11}, "process_profile": processresource.IssuerProfile, "participation_root": participation, "spend_root": t.TempDir(), "hosting_root": routeProcessBudget(t), "certificate": certPath, "private_key": keyPath, "work": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "bootstrap_work": hosting.Traffic{Tx: 128 << 10, Rx: 128 << 10}, "issuance": nativeIssuerIssuanceInput(f.plan), "retirement_wait_ms": 1}
			sockets[[32]byte{11}]()
			process := startNativeIssuer(t, binary, hostingConfig(t, plan))
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			member, err := view.Member([32]byte{11}, view.ObservedAt())
			if err != nil {
				t.Fatal(err)
			}
			retained, err := view.RetainDuty(member.NodeID, view.ObservedAt())
			if err != nil {
				t.Fatal(err)
			}
			journal, err := duty.OpenOperation(t.Context(), duty.Config{Root: participation, Clock: time.Now})
			if err != nil {
				t.Fatal(err)
			}
			defer journal.Close()
			if err := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal=SIGINT", process.unit).Run(); err != nil {
				t.Fatal(err)
			}
			// The public bounded timeout is observed while the actual producer close
			// is waiting on its occupied native journal; process absence is not join.
			deadline := time.Now().Add(500 * time.Millisecond)
			for !bytes.Contains([]byte(process.diagnostic.String()), []byte("completion-unavailable")) {
				if time.Now().After(deadline) {
					t.Fatal("original command did not report bounded unavailable completion", process.diagnostic.String())
				}
				select {
				case <-time.After(5 * time.Millisecond):
				case <-t.Context().Done():
					t.Fatal(t.Context().Err())
				}
			}
			if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(process.pid))); err != nil {
				t.Fatal("Issuer exited ahead of original producer join", err, process.diagnostic.String())
			}
			waitErr := process.command.Wait()
			process.joined = true // Actual manager/process join, not Node completion.
			_ = process.stdout.Close()
			var exit *exec.ExitError
			var event map[string]string
			decoder := json.NewDecoder(bytes.NewBufferString(process.diagnostic.String()))
			decodeErr := decoder.Decode(&event)
			if !errors.As(waitErr, &exit) || exit.ExitCode() != 1 || decodeErr != nil || event["phase"] != "retiring" || event["outcome"] != "completion-unavailable" {
				t.Fatal("bounded command wait fabricated original completion", waitErr, decodeErr, process.diagnostic.String())
			}
			var terminal map[string]string
			if err := decoder.Decode(&terminal); err != nil || terminal["phase"] != "joined" || terminal["outcome"] != "failed" || terminal["cleanup_owner"] != "participation" {
				t.Fatal("unavailable command lost retained failed producer cleanup", err, terminal, process.diagnostic.String())
			}
			if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(process.pid))); !os.IsNotExist(err) {
				t.Fatal("original process remains after manager join", err)
			}
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
			fresh, err := duty.OpenNodeProducer(context.Background(), duty.Config{Root: participation, Clock: time.Now}, duty.NodeParticipation{Identity: member.NodeID, FamilyID: member.FamilyID, Assignment: member.Assignment, EpochUntil: retained.Epoch.ValidUntil, RecordUntil: member.NotAfter()})
			if fresh != nil || !errors.Is(err, duty.ErrLocalRoleConflict) {
				if fresh != nil {
					_ = fresh.Close(context.Background())
				}
				t.Fatal("replacement producer adopted unresolved original", err)
			}
		})
	}
}
