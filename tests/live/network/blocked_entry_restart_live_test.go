//go:build live

package network_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type blockedRestartPhase struct {
	Regime, Attempt bool
	AttemptID       [32]byte
	Deadline        int64
	Contacts        int
	Terminal        string
}

func TestBlockedEntryFinalHostileRestart(t *testing.T) {
	if os.Getenv("ARDENTS_BLOCKED_ROLE") != "" {
		t.Skip("host orchestrator only")
	}
	cell, ok := selectedHostileRestartCell(os.Getenv("ARDENTS_FINAL_CELL"))
	if !ok {
		t.Skip("selected G4 atomic-publication cell only")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("live tests require Docker: %v", err)
	}
	client := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_CLIENT", blockedClientHash)
	server := requireBlockedCandidate(t, "ARDENTS_WEBTUNNEL_SERVER", blockedServerHash)
	repository := repositoryRoot(t)
	image, ownedImage := finalProductImage(t, fmt.Sprintf("ardents-s55-g4-%d:test", time.Now().UnixNano()))
	fixture := newBlockedEntryFixture(t, client, server)
	project := finalProjectName(fmt.Sprintf("ardents-s55-g4-%d", time.Now().UnixNano()))
	compose := blockedCompose(repository, project, image, fixture, "C2")
	cleanup := blockedProjectCleanup(t, compose, project)
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if ownedImage {
		if output, err := compose(ctx, "build", "endpoint"); err != nil {
			t.Fatalf("build G4 product image: %v\n%s", err, output)
		}
	}
	started := time.Now()
	bindFinalFixtureSeed(t, fixture, cell, "restart-phase")
	startBlockedNetwork(t, ctx, compose, "C2")
	waitForBlockedJSON(t, ctx, compose, "bridge", func(line []byte) bool {
		var value struct{ Kind, State string }
		return json.Unmarshal(line, &value) == nil && value.Kind == "adapter" && value.State == "READY"
	})
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder", "publisher"} {
		waitForKind(t, ctx, compose, role, "ready")
	}
	if output, err := compose(ctx, "up", "-d", "--no-build", "--no-deps", "endpoint", "endpoint-observer", "policy"); err != nil {
		t.Fatalf("start G4 Endpoint: %v\n%s", err, output)
	}
	waitForBlockedJSON(t, ctx, compose, "policy", func(line []byte) bool {
		var value struct{ Kind, State string }
		return json.Unmarshal(line, &value) == nil && value.Kind == "policy" && value.State == "READY"
	})
	phase := waitBlockedRestartPhase(t, ctx, fixture.root, "endpoint", func(value blockedRestartPhase) bool {
		return value.Regime && value.Attempt && value.Contacts == 1 && value.Terminal == ""
	})
	if !phase.Regime || phase.Contacts != 1 {
		t.Fatalf("G4 durable phase=%+v", phase)
	}
	beforeRestart := phase
	if output, err := compose(ctx, "kill", "-s", "SIGKILL", "endpoint"); err != nil {
		t.Fatalf("kill G4 Endpoint: %v\n%s", err, output)
	}
	waitBlockedRestartExit(t, ctx, compose, "endpoint", 137)
	phase = waitBlockedRestartPhase(t, ctx, fixture.root, "endpoint", func(value blockedRestartPhase) bool {
		return value.Regime && value.Attempt && value.Contacts == 1 && value.Terminal == ""
	})
	if phase.AttemptID != beforeRestart.AttemptID || phase.Deadline != beforeRestart.Deadline {
		t.Fatalf("G4 restart reset attempt bounds: before=%+v after=%+v", beforeRestart, phase)
	}
	writeLiveFile(t, filepath.Join(fixture.root, "sync", "endpoint", "expected-terminal"), []byte("bridge-interrupted\n"))
	if output, err := compose(ctx, "up", "-d", "--no-build", "--no-deps", "endpoint", "endpoint-observer"); err != nil {
		t.Fatalf("restart G4 Endpoint: %v\n%s", err, output)
	}
	waitBlockedRestartExit(t, ctx, compose, "endpoint", 0)
	phase = waitBlockedRestartPhase(t, ctx, fixture.root, "endpoint", func(value blockedRestartPhase) bool {
		return value.Terminal == "bridge-interrupted"
	})
	if phase.Contacts != 1 || phase.Terminal != "bridge-interrupted" ||
		phase.AttemptID != beforeRestart.AttemptID || phase.Deadline != beforeRestart.Deadline {
		t.Fatalf("G4 restart state=%+v", phase)
	}
	armFinalWorkerTerminal("bridge-interrupted")
	publishFinalWorkerTerminal()
	writeLiveFile(t, filepath.Join(fixture.root, "sync", "endpoint", "blocked-condition.json"), []byte("done\n"))
	writeLiveFile(t, filepath.Join(fixture.root, "sync", "bridge", "bridge-stop"), []byte("stop\n"))
	for _, service := range restartPhaseServices() {
		waitBlockedContainer(t, ctx, compose, service)
	}
	cleanup()
	emitFinalWorkerCell(t, cell, "bridge-interrupted", started, fixture.root)
}

