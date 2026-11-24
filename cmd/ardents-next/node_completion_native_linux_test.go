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

	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
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
			var first struct{ Kind, Phase, Cause string }
			if err := json.Unmarshal([]byte(event["first_failure"]), &first); err != nil || first.Kind != "logical" || first.Phase != "serving" || first.Cause != "canceled" {
				t.Fatal("bounded unknown completion lost known original caller cause", err, process.diagnostic.String())
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

// Occupying the actual participation journal delays original producer cleanup.
// The real command retains State/process until join and never converts its
// bounded unavailable report into successful withdrawal after failed cleanup.
func TestNativeDescriptorUnavailableCompletionRetainsOriginalProducerTCP(t *testing.T) {
	binary := nodeNativeCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			f, sockets, certificates := descriptorRouteFixture(t, carrier)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := joinProcessClock(t)
			defer closeClock()
			certificate, key := nativeDescriptorCredential(t, certificates[[32]byte{17}])
			participation := filepath.Join(t.TempDir(), "participation")
			networkPlan := seedNativeNodeState(t, f, bundle, clock)
			plan := map[string]any{"network": networkPlan, "node_id": [32]byte{17}, "process_profile": "ardents-descriptor-process-v1", "participation_root": participation, "spend_root": t.TempDir(), "hosting_root": routeProcessBudget(t), "descriptor_root": t.TempDir(), "certificate": certificate, "private_key": key, "work": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 1}
			sockets[[32]byte{17}]()
			process := startNativeDescriptor(t, binary, hostingConfig(t, plan), 91)
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			member, err := view.Member([32]byte{17}, view.ObservedAt())
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
			deadline := time.Now().Add(500 * time.Millisecond)
			for !bytes.Contains([]byte(process.diagnostic.String()), []byte("completion-unavailable")) {
				if time.Now().After(deadline) {
					t.Fatal("bounded unavailable report absent", process.diagnostic.String())
				}
				time.Sleep(5 * time.Millisecond)
			}
			if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(process.pid))); err != nil {
				t.Fatal("command exited before original producer join", err)
			}
			stateConfig, err := networkPlan.Config()
			if err != nil {
				t.Fatal(err)
			}
			stateConfig.LocalRoleStateRoot = participation
			if duplicate, err := state.Open(stateConfig); err == nil {
				_ = duplicate.Close()
				t.Fatal("command returned original State before physical join")
			}
			waitErr := process.command.Wait()
			process.joined = true
			_ = process.stdout.Close()
			var exit *exec.ExitError
			decoder := json.NewDecoder(bytes.NewBufferString(process.diagnostic.String()))
			var unavailable, terminal map[string]string
			if !errors.As(waitErr, &exit) || exit.ExitCode() != 1 {
				t.Fatal("bounded failure replaced by success", waitErr, process.diagnostic.String())
			}
			if err := decoder.Decode(&unavailable); err != nil || unavailable["operation"] != "node.descriptor" || unavailable["phase"] != "retiring" || unavailable["outcome"] != "completion-unavailable" {
				t.Fatal("unavailable projection changed", unavailable, err)
			}
			if err := decoder.Decode(&terminal); err != nil || terminal["phase"] != "joined" || terminal["outcome"] != "failed" || terminal["cleanup_owner"] != "participation" {
				t.Fatal("failed producer cleanup reported withdrawal", terminal, err, process.diagnostic.String())
			}
			if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(process.pid))); !os.IsNotExist(err) {
				t.Fatal("original process remains after manager join", err)
			}
			conflict, err := journal.Conflict(member.NodeID, member.FamilyID)
			if err != nil || !conflict {
				t.Fatal("failed cleanup erased original producer", conflict, err)
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
				t.Fatal("replacement adopted unresolved original", err)
			}
		})
	}
}

// Occupying the actual participation journal delays original producer cleanup.
// The real command retains State/process until join and never converts its
// bounded unavailable report into successful withdrawal after failed cleanup.
func TestNativeIntroductionUnavailableCompletionRetainsOriginalProducerTCP(t *testing.T) {
	binary := nodeNativeCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			f, sockets, certificates := publicationRouteNetwork(t, carrier)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := joinProcessClock(t)
			defer closeClock()
			certificate, key := nativeDescriptorCredential(t, certificates[[32]byte{25}])
			participation := filepath.Join(t.TempDir(), "participation")
			networkPlan := seedNativeNodeState(t, f, bundle, clock)
			plan := map[string]any{"network": networkPlan, "node_id": [32]byte{25}, "process_profile": "ardents-introduction-process-v1", "participation_root": participation, "spend_root": t.TempDir(), "hosting_root": routeProcessBudget(t), "introduction_root": t.TempDir(), "certificate": certificate, "private_key": key, "work": hosting.Traffic{Tx: 1 << 20, Rx: 1 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 1}
			if err := os.Chmod(plan["introduction_root"].(string), 0700); err != nil {
				t.Fatal(err)
			}
			sockets[[32]byte{25}]()
			process := startNativeIntroduction(t, binary, hostingConfig(t, plan), 91)
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			member, err := view.Member([32]byte{25}, view.ObservedAt())
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
			deadline := time.Now().Add(500 * time.Millisecond)
			for !bytes.Contains([]byte(process.diagnostic.String()), []byte("completion-unavailable")) {
				if time.Now().After(deadline) {
					t.Fatal("bounded unavailable report absent", process.diagnostic.String())
				}
				time.Sleep(5 * time.Millisecond)
			}
			if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(process.pid))); err != nil {
				t.Fatal("command exited before original producer join", err)
			}
			stateConfig, err := networkPlan.Config()
			if err != nil {
				t.Fatal(err)
			}
			stateConfig.LocalRoleStateRoot = participation
			if duplicate, err := state.Open(stateConfig); err == nil {
				_ = duplicate.Close()
				t.Fatal("command returned original State before physical join")
			}
			waitErr := process.command.Wait()
			process.joined = true
			_ = process.stdout.Close()
			var exit *exec.ExitError
			decoder := json.NewDecoder(bytes.NewBufferString(process.diagnostic.String()))
			var unavailable, terminal map[string]string
			if !errors.As(waitErr, &exit) || exit.ExitCode() != 1 {
				t.Fatal("bounded failure replaced by success", waitErr, process.diagnostic.String())
			}
			if err := decoder.Decode(&unavailable); err != nil || unavailable["operation"] != "node.introduction" || unavailable["phase"] != "retiring" || unavailable["outcome"] != "completion-unavailable" {
				t.Fatal("unavailable projection changed", unavailable, err)
			}
			if err := decoder.Decode(&terminal); err != nil || terminal["phase"] != "joined" || terminal["outcome"] != "failed" || terminal["cleanup_owner"] != "participation" {
				t.Fatal("failed producer cleanup reported withdrawal", terminal, err, process.diagnostic.String())
			}
			if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(process.pid))); !os.IsNotExist(err) {
				t.Fatal("original process remains after manager join", err)
			}
			conflict, err := journal.Conflict(member.NodeID, member.FamilyID)
			if err != nil || !conflict {
				t.Fatal("failed cleanup erased original producer", conflict, err)
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
				t.Fatal("replacement adopted unresolved original", err)
			}
		})
	}
}

