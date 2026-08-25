package state_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/tests/epochfixture/assignment"
)

type processCert struct {
	certificate string
	key         string
	root        string
	pin         [32]byte
	sourcePin   [32]byte
	private     ed25519.PrivateKey
}

type nodeProcess struct {
	command *exec.Cmd
	events  chan nodeEvent
	done    chan struct{}
	stderr  *bytes.Buffer
	waitMu  sync.Mutex
	waitErr error
}

type nodeEvent struct {
	Schema           string   `json:"schema"`
	State            string   `json:"state"`
	Epoch            uint64   `json:"epoch"`
	Assignment       string   `json:"assignment"`
	AssignmentDigest [32]byte `json:"assignment_digest"`
}

func TestTwoNodeProcessesRefreshWithdrawRestartAndReassign(t *testing.T) {
	roleAddresses := [2]string{freeAddress(t), freeAddress(t)}
	fixture := newLifecycleStateFixture(t, roleAddresses)
	ardents := buildCommand(t, "ardents")
	nodeBinary := buildCommand(t, "ardents-node")
	nodeRoots := [2]string{t.TempDir(), t.TempDir()}
	sourceRoots := [2]string{t.TempDir(), t.TempDir()}
	for index := range 2 {
		acceptEpoch(t, ardents, nodeRoots[index], fixture, fixture.genesis)
		acceptEpoch(t, ardents, sourceRoots[index], fixture, fixture.genesis)
		acceptEpoch(t, ardents, sourceRoots[index], fixture, fixture.successor)
	}
	sourceClientCA := makeAuthority(t, "source-client-root")
	var sourceClients [3]processCert
	for index := range sourceClients {
		sourceClients[index] = makeLeaf(t, sourceClientCA, fmt.Sprintf("source-client-%d.test", index), false)
	}
	clientPins := make([][32]byte, len(sourceClients))
	for index := range sourceClients {
		clientPins[index] = sourceClients[index].sourcePin
	}
	sourceAddresses := [2]string{freeAddress(t), freeAddress(t)}
	var sourceServers [2]processCert
	var stopSources [2]func()
	for index := range 2 {
		authority := makeAuthority(t, fmt.Sprintf("source-%d-root", index))
		sourceServers[index] = makeLeaf(t, authority, fmt.Sprintf("source-%d.test", index), true)
		plan := writeJSON(t, fmt.Sprintf("source-%d.json", index), sourceServerPlan(fixture, sourceRoots[index], sourceAddresses[index], sourceServers[index], sourceClientCA.root, clientPins))
		stopSources[index] = startSource(t, nodeBinary, plan)
		defer stopSources[index]()
	}
	diagnosticRoot := t.TempDir()
	acceptEpoch(t, ardents, diagnosticRoot, fixture, fixture.genesis)
	diagnosticPlan := writeJSON(t, "diagnostic-source-plan.json", sourcePlan(fixture, sourceAddresses, sourceServers, sourceClients[0]))
	if output, err := exec.Command(ardents, "refresh-sources", "--state-root", diagnosticRoot, "--source-plan", diagnosticPlan).CombinedOutput(); err != nil {
		t.Fatalf("sources cannot refresh a real endpoint: %v\n%s", err, output)
	}
	harnessCA := makeAuthority(t, "harness-root")
	harness := makeLeaf(t, harnessCA, "harness.test", false)
	var roleServers [2]processCert
	var plans [2]string
	var stopObservers [2]func()
	for index := range 2 {
		roleCA := makeAuthority(t, fmt.Sprintf("role-%d-root", index))
		roleServers[index] = makeLeaf(t, roleCA, fmt.Sprintf("node-%d.test", index), true)
		identityPath := writePrivateKey(t, fmt.Sprintf("identity-%d.pem", index), fixture.records[index].private)
		observationPath := filepath.Join(t.TempDir(), fmt.Sprintf("clock-%d.observation", index))
		stopObservers[index] = startClockObserver(t, observationPath)
		defer stopObservers[index]()
		plans[index] = writeJSON(t, fmt.Sprintf("node-%d.json", index), nodePlan(fixture, nodeRoots[index], uint32(index), identityPath,
			fixture.records[index].endpoint, roleServers[index], harnessCA.root, harness.pin, observationPath, sourceAddresses, sourceServers, sourceClients[index+1]))
	}
	var first [2]*nodeProcess
	var firstReady [2]nodeEvent
	for index := range 2 {
		first[index] = startNode(t, nodeBinary, plans[index])
		process := first[index]
		t.Cleanup(func() { stopProcess(process) })
		firstReady[index] = waitNodeState(t, first[index], "READY", 5*time.Second)
		assertAssignment(t, fixture, fixture.genesis, index, firstReady[index])
		probeNode(t, fixture.records[index].endpoint, fmt.Sprintf("node-%d.test", index), roleServers[index], harness, fixture, fixture.genesis, index, firstReady[index])
	}
	if first[0].command.Process.Pid == first[1].command.Process.Pid {
		t.Fatal("N1 and N2 share one process")
	}
	var established [2]*tls.Conn
	for index := range 2 {
		established[index] = dialNode(t, fixture.records[index].endpoint, fmt.Sprintf("node-%d.test", index), roleServers[index], harness)
		contender := startNode(t, nodeBinary, plans[index])
		t.Cleanup(func() { stopProcess(contender) })
		err, exited := waitProcess(contender, 2*time.Second)
		if !exited {
			t.Fatalf("N%d overlapping replacement remained alive while old duty was READY", index+1)
		}
		if err == nil {
			t.Fatalf("N%d allowed overlapping replacement while old duty was READY", index+1)
		}
	}
	for index := range 2 {
		waitNodeState(t, first[index], "DRAINING", 12*time.Second)
		completeNodeProbe(t, established[index], fixture, fixture.genesis, index, firstReady[index])
		_ = established[index].Close()
		err, exited := waitProcess(first[index], 3*time.Second)
		if !exited || err != nil {
			t.Fatalf("old N%d did not withdraw cleanly: %v stderr=%s", index+1, err, first[index].stderr)
		}
	}
	changed := false
	for index := range 2 {
		restarted := startNode(t, nodeBinary, plans[index])
		t.Cleanup(func() { stopProcess(restarted) })
		ready := waitNodeState(t, restarted, "READY", 5*time.Second)
		assertAssignment(t, fixture, fixture.successor, index, ready)
		if restarted.command.Process.Pid == first[index].command.Process.Pid {
			t.Fatalf("N%d restart reused the old PID", index+1)
		}
		changed = changed || ready.Assignment != firstReady[index].Assignment
		probeNode(t, fixture.records[index].endpoint, fmt.Sprintf("node-%d.test", index), roleServers[index], harness, fixture, fixture.successor, index, ready)
	}
	if !changed {
		t.Fatal("successor did not reassign either Node")
	}
}

