//go:build linux && join_platform_native

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/processresource"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// Installed JOIN operator acceptance (#604 FJD stage C) against the REAL
// platform systemd manager: the mandatory privileged platform check runs as the
// actual ExecStartPre of ardents-join.service, node join --retained-config runs
// as the shared fixed ardents-node main process, and stop/relaunch/forward
// program change/crash recovery follow the operator procedure. The crash leg
// uses the bounded node join repair-retained (stage B) against real failed-unit
// manager facts and per-unit cgroup quiescence. This is the native installed
// evidence gate before the transitional JOIN orchestration retires (stage D,
// ADR-0125).
//
// The first installation runs the fresh --config composition under a transient
// envelope to create the participation and receiving histories; the installed
// unit then opens them retained. The lifecycle/repair legs are single-unit: one
// JOIN pair member (id 20) owns its own participation root and sole node-duty
// fence; pair semantics affect only the duplex-byte workload. Honest limits: one
// fixture Release authority and one operator, not independent participants or
// whole-host privacy qualification. The forward executable is a distinct build
// of the same checked source. Genuine paired JOIN byte-transfer is proven by
// TestNativeJoinDuplexAndReopenTCP; this gate proves the platform-led installed
// lifecycle and receiver listening admission. The transitional JOIN
// orchestration is not exercised or retired here.

type joinPlatformStand struct {
	ns                string
	binaryBytes       []byte
	base              string
	distribution      string
	releaseRoot       string
	programDir        string
	program           string
	etcDir            string
	rolesBase         string
	stateRoot         string
	participationRoot string
	spendRoot         string
	hostingRoot       string
	clockPath         string
	planPath          string
	launchPath        string
	certPath          string
	keyPath           string
	launch            platformLaunch
	pin               string
	unit              []byte
	uid               int
	authority         *consumerReleaseAuthority
	networkPlan       networkAuthorityPlan
	address           string
	nodeID            [32]byte
	familyID          [32]byte
	fixture           *networkAdmissionFixture
	sockets           map[[32]byte]func()
	certificates      map[[32]byte]tls.Certificate
}