// The actual original producer's cleanup encounters a genuinely occupied
// journal lease. The command's one-millisecond wait must remain unavailable,
// even after the manager proves process termination. Neither process absence
// nor a new caller may erase or adopt the unresolved producer.
func TestNativeIssuerUnavailableCompletionRetainsOriginalProducerTCP(t *testing.T) {
	binary := nodeNativeCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
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

// Occupying the actual participation journal delays original producer cleanup.
// The real command retains State/process until join and never converts its
// bounded unavailable report into successful withdrawal after failed cleanup.
func TestNativeJoinUnavailableCompletionRetainsOriginalProducerTCP(t *testing.T) {
	binary := nodeNativeCommand(t)
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			f, sockets, certificates := newJoinRouteFixture(t, carrier)
			bundle, err := networkfixture.BuildClosed(f.spec)
			if err != nil {
				t.Fatal(err)
			}
			clock, closeClock := joinProcessClock(t)
			defer closeClock()
			certificate, key := nativeDescriptorCredential(t, certificates[[32]byte{20}])
			participation := filepath.Join(t.TempDir(), "participation")
			networkPlan := seedNativeNodeState(t, f, bundle, clock)
			plan := map[string]any{"network": networkPlan, "node_id": [32]byte{20}, "process_profile": "ardents-join-process-v1", "participation_root": participation, "spend_root": t.TempDir(), "hosting_root": routeProcessBudget(t), "certificate": certificate, "private_key": key, "work": hosting.Traffic{Tx: 32 << 20, Rx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 1}
			sockets[[32]byte{20}]()
			process := startNativeJoin(t, binary, hostingConfig(t, plan), 91)
			view, err := f.current()
			if err != nil {
				t.Fatal(err)
			}
			member, err := view.Member([32]byte{20}, view.ObservedAt())
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
			deadline := time.Now().Add(500 * time.Millisecond)
			for !bytes.Contains([]byte(process.diagnostic.String()), []byte("completion-unavailable")) {
				if time.Now().After(deadline) {
					t.Fatal("bounded unavailable report absent", process.diagnostic.String())
				}
				time.Sleep(5 * time.Millisecond)
			}
			if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(process.pid))); err != nil {
				t.Fatal("command exited before original producer join", err)
			}
			stateConfig, err := networkPlan.Config()
			if err != nil {
				t.Fatal(err)
			}
			stateConfig.LocalRoleStateRoot = participation
			if duplicate, err := state.Open(stateConfig); err == nil {
				_ = duplicate.Close()
				t.Fatal("command returned original State before physical join")
			}
			waitErr := process.command.Wait()
			process.joined = true
			_ = process.stdout.Close()
			var exit *exec.ExitError
			decoder := json.NewDecoder(bytes.NewBufferString(process.diagnostic.String()))
			var unavailable, terminal map[string]string
			if !errors.As(waitErr, &exit) || exit.ExitCode() != 1 {
				t.Fatal("bounded failure replaced by success", waitErr, process.diagnostic.String())
			}
			if err := decoder.Decode(&unavailable); err != nil || unavailable["operation"] != "node.join" || unavailable["phase"] != "retiring" || unavailable["outcome"] != "completion-unavailable" {
				t.Fatal("unavailable projection changed", unavailable, err)
			}
			if err := decoder.Decode(&terminal); err != nil || terminal["phase"] != "joined" || terminal["outcome"] != "failed" || terminal["cleanup_owner"] != "participation" {
				t.Fatal("failed producer cleanup reported withdrawal", terminal, err, process.diagnostic.String())
			}
			if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(process.pid))); !os.IsNotExist(err) {
				t.Fatal("original process remains after manager join", err)
			}
			conflict, err := journal.Conflict(member.NodeID, member.FamilyID)
			if err != nil || !conflict {
				t.Fatal("failed cleanup erased original producer", conflict, err)
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
				t.Fatal("replacement adopted unresolved original", err)
			}
		})
	}
}