func buildCommand(t *testing.T, name string) string {
	t.Helper()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	path := filepath.Join(t.TempDir(), name+suffix)
	command := exec.Command("go", "build", "-o", path, "./cmd/"+name)
	command.Dir = filepath.Join("..", "..", "..")
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, output)
	}
	return path
}

func acceptEpoch(t *testing.T, binary, root string, fixture lifecycleStateFixture, epoch lifecycleEpoch) {
	t.Helper()
	directory := t.TempDir()
	epochPath := filepath.Join(directory, "epoch.bin")
	inputs := filepath.Join(directory, "inputs")
	material := filepath.Join(directory, "material.bin")
	if err := os.Mkdir(inputs, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(epochPath, epoch.raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(material, epoch.materials[0], 0o600); err != nil {
		t.Fatal(err)
	}
	for index, raw := range epoch.inputs {
		if err := os.WriteFile(filepath.Join(inputs, fmt.Sprintf("%04d.bin", index)), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	arguments := []string{"accept-offline", "--state-root", root, "--network-id", hex.EncodeToString(fixture.network[:]),
		"--authorities", hex.EncodeToString(fixture.authorityPublic), "--threshold", "1", "--at", time.Unix(fixture.now, 0).UTC().Format(time.RFC3339),
		"--epoch", epochPath, "--inputs", inputs, "--materialization", material}
	if output, err := exec.Command(binary, arguments...).CombinedOutput(); err != nil {
		t.Fatalf("accept Epoch: %v\n%s", err, output)
	}
}

func sourceServerPlan(fixture lifecycleStateFixture, root, address string, server processCert, clientRoot string, clientPins [][32]byte) map[string]any {
	encodedPins := make([]string, len(clientPins))
	for index := range clientPins {
		encodedPins[index] = hex.EncodeToString(clientPins[index][:])
	}
	return map[string]any{"schema": "ardents-source-server-v1", "state_root": root,
		"local_role_state_root": root + "-local-roles", "network_id": hex.EncodeToString(fixture.network[:]),
		"authority_public": []string{hex.EncodeToString(fixture.authorityPublic)}, "threshold": 1,
		"at": time.Unix(fixture.now, 0).UTC().Format(time.RFC3339), "listen": address, "server_certificate": server.certificate,
		"server_key": server.key, "client_root": clientRoot, "client_key_digests": encodedPins, "materialization_index": 0}
}

func nodePlan(fixture lifecycleStateFixture, root string, index uint32, identity, listen string, server processCert, clientRoot string, clientPin [32]byte,
	observation string, sourceAddresses [2]string, sourceServers [2]processCert, sourceClient processCert) map[string]any {
	sources := make([]map[string]any, 2)
	for source := range 2 {
		identityDigest := sha256.Sum256([]byte(fmt.Sprintf("source-%d-identity", source)))
		sources[source] = map[string]any{"address": sourceAddresses[source], "server_name": fmt.Sprintf("source-%d.test", source),
			"identity": hex.EncodeToString(identityDigest[:]), "family": fmt.Sprintf("source-%d-family", source),
			"endpoint_handle": fmt.Sprintf("source-%d-handle", source), "root_ca": sourceServers[source].root,
			"leaf_key_digest": hex.EncodeToString(sourceServers[source].sourcePin[:])}
	}
	return map[string]any{"schema": "ardents-node-plan-v1", "state_root": root,
		"local_role_state_root": root + "-local-roles", "network_id": hex.EncodeToString(fixture.network[:]),
		"authority_public": []string{hex.EncodeToString(fixture.authorityPublic)}, "threshold": 1, "at": time.Unix(fixture.now, 0).UTC().Format(time.RFC3339),
		"listen": listen, "server_certificate": server.certificate, "server_key": server.key, "client_root": clientRoot,
		"client_key_digests": []string{hex.EncodeToString(clientPin[:])}, "materialization_index": index,
		"clock_observation_file": observation, "order_seed": strings.Repeat("44", 32),
		"source_client_certificate": sourceClient.certificate, "source_client_key": sourceClient.key,
		"sources": sources, "node_id": hex.EncodeToString(fixture.records[index].nodeID[:]), "identity_key": identity,
		"maximum_duty_ms": 6000, "drain_timeout_ms": 6000}
}

func sourcePlan(fixture lifecycleStateFixture, addresses [2]string, servers [2]processCert, client processCert) map[string]any {
	sources := make([]map[string]any, 2)
	for index := range 2 {
		identity := sha256.Sum256([]byte(fmt.Sprintf("source-%d-identity", index)))
		sources[index] = map[string]any{"address": addresses[index], "server_name": fmt.Sprintf("source-%d.test", index),
			"identity": hex.EncodeToString(identity[:]), "family": fmt.Sprintf("source-%d-family", index),
			"endpoint_handle": fmt.Sprintf("source-%d-handle", index), "root_ca": servers[index].root,
			"leaf_key_digest": hex.EncodeToString(servers[index].sourcePin[:])}
	}
	return map[string]any{"schema": "ardents-source-plan-v1", "local_role_state_root": filepath.Join(filepath.Dir(client.certificate), "local-roles"),
		"network_id":       hex.EncodeToString(fixture.network[:]),
		"authority_public": []string{hex.EncodeToString(fixture.authorityPublic)}, "threshold": 1,
		"clock_observed_at": time.Now().UTC().Format(time.RFC3339), "order_seed": strings.Repeat("44", 32),
		"materialization_index": 0, "client_certificate": client.certificate, "client_key": client.key, "sources": sources}
}

func startSource(t *testing.T, binary, plan string) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	command := exec.CommandContext(ctx, binary, "source", "--config", plan)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	ready := make(chan bool, 1)
	go func() {
		ready <- scanner.Scan() && strings.Contains(scanner.Text(), `"schema":"ardents-source-event-v1"`) &&
			strings.Contains(scanner.Text(), `"kind":"source-ready"`)
	}()
	select {
	case ok := <-ready:
		if !ok {
			cancel()
			_ = command.Wait()
			t.Fatalf("source not ready: %s", stderr.String())
		}
	case <-time.After(5 * time.Second):
		cancel()
		_ = command.Wait()
		t.Fatal("source readiness timed out")
	}
	return func() { cancel(); _ = command.Wait() }
}

func startNode(t *testing.T, binary, plan string) *nodeProcess {
	t.Helper()
	command := exec.Command(binary, "node", "--config", plan)
	command.Env = append(os.Environ(), "GOMAXPROCS=1", "GOMEMLIMIT=320MiB")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	process := &nodeProcess{command: command, events: make(chan nodeEvent, 32), done: make(chan struct{}), stderr: new(bytes.Buffer)}
	command.Stderr = process.stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			var event nodeEvent
			if json.Unmarshal(scanner.Bytes(), &event) == nil {
				process.events <- event
			}
		}
		close(process.events)
	}()
	go func() {
		err := command.Wait()
		process.waitMu.Lock()
		process.waitErr = err
		process.waitMu.Unlock()
		close(process.done)
	}()
	return process
}

func waitNodeState(t *testing.T, process *nodeProcess, state string, timeout time.Duration) nodeEvent {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case event, open := <-process.events:
			if !open {
				select {
				case <-process.done:
					t.Fatalf("Node output closed before %s: %v stderr=%s", state, process.terminalErr(), process.stderr)
				case <-timer.C:
					t.Fatalf("Node output closed before %s; stderr=%s", state, process.stderr)
				}
			}
			if event.Schema != "ardents-node-event-v1" {
				t.Fatalf("Node event schema = %q, want ardents-node-event-v1", event.Schema)
			}
			t.Logf("Node %d event: state=%s epoch=%d assignment=%s", process.command.Process.Pid, event.State, event.Epoch, event.Assignment)
			if event.State == state {
				return event
			}
		case <-process.done:
			for event := range process.events {
				if event.Schema != "ardents-node-event-v1" {
					t.Fatalf("Node event schema = %q, want ardents-node-event-v1", event.Schema)
				}
				t.Logf("Node %d event: state=%s epoch=%d assignment=%s", process.command.Process.Pid, event.State, event.Epoch, event.Assignment)
				if event.State == state {
					return event
				}
			}
			t.Fatalf("Node exited before %s: %v stderr=%s", state, process.terminalErr(), process.stderr)
		case <-timer.C:
			t.Fatalf("Node did not reach %s; stderr=%s", state, process.stderr)
		}
	}
}

