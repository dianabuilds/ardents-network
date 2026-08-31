//go:build referencec2 && h4_5_rendezvous

package service_test

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
	"testing"
	"time"
)

const h45ContributorCommand = "/usr/lib/ardents-contributor/current/ardents-node"

type h45BundleFixture struct {
	Network, Digest, TransitAuthority string
	Rendezvous                        struct {
		NodeID, Certificate, PrivateKey, Endpoint string
	}
	TransitStateMaterials map[string]uint32
	TransitStateSources   []struct {
		Address, ServerName, Root, LeafKeyDigest string
	}
	TransitStateClient struct{ Certificate, PrivateKey string }
}

func TestH45InstalledRendezvousPublisherToUserLifecycle(t *testing.T) {
	runH45InstalledRendezvous(t, referenceC2Scenario{transparentApplication: true}, 4*time.Minute)
}

func TestH45InstalledRendezvousBoundedMixedWorkload(t *testing.T) {
	scenario := referenceC2Scenario{transparentApplication: true, dynamicWorkload: referenceC2DynamicWorkload{
		Cycles: 260, IntervalMilliseconds: 250, CycleDeadlineMilliseconds: 1_000,
		NoFallbackEvery: 60, BytesEachDirection: 4 << 20}}
	runH45InstalledRendezvous(t, scenario, scenario.dynamicWorkload.timeBudget(2*time.Minute))
}

