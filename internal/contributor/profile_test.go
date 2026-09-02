package contributor_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/contributor"
)

func TestPinnedRendezvousBundleInstallsAndBecomesReady(t *testing.T) {
	hostRoot := t.TempDir()
	bundle, pin := writeContributorBundle(t, 1, strings.Repeat("31", 32))
	supervisor := &profileSupervisor{hostRoot: hostRoot}
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	report, err := profile.Apply(t.Context(), bundle, pin)
	if err != nil {
		t.Fatal(err)
	}
	if report.Profile != "ardents-rendezvous-dedicated-host-v1" || report.Generation != 1 || report.LifecycleState != "READY" || !report.Active {
		t.Fatalf("install report = %+v", report)
	}
	program := filepath.Join(hostRoot, "usr", "lib", "ardents-contributor", "current", "ardents-node")
	if raw, readErr := os.ReadFile(program); readErr != nil || string(raw) != "functional-alpha-rendezvous-program-v1" {
		t.Fatalf("installed program = %q, %v", raw, readErr)
	}
	unitPath := filepath.Join(hostRoot, "etc", "systemd", "system", "ardents-rendezvous-contributor.service")
	unit, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"DynamicUser=yes", "User=ardents-contributor", "Group=ardents-contributor",
		"ExecStartPre=+/bin/chown -R -- ardents-contributor:ardents-contributor /var/lib/private/ardents-contributor",
		"CPUQuota=100%", "MemoryHigh=192M", "MemoryMax=256M", "TasksMax=64", "LimitNOFILE=256", "GOMAXPROCS=1", "GOMEMLIMIT=134217728", "StandardOutput=null", "StandardError=journal"} {
		if !strings.Contains(string(unit), required) {
			t.Fatalf("installed unit lacks %q:\n%s", required, unit)
		}
	}
}

func TestLifecycleDeadlineUsesOwnedMonotonicWait(t *testing.T) {
	hostRoot := t.TempDir()
	bundle, pin := writeContributorBundle(t, 1, strings.Repeat("39", 32))
	supervisor := &profileSupervisor{hostRoot: hostRoot, suppressStartLifecycle: true}
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC)
	waits := 0
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor,
		Now: func() time.Time { return now }, Wait: func(context.Context, time.Duration) error {
			waits++
			now = now.Add(5 * time.Second)
			return nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Apply(t.Context(), bundle, pin); err == nil || !strings.Contains(err.Error(), "did not reach READY") {
		t.Fatalf("missing READY error = %v", err)
	}
	if waits != 3 {
		t.Fatalf("monotonic waits = %d, want 3", waits)
	}
}

func TestSamePinnedBundleRecoversInterruptedFirstInstallation(t *testing.T) {
	hostRoot := t.TempDir()
	deployment := strings.Repeat("30", 32)
	bundle, pin := writeContributorBundle(t, 1, deployment)
	programRoot := filepath.Join(hostRoot, "usr", "lib", "ardents-contributor")
	if err := os.MkdirAll(programRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(programRoot, "partial"), []byte("interrupted"), 0o600); err != nil {
		t.Fatal(err)
	}
	markerPath := filepath.Join(hostRoot, "var", "lib", "private", "ardents-contributor-installing.json")
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
		t.Fatal(err)
	}
	marker, _ := json.Marshal(map[string]any{"schema": "ardents-contributor-installing-v1", "deployment_id": deployment,
		"generation": 1, "manifest_digest": pin})
	if err := os.WriteFile(markerPath, marker, 0o600); err != nil {
		t.Fatal(err)
	}
	supervisor := &profileSupervisor{hostRoot: hostRoot}
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	report, err := profile.Apply(t.Context(), bundle, pin)
	if err != nil {
		t.Fatal(err)
	}
	if report.Generation != 1 || !report.Active || report.LifecycleState != "READY" {
		t.Fatalf("recovered install report = %+v", report)
	}
	if _, err := os.Lstat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("installation marker remains: %v", err)
	}
}

func TestPinnedSuccessorUpdatesAndRestartsSameDeployment(t *testing.T) {
	hostRoot := t.TempDir()
	deployment := strings.Repeat("32", 32)
	supervisor := &profileSupervisor{hostRoot: hostRoot}
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	first, firstPin := writeContributorBundle(t, 1, deployment)
	if _, err := profile.Apply(t.Context(), first, firstPin); err != nil {
		t.Fatal(err)
	}
	second, secondPin := writeContributorBundle(t, 2, deployment)
	report, err := profile.Apply(t.Context(), second, secondPin)
	if err != nil {
		t.Fatal(err)
	}
	if report.Generation != 2 || report.LifecycleState != "READY" || !report.Active {
		t.Fatalf("successor report = %+v", report)
	}
	program := filepath.Join(hostRoot, "usr", "lib", "ardents-contributor", "current", "ardents-node")
	raw, err := os.ReadFile(program)
	if err != nil || string(raw) != "functional-alpha-rendezvous-program-v2" {
		t.Fatalf("updated program = %q, %v", raw, err)
	}
}

