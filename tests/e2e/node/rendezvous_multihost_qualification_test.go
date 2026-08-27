//go:build h4_2_multihost

package state_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// TestH42MultiHostRendezvousQualification proves one deliberately narrow
// H4-2 fact across two hosts. A local direct peer-leg client reaches a real
// State-authorized ardents-node Rendezvous in a temporary host-network Docker
// container on the declared VPS. The remote container also runs the two
// mutually authenticated product State Sources that select the Node duty.
//
// It does not claim a public deployment, host independence, hostile-network
// resilience, a capacity profile, or H4-3 Service behavior. The direct local
// legs are intentional: the existing product Node process owns the native
// Rendezvous duty, while Endpoint/Service peer duties have their own slices.
func TestH42MultiHostRendezvousQualification(t *testing.T) {
	environment := requireH42MultiHostEnvironment(t)
	endpoint := net.JoinHostPort(environment.host, strconv.Itoa(environment.port))
	fixture := newRendezvousStateFixture(t, endpoint)
	stage := stageH42RemoteRendezvous(t, fixture, environment)
	t.Logf("H4-2 multi-host inputs: profile=%s state_epoch=%d state_digest=%s ardents_sha256=%s ardents_node_sha256=%s",
		route.Profile, fixture.epoch.number, hex.EncodeToString(fixture.epoch.digest[:]), h42FileDigest(t, filepath.Join(stage, "ardents")), h42FileDigest(t, filepath.Join(stage, "ardents-node")))
	remote := h42RemoteRendezvous{environment: environment}
	t.Cleanup(func() { remote.remove(t) })
	t.Logf("H4-2 remote host envelope: %s", remote.hostEnvelope(t))
	remote.start(t, stage)
	remote.waitReady(t)

	attachment := [32]byte{0xa2}
	initiator, err := openRendezvousProcessLeg(t.Context(), endpoint, fixture.initiator.certificate, fixture.rendezvous.public,
		fixture.leg(attachment, route.InitiatorRole))
	if err != nil {
		t.Fatalf("open local Initiator leg to remote product Rendezvous: %v\n%s", err, remote.logs(t))
	}
	defer initiator.Close()
	responder, err := openRendezvousProcessLeg(t.Context(), endpoint, fixture.responder.certificate, fixture.rendezvous.public,
		fixture.leg(attachment, route.ResponderRole))
	if err != nil {
		t.Fatalf("open local Responder leg to remote product Rendezvous: %v\n%s", err, remote.logs(t))
	}
	defer responder.Close()
	const payload = "H4-2 multi-host native Rendezvous carriage"
	if _, err := initiator.Write([]byte(payload)); err != nil {
		t.Fatalf("write local Initiator payload: %v", err)
	}
	if received := readProcessExact(t, responder, len(payload)); string(received) != payload {
		t.Fatalf("remote Rendezvous carriage = %q, want %q", received, payload)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	// Retain the Initiator certificate identity while falsely claiming
	// Responder. This isolates State-role rejection from a certificate mismatch.
	if err := submitRejectedRendezvousProcessLeg(ctx, endpoint, fixture.initiator.certificate, fixture.rendezvous.public,
		fixture.legForSender([32]byte{0xa3}, route.ResponderRole, fixture.initiator.nodeID)); err == nil {
		t.Fatal("remote product Rendezvous accepted a State-unauthorized LegBinding identity")
	}
}

type h42MultiHostEnvironment struct {
	host, sshKey, sshPath, user, knownHosts string
	candidate, candidateDigest              string
	port                                    int
	remoteDirectory                         string
	container                               string
}

func requireH42MultiHostEnvironment(t *testing.T) h42MultiHostEnvironment {
	t.Helper()
	host := os.Getenv("ARDENTS_H4_2_VPS")
	if net.ParseIP(host) == nil {
		t.Fatal("ARDENTS_H4_2_VPS must be one literal VPS IP address")
	}
	sshKey := os.Getenv("ARDENTS_H4_2_SSH_KEY")
	if info, err := os.Stat(sshKey); err != nil || !info.Mode().IsRegular() {
		t.Fatal("ARDENTS_H4_2_SSH_KEY must name an existing private-key file")
	}
	candidate := os.Getenv("ARDENTS_H4_2_CANDIDATE")
	if info, err := os.Stat(candidate); err != nil || !info.Mode().IsRegular() {
		t.Fatal("ARDENTS_H4_2_CANDIDATE must name the exact Linux amd64 candidate under qualification")
	}
	candidateDigest := strings.ToLower(os.Getenv("ARDENTS_H4_2_CANDIDATE_SHA256"))
	if len(candidateDigest) != sha256.Size*2 {
		t.Fatal("ARDENTS_H4_2_CANDIDATE_SHA256 must be the expected candidate SHA-256 digest")
	}
	if _, err := hex.DecodeString(candidateDigest); err != nil {
		t.Fatal("ARDENTS_H4_2_CANDIDATE_SHA256 must be the expected candidate SHA-256 digest")
	}
	if actual := h42FileDigest(t, candidate); actual != candidateDigest {
		t.Fatalf("ARDENTS_H4_2_CANDIDATE digest = %s, want %s", actual, candidateDigest)
	}
	sshPath := os.Getenv("ARDENTS_H4_2_SSH")
	if sshPath == "" {
		var err error
		sshPath, err = exec.LookPath("ssh")
		if err != nil {
			sshPath, err = exec.LookPath("ssh.exe")
		}
		if err != nil {
			t.Fatal("H4-2 multi-host qualification requires the ssh command")
		}
	}
	if info, err := os.Stat(sshPath); err != nil || info.IsDir() {
		t.Fatal("ARDENTS_H4_2_SSH must name an existing ssh executable")
	}
	port := 47926
	if value := os.Getenv("ARDENTS_H4_2_VPS_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1024 || parsed > 65535 {
			t.Fatal("ARDENTS_H4_2_VPS_PORT must be one unprivileged TCP port")
		}
		port = parsed
	}
	if port > 65532 {
		t.Fatal("ARDENTS_H4_2_VPS_PORT must leave three following unprivileged local test ports")
	}
	user := os.Getenv("ARDENTS_H4_2_VPS_USER")
	if user == "" {
		user = "root"
	}
	if strings.Trim(user, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-") != "" {
		t.Fatal("ARDENTS_H4_2_VPS_USER contains unsupported characters")
	}
	var nonce [6]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	suffix := hex.EncodeToString(nonce[:])
	return h42MultiHostEnvironment{host: host, sshKey: sshKey, sshPath: sshPath, user: user, knownHosts: filepath.Join(t.TempDir(), "known_hosts"),
		candidate: candidate, candidateDigest: candidateDigest, port: port,
		remoteDirectory: "/tmp/ardents-h4-2-multihost-" + suffix, container: "ardents-h4-2-multihost-" + suffix}
}