func TestNativeJoinPlatformInstalledLifecycle(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("JOIN installed qualification requires a root driver")
	}
	if _, err := exec.Command("/usr/bin/busctl", "--system", "--no-pager", "call", "org.freedesktop.systemd1", "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties", "Get", "ss", "org.freedesktop.systemd1.Manager", "Version").Output(); err != nil {
		t.Fatal("real platform systemd manager unavailable:", err)
	}
	stand := provisionJoinPlatformStand(t)

	// First installed launch: the real ExecStartPre check accepts the genuine
	// provisioned delivery against the loaded manager, then the retained runtime
	// claims the original node-duty producer and listens.
	startJoinPlatformUnit(t)
	waitJoinPlatformReady(t, stand, "first launch")
	joinPlatformProducerPresent(t, stand, true, "first launch")
	if out, err := runAsJoinNode(t, stand.program, "node", "join", "repair-retained", stand.planPath); err == nil {
		t.Fatal("repair accepted a live invocation", string(out))
	}
	stopJoinPlatformUnit(t, "first launch")
	joinPlatformProducerPresent(t, stand, false, "first completion")
	if out, err := runAsJoinNode(t, stand.program, "node", "join", "repair-retained", stand.planPath); err == nil {
		t.Fatal("repair accepted a clean root", string(out))
	}

	// Retained relaunch over the same histories.
	startJoinPlatformUnit(t)
	waitJoinPlatformReady(t, stand, "relaunch")
	stopJoinPlatformUnit(t, "relaunch")
	joinPlatformProducerPresent(t, stand, false, "relaunch completion")

	// Forward program change: a distinct working executable authenticated against
	// the retained floors through the same mandatory pre-start check.
	forwardBytes := compiledJoinPlatformCommand(t)
	if err := os.RemoveAll(stand.distribution); err != nil {
		t.Fatal(err)
	}
	infrastructureDistributionInvocation(t, stand.base, forwardBytes, stand.authority, 2, nil, "join", []byte(platformUnitTemplate))
	if err := os.WriteFile(stand.program, forwardBytes, 0555); err != nil {
		t.Fatal(err)
	}
	startJoinPlatformUnit(t)
	waitJoinPlatformReady(t, stand, "forward launch")
	stopJoinPlatformUnit(t, "forward launch")
	// Rollback to the committed predecessor must refuse before serving.
	if err := os.RemoveAll(stand.distribution); err != nil {
		t.Fatal(err)
	}
	infrastructureDistributionInvocation(t, stand.base, stand.binaryBytes, stand.authority, 1, nil, "join", []byte(platformUnitTemplate))
	if err := os.WriteFile(stand.program, stand.binaryBytes, 0555); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.CommandContext(t.Context(), "systemctl", "start", "ardents-join.service").CombinedOutput(); err == nil {
		t.Fatal("backward program change accepted", string(out))
	}
	if err := os.RemoveAll(stand.distribution); err != nil {
		t.Fatal(err)
	}
	infrastructureDistributionInvocation(t, stand.base, forwardBytes, stand.authority, 2, nil, "join", []byte(platformUnitTemplate))
	if err := os.WriteFile(stand.program, forwardBytes, 0555); err != nil {
		t.Fatal(err)
	}

	// Crash recovery: a killed invocation supplies no completion evidence; the
	// live node-duty fence stays retained, a plain start refuses, and the bounded
	// repair against real manager facts removes exactly that producer.
	startJoinPlatformUnit(t)
	waitJoinPlatformReady(t, stand, "crash launch")
	joinPlatformProducerPresent(t, stand, true, "crash launch")
	if err := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal=SIGKILL", "ardents-join.service").Run(); err != nil {
		t.Fatal("original crash signal", err)
	}
	waitJoinPlatformFailed(t)
	joinPlatformProducerPresent(t, stand, true, "after crash")
	// A plain start must refuse the retained producer. Type=exec returns once the
	// main process forks, so the runtime's asynchronous refusal is observed as the
	// unit reaching failed state (an adopting runtime would stay active).
	_ = exec.CommandContext(t.Context(), "systemctl", "start", "ardents-join.service").Run()
	waitJoinPlatformFailed(t)
	joinPlatformProducerPresent(t, stand, true, "after refused relaunch")
	out, err := runAsJoinNode(t, stand.program, "node", "join", "repair-retained", stand.planPath)
	if err != nil || !bytes.Contains(out, []byte("repaired")) {
		t.Fatal("bounded repair against real manager facts", string(out), err)
	}
	joinPlatformProducerPresent(t, stand, false, "after repair")
	startJoinPlatformUnit(t)
	waitJoinPlatformReady(t, stand, "post-repair launch")
	stopJoinPlatformUnit(t, "post-repair launch")

	// Causal pre-effect refusals against the committed delivery.
	original, err := os.ReadFile(stand.program)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Clone(original)
	changed[len(changed)-1] ^= 1
	if err := os.WriteFile(stand.program, changed, 0555); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.CommandContext(t.Context(), "systemctl", "start", "ardents-join.service").CombinedOutput(); err == nil {
		t.Fatal("substituted program accepted", string(out))
	}
	if err := os.WriteFile(stand.program, original, 0555); err != nil {
		t.Fatal(err)
	}
	launchRaw, err := os.ReadFile(stand.launchPath)
	if err != nil {
		t.Fatal(err)
	}
	foreign := bytes.Replace(launchRaw, []byte(stand.launch.ConfigSHA256), []byte(strings.Repeat("0", 64)), 1)
	if err := os.WriteFile(stand.launchPath, foreign, 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.CommandContext(t.Context(), "systemctl", "start", "ardents-join.service").CombinedOutput(); err == nil {
		t.Fatal("foreign launch pin accepted", string(out))
	}
	if err := os.WriteFile(stand.launchPath, launchRaw, 0644); err != nil {
		t.Fatal(err)
	}
	startJoinPlatformUnit(t)
	waitJoinPlatformReady(t, stand, "restored launch")
	stopJoinPlatformUnit(t, "restored launch")
}