func runH45InstalledRendezvous(t *testing.T, scenario referenceC2Scenario, duration time.Duration) {
	t.Helper()
	environment := requireH43MultiHostEnvironment(t)
	deadline := time.Now().UTC().Truncate(time.Second).Add(duration)
	remote := h43RemoteC2{environment: environment}
	t.Cleanup(func() { remote.remove(t) })
	t.Cleanup(func() {
		if t.Failed() {
			h45RetainFailureEvidence(t, remote)
		}
	})
	stage := stageH43RemoteC2(t, environment, deadline, scenario)
	pairByteLimit := max(uint64(64<<20), scenario.dynamicWorkload.transitRelayByteLimit())
	deployment, pin := stageH45Bundle(t, stage.root, 1, pairByteLimit)
	nextDeployment, nextPin := stageH45Bundle(t, stage.root, 2, pairByteLimit)
	if nextDeployment != deployment {
		t.Fatal("H4-5 successor changed the deployment identity")
	}
	h43WriteFile(t, filepath.Join(stage.root, "run.sh"), []byte(h45RemoteRunner()), 0o700)
	h43WriteFile(t, filepath.Join(stage.root, "runtime-sampler.sh"), []byte(h45RuntimeSampler()), 0o700)
	removed := false
	t.Cleanup(func() {
		if removed {
			return
		}
		command := fmt.Sprintf(`set +e
touch %s/stop-clock 2>/dev/null || true
if [ -x %s ]; then
  %s contributor withdraw >/dev/null 2>&1 || true
  %s contributor remove --confirm %s >/dev/null 2>&1
  status=$?
else
  status=0
fi
exit "$status"`, h43ShellQuote(environment.remoteDirectory), h45ContributorCommand,
			h45ContributorCommand, h45ContributorCommand, h43ShellQuote(deployment))
		if output, err := remote.runCleanup(command); err != nil {
			t.Errorf("cleanup installed H4-5 Contributor: %v\n%s", err, output)
		}
	})

	remote.start(t, stage.root)
	remote.waitFile(t, environment.remoteDirectory+"/sources-ready", deadline)
	applyBinary := environment.remoteDirectory + "/bundle-1/ardents-node"
	if output, err := remote.run(t, "set -eu; chmod 700 "+h43ShellQuote(applyBinary)); err != nil {
		t.Fatalf("restore H4-5 staged executable mode: %v\n%s", err, output)
	}
	h45RunLifecycle(t, remote, "apply", applyBinary, fmt.Sprintf("contributor apply --bundle %s --manifest-pin %s",
		h43ShellQuote(environment.remoteDirectory+"/bundle-1"), h43ShellQuote(pin)))
	h45RunLifecycle(t, remote, "diagnose", h45ContributorCommand, "contributor diagnose")
	h45RunLifecycle(t, remote, "restart", h45ContributorCommand, "contributor restart")
	updated := h45RunLifecycle(t, remote, "update-idle", h45ContributorCommand, fmt.Sprintf("contributor apply --bundle %s --manifest-pin %s",
		h43ShellQuote(environment.remoteDirectory+"/bundle-2"), h43ShellQuote(nextPin)))
	if !strings.Contains(string(updated), `"generation":2`) {
		t.Fatalf("H4-5 idle update report = %q, want generation 2", updated)
	}
	h45CapturePlacement(t, remote)
	h45CaptureRuntimeSample(t, remote, "before")
	h45StartRuntimeSampler(t, remote)
	if output, err := remote.run(t, fmt.Sprintf("set -eu; printf 'ready\\n' >%s/contributor-ready",
		h43ShellQuote(environment.remoteDirectory))); err != nil {
		t.Fatalf("release H4-5 topology roles: %v\n%s", err, output)
	}

	remote.copyFile(t, environment.remoteDirectory+"/publication.json", stage.publication, deadline)
	remote.copyFile(t, environment.remoteDirectory+"/gateway-profile.json", stage.gatewayProfile, deadline)
	remote.copyFile(t, environment.remoteDirectory+"/alpha-relay-ready.json", stage.relayReady, deadline)
	stageReferenceC2AlphaCorpus(t, h43LocalProductCommand(t, "ardents"), h43LocalProductCommand(t, "ardents-control"),
		stage.publication, stage.alphaAuthority, stage.alphaPrivate, filepath.Join(stage.localRoot, "alpha-floor"), stage.network, stage.alphaLink)

	ctx, cancel := context.WithDeadline(t.Context(), deadline)
	defer cancel()
	proofResult := make(chan error, 1)
	go func() {
		proofResult <- h43MirrorRemoteProof(ctx, remote, environment.remoteDirectory+"/reference-resources", stage.proof)
	}()
	user := startKillableCommand(ctx, stage.localRoot, stage.fixture, "user", stage.localConfig)
	result := <-user.result
	if result.err != nil {
		h43AbortProofAfterUserFailure(cancel, proofResult)
		h43AssertUserResult(t, result, scenario)
		return
	}
	if err := <-proofResult; err != nil {
		t.Fatalf("mirror H4-5 remote proof: %v", err)
	}
	remote.complete(t)
	remote.wait(t)
	minimumSamples := 2
	if scenario.dynamicWorkload.Cycles > 0 {
		workloadSeconds := int(uint64(scenario.dynamicWorkload.Cycles) * uint64(scenario.dynamicWorkload.IntervalMilliseconds) / 1_000)
		minimumSamples = min(450, max(2, workloadSeconds-2))
	}
	h45StopRuntimeSampler(t, remote, minimumSamples)
	h45CaptureRuntimeSample(t, remote, "after")
	h43AssertUserResult(t, result, scenario)
	for _, role := range []string{"initiator", "introduction", "responder", "gateway", "alpha-gateway", "alpha-relay"} {
		if roleResult := h43RemoteResult(t, remote, role); roleResult.Class != "drained" {
			t.Fatalf("H4-5 remote %s class = %q, want drained", role, roleResult.Class)
		}
	}
	if application := h43RemoteResult(t, remote, "publisher-app"); application.Class != "served" {
		t.Fatalf("H4-5 Publisher Application class = %q, want served", application.Class)
	}
	if _, err := remote.readFile(t, environment.remoteDirectory+"/rendezvous.log"); err == nil {
		t.Fatal("H4-5 topology started the fixture Rendezvous")
	}

	if scenario.dynamicWorkload.Cycles == 0 {
		h45ExerciseInstalledStoragePressure(t, remote)
	}
	h45RunLifecycle(t, remote, "drain", h45ContributorCommand, "contributor drain")
	h45AssertConnectionRefused(t, remote, "drain")
	h45RunLifecycle(t, remote, "withdraw", h45ContributorCommand, "contributor withdraw")
	h45AssertConnectionRefused(t, remote, "withdrawal")
	h45RunLifecycle(t, remote, "remove", h45ContributorCommand, "contributor remove --confirm "+h43ShellQuote(deployment))
	removed = true
	h45RetainEvidence(t, remote, result, scenario.dynamicWorkload.Cycles == 0)
}

