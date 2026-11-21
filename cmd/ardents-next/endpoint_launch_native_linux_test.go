//go:build linux && endpoint_launch_native

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/allocation"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// platformLedNetwork carries only public signed material and trust pins; every
// participant authenticates it through its own actual State owner.
type platformLedNetwork struct {
	NetworkID, Seed, Authority [32]byte
	Epoch, Profile             []byte
	Inputs, Materials          [][]byte
}

// This native qualification proves the FULL platform-led launch chain to
// protected text Service readiness for ONE dual-role (publisher+reader) Endpoint
// on the real admitted Ubuntu/systemd host (#391 S1). Platform preparation
// sets up the account and Worker resources, independently initializes trust,
// seals the generation via the production platform command and prepares the genuine
// histories, installs the platform main unit (sole privileged F-25 ExecStartPre),
// lets systemd start the participant (no root ACK), grants permissions through an
// independent fixture granter and observes the original-process listeners.
//
// It is component/acceptance evidence with fixture Release/Network authority for
// one dual-role Endpoint. It is NOT the two-participant privacy qualification
// (that is #391 S2, two separately admitted managers). The transitional
// guarded-start/ACK/recovery orchestration is not exercised or retired here.

func TestNativeEndpointPlatformLedLaunch(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("endpoint platform-led launch qualification requires a root driver")
	}
	if _, err := exec.Command("/usr/bin/busctl", "--system", "--no-pager", "call", "org.freedesktop.systemd1", "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties", "Get", "ss", "org.freedesktop.systemd1.Manager", "Version").Output(); err != nil {
		t.Fatal("real platform systemd manager unavailable:", err)
	}
	stand := provisionPlatformLedLaunch(t)
	t.Cleanup(func() {
		if t.Failed() {
			journal, _ := exec.Command("journalctl", "--no-pager", "-u", "ardents-endpoint.service", "-n", "40", "-o", "cat").CombinedOutput()
			t.Logf("participant journal:\n%s", journal)
		}
	})
	original := stand.startPlatformLed(t)
	stand.grantPermissions(t, original)
	stand.waitServiceReady(t, original)
	stand.requireNoStartupExchange(t)
	t.Logf("platform-led Endpoint reached Service readiness; MainPID=%s InvocationID=%s", original[0], original[1])
	manager, err := systemd.RetainEndpoint(t.Context())
	if err != nil {
		t.Fatal("retain original Endpoint unit:", err)
	}
	defer func() {
		if err := manager.Close(); err != nil {
			t.Error("release Endpoint unit:", err)
		}
	}()
	pid, err := strconv.ParseUint(original[0], 10, 32)
	if err != nil {
		t.Fatal("parse original Endpoint PID:", err)
	}
	scopes, err := cgroup.Retain(t.Context(), uint32(pid), stand.uid)
	if err != nil {
		t.Fatal("retain original Endpoint and worker scopes:", err)
	}
	defer func() {
		if err := scopes.Close(); err != nil {
			t.Error("release original scopes:", err)
		}
	}()
	if joined, err := scopes.Joined(); err != nil || joined {
		t.Fatalf("live Endpoint did not retain a populated scope: joined=%v err=%v", joined, err)
	}
	stand.stopParticipant(t, original)
	stand.requireJoinedParticipant(t, original)
	if joined, err := scopes.Joined(); err != nil || !joined {
		t.Fatalf("original scopes did not join: joined=%v err=%v", joined, err)
	}
	for _, role := range []string{"reader", "publisher"} {
		unit, _, err := systemd.Activation(t.Context(), role)
		if err != nil {
			t.Fatalf("observe stopped %s activation socket: %v", role, err)
		}
		if !systemd.Matches(unit, "Id", "s", "ardents-text-"+role+".socket") ||
			!systemd.Matches(unit, "ActiveState", "s", "inactive") ||
			!systemd.Matches(unit, "SubState", "s", "dead") ||
			!systemd.Matches(unit, "Job", "(uo)", []any{uint32(0), "/"}) {
			t.Fatalf("activation socket %s did not stop cleanly", role)
		}
	}
	if err := cgroup.ObserveEmpty(t.Context()); err != nil {
		t.Fatal("observe empty Endpoint and worker scope inventory:", err)
	}
	if err := manager.Observe(t.Context()); err != nil {
		t.Fatal("observe original manager after Stop:", err)
	}
	stand.requireNoStartupExchange(t)
}

