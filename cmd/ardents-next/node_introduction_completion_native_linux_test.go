//go:build linux && node_process_native

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

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
			stateConfig, err := networkStateConfig(&networkPlan)
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