func TestFailedSuccessorRestoresPreviousReadyGeneration(t *testing.T) {
	hostRoot := t.TempDir()
	deployment := strings.Repeat("36", 32)
	supervisor := &profileSupervisor{hostRoot: hostRoot}
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	first, firstPin := writeContributorBundle(t, 1, deployment)
	if _, err := profile.Apply(t.Context(), first, firstPin); err != nil {
		t.Fatal(err)
	}
	supervisor.failNextStart = true
	second, secondPin := writeContributorBundle(t, 2, deployment)
	if _, err := profile.Apply(t.Context(), second, secondPin); err == nil {
		t.Fatal("failed successor was reported as installed")
	}
	report, err := profile.Control(t.Context(), contributor.Diagnose, "")
	if err != nil || report.Generation != 1 || !report.Active || report.LifecycleState != "READY" {
		t.Fatalf("rolled-back report = %+v, %v", report, err)
	}
	program := filepath.Join(hostRoot, "usr", "lib", "ardents-contributor", "current", "ardents-node")
	raw, err := os.ReadFile(program)
	if err != nil || string(raw) != "functional-alpha-rendezvous-program-v1" {
		t.Fatalf("rolled-back program = %q, %v", raw, err)
	}
}

func TestAmbiguousUpdateStopFailureRestartsPreviousReadyGeneration(t *testing.T) {
	hostRoot := t.TempDir()
	deployment := strings.Repeat("38", 32)
	supervisor := &profileSupervisor{hostRoot: hostRoot}
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	first, firstPin := writeContributorBundle(t, 1, deployment)
	if _, err := profile.Apply(t.Context(), first, firstPin); err != nil {
		t.Fatal(err)
	}
	supervisor.failNextStopAfterAction = true
	second, secondPin := writeContributorBundle(t, 2, deployment)
	if _, err := profile.Apply(t.Context(), second, secondPin); err == nil {
		t.Fatal("ambiguous stop failure was reported as an installed successor")
	}
	report, err := profile.Control(t.Context(), contributor.Diagnose, "")
	if err != nil || report.Generation != 1 || !report.Active || report.LifecycleState != "READY" {
		t.Fatalf("restarted previous report = %+v, %v", report, err)
	}
}

func TestNextCommandRecoversUpdateInterruptedAfterPreviousGenerationWasMoved(t *testing.T) {
	hostRoot := t.TempDir()
	deployment := strings.Repeat("37", 32)
	supervisor := &profileSupervisor{hostRoot: hostRoot}
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	first, firstPin := writeContributorBundle(t, 1, deployment)
	if _, err := profile.Apply(t.Context(), first, firstPin); err != nil {
		t.Fatal(err)
	}
	programRoot := filepath.Join(hostRoot, "usr", "lib", "ardents-contributor")
	configRoot := filepath.Join(hostRoot, "var", "lib", "private", "ardents-contributor", "config")
	if err := os.Rename(filepath.Join(programRoot, "current"), filepath.Join(programRoot, "previous")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(configRoot, "current"), filepath.Join(configRoot, "previous")); err != nil {
		t.Fatal(err)
	}
	report, err := profile.Control(t.Context(), contributor.Diagnose, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Generation != 1 || !report.Active || report.LifecycleState != "READY" {
		t.Fatalf("recovered report = %+v", report)
	}
	for _, path := range []string{filepath.Join(programRoot, "previous"), filepath.Join(configRoot, "previous")} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("recovery residue %s remains: %v", path, err)
		}
	}
}

func TestDiagnoseAndRestartReturnVerifiedReadyInstallation(t *testing.T) {
	hostRoot := t.TempDir()
	bundle, pin := writeContributorBundle(t, 1, strings.Repeat("33", 32))
	supervisor := &profileSupervisor{hostRoot: hostRoot}
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Apply(t.Context(), bundle, pin); err != nil {
		t.Fatal(err)
	}
	diagnosed, err := profile.Control(t.Context(), contributor.Diagnose, "")
	if err != nil || !diagnosed.Active || diagnosed.LifecycleState != "READY" {
		t.Fatalf("diagnose = %+v, %v", diagnosed, err)
	}
	restarted, err := profile.Control(t.Context(), contributor.Restart, "")
	if err != nil || !restarted.Active || restarted.LifecycleState != "READY" || restarted.Generation != 1 {
		t.Fatalf("restart = %+v, %v", restarted, err)
	}
}