func stageH42RemoteRendezvous(t *testing.T, fixture rendezvousStateFixture, environment h42MultiHostEnvironment) string {
	t.Helper()
	stage := t.TempDir()
	h42CopyInput(t, stage, "ardents", environment.candidate)
	if actual := h42FileDigest(t, filepath.Join(stage, "ardents")); actual != environment.candidateDigest {
		t.Fatalf("staged H4-2 candidate digest = %s, want %s", actual, environment.candidateDigest)
	}
	buildH42LinuxCommand(t, "ardents-node", filepath.Join(stage, "ardents-node"))

	sources, client := h42SourceCredentials(t)
	h42CopyInput(t, stage, "rendezvous-cert.pem", fixture.rendezvous.certificatePath)
	h42CopyInput(t, stage, "rendezvous-key.pem", fixture.rendezvous.credentials.key)
	h42CopyInput(t, stage, "rendezvous-identity.pem", fixture.rendezvous.credentials.key)
	h42CopyInput(t, stage, "source-client-root.pem", client.root)
	h42CopyInput(t, stage, "source-client-cert.pem", client.certificate)
	h42CopyInput(t, stage, "source-client-key.pem", client.key)
	for index, source := range sources {
		prefix := fmt.Sprintf("source-%c", 'a'+index)
		h42CopyInput(t, stage, prefix+"-root.pem", source.root)
		h42CopyInput(t, stage, prefix+"-cert.pem", source.certificate)
		h42CopyInput(t, stage, prefix+"-key.pem", source.key)
	}
	h42WriteStateInputs(t, stage, fixture)
	h42WriteJSON(t, filepath.Join(stage, "source-a-plan.json"), h42SourcePlan(fixture, "a", environment.port+1, client))
	h42WriteJSON(t, filepath.Join(stage, "source-b-plan.json"), h42SourcePlan(fixture, "b", environment.port+2, client))
	h42WriteJSON(t, filepath.Join(stage, "rendezvous-node-plan.json"), h42RendezvousPlan(fixture, environment, sources, client))
	h42WriteFile(t, filepath.Join(stage, "run.sh"), []byte(h42RemoteRunner(fixture)), 0o700)
	return stage
}