func provisionJoinPlatformStand(t *testing.T) *joinPlatformStand {
	t.Helper()
	ctx := t.Context()
	binary := compiledCommand(t)
	binaryBytes, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	stand := &joinPlatformStand{ns: fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano()), binaryBytes: binaryBytes}
	stand.base = filepath.Join("/var/lib/ardents", "join-installed-"+stand.ns)
	stand.distribution = filepath.Join(stand.base, "distribution")
	stand.releaseRoot = filepath.Join(stand.base, "release-trust")
	stand.programDir = filepath.Join("/usr/lib/ardents", "join-installed-"+stand.ns)
	stand.program = filepath.Join(stand.programDir, "ardents-next")
	stand.etcDir = filepath.Join("/etc/ardents", "join-installed-"+stand.ns)
	stand.rolesBase = filepath.Join("/var/lib/ardents/roles/join", "installed-"+stand.ns)
	stand.stateRoot = filepath.Join(stand.rolesBase, "state")
	stand.participationRoot = filepath.Join(stand.rolesBase, "participation")
	stand.spendRoot = filepath.Join(stand.rolesBase, "spend")
	stand.hostingRoot = filepath.Join(stand.rolesBase, "hosting")
	stand.clockPath = filepath.Join(stand.rolesBase, "clock")
	stand.planPath = filepath.Join(stand.etcDir, "join.json")
	stand.launchPath = filepath.Join(stand.etcDir, "launch.json")
	stand.certPath = filepath.Join(stand.etcDir, "node.crt")
	stand.keyPath = filepath.Join(stand.etcDir, "node.key")
	stand.nodeID = [32]byte{20}

	stand.uid = joinPlatformAccount(t)
	t.Cleanup(func() { cleanupJoinPlatformStand(t, stand) })
	for _, dir := range []string{stand.base, stand.releaseRoot, stand.programDir, stand.etcDir, stand.rolesBase} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(stand.releaseRoot, 0700); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{stand.stateRoot, stand.participationRoot, stand.spendRoot} {
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chown(root, stand.uid, stand.uid); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(stand.clockPath, nil, 0644); err != nil {
		t.Fatal(err)
	}
	clockStop, clockJoined := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(clockJoined)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-clockStop:
				return
			case now := <-ticker.C:
				if err := os.Chtimes(stand.clockPath, now, now); err != nil {
					t.Error("clock observation updater", err)
					return
				}
			}
		}
	}()
	t.Cleanup(func() { close(clockStop); <-clockJoined })

	f, sockets, certificates := newJoinRouteFixtureConfigured(t, transport.ClosedCarrierTCP, nil)
	stand.fixture, stand.sockets, stand.certificates = f, sockets, certificates
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	var key [32]byte
	copy(key[:], f.spec.Authority.Public().(ed25519.PublicKey))
	stand.networkPlan = networkAuthorityPlan{Root: stand.stateRoot, NetworkID: f.spec.NetworkID, Authorities: [][32]byte{key}, Threshold: 1, ProfileAuthority: key, ClockObservationFile: stand.clockPath}
	config, err := networkStateConfig(&stand.networkPlan)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.AcceptEpoch(ctx, bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials); err != nil {
		_ = owner.Close()
		t.Fatal(err)
	}
	if _, err := owner.AcceptClosedProfile(bundle.Profile); err != nil {
		_ = owner.Close()
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := chownTreeJoin(stand.stateRoot, stand.uid); err != nil {
		t.Fatal(err)
	}
	view, err := f.current()
	if err != nil {
		t.Fatal(err)
	}
	member, err := view.Member(stand.nodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	stand.address, stand.familyID = member.Endpoint, member.FamilyID

	certificate := certificates[stand.nodeID]
	certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]})
	keyDER, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	clear(keyDER)
	if err := os.WriteFile(stand.certPath, certBytes, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stand.keyPath, keyBytes, 0600); err != nil {
		t.Fatal(err)
	}
	certDigest := platformDigest(certBytes)
	keyDigest := platformDigest(keyBytes)
	clear(keyBytes)
	if err := os.Chown(stand.keyPath, stand.uid, stand.uid); err != nil {
		t.Fatal(err)
	}

	start := time.Now().UTC().Truncate(time.Hour)
	if err := hosting.Initialize(stand.hostingRoot, hosting.Policy{Provider: "JOIN installed test", Start: start, End: start.Add(2 * time.Hour), Unit: "MiB", Quantity: 100, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1000}); err != nil {
		t.Fatal(err)
	}
	if err := chownTreeJoin(stand.hostingRoot, stand.uid); err != nil {
		t.Fatal(err)
	}

	plan := map[string]any{"network": stand.networkPlan, "node_id": stand.nodeID, "process_profile": processresource.JoinProfile,
		"participation_root": stand.participationRoot, "spend_root": stand.spendRoot, "hosting_root": stand.hostingRoot,
		"certificate": stand.certPath, "private_key": stand.keyPath,
		"work": hosting.Traffic{Tx: 32 << 20}, "termination": hosting.Traffic{Tx: 64 << 10, Rx: 64 << 10}, "retirement_wait_ms": 5000}
	planRaw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stand.planPath, planRaw, 0644); err != nil {
		t.Fatal(err)
	}

	stand.authority = newConsumerReleaseAuthority(t, infrastructureReference.Add(365*24*time.Hour))
	_, inventoryPin, _ := infrastructureDistributionInvocation(t, stand.base, binaryBytes, stand.authority, 1, nil, "join", []byte(platformUnitTemplate))
	if err := os.WriteFile(stand.program, binaryBytes, 0555); err != nil {
		t.Fatal(err)
	}
	stand.launch = platformLaunch{
		Role:          "join",
		Distribution:  stand.distribution,
		ReleaseRoot:   stand.releaseRoot,
		Environment:   "h3-test",
		Network:       "new-release-test",
		Program:       stand.program,
		Configuration: stand.planPath,
		ConfigSHA256:  platformDigest(planRaw),
		WritableRoots: []string{stand.stateRoot, stand.participationRoot, stand.spendRoot, stand.hostingRoot},
		Inputs: map[string]string{
			stand.certPath: certDigest,
			stand.keyPath:  keyDigest,
		},
	}
	launchRaw, err := json.Marshal(stand.launch)
	if err != nil {
		t.Fatal(err)
	}
	launchRaw = append(launchRaw, '\n')
	stand.pin = platformDigest(launchRaw)
	if err := os.WriteFile(stand.launchPath, launchRaw, 0644); err != nil {
		t.Fatal(err)
	}
	enrolled := filepath.Join(stand.distribution, infrastructure.ProgramName)
	if out, err := exec.CommandContext(ctx, enrolled, "platform", "initialize-trust", stand.launchPath, inventoryPin).CombinedOutput(); err != nil {
		t.Fatal("explicit installed first pin", string(out), err)
	}
	unitOut, err := exec.CommandContext(ctx, enrolled, "platform", "unit", stand.launchPath).CombinedOutput()
	if err != nil {
		t.Fatal("platform unit render", string(unitOut), err)
	}
	stand.unit = unitOut
	if !bytes.Contains(stand.unit, []byte("ExecStartPre=+:")) || !bytes.Contains(stand.unit, []byte("node join --retained-config "+stand.planPath)) {
		t.Fatal("rendered unit lacks the mandatory precheck or retained runtime", string(stand.unit))
	}
	if err := os.WriteFile("/etc/systemd/system/ardents-join.service", stand.unit, 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		t.Fatal("daemon-reload", err)
	}

	// First installation: the fresh --config composition creates the participation
	// and receiving histories under a transient envelope, then joins cleanly. The
	// installed unit subsequently opens those histories retained.
	stand.sockets[stand.nodeID]()
	preinitJoinHistories(t, stand, stand.planPath)
	return stand
}