func TestDrainStopsWorkAndWithdrawalAlsoDisablesService(t *testing.T) {
	hostRoot := t.TempDir()
	bundle, pin := writeContributorBundle(t, 1, strings.Repeat("34", 32))
	supervisor := &profileSupervisor{hostRoot: hostRoot}
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Apply(t.Context(), bundle, pin); err != nil {
		t.Fatal(err)
	}
	drained, err := profile.Control(t.Context(), contributor.Drain, "")
	if err != nil || drained.Active || !drained.Enabled || drained.LifecycleState != "WITHDRAWN" {
		t.Fatalf("drain = %+v, %v", drained, err)
	}
	if _, err := profile.Control(t.Context(), contributor.Restart, ""); err != nil {
		t.Fatal(err)
	}
	withdrawn, err := profile.Control(t.Context(), contributor.Withdraw, "")
	if err != nil || withdrawn.Active || withdrawn.Enabled || withdrawn.LifecycleState != "WITHDRAWN" {
		t.Fatalf("withdraw = %+v, %v", withdrawn, err)
	}
}

func TestRemovalRequiresExactWithdrawnDeploymentAndLeavesBundle(t *testing.T) {
	hostRoot := t.TempDir()
	deployment := strings.Repeat("35", 32)
	bundle, pin := writeContributorBundle(t, 1, deployment)
	supervisor := &profileSupervisor{hostRoot: hostRoot}
	profile, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Apply(t.Context(), bundle, pin); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Control(t.Context(), contributor.Remove, strings.Repeat("00", 32)); err == nil {
		t.Fatal("removal accepted the wrong deployment confirmation")
	}
	program := filepath.Join(hostRoot, "usr", "lib", "ardents-contributor", "current", "ardents-node")
	if _, err := os.Stat(program); err != nil {
		t.Fatalf("wrong confirmation changed installation: %v", err)
	}
	if _, err := profile.Control(t.Context(), contributor.Withdraw, ""); err != nil {
		t.Fatal(err)
	}
	removed, err := profile.Control(t.Context(), contributor.Remove, deployment)
	if err != nil {
		t.Fatal(err)
	}
	if removed.LifecycleState != "REMOVED" || removed.Active || removed.Enabled || removed.DeploymentID != deployment {
		t.Fatalf("removal report = %+v", removed)
	}
	for _, path := range []string{
		filepath.Join(hostRoot, "usr", "lib", "ardents-contributor"),
		filepath.Join(hostRoot, "var", "lib", "private", "ardents-contributor"),
		filepath.Join(hostRoot, "etc", "systemd", "system", "ardents-rendezvous-contributor.service"),
	} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("removed path %s remains: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(bundle, "manifest.json")); err != nil {
		t.Fatalf("removal changed source bundle: %v", err)
	}
}

type profileSupervisor struct {
	hostRoot                string
	mu                      sync.Mutex
	active                  bool
	enabled                 bool
	failNextStart           bool
	failNextStopAfterAction bool
	suppressStartLifecycle  bool
	stopEntered             chan struct{}
	releaseStop             chan struct{}
	stopPaused              bool
}

func (supervisor *profileSupervisor) Do(ctx context.Context, action contributor.SupervisorAction) (contributor.SupervisorState, error) {
	if err := supervisor.pauseFirstStop(ctx, action); err != nil {
		return contributor.SupervisorState{}, err
	}
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	switch action {
	case contributor.SupervisorReload:
	case contributor.SupervisorEnable:
		supervisor.enabled = true
	case contributor.SupervisorStart, contributor.SupervisorRestart:
		if supervisor.failNextStart {
			supervisor.failNextStart = false
			return contributor.SupervisorState{}, errors.New("injected successor start failure")
		}
		supervisor.active = true
		if !supervisor.suppressStartLifecycle {
			writeLifecycle(tWriter{root: supervisor.hostRoot}, "READY")
		}
	case contributor.SupervisorStop:
		supervisor.active = false
		writeLifecycle(tWriter{root: supervisor.hostRoot}, "WITHDRAWN")
		if supervisor.failNextStopAfterAction {
			supervisor.failNextStopAfterAction = false
			return contributor.SupervisorState{}, errors.New("injected ambiguous stop failure")
		}
	case contributor.SupervisorDisable:
		supervisor.enabled = false
	}
	return contributor.SupervisorState{Active: supervisor.active, Enabled: supervisor.enabled}, nil
}