func (s *platformLedLaunchStand) requireNoStartupExchange(t *testing.T) {
	t.Helper()
	for _, name := range []string{"start-guard.json", "start-socket.json", "start-completion.socket"} {
		if _, err := os.Lstat(filepath.Join(s.installationRoot, name)); !os.IsNotExist(err) {
			t.Fatalf("platform-led Endpoint retained guarded-start artifact %s: %v", name, err)
		}
	}
}

// TestNativeEndpointPlatformLedLaunchRefusals verifies the platform pre-start
// trust transfer refuses foreign substitutions on the genuine provisioned root.
// Each fault is applied before the privileged check and refused pre-effect; the
// healthy positive launch above establishes the baseline.
func TestNativeEndpointPlatformLedLaunchRefusals(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("endpoint platform-led launch qualification requires a root driver")
	}
	stand := provisionPlatformLedLaunch(t)
	for _, fault := range []string{"substituted-program", "substituted-unit", "wrong-account", "foreign-pin", "substituted-credential"} {
		t.Run(fault, func(t *testing.T) {
			restore := mutateLaunchFault(t, stand, fault)
			defer restore()
			if out, err := exec.CommandContext(t.Context(), stand.binary, "platform", "check", stand.manifest, stand.pin).CombinedOutput(); err == nil {
				t.Fatalf("causal refusal missing for %s: %s", fault, out)
			}
		})
	}
}

func mutateLaunchFault(t *testing.T, s *platformLedLaunchStand, fault string) func() {
	t.Helper()
	switch fault {
	case "substituted-program":
		original, err := os.ReadFile(s.program)
		if err != nil {
			t.Fatal(err)
		}
		changed := bytes.Clone(original)
		changed[len(changed)-1] ^= 1
		if err := os.WriteFile(s.program, changed, 0555); err != nil {
			t.Fatal(err)
		}
		return func() { _ = os.WriteFile(s.program, original, 0555) }
	case "substituted-unit":
		original, err := os.ReadFile("/etc/systemd/system/ardents-endpoint.service")
		if err != nil {
			t.Fatal(err)
		}
		changed := append(bytes.Clone(original), []byte("# foreign drop-in comment\n")...)
		if err := os.WriteFile("/etc/systemd/system/ardents-endpoint.service", changed, 0644); err != nil {
			t.Fatal(err)
		}
		_ = exec.Command("systemctl", "daemon-reload").Run()
		return func() {
			_ = os.WriteFile("/etc/systemd/system/ardents-endpoint.service", original, 0644)
			_ = exec.Command("systemctl", "daemon-reload").Run()
		}
	case "wrong-account":
		if err := exec.Command("usermod", "-s", "/bin/bash", "ardents-endpoint").Run(); err != nil {
			t.Fatal(err)
		}
		return func() { _ = exec.Command("usermod", "-s", "/usr/sbin/nologin", "ardents-endpoint").Run() }
	case "foreign-pin":
		original, err := os.ReadFile(s.manifest)
		if err != nil {
			t.Fatal(err)
		}
		var launch platformLaunch
		if err := json.Unmarshal(original, &launch); err != nil {
			t.Fatal(err)
		}
		changed := bytes.Replace(original, []byte(launch.ConfigSHA256), []byte(strings.Repeat("0", 64)), 1)
		if err := os.WriteFile(s.manifest, changed, 0644); err != nil {
			t.Fatal(err)
		}
		return func() { _ = os.WriteFile(s.manifest, original, 0644) }
	case "substituted-credential":
		keyPath := filepath.Join(s.etcDir, "client.key")
		original, err := os.ReadFile(keyPath)
		if err != nil {
			t.Fatal(err)
		}
		changed := bytes.Clone(original)
		changed[len(changed)-1] ^= 1
		if err := os.WriteFile(keyPath, changed, 0600); err != nil {
			t.Fatal(err)
		}
		return func() { _ = os.WriteFile(keyPath, original, 0600) }
	default:
		t.Fatal("unknown fault", fault)
		return func() {}
	}
}