// preinitJoinHistories runs the fresh --config composition under a transient
// envelope to create the participation and receiving histories, then joins
// cleanly. The installed unit subsequently opens those histories retained.
func preinitJoinHistories(t *testing.T, stand *joinPlatformStand, planPath string) {
	t.Helper()
	unit := fmt.Sprintf("ardents-join-preinit-%d-%d.service", os.Getpid(), time.Now().UnixNano())
	cmd := exec.CommandContext(t.Context(), "systemd-run", "--quiet", "--wait", "--pipe", "--collect", "--unit="+unit,
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
		fmt.Sprintf("--property=User=%d", stand.uid), fmt.Sprintf("--property=Group=%d", stand.uid),
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--", stand.program, "node", "join", "--config", planPath)
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var event map[string]string
	if err := json.NewDecoder(stdout).Decode(&event); err != nil || event["operation"] != "node.join" || event["phase"] != "listening" {
		_ = exec.Command("systemctl", "stop", unit).Run()
		_ = cmd.Wait()
		t.Fatal("preinit fresh JOIN startup refused", event, err, diagnostic.String())
	}
	if err := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal=SIGINT", unit).Run(); err != nil {
		t.Fatal("preinit stop signal", err)
	}
	_ = cmd.Wait()
	_ = stdout.Close()
}

// compiledJoinPlatformCommand builds a distinct working executable of the same
// checked source (canonical flags without -w), a genuine forward artifact.
func compiledJoinPlatformCommand(t *testing.T) []byte {
	t.Helper()
	dir, err := os.MkdirTemp("", "ardents-join-platform-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "ardents-next")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-o", path, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal("forward command build", string(out), err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func chownTreeJoin(root string, uid int) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, uid)
	})
}