func waitProcess(process *nodeProcess, timeout time.Duration) (error, bool) {
	select {
	case <-process.done:
		return process.terminalErr(), true
	case <-time.After(timeout):
		return nil, false
	}
}

func (process *nodeProcess) terminalErr() error {
	process.waitMu.Lock()
	defer process.waitMu.Unlock()
	return process.waitErr
}

func stopProcess(process *nodeProcess) {
	if process.command.Process != nil {
		_ = process.command.Process.Kill()
	}
	select {
	case <-process.done:
	case <-time.After(time.Second):
	}
}

func probeNode(t *testing.T, address, serverName string, server, client processCert, fixture lifecycleStateFixture, epoch lifecycleEpoch, index int, ready nodeEvent) {
	t.Helper()
	connection := dialNode(t, address, serverName, server, client)
	defer connection.Close()
	completeNodeProbe(t, connection, fixture, epoch, index, ready)
}

func dialNode(t *testing.T, address, serverName string, server, client processCert) *tls.Conn {
	t.Helper()
	roots := x509.NewCertPool()
	raw, err := os.ReadFile(server.root)
	if err != nil || !roots.AppendCertsFromPEM(raw) {
		t.Fatal("read role root")
	}
	connection, err := tls.Dial("tcp", address, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, RootCAs: roots,
		ServerName: serverName, Certificates: []tls.Certificate{loadCertificate(t, client)}, SessionTicketsDisabled: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return fmt.Errorf("node leaf certificate is missing")
			}
			key, keyErr := x509.MarshalPKIXPublicKey(state.PeerCertificates[0].PublicKey)
			if keyErr != nil || sha256.Sum256(key) != server.pin {
				return fmt.Errorf("node leaf key pin does not match")
			}
			return nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	return connection
}