func h42SourceCredentials(t *testing.T) ([2]processCert, processCert) {
	t.Helper()
	clientAuthority := makeAuthority(t, "h42-multihost-source-client-root")
	client := makeLeaf(t, clientAuthority, "h42-multihost-source-client.test", false)
	var sources [2]processCert
	for index := range sources {
		authority := makeAuthority(t, fmt.Sprintf("h42-multihost-source-%c-root", 'a'+index))
		sources[index] = makeLeaf(t, authority, fmt.Sprintf("h42-multihost-source-%c.test", 'a'+index), true)
	}
	return sources, client
}

func h42WriteStateInputs(t *testing.T, stage string, fixture rendezvousStateFixture) {
	t.Helper()
	h42WriteFile(t, filepath.Join(stage, "epoch.bin"), fixture.epoch.raw, 0o600)
	inputs := filepath.Join(stage, "inputs")
	if err := os.Mkdir(inputs, 0o700); err != nil {
		t.Fatal(err)
	}
	for index, raw := range fixture.epoch.inputs {
		h42WriteFile(t, filepath.Join(inputs, fmt.Sprintf("%04d.bin", index)), raw, 0o600)
	}
	h42WriteFile(t, filepath.Join(stage, "material-rendezvous.bin"), fixture.epoch.materials[fixture.rendezvousIndex], 0o600)
	h42WriteFile(t, filepath.Join(stage, "material-source.bin"), fixture.epoch.materials[0], 0o600)
}

func h42SourcePlan(fixture rendezvousStateFixture, suffix string, port int, client processCert) map[string]any {
	return map[string]any{"schema": "ardents-source-server-v1", "state_root": "/work/source-" + suffix + "-state",
		"local_role_state_root": "/work/source-" + suffix + "-roles", "network_id": hex.EncodeToString(fixture.network[:]),
		"authority_public": []string{hex.EncodeToString(fixture.authorityPublic)}, "threshold": 1,
		"at": fixture.now.Format(time.RFC3339), "listen": net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
		"server_certificate": "/work/source-" + suffix + "-cert.pem", "server_key": "/work/source-" + suffix + "-key.pem",
		"client_root": "/work/source-client-root.pem", "client_key_digests": []string{hex.EncodeToString(client.sourcePin[:])},
		"materialization_index": 0, "native_rendezvous_profile": true}
}

