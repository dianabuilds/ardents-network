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
func TestNativeNodeUnavailableCompletionRetainsOriginalProducerTCP(t *testing.T) {
	binary := nodeNativeCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			f, sockets, certificates := newRouteFixture(t, carrier)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := joinProcessClock(t)
			defer closeClock()
			certificate := certificates[[32]byte{12}]
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
			plan := map[string]any{"network": seedNativeNodeState(t, f, bundle, clock), "node_id": [32]byte{12}, "process_profile": processresource.Profile, "participation_root": participation, "spend_root": t.TempDir(), "hosting_root": routeProcessBudget(t), "certificate": certPath, "private_key": keyPath, "work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 1}
			sockets[[32]byte{12}]()
			process := startNativeForwarding(t, binary, hostingConfig(t, plan), 91)
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			member, err := view.Member([32]byte{12}, view.ObservedAt())
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
			waitErr := process.command.Wait()
			process.joined = true // Actual manager/process join, not Node completion.
			_ = process.stdout.Close()
			var exit *exec.ExitError
			var event map[string]string
			decodeErr := json.NewDecoder(bytes.NewBufferString(process.diagnostic.String())).Decode(&event)
			if !errors.As(waitErr, &exit) || exit.ExitCode() != 1 || decodeErr != nil || event["phase"] != "retiring" || event["outcome"] != "completion-unavailable" {
				t.Fatal("bounded command wait fabricated original completion", waitErr, decodeErr, process.diagnostic.String())
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
