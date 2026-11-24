//go:build linux && source_installed_native

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
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

	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"

	sourcefixture "github.com/dianabuilds/ardents-network/tests/e2e/source"
)

// Installed Source operator acceptance (#604 stage 1.5) against the REAL
// platform systemd manager: the mandatory privileged platform check runs as
// the actual ExecStartPre of ardents-source.service, serve-retained runs as
// the fixed unprivileged main process, and stop/relaunch/forward program
// change/crash recovery follow the operator procedure. Repair-retained
// observes real failed-unit manager facts, kernel cgroup and procfs
// quiescence. This is the native evidence gate before the transitional Source
// orchestration retires (stage 1.6, ADR-0125).
//
// Honest limits: one fixture Release authority and one operator, not
// independent participants or whole-host privacy qualification. The forward
// executable is a distinct build of the same checked source. The transitional
// orchestration is not exercised or retired here.

type sourceInstalledStand struct {
	ns                string
	binaryBytes       []byte
	base              string
	distribution      string
	releaseRoot       string
	programDir        string
	program           string
	etcDir            string
	planPath          string
	launchPath        string
	bundlePath        string
	certPath          string
	keyPath           string
	caPath            string
	rolesBase         string
	stateRoot         string
	participationRoot string
	clockPath         string
	launch            platformLaunch
	pin               string
	unit              []byte
	uid               int
	authority         *consumerReleaseAuthority
	networkPlan       state.AuthorityDeclaration
	clientConfig      source.Config
	bundle            networkfixture.Closed
	authorities       map[[32]byte]ed25519.PublicKey
	dutyIdentity      [32]byte
	dutyFamily        [32]byte
	address           string
}

func TestNativeSourceInstalledLifecycle(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("Source installed qualification requires a root driver")
	}
	if _, err := exec.Command("/usr/bin/busctl", "--system", "--no-pager", "call", "org.freedesktop.systemd1", "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties", "Get", "ss", "org.freedesktop.systemd1.Manager", "Version").Output(); err != nil {
		t.Fatal("real platform systemd manager unavailable:", err)
	}
	stand := provisionSourceInstalledStand(t)

	// First launch: the real ExecStartPre check accepts the genuine provisioned
	// delivery against the loaded manager, then serve-retained claims the
	// original producer and serves authenticated signed bytes.
	startSourceUnit(t)
	waitSourceFetch(t, stand, "first launch")
	installedHealthyFetch(t, stand, "first launch")
	// Recovery while the original service is live must refuse.
	if out, err := runAsSource(t, stand.program, "source", "repair-retained", stand.planPath); err == nil {
		t.Fatal("repair accepted a live invocation", string(out))
	}
	stopSourceUnit(t, "first launch")
	installedProducerPresent(t, stand, false, "first completion")
	reopenInstalledState(t, stand)
	// A clean root refuses repair without a Claim probe.
	if out, err := runAsSource(t, stand.program, "source", "repair-retained", stand.planPath); err == nil {
		t.Fatal("repair accepted a clean root", string(out))
	}

	// Relaunch is a new independently checked invocation over the same roots.
	startSourceUnit(t)
	waitSourceFetch(t, stand, "relaunch")
	installedHealthyFetch(t, stand, "relaunch")
	stopSourceUnit(t, "relaunch")
	installedProducerPresent(t, stand, false, "relaunch completion")

	// Forward program change: a distinct working executable authenticated
	// against the retained floors through the same mandatory pre-start check.
	// The operator delivers a fresh immutable distribution at the same path.
	forwardBytes := compiledForwardCommand(t)
	if err := os.RemoveAll(stand.distribution); err != nil {
		t.Fatal(err)
	}
	infrastructureDistributionInvocation(t, stand.base, forwardBytes, stand.authority, 2, nil, "source", []byte(platformUnitTemplate))
	if err := os.WriteFile(stand.program, forwardBytes, 0555); err != nil {
		t.Fatal(err)
	}
	startSourceUnit(t)
	waitSourceFetch(t, stand, "forward launch")
	installedHealthyFetch(t, stand, "forward launch")
	stopSourceUnit(t, "forward launch")
	// Rollback to the committed predecessor must refuse before serving.
	if err := os.RemoveAll(stand.distribution); err != nil {
		t.Fatal(err)
	}
	infrastructureDistributionInvocation(t, stand.base, stand.binaryBytes, stand.authority, 1, nil, "source", []byte(platformUnitTemplate))
	if err := os.WriteFile(stand.program, stand.binaryBytes, 0555); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.CommandContext(t.Context(), "systemctl", "start", "ardents-source.service").CombinedOutput(); err == nil {
		t.Fatal("backward program change accepted", string(out))
	}
	if err := os.RemoveAll(stand.distribution); err != nil {
		t.Fatal(err)
	}
	infrastructureDistributionInvocation(t, stand.base, forwardBytes, stand.authority, 2, nil, "source", []byte(platformUnitTemplate))
	if err := os.WriteFile(stand.program, forwardBytes, 0555); err != nil {
		t.Fatal(err)
	}

	// Crash recovery: a killed invocation supplies no completion evidence.
	startSourceUnit(t)
	waitSourceFetch(t, stand, "crash launch")
	installedProducerPresent(t, stand, true, "crash launch")
	if err := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal=SIGKILL", "ardents-source.service").Run(); err != nil {
		t.Fatal("original crash signal", err)
	}
	waitSourceFailed(t)
	installedProducerPresent(t, stand, true, "after crash")
	if out, err := exec.CommandContext(t.Context(), "systemctl", "start", "ardents-source.service").CombinedOutput(); err == nil {
		t.Fatal("retained producer adopted after crash", string(out))
	}
	out, err := runAsSource(t, stand.program, "source", "repair-retained", stand.planPath)
	if err != nil || !bytes.Contains(out, []byte("repaired")) {
		t.Fatal("bounded repair against real manager facts", string(out), err)
	}
	installedProducerPresent(t, stand, false, "after repair")
	startSourceUnit(t)
	waitSourceFetch(t, stand, "post-repair launch")
	installedHealthyFetch(t, stand, "post-repair launch")
	stopSourceUnit(t, "post-repair launch")

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
	if out, err := exec.CommandContext(t.Context(), "systemctl", "start", "ardents-source.service").CombinedOutput(); err == nil {
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
	if out, err := exec.CommandContext(t.Context(), "systemctl", "start", "ardents-source.service").CombinedOutput(); err == nil {
		t.Fatal("foreign launch pin accepted", string(out))
	}
	if err := os.WriteFile(stand.launchPath, launchRaw, 0644); err != nil {
		t.Fatal(err)
	}
	startSourceUnit(t)
	waitSourceFetch(t, stand, "restored launch")
	stopSourceUnit(t, "restored launch")
}