func selectedHostileRestartCell(cell string) (string, bool) {
	parts := strings.Split(cell, "/")
	if len(parts) != 4 || parts[0] != "hostile" {
		return "", false
	}
	episode, err := strconv.Atoi(parts[3])
	if err != nil || episode < 0 || episode >= 5 {
		return "", false
	}
	if parts[1] == "G4-restart" && (parts[2] == "after-regime-publication" || parts[2] == "after-exposure-0") {
		return cell, true
	}
	if parts[1] == "G8-lifecycle" && parts[2] == "endpoint-restart" {
		return cell, true
	}
	return "", false
}

func waitBlockedRestartPhase(t *testing.T, ctx context.Context, root, role string,
	want func(blockedRestartPhase) bool,
) blockedRestartPhase {
	t.Helper()
	for {
		value, present, err := readBlockedRestartPhase(filepath.Join(root, "state", role, "bridge"))
		if err != nil {
			t.Fatal(err)
		}
		if present && want(value) {
			return value
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait durable G4 phase: %v", ctx.Err())
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func readBlockedRestartPhase(root string) (blockedRestartPhase, bool, error) {
	pointer, err := os.ReadFile(filepath.Join(root, "current"))
	if os.IsNotExist(err) {
		return blockedRestartPhase{}, false, nil
	}
	name := strings.TrimSuffix(string(pointer), "\n")
	if err != nil || len(pointer) != 65 || len(name) != 64 {
		return blockedRestartPhase{}, false, fmt.Errorf("Bridge state pointer is invalid")
	}
	raw, err := os.ReadFile(filepath.Join(root, "state-"+name))
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != name {
		return blockedRestartPhase{}, false, fmt.Errorf("Bridge state generation is invalid")
	}
	var state struct {
		Regime, Attempt json.RawMessage
		Contacts        []json.RawMessage `json:"contacts"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return blockedRestartPhase{}, false, err
	}
	var attempt struct {
		AttemptID [32]byte `json:"attempt_id"`
		Deadline  int64    `json:"deadline_unix_nano"`
		Terminal  string   `json:"terminal"`
	}
	if len(state.Attempt) != 0 && string(state.Attempt) != "null" {
		if err := json.Unmarshal(state.Attempt, &attempt); err != nil {
			return blockedRestartPhase{}, false, err
		}
	}
	return blockedRestartPhase{Regime: len(state.Regime) != 0 && string(state.Regime) != "null",
		Attempt: len(state.Attempt) != 0 && string(state.Attempt) != "null", AttemptID: attempt.AttemptID,
		Deadline: attempt.Deadline, Contacts: len(state.Contacts), Terminal: attempt.Terminal}, true, nil
}

func waitBlockedRestartExit(t *testing.T, ctx context.Context, compose composeCall, service string, want int) {
	t.Helper()
	identity, err := compose(ctx, "ps", "--all", "-q", service)
	if err != nil || strings.TrimSpace(string(identity)) == "" {
		t.Fatalf("resolve %s: %v\n%s", service, err, identity)
	}
	output, err := dockerOutput(ctx, "wait", strings.TrimSpace(string(identity)))
	if err != nil || strings.TrimSpace(string(output)) != strconv.Itoa(want) {
		t.Fatalf("%s exit=%q want=%d: %v", service, output, want, err)
	}
}

func restartPhaseServices() []string {
	services := []string{"endpoint", "endpoint-observer", "policy", "bridge", "bridge-observer"}
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder", "publisher"} {
		services = append(services, role, role+"-observer")
	}
	return services
}

func TestReadBlockedRestartPhase(t *testing.T) {
	root := t.TempDir()
	raw := []byte(`{"regime":{},"attempt":{"attempt_id":[1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"deadline_unix_nano":42,"terminal":"bridge-interrupted"},"contacts":[{}]}`)
	name := fmt.Sprintf("%x", sha256.Sum256(raw))
	if err := os.WriteFile(filepath.Join(root, "state-"+name), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "current"), []byte(name+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	phase, present, err := readBlockedRestartPhase(root)
	if err != nil || !present || !phase.Regime || !phase.Attempt || phase.Contacts != 1 ||
		phase.AttemptID[0] != 1 || phase.Deadline != 42 || phase.Terminal != "bridge-interrupted" {
		t.Fatalf("phase=%+v present=%t err=%v", phase, present, err)
	}
	if err := os.WriteFile(filepath.Join(root, "current"), []byte(strings.Repeat("0", 64)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readBlockedRestartPhase(root); err == nil {
		t.Fatal("unbound durable state was accepted")
	}
}
