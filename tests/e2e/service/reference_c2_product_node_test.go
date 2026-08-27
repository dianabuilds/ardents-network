package service_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// referenceC2ProductNode is a test observer for one product ardents-node
// process. It owns only process lifetime and JSON lifecycle observation; the
// State input and all Node behavior remain product code.
type referenceC2ProductNode struct {
	role    string
	command *exec.Cmd
	events  chan referenceC2ProductNodeEvent
	done    chan struct{}
	stderr  *bytes.Buffer
	stop    func()
	eventMu sync.Mutex
	states  []string
}

type referenceC2ProductNodeEvent struct {
	Schema, State, Assignment string
	Epoch                     uint64
}

func referenceC2StartProductNode(t *testing.T, ctx context.Context, binary, root string, fixture referenceC2StateFixture, role, stateRoot string,
	sources [2]referenceC2SourceEndpoint, client referenceC2SourceCredential,
) *referenceC2ProductNode {
	t.Helper()
	record, present := fixture.roles[role]
	if !present || stateRoot == "" {
		t.Fatalf("product Node input for %q is unavailable", role)
	}
	certificatePath := referenceC2WriteProductNodeInput(t, root, role+"-node-cert.pem", []byte(record.material.certificate))
	keyPath := referenceC2WriteProductNodeInput(t, root, role+"-node-key.pem", []byte(record.material.privateKey))
	clockPath := filepath.Join(root, role+"-node-clock.observation")
	stopClock := referenceC2StartProductNodeClock(t, clockPath)
	plan := referenceC2ProductNodePlan(t, root, fixture, role, stateRoot, certificatePath, keyPath, clockPath, sources, client)
	planPath := filepath.Join(root, role+"-product-node.json")
	raw, err := json.Marshal(plan)
	if err != nil || os.WriteFile(planPath, raw, 0o600) != nil {
		stopClock()
		t.Fatal("write product Node plan")
	}
	command := exec.CommandContext(ctx, binary, "node", "--config", planPath)
	stdout, err := command.StdoutPipe()
	if err != nil {
		stopClock()
		t.Fatal(err)
	}
	process := &referenceC2ProductNode{role: role, command: command, events: make(chan referenceC2ProductNodeEvent, 16), done: make(chan struct{}), stderr: new(bytes.Buffer), stop: stopClock}
	command.Stderr = process.stderr
	if err := command.Start(); err != nil {
		stopClock()
		t.Fatal(err)
	}
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			var event referenceC2ProductNodeEvent
			if json.Unmarshal(scanner.Bytes(), &event) == nil {
				process.record(event)
				select {
				case process.events <- event:
				default:
				}
			}
		}
		close(process.events)
	}()
	go func() {
		_ = command.Wait()
		close(process.done)
	}()
	t.Cleanup(func() { referenceC2StopProductNode(t, process) })
	return process
}

func referenceC2ProductNodePlan(t *testing.T, root string, fixture referenceC2StateFixture, role, stateRoot, certificatePath, keyPath, clockPath string,
	sources [2]referenceC2SourceEndpoint, client referenceC2SourceCredential,
) map[string]any {
	t.Helper()
	record := fixture.roles[role]
	authority := fixture.authority.Public().(ed25519.PublicKey)
	declaredSources := make([]map[string]string, len(sources))
	for index, source := range sources {
		rootPath := referenceC2WriteProductNodeInput(t, root, fmt.Sprintf("%s-source-%d-root.pem", role, index), []byte(source.root))
		identity := sha256.Sum256([]byte("reference-c2-state-source-identity-" + source.address))
		declaredSources[index] = map[string]string{"address": source.address, "server_name": source.serverName,
			"identity": hex.EncodeToString(identity[:]), "family": "reference-c2-state-source-" + string(rune('a'+index)),
			"endpoint_handle": "reference-c2-state-source-" + string(rune('a'+index)), "root_ca": rootPath,
			"leaf_key_digest": hex.EncodeToString(source.leafDigest[:])}
	}
	plan := map[string]any{"schema": "ardents-node-plan-v1", "state_root": stateRoot,
		"local_role_state_root": filepath.Join(root, role+"-product-node-role"), "network_id": referenceC2Hex(fixture.network),
		"authority_public": []string{hex.EncodeToString(authority)}, "threshold": 1, "at": fixture.now.Format(time.RFC3339),
		"listen": record.endpoint, "server_certificate": certificatePath, "server_key": keyPath,
		"client_root": declaredSources[0]["root_ca"], "client_key_digests": []string{hex.EncodeToString(client.leafDigest[:])},
		"materialization_index": record.materializationIndex, "order_seed": referenceC2Hex(fixture.digest),
		"source_client_certificate": client.certificate, "source_client_key": client.privateKey, "sources": declaredSources,
		"node_id": referenceC2Hex(record.nodeID), "identity_key": keyPath, "clock_observation_file": clockPath,
		"maximum_duty_ms": 1000, "drain_timeout_ms": 1000}
	switch role {
	case "rendezvous":
		plan["rendezvous"] = map[string]any{"handshake_limit": 2, "waiting_limit": 2, "pair_limit": 1, "pair_byte_limit": 256 << 10, "admission_timeout_ms": 3000, "drain_timeout_ms": 1000}
	case "initiator":
		plan["initiator"] = map[string]any{"handshake_limit": 2, "relay_limit": 2, "relay_byte_limit": 256 << 10, "admission_timeout_ms": 3000, "drain_timeout_ms": 1000}
	case "introduction":
		plan["introduction"] = map[string]any{"handshake_limit": 3, "slot_limit": 1, "delivery_limit": 1, "admission_timeout_ms": 3000, "drain_timeout_ms": 1000}
	case "responder":
		plan["responder"] = map[string]any{"handshake_limit": 2, "relay_limit": 1, "relay_byte_limit": 256 << 10, "admission_timeout_ms": 3000, "drain_timeout_ms": 1000}
	default:
		t.Fatalf("unsupported product Node role %q", role)
	}
	return plan
}