func provisionSourceInstalledStand(t *testing.T) *sourceInstalledStand {
	t.Helper()
	ctx := t.Context()
	binary := compiledCommand(t)
	binaryBytes, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	stand := &sourceInstalledStand{ns: fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano()), binaryBytes: binaryBytes}
	stand.base = filepath.Join("/var/lib/ardents", "source-installed-"+stand.ns)
	stand.distribution = filepath.Join(stand.base, "distribution")
	stand.releaseRoot = filepath.Join(stand.base, "release-trust")
	stand.programDir = filepath.Join("/usr/lib/ardents", "source-installed-"+stand.ns)
	stand.program = filepath.Join(stand.programDir, "ardents-next")
	stand.etcDir = filepath.Join("/etc/ardents", "source-installed-"+stand.ns)
	stand.rolesBase = filepath.Join("/var/lib/ardents/roles/source", "installed-"+stand.ns)
	stand.stateRoot = filepath.Join(stand.rolesBase, "state")
	stand.participationRoot = filepath.Join(stand.rolesBase, "participation")
	stand.clockPath = filepath.Join(stand.rolesBase, "clock")
	stand.planPath = filepath.Join(stand.etcDir, "source.json")
	stand.launchPath = filepath.Join(stand.etcDir, "launch.json")
	stand.bundlePath = filepath.Join(stand.etcDir, "bundle.bin")
	stand.certPath = filepath.Join(stand.etcDir, "node.crt")
	stand.keyPath = filepath.Join(stand.etcDir, "node.key")
	stand.caPath = filepath.Join(stand.etcDir, "client-ca.pem")

	stand.uid = sourceInstalledAccount(t)
	// Register joined cleanup before any further provisioning step can fail.
	t.Cleanup(func() { cleanupSourceInstalledStand(t, stand) })
	for _, dir := range []string{stand.base, stand.releaseRoot, stand.programDir, stand.etcDir, stand.rolesBase} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(stand.releaseRoot, 0700); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{stand.stateRoot, stand.participationRoot} {
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

	f := newNetworkAdmissionFixture(t)
	bundle, err := networkfixture.BuildClosed(f.spec)
	if err != nil {
		t.Fatal(err)
	}
	stand.bundle = bundle
	var key [32]byte
	copy(key[:], f.spec.Authority.Public().(ed25519.PublicKey))
	stand.authorities = map[[32]byte]ed25519.PublicKey{sha256.Sum256(key[:]): f.spec.Authority.Public().(ed25519.PublicKey)}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	stand.address = listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	server, client, _, rootPEM, serverPin, clientPin := sourcefixture.NativeTLS(t)
	certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate[0]})
	keyDER, err := x509.MarshalPKCS8PrivateKey(server.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	clear(keyDER)
	for path, raw := range map[string][]byte{stand.certPath: certBytes, stand.keyPath: keyBytes, stand.caPath: rootPEM} {
		mode := os.FileMode(0644)
		if path == stand.keyPath {
			mode = 0600
		}
		if err := os.WriteFile(path, raw, mode); err != nil {
			t.Fatal(err)
		}
	}
	certDigest := platformDigest(certBytes)
	keyDigest := platformDigest(keyBytes)
	caDigest := platformDigest(rootPEM)
	clear(keyBytes)
	// The runtime private-key custody binds the invoking principal: the fixed
	// account owns its key (the privileged pre-start admits uid 0 or 100-999).
	if err := os.Chown(stand.keyPath, stand.uid, stand.uid); err != nil {
		t.Fatal(err)
	}
	stand.networkPlan = state.AuthorityDeclaration{Root: stand.stateRoot, NetworkID: f.spec.NetworkID, Authorities: [][32]byte{key}, Threshold: 1, ProfileAuthority: key, ClockObservationFile: stand.clockPath}
	plan := servingdeclaration.ServingDeclaration{Network: (*servingdeclaration.ServingNetwork)(&stand.networkPlan), ProcessProfile: "ardents-source-process-v1",
		ParticipationRoot: stand.participationRoot, Address: stand.address, Certificate: stand.certPath, PrivateKey: stand.keyPath, ClientRoot: stand.caPath, ClientPins: [][32]byte{clientPin}}
	planRaw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stand.planPath, planRaw, 0644); err != nil {
		t.Fatal(err)
	}
	bundleRaw, err := source.EncodeBundle(source.Bundle{Epoch: bundle.Epoch.Raw, Inputs: bundle.Epoch.Inputs, Materials: [][]byte{bundle.Epoch.Materials[0]}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stand.bundlePath, bundleRaw, 0644); err != nil {
		t.Fatal(err)
	}
	stand.clientConfig = source.Config{Sources: [2]source.Source{
		{Address: stand.address, ServerName: "source-native.test", Identity: [32]byte{71}, Family: "source-one", EndpointHandle: "source-one", RootPEM: rootPEM, LeafKeyDigest: serverPin},
		{Address: "127.0.0.1:9", ServerName: "unused.test", Identity: [32]byte{72}, Family: "source-two", EndpointHandle: "source-two", RootPEM: rootPEM, LeafKeyDigest: [32]byte{73}},
	}, ClientCertificate: client, VerificationClock: time.Now}

	// Genuine prepare-state through the exact Endpoint binary as the fixed Source account.
	if err := os.WriteFile(stand.program, binaryBytes, 0555); err != nil {
		t.Fatal(err)
	}
	if out, err := runAsSource(t, stand.program, "source", "prepare-state", stand.planPath, stand.bundlePath); err != nil || !bytes.Contains(out, []byte("state-prepared")) {
		t.Fatal("actual prepare-state", string(out), err)
	}
	if out, err := runAsSource(t, stand.program, "source", "prepare-state", stand.planPath, stand.bundlePath); err == nil {
		t.Fatal("repeated prepare-state adopted non-vacant roots", string(out))
	}
	config, err := stand.networkPlan.Config()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := owner.Current()
	if err != nil {
		_ = owner.Close()
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	stand.dutyIdentity, stand.dutyFamily = snapshot.SourceNodeID, sha256.Sum256([]byte(snapshot.SourceFamily))

	// Signed distribution of the actual command bytes; the first pin runs the
	// enrolled executable itself (platformOwnExecutable /proc/self/exe).
	stand.authority = newConsumerReleaseAuthority(t, infrastructureReference.Add(365*24*time.Hour))
	_, inventoryPin, _ := infrastructureDistributionInvocation(t, stand.base, binaryBytes, stand.authority, 1, nil, "source", []byte(platformUnitTemplate))
	stand.launch = platformLaunch{
		Role:          "source",
		Distribution:  stand.distribution,
		ReleaseRoot:   stand.releaseRoot,
		Environment:   "h3-test",
		Network:       "new-release-test",
		Program:       stand.program,
		Configuration: stand.planPath,
		ConfigSHA256:  platformDigest(planRaw),
		WritableRoots: []string{stand.stateRoot, stand.participationRoot},
		Inputs: map[string]string{
			stand.certPath: certDigest,
			stand.keyPath:  keyDigest,
			stand.caPath:   caDigest,
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
	if !bytes.Contains(stand.unit, []byte("ExecStartPre=+:")) || !bytes.Contains(stand.unit, []byte("source serve-retained "+stand.planPath)) {
		t.Fatal("rendered unit lacks the mandatory precheck or retained runtime", string(stand.unit))
	}
	if err := os.WriteFile("/etc/systemd/system/ardents-source.service", stand.unit, 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		t.Fatal("daemon-reload", err)
	}
	return stand
}

// sourceInstalledAccount creates the fixed unprivileged Source principal with
// a unique 100-999 UID/GID, nologin shell and /nonexistent home.
func sourceInstalledAccount(t *testing.T) int {
	t.Helper()
	if exec.Command("getent", "passwd", "ardents-source").Run() == nil || exec.Command("getent", "group", "ardents-source").Run() == nil {
		t.Fatal("ardents-source principal already present; invalid environment")
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
	if err := exec.Command("groupadd", "-g", fmt.Sprintf("%d", uid), "ardents-source").Run(); err != nil {
		t.Fatal("groupadd", err)
	}
	if err := exec.Command("useradd", "-u", fmt.Sprintf("%d", uid), "-g", fmt.Sprintf("%d", uid), "-d", "/nonexistent", "-s", "/usr/sbin/nologin", "-M", "ardents-source").Run(); err != nil {
		t.Fatal("useradd", err)
	}
	return uid
}

// compiledForwardCommand builds a distinct working executable of the same
// checked source (canonical flags without -w), a genuine forward artifact.
func compiledForwardCommand(t *testing.T) []byte {
	t.Helper()
	dir, err := os.MkdirTemp("", "ardents-forward-installed-")
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

func runAsSource(t *testing.T, name string, args ...string) ([]byte, error) {
	t.Helper()
	full := append([]string{"-u", "ardents-source", "--", name}, args...)
	cmd := exec.CommandContext(t.Context(), "runuser", full...)
	cmd.Dir = "/"
	return cmd.CombinedOutput()
}

func startSourceUnit(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "systemctl", "start", "ardents-source.service").CombinedOutput(); err != nil {
		t.Fatalf("installed Source start: %v\n%s\njournal:\n%s", err, out, sourceUnitJournal(t))
	}
}

func stopSourceUnit(t *testing.T, phase string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "systemctl", "stop", "ardents-source.service").CombinedOutput(); err != nil {
		t.Fatalf(phase, "installed Source stop", string(out), err)
	}
	// The installed unit admits exit 130 as success; systemd normalizes the
	// reported main status, so the runtime's own joined-cancellation report and
	// the manager result are the clean-stop evidence (the raw exit-130
	// projection is separately proven under the transient envelope).
	result, err := exec.Command("systemctl", "show", "ardents-source.service", "-p", "Result", "--value").Output()
	if err != nil || strings.TrimSpace(string(result)) != "success" {
		t.Fatal(phase, "clean stop manager result", string(result), err)
	}
	var terminal struct {
		Phase, Outcome string
	}
	found := false
	for _, line := range strings.Split(sourceUnitJournal(t), "\n") {
		if json.Unmarshal([]byte(line), &terminal) == nil && terminal.Phase != "" {
			found = true
		}
	}
	if !found || terminal.Phase != "joined" || terminal.Outcome != "canceled" {
		t.Fatal(phase, "original joined cancellation report", terminal, sourceUnitJournal(t))
	}
}

func waitSourceFailed(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if exec.Command("systemctl", "is-failed", "ardents-source.service").Run() == nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("killed unit did not reach failed state", sourceUnitJournal(t))
}

func sourceUnitJournal(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("journalctl", "-u", "ardents-source.service", "-o", "cat", "-n", "60", "--no-pager").Output()
	if err != nil {
		return "journal unavailable: " + err.Error()
	}
	return string(out)
}

func waitSourceFetch(t *testing.T, stand *sourceInstalledStand, phase string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		c, _, err := source.New(stand.clientConfig, stand.authorities)
		if err == nil {
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			response, fetchErr := c.Fetch(ctx, 0, source.Message{Operation: "latest", NetworkDigest: source.NetworkDigest(stand.networkPlan.NetworkID)})
			cancel()
			if fetchErr == nil && response.Status == "ok" {
				return
			}
		}
		if exec.Command("systemctl", "is-failed", "ardents-source.service").Run() == nil {
			t.Fatal(phase, "unit failed before serving", sourceUnitJournal(t))
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal(phase, "installed Source never served", sourceUnitJournal(t))
}

func installedHealthyFetch(t *testing.T, stand *sourceInstalledStand, phase string) {
	t.Helper()
	for _, operation := range []string{"latest", "by-digest"} {
		request := source.Message{Operation: operation, NetworkDigest: source.NetworkDigest(stand.networkPlan.NetworkID)}
		if operation == "by-digest" {
			request.ObjectDigest = stand.bundle.Epoch.Digest
		}
		c, _, err := source.New(stand.clientConfig, stand.authorities)
		if err != nil {
			t.Fatal(phase, err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		response, err := c.Fetch(ctx, 0, request)
		cancel()
		if err != nil || response.Status != "ok" || response.ObjectDigest != stand.bundle.Epoch.Digest {
			t.Fatal(phase, "genuine authenticated exchange", operation, response.Status, err)
		}
		decoded, err := source.DecodeBundle(response.Payload)
		if err != nil || !bytes.Equal(decoded.Epoch, stand.bundle.Epoch.Raw) || len(decoded.Materials) != 1 || !bytes.Equal(decoded.Materials[0], stand.bundle.Epoch.Materials[0]) {
			t.Fatal(phase, "canonical signed byte oracle differs", err)
		}
		verified, err := epoch.Verify(epoch.Policy{NetworkID: stand.networkPlan.NetworkID, Authorities: stand.authorities, Threshold: 1, Profile: "ardents-route-v3", Now: time.Now()}, decoded.Epoch, decoded.Inputs, decoded.Materials, true)
		if err != nil || verified.Header.Digest != stand.bundle.Epoch.Digest {
			t.Fatal(phase, "independent new Epoch verification", err)
		}
	}
}

func installedProducerPresent(t *testing.T, stand *sourceInstalledStand, want bool, phase string) {
	t.Helper()
	conflict, err := duty.ReadConflict(stand.participationRoot, time.Now, stand.dutyIdentity, stand.dutyFamily)
	if err != nil || conflict != want {
		t.Fatal(phase, "serving producer presence", conflict, want, err)
	}
}

func reopenInstalledState(t *testing.T, stand *sourceInstalledStand) {
	t.Helper()
	config, err := stand.networkPlan.Config()
	if err != nil {
		t.Fatal(err)
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal("original State lease not released", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

func cleanupSourceInstalledStand(t *testing.T, stand *sourceInstalledStand) {
	t.Helper()
	_ = exec.Command("systemctl", "stop", "ardents-source.service").Run()
	_ = os.Remove("/etc/systemd/system/ardents-source.service")
	_ = exec.Command("systemctl", "daemon-reload").Run()
	_ = exec.Command("userdel", "ardents-source").Run()
	_ = exec.Command("groupdel", "ardents-source").Run()
	_ = os.RemoveAll(stand.etcDir)
	_ = os.RemoveAll(stand.programDir)
	_ = os.RemoveAll(stand.base)
	_ = os.RemoveAll(stand.rolesBase)
}