func joinPlatformAccount(t *testing.T) int {
	t.Helper()
	if exec.Command("getent", "passwd", "ardents-node").Run() == nil || exec.Command("getent", "group", "ardents-node").Run() == nil {
		t.Fatal("ardents-node principal already present; invalid environment")
	}
	uid := 0
	for candidate := 990; candidate >= 100; candidate-- {
		name := fmt.Sprintf("%d", candidate)
		if exec.Command("getent", "passwd", name).Run() != nil && exec.Command("getent", "group", name).Run() != nil {
			uid = candidate
			break
		}
	}
	if uid == 0 {
		t.Fatal("no free unprivileged system UID/GID available")
	}
	if err := exec.Command("groupadd", "-g", fmt.Sprintf("%d", uid), "ardents-node").Run(); err != nil {
		t.Fatal("groupadd", err)
	}
	if err := exec.Command("useradd", "-u", fmt.Sprintf("%d", uid), "-g", fmt.Sprintf("%d", uid), "-d", "/nonexistent", "-s", "/usr/sbin/nologin", "-M", "ardents-node").Run(); err != nil {
		t.Fatal("useradd", err)
	}
	return uid
}

func runAsJoinNode(t *testing.T, name string, args ...string) ([]byte, error) {
	t.Helper()
	full := append([]string{"-u", "ardents-node", "--", name}, args...)
	cmd := exec.CommandContext(t.Context(), "runuser", full...)
	cmd.Dir = "/"
	return cmd.CombinedOutput()
}

func startJoinPlatformUnit(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "systemctl", "start", "ardents-join.service").CombinedOutput(); err != nil {
		t.Fatalf("installed JOIN start: %v\n%s\njournal:\n%s", err, out, joinPlatformJournal(t))
	}
}

func stopJoinPlatformUnit(t *testing.T, phase string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "systemctl", "stop", "ardents-join.service").CombinedOutput(); err != nil {
		t.Fatalf(phase, "installed JOIN stop", string(out), err)
	}
	result, err := exec.Command("systemctl", "show", "ardents-join.service", "-p", "Result", "--value").Output()
	if err != nil || strings.TrimSpace(string(result)) != "success" {
		t.Fatal(phase, "clean stop manager result", string(result), err)
	}
	var terminal struct{ Phase, Outcome string }
	found := false
	for _, line := range strings.Split(joinPlatformJournal(t), "\n") {
		if json.Unmarshal([]byte(line), &terminal) == nil && terminal.Phase != "" {
			found = true
		}
	}
	if !found || terminal.Phase != "joined" || terminal.Outcome != "canceled" {
		t.Fatal(phase, "original joined cancellation report", terminal, joinPlatformJournal(t))
	}
}

func waitJoinPlatformFailed(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if exec.Command("systemctl", "is-failed", "ardents-join.service").Run() == nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("killed unit did not reach failed state", joinPlatformJournal(t))
}

func joinPlatformJournal(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("journalctl", "-u", "ardents-join.service", "-o", "cat", "-n", "60", "--no-pager").Output()
	if err != nil {
		return "journal unavailable: " + err.Error()
	}
	return string(out)
}

// waitJoinPlatformReady proves the retained runtime opened its shared carrier
// listener at the signed member Endpoint (receiver.Listen admission).
func waitJoinPlatformReady(t *testing.T, stand *joinPlatformStand, phase string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", stand.address, 2*time.Second)
		if err == nil {
			_ = conn.Close()
			return
		}
		if exec.Command("systemctl", "is-failed", "ardents-join.service").Run() == nil {
			t.Fatal(phase, "unit failed before listening", joinPlatformJournal(t))
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal(phase, "installed JOIN never listened", joinPlatformJournal(t))
}

func joinPlatformProducerPresent(t *testing.T, stand *joinPlatformStand, want bool, phase string) {
	t.Helper()
	conflict, err := duty.ReadConflict(stand.participationRoot, time.Now, stand.nodeID, stand.familyID)
	if err != nil || conflict != want {
		t.Fatal(phase, "node-duty producer presence", conflict, want, err)
	}
}

func cleanupJoinPlatformStand(t *testing.T, stand *joinPlatformStand) {
	t.Helper()
	_ = exec.Command("systemctl", "stop", "ardents-join.service").Run()
	_ = os.Remove("/etc/systemd/system/ardents-join.service")
	_ = exec.Command("systemctl", "daemon-reload").Run()
	_ = exec.Command("userdel", "ardents-node").Run()
	_ = exec.Command("groupdel", "ardents-node").Run()
	_ = os.RemoveAll(stand.etcDir)
	_ = os.RemoveAll(stand.programDir)
	_ = os.RemoveAll(stand.base)
	_ = os.RemoveAll(stand.rolesBase)
}