func h42RendezvousPlan(fixture rendezvousStateFixture, environment h42MultiHostEnvironment, sources [2]processCert, client processCert) map[string]any {
	declaredSources := make([]map[string]any, len(sources))
	for index, source := range sources {
		suffix := string(rune('a' + index))
		identity := sha256.Sum256([]byte("h42-multihost-source-identity-" + suffix))
		declaredSources[index] = map[string]any{"address": net.JoinHostPort("127.0.0.1", strconv.Itoa(environment.port+1+index)),
			"server_name": "h42-multihost-source-" + suffix + ".test", "identity": hex.EncodeToString(identity[:]),
			"family": "h42-multihost-source-" + suffix, "endpoint_handle": "h42-multihost-source-" + suffix,
			"root_ca": "/work/source-" + suffix + "-root.pem", "leaf_key_digest": hex.EncodeToString(source.sourcePin[:])}
	}
	return map[string]any{"schema": "ardents-node-plan-v1", "state_root": "/work/rendezvous-state",
		"local_role_state_root": "/work/rendezvous-roles", "network_id": hex.EncodeToString(fixture.network[:]),
		"authority_public": []string{hex.EncodeToString(fixture.authorityPublic)}, "threshold": 1, "at": fixture.now.Format(time.RFC3339),
		"listen": net.JoinHostPort(environment.host, strconv.Itoa(environment.port)), "server_certificate": "/work/rendezvous-cert.pem",
		"server_key": "/work/rendezvous-key.pem", "client_root": "/work/source-a-root.pem",
		"client_key_digests": []string{hex.EncodeToString(client.sourcePin[:])}, "materialization_index": fixture.rendezvousIndex,
		"clock_observation_file": "/work/clock.observation", "order_seed": strings.Repeat("39", 32),
		"source_client_certificate": "/work/source-client-cert.pem", "source_client_key": "/work/source-client-key.pem",
		"sources": declaredSources, "node_id": hex.EncodeToString(fixture.rendezvous.nodeID[:]), "identity_key": "/work/rendezvous-identity.pem",
		"maximum_duty_ms": 3000, "drain_timeout_ms": 3000,
		"rendezvous": map[string]any{"handshake_limit": 2, "waiting_limit": 2, "pair_limit": 1, "pair_byte_limit": 1 << 20, "admission_timeout_ms": 3000, "drain_timeout_ms": 3000}}
}

func h42RemoteRunner(fixture rendezvousStateFixture) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu
work=/work
node_pid=
source_a_pid=
source_b_pid=
clock_pid=
tee_pid=
cleanup() {
  status=$?
  trap - EXIT INT TERM
  for pid in "$node_pid" "$source_a_pid" "$source_b_pid" "$clock_pid" "$tee_pid"; do
    if [ -n "$pid" ]; then kill "$pid" 2>/dev/null || true; fi
  done
  for pid in "$node_pid" "$source_a_pid" "$source_b_pid" "$clock_pid" "$tee_pid"; do
    if [ -n "$pid" ]; then wait "$pid" 2>/dev/null || true; fi
  done
  exit "$status"
}
trap cleanup EXIT INT TERM
accept_state() {
  root=$1
  material=$2
  "$work/ardents" accept-offline --state-root "$work/$root" --network-id %q --authorities %q --threshold 1 --at %q --epoch "$work/epoch.bin" --inputs "$work/inputs" --materialization "$work/$material" --profile %q
}
accept_state rendezvous-state material-rendezvous.bin
accept_state source-a-state material-source.bin
accept_state source-b-state material-source.bin
"$work/ardents-node" source --config "$work/source-a-plan.json" >"$work/source-a.log" 2>"$work/source-a.err" &
source_a_pid=$!
"$work/ardents-node" source --config "$work/source-b-plan.json" >"$work/source-b.log" 2>"$work/source-b.err" &
source_b_pid=$!
wait_source() {
  log=$1
  pid=$2
  tries=0
  while [ "$tries" -lt 100 ]; do
    if grep -F '"kind":"source-ready"' "$log" >/dev/null 2>&1; then return 0; fi
    if ! kill -0 "$pid" 2>/dev/null; then cat "$log" >&2 || true; return 1; fi
    tries=$((tries + 1))
    sleep 0.1
  done
  cat "$log" >&2 || true
  return 1
}
wait_source "$work/source-a.log" "$source_a_pid"
wait_source "$work/source-b.log" "$source_b_pid"
touch "$work/clock.observation"
(
  while :; do
    touch "$work/clock.observation"
    sleep 0.1
  done
) &
clock_pid=$!
mkfifo "$work/node.events"
tee "$work/node.log" <"$work/node.events" &
tee_pid=$!
"$work/ardents-node" node --config "$work/rendezvous-node-plan.json" >"$work/node.events" 2>"$work/node.err" &
node_pid=$!
wait "$node_pid"
`, hex.EncodeToString(fixture.network[:]), hex.EncodeToString(fixture.authorityPublic), fixture.now.Format(time.RFC3339), route.Profile)
}

func buildH42LinuxCommand(t *testing.T, name, destination string) {
	t.Helper()
	command := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", destination, "./cmd/"+name)
	command.Dir = filepath.Join("..", "..", "..")
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOARCH=amd64", "GOOS=linux", "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("cross-build Linux %s: %v\n%s", name, err, output)
	}
	if err := os.Chmod(destination, 0o700); err != nil {
		t.Fatal(err)
	}
}

func h42CopyInput(t *testing.T, root, name, source string) {
	t.Helper()
	value, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	h42WriteFile(t, filepath.Join(root, name), value, 0o600)
}

func h42WriteFile(t *testing.T, path string, value []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, value, mode); err != nil {
		t.Fatal(err)
	}
}

func h42WriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	h42WriteFile(t, path, raw, 0o600)
}

func h42FileDigest(t *testing.T, path string) string {
	t.Helper()
	value, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

type h42RemoteRendezvous struct {
	environment h42MultiHostEnvironment
}

func (remote h42RemoteRendezvous) start(t *testing.T, stage string) {
	t.Helper()
	environment := remote.environment
	ports := fmt.Sprintf(":(%d|%d|%d|%d)[[:space:]]", environment.port, environment.port+1, environment.port+2, environment.port+3)
	if output, err := remote.run(t, fmt.Sprintf(`set -eu