func completeNodeProbe(t *testing.T, connection *tls.Conn, fixture lifecycleStateFixture, epoch lifecycleEpoch, index int, ready nodeEvent) {
	t.Helper()
	request := make([]byte, 4+1+6*32+2+32)
	copy(request, "ARNP")
	request[4] = 1
	offset := 5
	profile := sha256.Sum256([]byte("h3-role-probe-v1"))
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	for _, value := range [][32]byte{fixture.network, profile, epoch.digest, fixture.records[index].nodeID, ready.AssignmentDigest, nonce} {
		copy(request[offset:offset+32], value[:])
		offset += 32
	}
	binary.BigEndian.PutUint16(request[offset:], 32)
	payload := sha256.Sum256([]byte("bounded black-box work"))
	copy(request[len(request)-32:], payload[:])
	if _, err := connection.Write(request); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, len(request))
	if _, err := io.ReadFull(connection, response); err != nil || string(response[:4]) != "ARNS" {
		t.Fatalf("probe failed: %v", err)
	}
}

func assertAssignment(t *testing.T, fixture lifecycleStateFixture, epoch lifecycleEpoch, index int, event nodeEvent) {
	t.Helper()
	record := fixture.records[index]
	want := selectedDomain(fixture.network, epoch.number, epoch.seed, record.family)
	digest := assignment.Digest(fixture.network, epoch.number, epoch.seed, record.family, want)
	if event.Assignment != want || event.AssignmentDigest != digest {
		t.Fatalf("Node %d assignment = %s/%x, want %s/%x", index+1, event.Assignment, event.AssignmentDigest, want, digest)
	}
}

func startClockObserver(t *testing.T, path string) func() {
	t.Helper()
	if err := os.WriteFile(path, []byte("external clock observation\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(250 * time.Millisecond)
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
		cancel()
		<-done
	}
}

func writeJSON(t *testing.T, name string, value any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	raw, _ := json.Marshal(value)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	return address
}