func (supervisor *profileSupervisor) pauseFirstStop(ctx context.Context, action contributor.SupervisorAction) error {
	if action != contributor.SupervisorStop {
		return nil
	}
	supervisor.mu.Lock()
	if supervisor.stopEntered == nil || supervisor.releaseStop == nil || supervisor.stopPaused {
		supervisor.mu.Unlock()
		return nil
	}
	supervisor.stopPaused = true
	entered, release := supervisor.stopEntered, supervisor.releaseStop
	supervisor.mu.Unlock()
	close(entered)
	select {
	case <-release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type tWriter struct{ root string }

func writeLifecycle(target tWriter, state string) {
	path := filepath.Join(target.root, "var", "lib", "private", "ardents-contributor", "diagnostics", "lifecycle.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	raw, _ := json.Marshal(map[string]any{
		"schema": "ardents-node-event-v1", "kind": "lifecycle", "state": state,
		"at": "2026-08-29T10:00:00Z", "epoch": 7, "generation": "alpha",
		"assignment": "rendezvous", "carrier_profile": "route-v1",
		"assignment_digest": [32]byte{1},
	})
	_ = os.WriteFile(path, append(raw, '\n'), 0o600)
}

func writeContributorBundle(t *testing.T, generation uint64, deployment string) (string, string) {
	t.Helper()
	return writeContributorBundleProfiles(t, generation, deployment,
		"ardents-rendezvous-dedicated-host-v1", "ardents-rendezvous-dedicated-host-v1")
}

func writeContributorBundleProfiles(t *testing.T, generation uint64, deployment, manifestProfile, planProfile string) (string, string) {
	t.Helper()
	bundle := t.TempDir()
	files := map[string][]byte{
		"ardents-node":            []byte(fmt.Sprintf("functional-alpha-rendezvous-program-v%d", generation)),
		"rendezvous-cert.pem":     []byte("rendezvous certificate\n"),
		"rendezvous-key.pem":      []byte("rendezvous key\n"),
		"rendezvous-identity.pem": []byte("rendezvous identity\n"),
		"source-client-cert.pem":  []byte("source client certificate\n"),
		"source-client-key.pem":   []byte("source client key\n"),
		"source-a-root.pem":       []byte("source A root\n"),
		"source-b-root.pem":       []byte("source B root\n"),
		"clock.observation":       []byte("authenticated clock observation\n"),
	}
	plan := map[string]any{
		"schema": "ardents-node-plan-v1", "state_root": "/var/lib/private/ardents-contributor/network",
		"local_role_state_root": "/var/lib/private/ardents-contributor/role", "network_id": strings.Repeat("11", 32),
		"authority_public": []string{strings.Repeat("12", 32)}, "threshold": 1,
		"server_certificate": "/var/lib/private/ardents-contributor/config/current/rendezvous-cert.pem",
		"server_key":         "/var/lib/private/ardents-contributor/config/current/rendezvous-key.pem", "materialization_index": 0,
		"clock_observation_file": "/var/lib/private/ardents-contributor/config/current/clock.observation", "order_seed": strings.Repeat("13", 32),
		"source_client_certificate": "/var/lib/private/ardents-contributor/config/current/source-client-cert.pem",
		"source_client_key":         "/var/lib/private/ardents-contributor/config/current/source-client-key.pem",
		"sources": []map[string]any{
			{"address": "192.0.2.10:48010", "server_name": "source-a.test", "identity": strings.Repeat("14", 32), "family": "source-a", "endpoint_handle": "source-a", "root_ca": "/var/lib/private/ardents-contributor/config/current/source-a-root.pem", "leaf_key_digest": strings.Repeat("15", 32)},
			{"address": "192.0.2.11:48011", "server_name": "source-b.test", "identity": strings.Repeat("16", 32), "family": "source-b", "endpoint_handle": "source-b", "root_ca": "/var/lib/private/ardents-contributor/config/current/source-b-root.pem", "leaf_key_digest": strings.Repeat("17", 32)},
		},
		"node_id": strings.Repeat("18", 32), "identity_key": "/var/lib/private/ardents-contributor/config/current/rendezvous-identity.pem",
		"node_resource_profile": planProfile, "diagnostic_directory": "/var/lib/private/ardents-contributor/diagnostics",
		"rendezvous": map[string]any{"handshake_limit": 4, "waiting_limit": 2, "pair_limit": 1, "pair_byte_limit": 64 << 20, "admission_timeout_ms": 5000, "drain_timeout_ms": 5000},
	}
	files["node.json"], _ = json.Marshal(plan)
	digests := make(map[string]string, len(files))
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(bundle, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(raw)
		digests[name] = hex.EncodeToString(digest[:])
	}
	manifest := map[string]any{"schema": "ardents-contributor-bundle-v1", "profile": manifestProfile, "deployment_id": deployment, "generation": generation, "files": digests}
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(bundle, "manifest.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	pinned := sha256.Sum256(raw)
	return bundle, hex.EncodeToString(pinned[:])
}