func referenceC2WaitForProductNodeReady(ctx context.Context, process *referenceC2ProductNode) error {
	for {
		select {
		case event, open := <-process.events:
			if !open {
				return fmt.Errorf("output closed before READY")
			}
			if event.Schema != "ardents-node-event-v1" {
				return fmt.Errorf("unexpected event schema %q", event.Schema)
			}
			if event.State == "READY" {
				if event.Assignment != process.role {
					return fmt.Errorf("READY assignment %q", event.Assignment)
				}
				return nil
			}
		case <-process.done:
			return fmt.Errorf("process exited before READY")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func referenceC2WaitForProductNodeState(ctx context.Context, process *referenceC2ProductNode, want string) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if process.hasState(want) {
			return nil
		}
		select {
		case <-process.done:
			if process.hasState(want) {
				return nil
			}
			return fmt.Errorf("process exited before %s", want)
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func referenceC2StopProductNodes(t *testing.T, processes map[string]*referenceC2ProductNode) {
	t.Helper()
	for _, process := range processes {
		referenceC2StopProductNode(t, process)
	}
}

// referenceC2HardStopProductNode models a sudden loss of the Node's whole
// local fault domain. It deliberately bypasses the product drain path; later
// cleanup may still stop the remaining independent Node processes gracefully.
func referenceC2HardStopProductNode(t *testing.T, process *referenceC2ProductNode) {
	t.Helper()
	if process == nil || process.command == nil || process.command.Process == nil {
		t.Fatal("hard-stop product Node is unavailable")
	}
	select {
	case <-process.done:
		t.Fatalf("product Node %s exited before the hard-stop fault", process.role)
	default:
	}
	if err := process.command.Process.Kill(); err != nil {
		t.Fatalf("hard-stop product Node %s: %v", process.role, err)
	}
	select {
	case <-process.done:
	case <-time.After(2 * time.Second):
		t.Fatalf("hard-stopped product Node %s did not exit", process.role)
	}
	if process.stop != nil {
		process.stop()
		process.stop = nil
	}
}

func referenceC2StopProductNode(t *testing.T, process *referenceC2ProductNode) {
	t.Helper()
	if process == nil {
		return
	}
	graceful := false
	running := true
	select {
	case <-process.done:
		running = false
	default:
	}
	if running && process.command.Process != nil {
		requested, err := referenceC2RequestProductNodeShutdown(process.command.Process)
		if err != nil {
			t.Fatalf("request product Node %s shutdown: %v", process.role, err)
		}
		graceful = requested
		if !requested {
			_ = process.command.Process.Kill()
		}
	}
	select {
	case <-process.done:
	case <-time.After(2 * time.Second):
		t.Fatalf("product Node %s did not exit after test cleanup", process.role)
	}
	if graceful {
		for range process.events {
		}
		if !process.withdrew() {
			t.Fatalf("product Node %s did not emit DRAINING then WITHDRAWN after SIGTERM", process.role)
		}
	}
	if process.stop != nil {
		process.stop()
		process.stop = nil
	}
}

func (process *referenceC2ProductNode) record(event referenceC2ProductNodeEvent) {
	process.eventMu.Lock()
	defer process.eventMu.Unlock()
	process.states = append(process.states, event.State)
}

func (process *referenceC2ProductNode) withdrew() bool {
	process.eventMu.Lock()
	defer process.eventMu.Unlock()
	draining := false
	for _, state := range process.states {
		if state == "DRAINING" {
			draining = true
		}
		if draining && state == "WITHDRAWN" {
			return true
		}
	}
	return false
}

func (process *referenceC2ProductNode) hasState(want string) bool {
	process.eventMu.Lock()
	defer process.eventMu.Unlock()
	for _, state := range process.states {
		if state == want {
			return true
		}
	}
	return false
}

func referenceC2WriteProductNodeInput(t *testing.T, root, name string, value []byte) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, value, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func referenceC2StartProductNodeClock(t *testing.T, path string) func() {
	t.Helper()
	if err := os.WriteFile(path, []byte("reference c2 product-node clock\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			now := time.Now()
			_ = os.Chtimes(path, now, now)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
}