if [ -e %[1]s ]; then printf 'H4-2 preflight: remote directory already exists\n' >&2; exit 1; fi
if docker container inspect %[2]s >/dev/null 2>&1; then printf 'H4-2 preflight: generated container already exists\n' >&2; exit 1; fi
if ! docker image inspect golang:1.26.6 >/dev/null; then printf 'H4-2 preflight: required golang image is unavailable\n' >&2; exit 1; fi
if ss -ltnH | grep -E %[3]s >/dev/null; then printf 'H4-2 preflight: one selected public or State Source port is already listening\n' >&2; exit 1; fi`,
		h42ShellQuote(environment.remoteDirectory), h42ShellQuote(environment.container), h42ShellQuote(ports))); err != nil {
		t.Fatalf("H4-2 remote qualification environment is unavailable: %v\n%s", err, output)
	}
	if err := remote.upload(t, stage); err != nil {
		t.Fatalf("upload bounded H4-2 remote bundle: %v", err)
	}
	command := fmt.Sprintf("set -eu; docker run --detach --name %s --network host --pids-limit 128 --memory 1g --cpus 1 -v %s:/work --workdir /work golang:1.26.6 /bin/sh /work/run.sh",
		h42ShellQuote(environment.container), h42ShellQuote(environment.remoteDirectory))
	if output, err := remote.run(t, command); err != nil {
		t.Fatalf("start remote H4-2 product Rendezvous: %v\n%s", err, output)
	}
}

func (remote h42RemoteRendezvous) waitReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		logs := remote.logs(t)
		if strings.Contains(logs, `"state":"READY"`) {
			return
		}
		status, err := remote.run(t, fmt.Sprintf("docker inspect --format '{{.State.Status}}' %s", h42ShellQuote(remote.environment.container)))
		if err != nil || strings.TrimSpace(status) != "running" {
			t.Fatalf("remote H4-2 product Rendezvous exited before READY: %v / %s\n%s", err, status, logs)
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("remote H4-2 product Rendezvous did not reach READY\n%s", remote.logs(t))
}

func (remote h42RemoteRendezvous) hostEnvelope(t *testing.T) string {
	t.Helper()
	output, err := remote.run(t, "set -eu; printf 'docker='; docker version --format '{{.Server.Version}}'; printf 'image_id='; docker image inspect golang:1.26.6 --format '{{.Id}}'; printf 'kernel='; uname -srmo; printf 'vcpus='; nproc; awk '/MemTotal:/ {printf \"memory_kib=%s\\n\", $2}' /proc/meminfo")
	if err != nil {
		t.Fatalf("read H4-2 remote host envelope: %v\n%s", err, output)
	}
	return strings.Join(strings.Fields(output), " ")
}

func (remote h42RemoteRendezvous) logs(t *testing.T) string {
	t.Helper()
	output, err := remote.run(t, fmt.Sprintf("set -eu; docker logs %s 2>&1 || true; for file in source-a.log source-a.err source-b.log source-b.err node.log node.err; do if [ -f %s/$file ]; then echo ===$file===; cat %s/$file; fi; done",
		h42ShellQuote(remote.environment.container), h42ShellQuote(remote.environment.remoteDirectory), h42ShellQuote(remote.environment.remoteDirectory)))
	if err != nil {
		return fmt.Sprintf("remote logs unavailable: %v\n%s", err, output)
	}
	return output
}

func (remote h42RemoteRendezvous) remove(t *testing.T) {
	t.Helper()
	command := fmt.Sprintf("set -eu; docker rm -f %s >/dev/null 2>&1 || true; if [ -e %s ]; then rm -rf -- %s; fi; test ! -e %s; ! docker container inspect %s >/dev/null 2>&1",
		h42ShellQuote(remote.environment.container), h42ShellQuote(remote.environment.remoteDirectory), h42ShellQuote(remote.environment.remoteDirectory),
		h42ShellQuote(remote.environment.remoteDirectory), h42ShellQuote(remote.environment.container))
	if output, err := remote.runCleanup(command); err != nil {
		t.Errorf("remove bounded H4-2 remote qualification resources: %v\n%s", err, output)
	}
}

func (remote h42RemoteRendezvous) upload(t *testing.T, stage string) error {
	t.Helper()
	command := fmt.Sprintf("set -eu; test ! -e %s; mkdir -m 700 %s; tar -xzf - -C %s; chmod 700 %s/ardents %s/ardents-node %s/run.sh",
		h42ShellQuote(remote.environment.remoteDirectory), h42ShellQuote(remote.environment.remoteDirectory), h42ShellQuote(remote.environment.remoteDirectory),
		h42ShellQuote(remote.environment.remoteDirectory), h42ShellQuote(remote.environment.remoteDirectory), h42ShellQuote(remote.environment.remoteDirectory))
	process := remote.command(t, command)
	stdin, err := process.StdinPipe()
	if err != nil {
		return err
	}
	var output bytes.Buffer
	process.Stdout, process.Stderr = &output, &output
	if err := process.Start(); err != nil {
		return err
	}
	archiveErr := h42WriteArchive(stage, stdin)
	closeErr := stdin.Close()
	waitErr := process.Wait()
	if archiveErr != nil || closeErr != nil || waitErr != nil {
		return fmt.Errorf("remote archive transfer: archive=%v close=%v remote=%v\n%s", archiveErr, closeErr, waitErr, output.String())
	}
	return nil
}

func (remote h42RemoteRendezvous) run(t *testing.T, command string) (string, error) {
	t.Helper()
	process := remote.command(t, command)
	output, err := process.CombinedOutput()
	return string(output), err
}

func (remote h42RemoteRendezvous) command(t *testing.T, remoteCommand string) *exec.Cmd {
	t.Helper()
	return remote.commandContext(t.Context(), remoteCommand)
}

func (remote h42RemoteRendezvous) runCleanup(remoteCommand string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	process := remote.commandContext(ctx, remoteCommand)
	output, err := process.CombinedOutput()
	return string(output), err
}

func (remote h42RemoteRendezvous) commandContext(ctx context.Context, remoteCommand string) *exec.Cmd {
	return exec.CommandContext(ctx, remote.environment.sshPath, "-i", remote.environment.sshKey, "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "-o", "StrictHostKeyChecking=accept-new", "-o", "UserKnownHostsFile="+remote.environment.knownHosts,
		remote.environment.user+"@"+remote.environment.host, "sh -lc "+h42ShellQuote(remoteCommand))
}