func stageH45Bundle(t *testing.T, root string, generation, pairByteLimit uint64) (string, string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "reference-c2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture h45BundleFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.TransitStateSources) != 2 || fixture.TransitStateMaterials["rendezvous"] == 0 {
		t.Fatal("H4-5 fixture lacks exact Rendezvous State or Sources")
	}
	bundle := filepath.Join(root, fmt.Sprintf("bundle-%d", generation))
	if err := os.Mkdir(bundle, 0o700); err != nil {
		t.Fatal(err)
	}
	sources := make([]map[string]any, 2)
	for index, source := range fixture.TransitStateSources {
		suffix := string(rune('a' + index))
		identity := sha256.Sum256([]byte("reference-c2-state-source-identity-" + source.Address))
		sources[index] = map[string]any{"address": source.Address, "server_name": source.ServerName,
			"identity": hex.EncodeToString(identity[:]), "family": "reference-c2-state-source-" + suffix,
			"endpoint_handle": "reference-c2-state-source-" + suffix,
			"root_ca":         "/var/lib/private/ardents-contributor/config/current/source-" + suffix + "-root.pem",
			"leaf_key_digest": source.LeafKeyDigest}
	}
	plan := map[string]any{
		"schema": "ardents-node-plan-v1", "state_root": "/var/lib/private/ardents-contributor/network",
		"local_role_state_root": "/var/lib/private/ardents-contributor/role", "network_id": fixture.Network,
		"authority_public": []string{fixture.TransitAuthority}, "threshold": 1,
		"server_certificate":     "/var/lib/private/ardents-contributor/config/current/rendezvous-cert.pem",
		"server_key":             "/var/lib/private/ardents-contributor/config/current/rendezvous-key.pem",
		"materialization_index":  fixture.TransitStateMaterials["rendezvous"],
		"clock_observation_file": "/var/lib/private/ardents-contributor/config/current/clock.observation", "order_seed": fixture.Digest,
		"source_client_certificate": "/var/lib/private/ardents-contributor/config/current/source-client-cert.pem",
		"source_client_key":         "/var/lib/private/ardents-contributor/config/current/source-client-key.pem", "sources": sources,
		"node_id": fixture.Rendezvous.NodeID, "identity_key": "/var/lib/private/ardents-contributor/config/current/rendezvous-identity.pem",
		"node_resource_profile": "ardents-rendezvous-dedicated-host-v1", "diagnostic_directory": "/var/lib/private/ardents-contributor/diagnostics",
		"rendezvous": map[string]any{"handshake_limit": 4, "waiting_limit": 2, "pair_limit": 1,
			"pair_byte_limit": pairByteLimit, "admission_timeout_ms": 5000, "drain_timeout_ms": 5000},
	}
	planRaw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"rendezvous-cert.pem": []byte(fixture.Rendezvous.Certificate), "rendezvous-key.pem": []byte(fixture.Rendezvous.PrivateKey),
		"rendezvous-identity.pem": []byte(fixture.Rendezvous.PrivateKey), "source-client-cert.pem": []byte(fixture.TransitStateClient.Certificate),
		"source-client-key.pem": []byte(fixture.TransitStateClient.PrivateKey), "clock.observation": []byte("rendezvous dedicated-host live clock observation\n"),
		"node.json": planRaw,
	}
	for _, suffix := range []string{"a", "b"} {
		contents, readErr := os.ReadFile(filepath.Join(root, "source-"+suffix+"-root.pem"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		files["source-"+suffix+"-root.pem"] = contents
	}
	executable, err := os.ReadFile(filepath.Join(root, "ardents-node"))
	if err != nil {
		t.Fatal(err)
	}
	files["ardents-node"] = executable
	digests := make(map[string]string, len(files))
	for name, contents := range files {
		mode := os.FileMode(0o600)
		if name == "ardents-node" {
			mode = 0o755
		}
		h43WriteFile(t, filepath.Join(bundle, name), contents, mode)
		digest := sha256.Sum256(contents)
		digests[name] = hex.EncodeToString(digest[:])
	}
	deploymentDigest := sha256.Sum256([]byte("ardents-rendezvous-dedicated-host:" + fixture.Network))
	deployment := hex.EncodeToString(deploymentDigest[:])
	manifest := map[string]any{"schema": "ardents-contributor-bundle-v1", "profile": "ardents-rendezvous-dedicated-host-v1",
		"deployment_id": deployment, "generation": generation, "files": digests}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	h43WriteFile(t, filepath.Join(bundle, "manifest.json"), manifestRaw, 0o600)
	pin := sha256.Sum256(manifestRaw)
	return deployment, hex.EncodeToString(pin[:])
}

func h45RunLifecycle(t *testing.T, remote h43RemoteC2, name, executable, arguments string) []byte {
	t.Helper()
	root := remote.environment.remoteDirectory
	command := h45LifecycleCommand(root, name, executable, arguments)
	if output, err := remote.run(t, command); err != nil {
		diagnosticsCommand := fmt.Sprintf(`set +e
{
  systemctl status ardents-rendezvous-contributor.service --no-pager --full
  journalctl -u ardents-rendezvous-contributor.service --since "2 minutes ago" --no-pager
} >%s/contributor-%s-systemd.log 2>&1
exit 0`, h43ShellQuote(root), name)
		_, _ = remote.run(t, diagnosticsCommand)
		report, _ := remote.readFile(t, root+"/contributor-"+name+".err")
		t.Fatalf("H4-5 Contributor %s: %v\n%s\n%s", name, err, output, report)
	}
	report, err := remote.readFile(t, root+"/contributor-"+name+".json")
	if err != nil || !strings.Contains(string(report), `"profile":"ardents-rendezvous-dedicated-host-v1"`) {
		t.Fatalf("H4-5 Contributor %s report = %q / %v", name, report, err)
	}
	return report
}

func h45CapturePlacement(t *testing.T, remote h43RemoteC2) {
	t.Helper()
	root := h43ShellQuote(remote.environment.remoteDirectory)
	command := fmt.Sprintf(`set -eu
systemctl show ardents-rendezvous-contributor.service -p ActiveState -p SubState -p MainPID -p CPUQuotaPerSecUSec -p MemoryHigh -p MemoryMax -p TasksMax -p LimitNOFILE >%s/contributor-systemd.txt
cgroup_relative=$(systemctl show ardents-rendezvous-contributor.service -p ControlGroup --value)
case "$cgroup_relative" in /system.slice/*) ;; *) exit 1 ;; esac
cgroup=/sys/fs/cgroup$cgroup_relative
test -d "$cgroup"
for name in cpu.max memory.high memory.max pids.max; do printf '%%s=' "$name"; cat "$cgroup/$name"; done >%s/contributor-cgroup.txt`, root, root)
	if output, err := remote.run(t, command); err != nil {
		t.Fatalf("capture H4-5 placement: %v\n%s", err, output)
	}
}

func h45CaptureRuntimeSample(t *testing.T, remote h43RemoteC2, name string) {
	t.Helper()
	if name != "before" && name != "after" {
		t.Fatal("H4-5 runtime sample name is outside its closed vocabulary")
	}
	root := h43ShellQuote(remote.environment.remoteDirectory)
	command := fmt.Sprintf(`set -eu
cgroup_relative=$(systemctl show ardents-rendezvous-contributor.service -p ControlGroup --value)
case "$cgroup_relative" in /system.slice/*) ;; *) exit 1 ;; esac
cgroup=/sys/fs/cgroup$cgroup_relative
{
  printf 'captured_at='; date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ
  for file in cpu.stat memory.current memory.events pids.current; do
    printf '[%%s]\n' "$file"
    cat "$cgroup/$file"
  done
  printf '[socket-summary]\n'; ss -s
  printf '[link-counters]\n'; ip -s link
} >%s/contributor-runtime-%s.txt`, root, name)
	if output, err := remote.run(t, command); err != nil {
		t.Fatalf("capture H4-5 %s runtime sample: %v\n%s", name, err, output)
	}
}

func h45RetainEvidence(t *testing.T, remote h43RemoteC2, user commandResult, pressure bool) {
	t.Helper()
	root := os.Getenv("ARDENTS_H4_5_EVIDENCE_DIR")
	if root == "" {
		return
	}
	if !filepath.IsAbs(root) {
		t.Fatal("ARDENTS_H4_5_EVIDENCE_DIR must be absolute")
	}
	h45PrepareEvidenceRoot(t, root)
	capture := remote.captureEvidence(t)
	if err := os.WriteFile(filepath.Join(root, "remote-capture.txt"), capture, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "user.stdout.log"), user.stdout, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "user.stderr.log"), user.stderr, 0o600); err != nil {
		t.Fatal(err)
	}
	names := []string{
		"contributor-apply.json", "contributor-apply.timing",
		"contributor-diagnose.json", "contributor-diagnose.timing",
		"contributor-restart.json", "contributor-restart.timing",
		"contributor-update-idle.json", "contributor-update-idle.timing",
		"contributor-systemd.txt", "contributor-cgroup.txt",
		"contributor-runtime-before.txt", "contributor-runtime-after.txt", "contributor-runtime-samples.tsv",
		"contributor-drain.json", "contributor-drain.timing",
		"contributor-withdraw.json", "contributor-withdraw.timing",
		"contributor-remove.json", "contributor-remove.timing",
	}
	if pressure {
		names = append(names, "contributor-resource-protect.json", "contributor-resource-normal.json",
			"contributor-resource-exit.json", "contributor-pressure-withdrawn.json",
			"contributor-restart-after-pressure.json", "contributor-restart-after-pressure.timing")
	}
	for _, name := range names {
		contents, err := remote.readFile(t, remote.environment.remoteDirectory+"/"+name)
		if err != nil {
			t.Fatalf("retain H4-5 operator evidence %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(root, name), contents, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func h45RetainFailureEvidence(t *testing.T, remote h43RemoteC2) {
	t.Helper()
	root := os.Getenv("ARDENTS_H4_5_EVIDENCE_DIR")
	if root == "" || !filepath.IsAbs(root) {
		return
	}
	h45PrepareEvidenceRoot(t, root)
	capture, err := remote.captureFailureEvidence()
	if err != nil && len(capture) == 0 {
		t.Errorf("capture failed H4-5 attempt: %v", err)
		return
	}
	name := "remote-failure-" + remote.environment.container + ".txt"
	file, openErr := os.OpenFile(filepath.Join(root, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if openErr != nil {
		t.Errorf("retain failed H4-5 attempt: %v", openErr)
		return
	}
	_, writeErr := file.Write(capture)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Errorf("retain failed H4-5 attempt: %v", errors.Join(writeErr, closeErr))
	}
}

func h45RemoteRunner() string {
	return `#!/bin/sh
set -eu
work=/work
pids=""
started_pid=""
cleanup() {
  status=$?
  trap - EXIT INT TERM
  for pid in $pids; do kill "$pid" 2>/dev/null || true; done
  for pid in $pids; do wait "$pid" 2>/dev/null || true; done
  exit "$status"
}
trap cleanup EXIT INT TERM
start() {
  name=$1
  shift
  "$@" >"$work/$name.log" 2>"$work/$name.err" &
  started_pid=$!
  pids="$pids $started_pid"
}
wait_file() {
  path=$1
  tries=0
  while [ "$tries" -lt 2400 ]; do
    if [ -s "$path" ]; then return 0; fi
    tries=$((tries + 1))
    sleep 0.1
  done
  return 1
}
wait_source() {
  log=$1
  tries=0
  while [ "$tries" -lt 400 ]; do
    if grep -F '"kind":"source-ready"' "$log" >/dev/null 2>&1; then return 0; fi
    tries=$((tries + 1))
    sleep 0.1
  done
  return 1
}
start source-a "$work/ardents-node" source --config "$work/source-a-plan.json"
source_a=$started_pid
start source-b "$work/ardents-node" source --config "$work/source-b-plan.json"
source_b=$started_pid
wait_source "$work/source-a.log"
wait_source "$work/source-b.log"
printf 'ready\n' >"$work/sources-ready"
wait_file "$work/contributor-ready"
transit_processes=""
for role in initiator introduction responder; do
  start "$role" "$work/reference-c2" "$role" "$work/reference-c2.json"
  transit_processes="$transit_processes $role:$started_pid"
done
for role in initiator introduction responder; do wait_file "$work/ready/$role"; done
start gateway "$work/reference-c2" gateway "$work/reference-c2.json"
gateway=$started_pid
start publisher "$work/reference-c2" publisher "$work/reference-c2.json"
publisher=$started_pid
wait_file "$work/publication.json"
start alpha-gateway "$work/reference-c2" alpha-gateway "$work/reference-c2.json"
alpha_gateway=$started_pid
wait_file "$work/alpha-gateway-ready.json"
start alpha-relay "$work/reference-c2" alpha-relay "$work/reference-c2.json"
alpha_relay=$started_pid
wait_file "$work/alpha-relay-ready.json"
wait_file "$work/ready/gateway"
start publisher-app "$work/reference-c2" publisher-app "$work/reference-c2.json"
publisher_app=$started_pid
wait_file "$work/complete"
for item in "publisher:$publisher" "publisher-app:$publisher_app" "gateway:$gateway" "alpha-gateway:$alpha_gateway" "alpha-relay:$alpha_relay" $transit_processes; do
  name=${item%%:*}
  pid=${item#*:}
  status=0
  wait "$pid" || status=$?
  printf '{"schema":"ardents-h4-5-fixture-role-exit-v1","role":"%s","pid":%s,"exit_status":%s}\n' "$name" "$pid" "$status" >>"$work/remote-role-exit-statuses.jsonl"
  [ "$status" -eq 0 ]
done
kill -TERM "$source_a" "$source_b"
wait "$source_a"
wait "$source_b"
`
}