type platformLedLaunchStand struct {
	base               string
	etcDir             string
	binary             string
	distribution       string
	floors             string
	installationRoot   string
	generationDigest   string
	program            string
	requestPath        string
	manifest           string
	pin                string
	uid, gid           uint32
	f                  *networkAdmissionFixture
	network            platformLedNetwork
	sourceServers      []interface{ Close() error }
	permissionDir      string
	publisherDir       string
	applicationSock    string
	administrationSock string
}

func provisionPlatformLedLaunch(t *testing.T) *platformLedLaunchStand {
	t.Helper()
	return provisionPlatformLedLaunchRefresh(t, 1000)
}

// provisionPlatformLedLaunchRefresh provisions the platform-led launch stand with
// an explicit State refresh cadence. The launch qualification uses the fast 1000 ms
// fixture cadence; the NET-32 idle measurement uses a normal idle cadence (60000 ms)
// so it observes ordinary refresh traffic, not the fast-test fixture's aggressive
// polling (#652).
func provisionPlatformLedLaunchRefresh(t *testing.T, refreshMS int) *platformLedLaunchStand {
	t.Helper()
	return provisionPlatformLedLaunchFixture(t, refreshMS, newNetworkAdmissionFixture(t))
}

// provisionPlatformLedLaunchFixture provisions the same platform-led launch stand
// against an already-created Network admission fixture. The cold/warm latency
// harness (#650) supplies the publication Route network fixture so the installed
// participant reads through genuine Forwarding/Descriptor/Introduction/JOIN/
// Responder/Issuer receivers; the launch and NET-32 qualifications supply the
// ordinary idle fixture.
func provisionPlatformLedLaunchFixture(t *testing.T, refreshMS int, f *networkAdmissionFixture) *platformLedLaunchStand {
	t.Helper()
	ctx := t.Context()
	stand := &platformLedLaunchStand{binary: compiledCommand(t), f: f}
	stand.base = filepath.Join("/var/lib/ardents", fmt.Sprintf("ep-launch-%d-%d", os.Getpid(), time.Now().UnixNano()))
	stand.etcDir = filepath.Join("/etc/ardents", fmt.Sprintf("ep-launch-%d-%d", os.Getpid(), time.Now().UnixNano()))
	stand.installationRoot = filepath.Join(stand.base, "installation")
	stand.floors = filepath.Join(stand.base, "floors")
	for _, dir := range []string{stand.base, stand.etcDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	// Registered first so joined cleanup runs last (LIFO), after the clock and
	// Source servers have stopped, before the root directories are removed.
	t.Cleanup(func() { stand.cleanup(t) })

	// Genuine Network authority + signed closed Epoch/profile.
	epoch, err := networkfixture.BuildClosed(stand.f.spec)
	if err != nil {
		t.Fatal(err)
	}
	var authority [32]byte
	copy(authority[:], stand.f.spec.Authority.Public().(ed25519.PublicKey))
	stand.network = platformLedNetwork{NetworkID: stand.f.spec.NetworkID, Seed: stand.f.spec.Seed, Authority: authority,
		Epoch: epoch.Epoch.Raw, Inputs: epoch.Epoch.Inputs, Materials: epoch.Epoch.Materials, Profile: epoch.Profile}

	// Independent clock observation file kept fresh by a joined controller.
	clock := filepath.Join(stand.base, "clock")
	launchWrite(t, clock, nil, 0644)
	stopClock, clockJoined := make(chan struct{}), make(chan error, 1)
	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stopClock:
				clockJoined <- nil
				return
			case now := <-tick.C:
				if err := os.Chtimes(clock, now, now); err != nil {
					clockJoined <- err
					return
				}
			}
		}
	}()
	t.Cleanup(func() {
		close(stopClock)
		if err := <-clockJoined; err != nil {
			t.Error(err)
		}
	})

	plan := &networkAuthorityPlan{Root: filepath.Join(stand.base, "state"), NetworkID: stand.network.NetworkID,
		Authorities: [][32]byte{authority}, Threshold: 1, ProfileAuthority: authority, ClockObservationFile: clock}
	config, err := networkStateConfig(plan)
	if err != nil {
		t.Fatal(err)
	}

	// Source credential pair below /etc/ardents: cert root-owned public, key
	// service-owned private (chowned after provision creates the account). Both
	// the F-25 pre-start check and the confined participant read them.
	clientCert, clientPEM, clientKey, clientPin := installedSourceCertificate(t, "client", false)
	_ = clientCert
	certPath := filepath.Join(stand.etcDir, "client.crt")
	keyPath := filepath.Join(stand.etcDir, "client.key")
	launchWrite(t, certPath, clientPEM, 0644)
	launchWrite(t, keyPath, clientKey, 0600)
	clear(clientKey)

	// Two genuine live Source servers, each its own accepted State root.
	credentials := filepath.Join(stand.base, "credentials")
	if err := os.Mkdir(credentials, 0755); err != nil {
		t.Fatal(err)
	}
	var addresses [2]string
	var sourcePins [2][32]byte
	for i := range addresses {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addresses[i] = listener.Addr().String()
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for i, address := range addresses {
		name := []string{"source-a.example", "source-b.example"}[i]
		certificate, rootPEM, keyPEM, pin := installedSourceCertificate(t, name, true)
		clear(keyPEM)
		sourcePins[i] = pin
		launchWrite(t, filepath.Join(credentials, []string{"a.pem", "b.pem"}[i]), rootPEM, 0644)
		cfg := config
		cfg.Root = filepath.Join(stand.base, fmt.Sprintf("source-state-%d", i))
		cfg.LocalRoleStateRoot = filepath.Join(stand.base, fmt.Sprintf("source-roles-%d", i))
		seed, err := state.Open(cfg)
		if err != nil {
			t.Fatal(err)
		}
		_, acceptErr := seed.Accept(ctx, stand.network.Epoch, stand.network.Inputs, stand.network.Materials)
		closeErr := seed.Close()
		if acceptErr != nil || closeErr != nil {
			t.Fatal(acceptErr, closeErr)
		}
		cfg.Source.ServeAddress = address
		cfg.Source.ServeCertificate = certificate
		cfg.Source.ServeClientRootPEM = clientPEM
		cfg.Source.ServeClientKeyDigests = [][32]byte{clientPin}
		server, err := state.Open(cfg)
		if err != nil {
			t.Fatal(err)
		}
		stand.sourceServers = append(stand.sourceServers, server)
	}
	t.Cleanup(func() {
		for _, server := range stand.sourceServers {
			if err := server.Close(); err != nil {
				t.Error(err)
			}
		}
	})

	// Genuine Release authority + enrolled protected candidate (version 1). The
	// provision command authenticates the initial pair and establishes floors.
	rootAuthority := newConsumerReleaseAuthority(t, time.Date(2031, 1, 2, 3, 4, 5, 0, time.UTC))
	resources := endpointPlatformNativeResources()
	// The worker executable must be a genuine compiled text worker, not fixture
	// placeholder bytes: socket activation spawns it under confinement.
	resources["ardents-text-linux-amd64"] = buildEndpointWorkerBinary(t)
	bundle, pin, initialProgram := signedConsumerRelease(t, "linux-amd64", true, true, 1, resources, rootAuthority, 1, 1)
	bundle, initialProgram = platformNativeDistribution(t, bundle, filepath.Join(stand.base, "initial-distribution"))
	// Provision establishes Release floors at metadata version 1.
	// Advance the signed metadata while retaining the exact staged release and descriptor.
	candidateBundle, _, _ := signedConsumerRelease(t, "linux-amd64", true, true, 1, resources, rootAuthority, 2, 1)
	stand.distribution, _ = platformNativeDistribution(t, candidateBundle, filepath.Join(stand.base, "candidate-distribution"))
	request := string(installationRequestBytes(t, bundle, pin, stand.floors))
	request = platformLedDualDeclaration(t, request, stand.base)
	quoted := func(s string) string {
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	for old, newPath := range map[string]string{
		"/installation": stand.installationRoot, "/state": plan.Root,
		"/entry": filepath.Join(stand.base, "entry"), "/roles": filepath.Join(stand.base, "roles"),
		"/tokens": filepath.Join(stand.base, "tokens"), "/clock": clock,
		"/socket/reader":          filepath.Join(stand.base, "application", "reader"),
		"/permissions/request":    filepath.Join(stand.base, "permission", "request"),
		"/permissions/response":   filepath.Join(stand.base, "permission", "response"),
		"/credentials/client.pem": certPath, "/credentials/client.key": keyPath,
		"/credentials/a.pem": filepath.Join(credentials, "a.pem"), "/credentials/b.pem": filepath.Join(credentials, "b.pem"),
	} {
		request = strings.ReplaceAll(request, quoted(old), quoted(newPath))
	}
	request = strings.ReplaceAll(request, strings.Repeat("02", 32), hex.EncodeToString(plan.NetworkID[:]))
	request = strings.ReplaceAll(request, strings.Repeat("ab", 32), hex.EncodeToString(authority[:]))
	request = strings.ReplaceAll(request, strings.Repeat("05", 32), hex.EncodeToString(stand.network.Seed[:]))
	for i, address := range addresses {
		request = strings.ReplaceAll(request, quoted([]string{"source-a.example:443", "source-b.example:443"}[i]), quoted(address))
		identity := sha256.Sum256([]byte([]string{"source-a.example", "source-b.example"}[i]))
		request = strings.ReplaceAll(request, strings.Repeat([]string{"06", "07"}[i], 32), hex.EncodeToString(identity[:]))
		request = strings.Replace(request, strings.Repeat("08", 32), hex.EncodeToString(sourcePins[i][:]), 1)
	}
	request = strings.Replace(request, "\"clock_observed_at\":\"2030-01-02T03:04:05Z\"", "\"clock_observed_at\":"+quoted(time.Now().UTC().Format(time.RFC3339)), 1)
	if strings.Count(request, `"refresh_interval_ms":1000`) != 1 {
		t.Fatal("platform-led launch fixture refresh interval changed")
	}
	request = strings.Replace(request, `"refresh_interval_ms":1000`, fmt.Sprintf(`"refresh_interval_ms":%d`, refreshMS), 1)
	// The request lives below /etc/ardents: provision reads it as a root-owned
	// canonical declaration, and the platform launch declaration binds it as its
	// Configuration (the platform check requires Configuration below /etc/ardents).
	stand.requestPath = filepath.Join(stand.etcDir, "request.json")
	launchWrite(t, stand.requestPath, []byte(request), 0600)
	stand.permissionDir = filepath.Join(stand.base, "permission")
	stand.publisherDir = filepath.Join(stand.base, "publisher-permission")
	stand.applicationSock = filepath.Join(stand.base, "application", "reader")
	stand.administrationSock = filepath.Join(stand.base, "administration", "socket")

	stand.uid, stand.gid = stagePlatformNativeEndpoint(t, stand.requestPath, initialProgram, pin, stand.distribution, keyPath)
	selection, err := os.ReadFile(filepath.Join(stand.installationRoot, "selection.json"))
	if err != nil {
		t.Fatal(err)
	}
	var selected struct {
		GenerationDigest string `json:"generation_digest"`
	}
	if err := json.Unmarshal(selection, &selected); err != nil || len(selected.GenerationDigest) != 64 {
		t.Fatalf("selection: %v: %s", err, selection)
	}
	stand.generationDigest = selected.GenerationDigest
	stand.program = filepath.Join(stand.installationRoot, "generations", selected.GenerationDigest, "ardents-linux-amd64")
	credential := &syscall.Credential{Uid: stand.uid, Gid: stand.gid, Groups: []uint32{}}

	// Genuine first histories through authenticated offline State acceptance.
	bootstrap := filepath.Join(stand.base, "bootstrap")
	if err := os.Mkdir(bootstrap, 0755); err != nil {
		t.Fatal(err)
	}
	var declaration struct{ Headless json.RawMessage }
	if err := json.Unmarshal([]byte(request), &declaration); err != nil {
		t.Fatal(err)
	}
	historyPlan := filepath.Join(bootstrap, "headless.json")
	launchWrite(t, historyPlan, declaration.Headless, 0644)
	bundleRaw, err := source.EncodeBundle(source.Bundle{Epoch: stand.network.Epoch, Inputs: stand.network.Inputs, Materials: stand.network.Materials[:1]})
	if err != nil {
		t.Fatal(err)
	}
	bundlePath, profilePath := filepath.Join(bootstrap, "state.bundle"), filepath.Join(bootstrap, "profile.bin")
	launchWrite(t, bundlePath, bundleRaw, 0644)
	launchWrite(t, profilePath, stand.network.Profile, 0644)
	prepare := exec.CommandContext(ctx, stand.program, "endpoint", "prepare-history", historyPlan, bundlePath, profilePath)
	prepare.SysProcAttr = &syscall.SysProcAttr{Credential: credential, Pdeathsig: syscall.SIGKILL}
	runtime.LockOSThread()
	prepared, prepareErr := prepare.CombinedOutput()
	runtime.UnlockOSThread()
	var preparation struct{ Phase, Outcome string }
	if prepareErr != nil || json.Unmarshal(prepared, &preparation) != nil || preparation.Phase != "history-preparation" || preparation.Outcome != "prepared" {
		t.Fatalf("explicit history preparation: %v: %s", prepareErr, prepared)
	}

	// Hosting provider budget is an explicit input, initialized under the account.
	period := time.Now().UTC().Truncate(time.Second)
	budgetInput, err := json.Marshal(map[string]any{
		"root": filepath.Join(stand.base, "entry", "hosting"),
		"policy": map[string]any{"provider": "platform-led-launch-fixture", "start": period.Add(-time.Minute),
			"end": period.Add(time.Hour), "unit": "GiB", "quantity": 16,
			"directions": "tx+rx", "interfaces": []string{"lo"},
			"initial_used_bytes": 0, "low_watermark_bytes": 1 << 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	budgetPath := filepath.Join(bootstrap, "hosting.json")
	launchWrite(t, budgetPath, budgetInput, 0644)
	launchCommand(t, stand.program, credential, "completed", "hosting", "initialize", "--config", budgetPath)

	stand.manifest, stand.pin = installPlatformNativeUnit(t, stand.requestPath, stand.distribution, stand.program)
	return stand
}

// platformLedDualDeclaration converts the reader fixture request to the dual
// publisher+reader role, preserving canonical field order and the newline.
func platformLedDualDeclaration(t *testing.T, raw, base string) string {
	t.Helper()
	quoted := func(value string) string {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}
	for _, change := range [][2]string{
		{`"role":"reader",`, ""},
		{`"maxima":[1,0,0]`, `"maxima":[32,32,0]`},
		{`"publisher_permission":{"request_path":"","response_path":"","maxima":[0,0,0]}`,
			`"publisher_permission":{"request_path":` + quoted(filepath.Join(base, "publisher-permission", "request")) + `,"response_path":` + quoted(filepath.Join(base, "publisher-permission", "response")) + `,"maxima":[32,32,8]}`},
		{`"administration_principal":""`, `"administration_principal":` + quoted(hex.EncodeToString(bytes.Repeat([]byte{9}, 32)))},
		{`"administration_socket":""`, `"administration_socket":` + quoted(filepath.Join(base, "administration", "socket"))},
		{`"publication_root":""`, `"publication_root":` + quoted(filepath.Join(base, "publication")) + `,"service_instance_root":` + quoted(filepath.Join(base, "instance"))},
	} {
		if strings.Count(raw, change[0]) != 1 {
			t.Fatal("platform-led declaration fixture changed")
		}
		raw = strings.Replace(raw, change[0], change[1], 1)
	}
	return raw
}

func (s *platformLedLaunchStand) startPlatformLed(t *testing.T) [3]string {
	t.Helper()
	if out, err := exec.Command("systemctl", "start", "ardents-endpoint.service").CombinedOutput(); err != nil {
		t.Fatalf("platform-led start: %v: %s", err, out)
	}
	original := launchIdentity(t)
	if original[0] == "0" || len(original[1]) != 32 || original[2] != "active" {
		t.Fatal("platform-led participant is not active", original)
	}
	return original
}

func (s *platformLedLaunchStand) grantPermissions(t *testing.T, original [3]string) {
	t.Helper()
	for _, role := range []string{"reader", "publisher"} {
		launchWait(t, func() ([]byte, bool) {
			raw, err := exec.CommandContext(t.Context(), "journalctl", "--no-pager", "-o", "cat", "_SYSTEMD_INVOCATION_ID="+original[1]).CombinedOutput()
			if err != nil {
				t.Fatal("original permission journal", err)
			}
			return raw, launchEvent(raw, "permission-pending", role)
		})
		directory := s.permissionDir
		if role == "publisher" {
			directory = s.publisherDir
		}
		request, err := os.ReadFile(filepath.Join(directory, "request"))
		if err != nil {
			t.Fatal(err)
		}
		response := s.approve(t, role, request)
		pending := filepath.Join(directory, "response.pending")
		launchWrite(t, pending, response, 0600)
		if err := os.Chown(pending, int(s.uid), int(s.gid)); err != nil {
			t.Fatal(err)
		}
		launchSync(t, pending)
		if _, err := os.Lstat(filepath.Join(directory, "response")); !os.IsNotExist(err) {
			t.Fatal("refuse existing permission response", err)
		}
		if err := os.Rename(pending, filepath.Join(directory, "response")); err != nil {
			t.Fatal(err)
		}
		launchSync(t, directory)
		launchWait(t, func() ([]byte, bool) {
			raw, err := exec.CommandContext(t.Context(), "journalctl", "--no-pager", "-o", "cat", "_SYSTEMD_INVOCATION_ID="+original[1]).CombinedOutput()
			if err != nil {
				t.Fatal(err)
			}
			return raw, launchEvent(raw, "permission-accepted", role)
		})
	}
}

func (s *platformLedLaunchStand) approve(t *testing.T, role string, request []byte) []byte {
	t.Helper()
	prepared, err := allocation.Prepare(request, s.f.profile.NetworkID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	decision, err := prepared.Decide(nil, s.f.profile.IssuanceAuthorityKey, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(s.base, "allocation-"+role)
	launchWrite(t, journal, decision.Journal(), 0600)
	readback, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(readback, decision.Journal()) {
		t.Fatal("allocation durable readback", err)
	}
	permission := decision.Permission()
	copy(permission.Signature[:], ed25519.Sign(s.f.spec.Authority, admission.PermissionTranscript(permission)))
	response, err := admission.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func (s *platformLedLaunchStand) waitServiceReady(t *testing.T, original [3]string) {
	t.Helper()
	launchWait(t, func() ([]byte, bool) {
		conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: s.applicationSock, Net: "unix"})
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
			return nil, false
		}
		if err != nil {
			t.Fatal("installed Reader listener", err)
		}
		defer conn.Close()
		raw, err := conn.SyscallConn()
		if err != nil {
			t.Fatal(err)
		}
		var peer *syscall.Ucred
		var peerErr error
		if err := raw.Control(func(fd uintptr) {
			peer, peerErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		}); err != nil || peerErr != nil {
			t.Fatal("installed listener peer", err, peerErr)
		}
		if peer == nil || strconv.FormatInt(int64(peer.Pid), 10) != original[0] || peer.Uid != s.uid || peer.Gid != s.gid {
			t.Fatal("Reader listener is not owned by the original installed process")
		}
		return nil, true
	})
	if current := launchIdentity(t); current != original {
		t.Fatal("Service readiness lost original invocation", current)
	}
}

func (s *platformLedLaunchStand) stopParticipant(t *testing.T, original [3]string) {
	t.Helper()
	if current := launchIdentity(t); current != original {
		t.Errorf("original installed invocation changed: %v", current)
		return
	}
	if out, err := exec.Command("systemctl", "stop", "ardents-endpoint.service").CombinedOutput(); err != nil {
		t.Errorf("original stop: %v: %s", err, out)
	}
}

// Check the manager's result before a reload or reset can discard the
// original exit observation. Stop job success alone does not establish it.
func (s *platformLedLaunchStand) requireJoinedParticipant(t *testing.T, original [3]string) {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "systemctl", "show", "ardents-endpoint.service",
		"--all", "--property=ActiveState,SubState,MainPID,ExecMainPID,ExecMainCode,ExecMainStatus,Result,Job").Output()
	if err != nil {
		t.Fatal("observe stopped Endpoint:", err)
	}
	properties := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("invalid stopped Endpoint property %q", line)
		}
		properties[name] = value
	}
	for name, want := range map[string]string{
		"ActiveState": "inactive", "SubState": "dead", "MainPID": "0", "ExecMainPID": original[0],
		"ExecMainCode": "1", "ExecMainStatus": "0", "Result": "success", "Job": "",
	} {
		got, ok := properties[name]
		if !ok || got != want {
			t.Fatalf("stopped Endpoint %s = %q, want %q", name, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join("/proc", original[0])); !os.IsNotExist(err) {
		t.Fatalf("original Endpoint PID %s still present or unobservable: %v", original[0], err)
	}
}

func (s *platformLedLaunchStand) cleanup(t *testing.T) {
	t.Helper()
	_ = exec.Command("systemctl", "stop", "ardents-endpoint.service").Run()
	_ = exec.Command("systemctl", "stop", "ardents-text-reader.socket", "ardents-text-publisher.socket").Run()
	_ = exec.Command("pkill", "-9", "-f", "ardents-linux-amd64").Run()
	_ = exec.Command("pkill", "-9", "-f", "ardents-text").Run()
	// Remove the main unit AND every fixed resource/managed path the delivery
	// staged, so a subsequent initial provision finds them absent (it never
	// adopts existing state). preflightInitialEffects checks each of these.
	for _, path := range []string{
		"/etc/systemd/system/ardents-endpoint.service",
		"/etc/systemd/system/ardents-text-reader@.service",
		"/etc/systemd/system/ardents-text-publisher@.service",
		"/etc/systemd/system/ardents-text-reader.socket",
		"/etc/systemd/system/ardents-text-publisher.socket",
		"/usr/share/polkit-1/rules.d/50-ardents-text.rules",
		"/usr/lib/tmpfiles.d/ardents-text.conf",
		"/etc/ardents/text-worker-artifact.json",
	} {
		_ = os.Remove(path)
	}
	_ = os.RemoveAll("/usr/lib/ardents/text-worker-root")
	_ = os.RemoveAll("/run/ardents-text")
	_ = exec.Command("systemctl", "daemon-reload").Run()
	_ = exec.Command("systemctl", "reset-failed", "ardents-endpoint.service").Run()
	// Socket activation spawns transient worker instances; reset every loaded
	// ardents-text unit so the next initial provision observes none loaded.
	_ = exec.Command("bash", "-c", "systemctl list-units --all 'ardents-text-*' --plain --no-legend | awk '{print $1}' | xargs -r systemctl reset-failed").Run()
	_ = exec.Command("userdel", "-f", "ardents-endpoint").Run()
	_ = exec.Command("groupdel", "ardents-endpoint").Run()
	_ = os.RemoveAll(s.etcDir)
	_ = os.RemoveAll(s.base)
}

func buildEndpointWorkerBinary(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ardents-text-linux-amd64")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", path, "github.com/dianabuilds/ardents-network/cmd/ardents-text")
	// The worker runs under RootDirectory chroot; it must be a static binary.
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build text worker: %v: %s", err, out)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func launchEvent(raw []byte, phase, role string) bool {
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		var event map[string]string
		if json.Unmarshal(line, &event) == nil && event["phase"] == phase && event["role"] == role {
			return true
		}
	}
	return false
}

func launchIdentity(t *testing.T) [3]string {
	t.Helper()
	var result [3]string
	for i, property := range []string{"MainPID", "InvocationID", "ActiveState"} {
		out, err := exec.Command("systemctl", "show", "ardents-endpoint.service", "-p", property, "--value").CombinedOutput()
		if err != nil {
			t.Fatalf("actual %s: %v: %s", property, err, out)
		}
		result[i] = strings.TrimSpace(string(out))
	}
	return result
}

func launchCommand(t *testing.T, program string, credential *syscall.Credential, outcome string, args ...string) []byte {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), program, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential, Pdeathsig: syscall.SIGKILL}
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	runtime.LockOSThread()
	out, err := cmd.Output()
	runtime.UnlockOSThread()
	var response struct{ Outcome string }
	if err != nil || json.Unmarshal(out, &response) != nil || response.Outcome != outcome {
		t.Fatalf("actual %v: %v: %s; diagnostics: %s", args, err, out, diagnostic.String())
	}
	return out
}

func launchWrite(t *testing.T, path string, raw []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		t.Fatal(err)
	}
	n, writeErr := file.Write(raw)
	modeErr := file.Chmod(mode)
	syncErr := file.Sync()
	closeErr := file.Close()
	if n != len(raw) || writeErr != nil || modeErr != nil || syncErr != nil || closeErr != nil {
		t.Fatal("durable fixture write", writeErr, modeErr, syncErr, closeErr)
	}
	launchSync(t, filepath.Dir(path))
}

func launchSync(t *testing.T, path string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	syncErr, closeErr := file.Sync(), file.Close()
	if syncErr != nil || closeErr != nil {
		t.Fatal(syncErr, closeErr)
	}
}

func launchWait(t *testing.T, observe func() ([]byte, bool)) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		raw, done := observe()
		if done {
			return raw
		}
		select {
		case <-ctx.Done():
			t.Fatal("actual installed observation exceeded bound", ctx.Err())
		case <-tick.C:
		}
	}
}
