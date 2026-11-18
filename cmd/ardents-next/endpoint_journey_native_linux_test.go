//go:build linux && endpoint_pair_native

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	endpointunit "github.com/dianabuilds/ardents-network/internal/successor/installation/unit"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// This native qualification proves the two-Endpoint platform-led text journey
// between TWO separately admitted Ubuntu/systemd managers (#391 S2.2), building
// on the S2.1 nspawn launch foundation. The controller keeps one shared fixture
// route network (Forwarding/Descriptor Store/Introduction/JOIN/Responder/Issuer
// receivers on the shared host loopback) and the signing authority; each guest
// independently provisions and platform-led-launches its own dual-role Endpoint
// (no root ACK), authenticates the public signed Network through its own State,
// and obtains separately approved permissions. The publisher guest publishes a
// snapshot through real REGISTER/Store ACK and exposes an accepting Link; the
// reader guest resolves the Descriptor, joins and reads the exact text through
// its actual AAI3 listener. Withdrawal retires the Link; a platform-led restart
// reaches Service readiness again under a new InvocationID with the same
// immutable selection and retained durable floors.
//
// It is component/acceptance evidence with fixture Release/Network authority for
// two independently admitted managers. It is NOT the whole-host two-participant
// privacy qualification. Descriptor automatic-refresh timing, QUIC pre-effect
// refusal and full containment observations are stated residuals (S2.3). The
// transitional guarded-start/ACK/recovery orchestration is neither exercised nor
// retired (ADR-0125 D6).

const (
	journeyRootEnv    = "ARDENTS_JOURNEY_ROOT"
	journeyRoleEnv    = "ARDENTS_JOURNEY_ROLE"
	journeyNonceEnv   = "ARDENTS_JOURNEY_NONCE"
	journeyAddressEnv = "ARDENTS_JOURNEY_ADDRESS"
)

type journeyParticipant struct {
	base     string
	original [3]string
	stop     func()
}

// TestNativeEndpointPlatformLedJourneyParticipant runs inside one guest manager.
// It receives only public signed Network material and actual permission
// requests/responses over the local fixture channel; no private key, live owner
// or accepting flag crosses managers.
func TestNativeEndpointPlatformLedJourneyParticipant(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("journey participant requires a root driver inside its own manager")
	}
	role := os.Getenv(journeyRoleEnv)
	if role != "publisher" && role != "reader" {
		t.Fatal("invalid environment: selected journey role required")
	}
	base := os.Getenv(journeyRootEnv)
	if !filepath.IsAbs(base) {
		t.Fatal("invalid environment: absolute journey root required")
	}
	nonce := os.Getenv(journeyNonceEnv)
	if decoded, err := hex.DecodeString(nonce); err != nil || len(decoded) != 32 {
		t.Fatal("invalid environment: original fixture nonce required")
	}
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", os.Getenv(journeyAddressEnv))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(6 * time.Minute))
	installedPairSend(t, conn, installedPairMessage{Operation: "hello", Role: role, Nonce: nonce})
	input := installedPairReceive(t, conn)
	if input.Operation != "network" || input.Network == nil {
		t.Fatal("journey fixture lacks signed Network input")
	}
	etcDir := filepath.Join("/etc/ardents", "journey-"+nonce)
	approve := func(_, permissionRole string, request []byte) []byte {
		installedPairSend(t, conn, installedPairMessage{Operation: "permission", Role: permissionRole, Body: request})
		response := installedPairReceive(t, conn)
		if response.Operation != "permission" || response.Role != permissionRole {
			t.Fatal("foreign fixture approval response")
		}
		return response.Body
	}
	exercise := func(participant journeyParticipant) {
		original := participant.original
		installedPairSend(t, conn, installedPairMessage{Operation: "ready", Original: original})
		for {
			command := installedPairReceive(t, conn)
			if command.Operation != "restart" && installedStartupIdentity(t) != original {
				t.Fatal("journey command lost original installed participant")
			}
			response := installedPairMessage{Operation: command.Operation, Original: original}
			switch command.Operation {
			case "publish":
				if role != "publisher" {
					t.Fatal("reader received publication fixture command")
				}
				ctx, cancel := context.WithTimeout(t.Context(), administration.SnapshotTimeout)
				_, err := administration.RequestSnapshot(ctx, filepath.Join(participant.base, "administration", "socket"), command.Body)
				cancel()
				if err != nil {
					journeyDumpJournal(t)
					t.Fatal("journey installed Snapshot", err)
				}
				ctx, cancel = context.WithTimeout(t.Context(), 15*time.Second)
				response.Link, err = administration.RequestPublishedLink(ctx, filepath.Join(participant.base, "administration", "socket"))
				cancel()
				if err != nil {
					t.Fatal("journey installed Link", err)
				}
			case "read":
				if role != "reader" {
					t.Fatal("publisher received reader fixture command")
				}
				ctx, cancel := context.WithTimeout(t.Context(), application.ReaderTimeout)
				stream, err := application.Dial(ctx, filepath.Join(participant.base, "application", "reader"), application.Request{Destination: application.TargetLink, Value: command.Link})
				if err == nil {
					response.Body, err = textdocument.Read(ctx, stream)
				}
				cancel()
				if err != nil {
					t.Fatal("journey installed reader exchange", err)
				}
			case "withdraw":
				if role != "publisher" {
					t.Fatal("reader received withdrawal fixture command")
				}
				ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
				_, err := administration.Request(ctx, filepath.Join(participant.base, "administration", "socket"), administration.Withdraw)
				cancel()
				if err != nil {
					t.Fatal("journey installed withdrawal", err)
				}
				ctx, cancel = context.WithTimeout(t.Context(), 15*time.Second)
				link, err := administration.RequestPublishedLink(ctx, filepath.Join(participant.base, "administration", "socket"))
				cancel()
				if err == nil || link != "" {
					t.Fatal("withdrawn installed publisher still exposes a Link")
				}
			case "restart":
				original = journeyRestartContinuity(t, participant, original)
				response.Original = original
				installedPairSend(t, conn, response)
				return
			case "finish":
				installedPairSend(t, conn, response)
				return
			default:
				t.Fatal("unknown journey fixture command")
			}
			installedPairSend(t, conn, response)
		}
	}
	runPlatformLedJourneyParticipant(t, base, etcDir, *input.Network, approve, exercise)
}

// runPlatformLedJourneyParticipant provisions and platform-led-launches one
// dual-role Endpoint against the shared signed Network, then exercises the
// fixture commands. It reuses the justified delivery staging (installed-stopped,
// no start-guard.json) and the S1 platform launch (platform main unit + canonical
// workers, systemd start without root ACK, F-25 pre-start transfer).
func runPlatformLedJourneyParticipant(t *testing.T, base, etcDir string, network installedStartupNetwork,
	approve func(base, role string, request []byte) []byte, exercise func(journeyParticipant)) {
	t.Helper()
	ctx := t.Context()
	for _, dir := range []string{base, etcDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	clock := filepath.Join(base, "clock")
	installedStartupWrite(t, clock, nil, 0644)
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
	authority := network.Authority
	plan := &networkAuthorityPlan{Root: filepath.Join(base, "state"), NetworkID: network.NetworkID,
		Authorities: [][32]byte{authority}, Threshold: 1, ProfileAuthority: authority, ClockObservationFile: clock}
	config, err := networkStateConfig(plan)
	if err != nil {
		t.Fatal(err)
	}
	_, clientPEM, clientKey, clientPin := installedSourceCertificate(t, "client", false)
	certPath, keyPath := filepath.Join(etcDir, "client.crt"), filepath.Join(etcDir, "client.key")
	installedStartupWrite(t, certPath, clientPEM, 0644)
	installedStartupWrite(t, keyPath, clientKey, 0600)
	clear(clientKey)
	credentials := filepath.Join(base, "credentials")
	if err := os.MkdirAll(credentials, 0755); err != nil {
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
	var sourceServers []interface{ Close() error }
	t.Cleanup(func() {
		for _, server := range sourceServers {
			if err := server.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	for i, address := range addresses {
		name := []string{"source-a.example", "source-b.example"}[i]
		certificate, rootPEM, keyPEM, pin := installedSourceCertificate(t, name, true)
		clear(keyPEM)
		sourcePins[i] = pin
		installedStartupWrite(t, filepath.Join(credentials, []string{"a.pem", "b.pem"}[i]), rootPEM, 0644)
		cfg := config
		cfg.Root = filepath.Join(base, fmt.Sprintf("source-state-%d", i))
		cfg.LocalRoleStateRoot = filepath.Join(base, fmt.Sprintf("source-roles-%d", i))
		seed, err := state.Open(cfg)
		if err != nil {
			t.Fatal(err)
		}
		_, acceptErr := seed.Accept(ctx, network.Epoch, network.Inputs, network.Materials)
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
		sourceServers = append(sourceServers, server)
	}
	rootAuthority := newConsumerReleaseAuthority(t, time.Date(2031, 1, 2, 3, 4, 5, 0, time.UTC))
	resources := endpointPlatformNativeResources()
	resources["ardents-text-linux-amd64"] = journeyBuildWorkerBinary(t)
	bundle, pin, initialProgram := signedConsumerRelease(t, "linux-amd64", true, true, 1, resources, rootAuthority, 1, 1)
	candidateBundle, _, _ := signedConsumerRelease(t, "linux-amd64", true, true, 2, resources, rootAuthority, 2, 2)
	request := string(installationRequestBytes(t, bundle, pin, filepath.Join(base, "floors")))
	request = installedServiceDeclaration(t, request, base)
	if strings.Count(request, `"refresh_interval_ms":1000`) != 1 {
		t.Fatal("original Source fixture cadence changed")
	}
	request = strings.Replace(request, `"refresh_interval_ms":1000`, `"refresh_interval_ms":60000`, 1)
	quoted := func(s string) string {
		raw, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	for old, newPath := range map[string]string{
		"/installation": filepath.Join(base, "installation"), "/state": plan.Root,
		"/entry": filepath.Join(base, "entry"), "/roles": filepath.Join(base, "roles"),
		"/tokens": filepath.Join(base, "tokens"), "/clock": clock,
		"/socket/reader":          filepath.Join(base, "application", "reader"),
		"/permissions/request":    filepath.Join(base, "permission", "request"),
		"/permissions/response":   filepath.Join(base, "permission", "response"),
		"/credentials/client.pem": certPath, "/credentials/client.key": keyPath,
		"/credentials/a.pem": filepath.Join(credentials, "a.pem"), "/credentials/b.pem": filepath.Join(credentials, "b.pem"),
	} {
		request = strings.ReplaceAll(request, quoted(old), quoted(newPath))
	}
	request = strings.ReplaceAll(request, strings.Repeat("02", 32), hex.EncodeToString(plan.NetworkID[:]))
	request = strings.ReplaceAll(request, strings.Repeat("ab", 32), hex.EncodeToString(authority[:]))
	request = strings.ReplaceAll(request, strings.Repeat("05", 32), hex.EncodeToString(network.Seed[:]))
	for i, address := range addresses {
		request = strings.ReplaceAll(request, quoted([]string{"source-a.example:443", "source-b.example:443"}[i]), quoted(address))
		identity := sha256.Sum256([]byte([]string{"source-a.example", "source-b.example"}[i]))
		request = strings.ReplaceAll(request, strings.Repeat([]string{"06", "07"}[i], 32), hex.EncodeToString(identity[:]))
		request = strings.Replace(request, strings.Repeat("08", 32), hex.EncodeToString(sourcePins[i][:]), 1)
	}
	request = strings.Replace(request, "\"clock_observed_at\":\"2030-01-02T03:04:05Z\"", "\"clock_observed_at\":"+quoted(time.Now().UTC().Format(time.RFC3339)), 1)
	requestPath := filepath.Join(etcDir, "request.json")
	installedStartupWrite(t, requestPath, []byte(request), 0600)
	installedStartupCommand(t, initialProgram, nil, "installed-stopped", "installation", "provision", "--request", requestPath)
	account, err := user.Lookup("ardents-endpoint")
	if err != nil {
		t.Fatal(err)
	}
	uid, err := strconv.ParseUint(account.Uid, 10, 32)
	if err != nil || uid == 0 {
		t.Fatal("invalid installed UID", err)
	}
	gid, err := strconv.ParseUint(account.Gid, 10, 32)
	if err != nil || gid == 0 {
		t.Fatal("invalid installed GID", err)
	}
	if err := os.Chown(keyPath, int(uid), int(gid)); err != nil {
		t.Fatal(err)
	}
	installationRoot := filepath.Join(base, "installation")
	selection, err := os.ReadFile(filepath.Join(installationRoot, "selection.json"))
	if err != nil {
		t.Fatal(err)
	}
	var selected struct {
		GenerationDigest string `json:"generation_digest"`
	}
	if err := json.Unmarshal(selection, &selected); err != nil || len(selected.GenerationDigest) != 64 {
		t.Fatalf("selection: %v: %s", err, selection)
	}
	program := filepath.Join(installationRoot, "generations", selected.GenerationDigest, "ardents-linux-amd64")
	credential := &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: []uint32{}}
	bootstrap := filepath.Join(base, "bootstrap")
	if err := os.MkdirAll(bootstrap, 0755); err != nil {
		t.Fatal(err)
	}
	var declaration struct{ Headless json.RawMessage }
	if err := json.Unmarshal([]byte(request), &declaration); err != nil {
		t.Fatal(err)
	}
	historyPlan := filepath.Join(bootstrap, "headless.json")
	installedStartupWrite(t, historyPlan, declaration.Headless, 0644)
	bundleRaw, err := source.EncodeBundle(source.Bundle{Epoch: network.Epoch, Inputs: network.Inputs, Materials: network.Materials[:1]})
	if err != nil {
		t.Fatal(err)
	}
	bundlePath, profilePath := filepath.Join(bootstrap, "state.bundle"), filepath.Join(bootstrap, "profile.bin")
	installedStartupWrite(t, bundlePath, bundleRaw, 0644)
	installedStartupWrite(t, profilePath, network.Profile, 0644)
	prepare := exec.CommandContext(ctx, program, "endpoint", "prepare-history", historyPlan, bundlePath, profilePath)
	prepare.SysProcAttr = &syscall.SysProcAttr{Credential: credential, Pdeathsig: syscall.SIGKILL}
	runtime.LockOSThread()
	prepared, prepareErr := prepare.CombinedOutput()
	runtime.UnlockOSThread()
	var preparation struct{ Phase, Outcome string }
	if prepareErr != nil || json.Unmarshal(prepared, &preparation) != nil || preparation.Phase != "history-preparation" || preparation.Outcome != "prepared" {
		t.Fatalf("explicit history preparation: %v: %s", prepareErr, prepared)
	}
	period := time.Now().UTC().Truncate(time.Second)
	budgetInput, err := json.Marshal(map[string]any{
		"root": filepath.Join(base, "entry", "hosting"),
		"policy": map[string]any{"provider": "platform-led-journey-fixture", "start": period.Add(-time.Minute),
			"end": period.Add(time.Hour), "unit": "GiB", "quantity": 16,
			"directions": "tx+rx", "interfaces": []string{"lo"},
			"initial_used_bytes": 0, "low_watermark_bytes": 1 << 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	budgetPath := filepath.Join(bootstrap, "hosting.json")
	installedStartupWrite(t, budgetPath, budgetInput, 0644)
	installedStartupCommand(t, program, credential, "completed", "hosting", "initialize", "--config", budgetPath)
	journeyInstallPlatformUnit(t, etcDir, requestPath, program, installationRoot, candidateBundle, filepath.Join(base, "floors"))
	original := journeyStartPlatformLed(t)
	installedServiceInstance(t, program, credential, base, network.NetworkID)
	journeyGrantPermissions(t, base, original, uid, gid, approve)
	if current := installedStartupIdentity(t); current != original {
		t.Fatal("accepted permission lost original invocation", current)
	}
	journeyWaitServiceReady(t, base, original, uint32(uid), uint32(gid))
	stop := func() {
		if current := installedStartupIdentity(t); current[2] == "active" {
			if out, err := exec.Command("systemctl", "stop", "ardents-endpoint.service").CombinedOutput(); err != nil {
				t.Errorf("journey stop: %v: %s", err, out)
			}
		}
	}
	t.Cleanup(stop)
	t.Logf("guest platform-led Endpoint reached Service readiness; MainPID=%s InvocationID=%s", original[0], original[1])
	exercise(journeyParticipant{base: base, original: original, stop: stop})
}

func journeyBuildWorkerBinary(t *testing.T) []byte {
	t.Helper()
	if prebuilt := os.Getenv("ARDENTS_PREBUILT_TEXT"); prebuilt != "" {
		raw, err := os.ReadFile(prebuilt)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	path := filepath.Join(t.TempDir(), "ardents-text-linux-amd64")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", path, "github.com/dianabuilds/ardents-network/cmd/ardents-text")
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

func journeyInstallPlatformUnit(t *testing.T, etcDir, requestPath, program, installationRoot, distribution, floors string) {
	t.Helper()
	requestRaw, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatal(err)
	}
	decl, err := platformEndpointDeclaration(requestRaw)
	if err != nil {
		t.Fatal(err)
	}
	roots, err := platformEndpointRoots(decl)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range roots {
		if err := os.MkdirAll(root, 0755); err != nil {
			t.Fatal(err)
		}
	}
	certBytes, err := os.ReadFile(filepath.Join(etcDir, "client.crt"))
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err := os.ReadFile(filepath.Join(etcDir, "client.key"))
	if err != nil {
		t.Fatal(err)
	}
	certDigest := sha256.Sum256(certBytes)
	keyDigest := sha256.Sum256(keyBytes)
	configDigest := sha256.Sum256(requestRaw)
	launch := platformLaunch{
		Role: "endpoint", Distribution: distribution, ReleaseRoot: floors,
		Environment: "h3-test", Network: "new-release-test", Program: program,
		Configuration: requestPath, ConfigSHA256: hex.EncodeToString(configDigest[:]),
		WritableRoots: roots,
		Inputs: map[string]string{
			filepath.Join(etcDir, "client.crt"): hex.EncodeToString(certDigest[:]),
			filepath.Join(etcDir, "client.key"): hex.EncodeToString(keyDigest[:]),
		},
	}
	launchRaw, err := json.Marshal(launch)
	if err != nil {
		t.Fatal(err)
	}
	launchRaw = append(launchRaw, '\n')
	launchDigest := sha256.Sum256(launchRaw)
	pin := hex.EncodeToString(launchDigest[:])
	manifest := filepath.Join(etcDir, "launch.json")
	installedStartupWrite(t, manifest, launchRaw, 0644)
	candidate, err := enrollment.ReadCandidate(t.Context(), distribution, enrollment.Headless)
	if err != nil {
		t.Fatal(err)
	}
	template, ok := candidate.File("ardents-endpoint.service")
	if !ok {
		t.Fatal("enrolled endpoint template absent")
	}
	prepared, err := endpointunit.PrepareEndpointTemplate(template, installationRoot, filepath.Dir(program))
	if err != nil {
		t.Fatal(err)
	}
	base, err := prepared.Render(roots)
	if err != nil {
		t.Fatal(err)
	}
	checkProgram := filepath.Join(distribution, "ardents-linux-amd64")
	unit, err := renderEndpointPlatformUnit(base, checkProgram, manifest, pin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("/etc/systemd/system/ardents-endpoint.service", unit, 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		t.Fatal("daemon-reload:", err)
	}
}

func journeyStartPlatformLed(t *testing.T) [3]string {
	t.Helper()
	if out, err := exec.Command("systemctl", "start", "ardents-endpoint.service").CombinedOutput(); err != nil {
		t.Fatalf("platform-led start: %v: %s", err, out)
	}
	original := installedStartupIdentity(t)
	if original[0] == "0" || len(original[1]) != 32 || original[2] != "active" {
		t.Fatal("platform-led participant is not active", original)
	}
	return original
}

func journeyWaitServiceReady(t *testing.T, base string, original [3]string, uid, gid uint32) {
	t.Helper()
	journeyWait(t, 60*time.Second, func() bool {
		conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: filepath.Join(base, "application", "reader"), Net: "unix"})
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
			return false
		}
		if err != nil {
			t.Fatal("installed reader listener", err)
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
		if peer == nil || strconv.FormatInt(int64(peer.Pid), 10) != original[0] || peer.Uid != uid || peer.Gid != gid {
			t.Fatal("reader listener is not owned by the original installed process")
		}
		return true
	})
	if current := installedStartupIdentity(t); current != original {
		t.Fatal("Service readiness lost original invocation", current)
	}
}

// journeyRestartContinuity proves a platform-led restart reaches Service
// readiness again under a new InvocationID with the same immutable selection and
// retained durable floors (the platform-led replacement for the transitional
// pair's restart-refusal). It returns the new original invocation.
func journeyRestartContinuity(t *testing.T, participant journeyParticipant, previous [3]string) [3]string {
	t.Helper()
	selectionPath := filepath.Join(participant.base, "installation", "selection.json")
	before, err := os.ReadFile(selectionPath)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("systemctl", "stop", "ardents-endpoint.service").CombinedOutput(); err != nil {
		t.Fatalf("restart stop: %v: %s", err, out)
	}
	journeyWait(t, 30*time.Second, func() bool { return installedStartupIdentity(t)[2] != "active" })
	if out, err := exec.Command("systemctl", "start", "ardents-endpoint.service").CombinedOutput(); err != nil {
		t.Fatalf("restart start: %v: %s", err, out)
	}
	current := installedStartupIdentity(t)
	if current[0] == "0" || len(current[1]) != 32 || current[2] != "active" {
		t.Fatal("restarted participant is not active", current)
	}
	if current[1] == previous[1] {
		t.Fatal("platform-led restart reused the original invocation", current)
	}
	after, err := os.ReadFile(selectionPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("platform-led restart changed the immutable selection", err)
	}
	journeyWaitServiceReady(t, participant.base, current, participantUID(t), participantGID(t))
	t.Logf("platform-led restart reached Service readiness; new InvocationID=%s, selection retained", current[1])
	return current
}

func participantUID(t *testing.T) uint32 {
	t.Helper()
	account, err := user.Lookup("ardents-endpoint")
	if err != nil {
		t.Fatal(err)
	}
	uid, err := strconv.ParseUint(account.Uid, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	return uint32(uid)
}

func participantGID(t *testing.T) uint32 {
	t.Helper()
	account, err := user.Lookup("ardents-endpoint")
	if err != nil {
		t.Fatal(err)
	}
	gid, err := strconv.ParseUint(account.Gid, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	return uint32(gid)
}

func journeyWait(t *testing.T, bound time.Duration, ready func() bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), bound)
	defer cancel()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if ready() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("journey observation exceeded bound:", ctx.Err())
		case <-tick.C:
		}
	}
}

// TestNativeEndpointPlatformLedJourney is the outer controller. It provisions two
// separately admitted managers (S2.1 nspawn foundation), keeps the shared fixture
// route network and signing authority, and coordinates the publish/link/read/
// withdraw/restart journey over the local fixture channel.
func TestNativeEndpointPlatformLedJourney(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("endpoint platform-led journey qualification requires a root driver")
	}
	for _, tool := range []string{"systemd-nspawn", "machinectl", "debootstrap", "cp", "systemd-run"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("invalid environment: %s unavailable: %v", tool, err)
		}
	}
	if out, err := exec.Command("stat", "-fc", "%T", "/sys/fs/cgroup").Output(); err != nil || strings.TrimSpace(string(out)) != "cgroup2fs" {
		t.Fatalf("invalid environment: host cgroup v2 required: %v: %s", err, out)
	}
	repoRoot := pairRepoRoot(t)
	goRoot := pairGoEnv(t, "GOROOT")
	modCache := pairGoEnv(t, "GOMODCACHE")
	buildCache := pairGoEnv(t, "GOCACHE")
	base := pairEnsureBaseRootfs(t)
	carrier := string(transport.ClosedCarrierTCP)
	f, _ := installedServiceNetwork(t, carrier)
	network := installedSignedNetwork(t, f)
	guestBinary := journeyCompileParticipant(t, repoRoot)
	// Prebuild the canonical consumer and static worker on the host so the guests
	// do not repeat Go builds under shared-host load; the publication deadlines
	// are finite and must not be weakened to absorb build contention.
	nextBinary, textBinary := journeyPrebuildBinaries(t, repoRoot)
	runID := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	var guests [2]*pairGuest
	for i, label := range []string{"pub", "read"} {
		guests[i] = pairBootGuest(t, base, runID, label, goRoot, repoRoot, modCache, buildCache)
	}
	pairVerifyDistinctManagers(t, guests[:])
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	stopListener := context.AfterFunc(t.Context(), func() { _ = listener.Close() })
	defer stopListener()
	var peers [2]net.Conn
	var commands [2]*exec.Cmd
	var outputs [2]bytes.Buffer
	var joined [2]bool
	t.Cleanup(func() {
		for _, peer := range peers {
			if peer != nil {
				_ = peer.Close()
			}
		}
		for i, command := range commands {
			if command != nil && !joined[i] {
				if err := command.Wait(); err != nil {
					t.Errorf("journey guest cleanup: %v\n%s", err, outputs[i].String())
				}
			}
		}
	})
	exchange := func(index int, command installedPairMessage) installedPairMessage {
		started := time.Now()
		t.Logf("journey participant %d: %s started", index, command.Operation)
		defer func() {
			t.Logf("journey participant %d: %s returned after %s", index, command.Operation, time.Since(started))
		}()
		installedPairSend(t, peers[index], command)
		response := installedPairReceive(t, peers[index])
		if response.Operation != command.Operation {
			t.Fatal("journey handoff lost operation")
		}
		return response
	}
	// Start and admit one participant at a time so the publication issuance runs
	// with the host's full resources: the finite blind-token batch deadlines are
	// product bounds and must not be weakened to absorb concurrent-guest load.
	startAdmit := func(i int, role string) [3]string {
		var nonce [32]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			t.Fatal(err)
		}
		guest := guests[i]
		journeyCopyIntoGuest(t, guestBinary, filepath.Join(guest.rootfs, "root", "journey.test"))
		journeyCopyIntoGuest(t, nextBinary, filepath.Join(guest.rootfs, "root", "ardents-next"))
		journeyCopyIntoGuest(t, textBinary, filepath.Join(guest.rootfs, "root", "ardents-text"))
		// Keep the mutable root short: the application/administration Unix socket
		// paths below it must fit the kernel sun_path bound (108 bytes).
		guestRoot := filepath.Join("/var/lib/ardents", "journey-"+hex.EncodeToString(nonce[:6]))
		script := fmt.Sprintf("cd %s/cmd/ardents-next && /root/journey.test -test.run='^TestNativeEndpointPlatformLedJourneyParticipant$' -test.v -test.timeout=6m", repoRoot)
		command := exec.Command("systemd-run", "-M", guest.name, "--pipe", "--quiet",
			"--setenv=PATH=/usr/local/go/bin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"--setenv=GOTOOLCHAIN=local",
			"--setenv=GOMODCACHE="+modCache,
			"--setenv=GOCACHE="+buildCache,
			"--setenv=GOFLAGS=-buildvcs=false",
			"--setenv=ARDENTS_PREBUILT_NEXT=/root/ardents-next",
			"--setenv=ARDENTS_PREBUILT_TEXT=/root/ardents-text",
			"--setenv="+journeyRootEnv+"="+guestRoot,
			"--setenv="+journeyRoleEnv+"="+role,
			"--setenv="+journeyNonceEnv+"="+hex.EncodeToString(nonce[:]),
			"--setenv="+journeyAddressEnv+"="+listener.Addr().String(),
			"/bin/bash", "-c", script)
		command.Stdout, command.Stderr = &outputs[i], &outputs[i]
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		commands[i] = command
		if tcp, ok := listener.(*net.TCPListener); ok {
			_ = tcp.SetDeadline(time.Now().Add(90 * time.Second))
		}
		conn, err := listener.Accept()
		if err != nil {
			t.Fatalf("journey participant %s did not connect: %v\n%s", role, err, outputs[i].String())
		}
		peers[i] = conn
		_ = conn.SetDeadline(time.Now().Add(6 * time.Minute))
		hello := installedPairReceive(t, conn)
		if hello.Operation != "hello" || hello.Role != role || hello.Nonce != hex.EncodeToString(nonce[:]) {
			t.Fatal("foreign journey fixture client")
		}
		installedPairSend(t, conn, installedPairMessage{Operation: "network", Network: &network})
		approvalRoot := filepath.Join(t.TempDir(), role)
		if err := os.MkdirAll(approvalRoot, 0700); err != nil {
			t.Fatal(err)
		}
		approved := make(map[string]bool)
		for {
			request := installedPairReceive(t, conn)
			if request.Operation == "ready" {
				if !approved["reader"] || !approved["publisher"] || request.Original[1] == "" {
					t.Fatal("participant omitted independent admission")
				}
				return request.Original
			}
			if request.Operation != "permission" || (request.Role != "reader" && request.Role != "publisher") || approved[request.Role] {
				t.Fatal("unexpected journey approval request")
			}
			response := installedServiceApproval(t, approvalRoot, request.Role, request.Body, f)
			approved[request.Role] = true
			installedPairSend(t, conn, installedPairMessage{Operation: "permission", Role: request.Role, Body: response})
		}
	}
	body := []byte("one platform-led request across independently admitted managers\n")
	identities := [2][3]string{}
	identities[0] = startAdmit(0, "publisher")
	publication := exchange(0, installedPairMessage{Operation: "publish", Body: body})
	if publication.Link == "" {
		t.Fatal("installed publisher supplied no Link")
	}
	identities[1] = startAdmit(1, "reader")
	if identities[0][1] == identities[1][1] {
		t.Fatal("participants share a manager invocation")
	}
	read := exchange(1, installedPairMessage{Operation: "read", Link: publication.Link})
	if !bytes.Equal(read.Body, body) {
		t.Fatal("installed reader returned different Application bytes")
	}
	exchange(0, installedPairMessage{Operation: "withdraw"})
	for i := range peers {
		response := exchange(i, installedPairMessage{Operation: "restart"})
		if response.Original[1] == identities[i][1] {
			t.Fatal("platform-led restart reused the original invocation")
		}
		_ = peers[i].Close()
		peers[i] = nil
		waitErr := commands[i].Wait()
		joined[i] = true
		t.Log(outputs[i].String())
		if waitErr != nil {
			t.Fatal("journey participant did not join", waitErr)
		}
	}
	t.Log("two independent platform-led admissions, one actual AAI3 text exchange, exact response, owning withdrawal and restart continuity completed")
}

// journeyGrantPermissions mirrors the installed dual-role permission bootstrap
// with a longer observation bound: two guests launch concurrently on the shared
// host, so the finite startup observation must tolerate the accumulated load.
func journeyGrantPermissions(t *testing.T, base string, original [3]string, uid, gid uint64, approve func(base, role string, request []byte) []byte) {
	t.Helper()
	for _, role := range []string{"reader", "publisher"} {
		journeyWait(t, 60*time.Second, func() bool {
			raw, err := exec.CommandContext(t.Context(), "journalctl", "--no-pager", "-o", "cat", "_SYSTEMD_INVOCATION_ID="+original[1]).CombinedOutput()
			if err != nil {
				t.Fatal("original permission journal", err)
			}
			return installedServiceEvent(raw, "permission-pending", role)
		})
		directory := filepath.Join(base, "permission")
		if role == "publisher" {
			directory = filepath.Join(base, "publisher-permission")
		}
		request, err := os.ReadFile(filepath.Join(directory, "request"))
		if err != nil {
			t.Fatal(err)
		}
		response := approve(base, role, request)
		pending := filepath.Join(directory, "response.pending")
		installedStartupWrite(t, pending, response, 0600)
		if err := os.Chown(pending, int(uid), int(gid)); err != nil {
			t.Fatal(err)
		}
		installedStartupSync(t, pending)
		if _, err := os.Lstat(filepath.Join(directory, "response")); !os.IsNotExist(err) {
			t.Fatal("refuse existing permission response", err)
		}
		if err := os.Rename(pending, filepath.Join(directory, "response")); err != nil {
			t.Fatal(err)
		}
		installedStartupSync(t, directory)
		journeyWait(t, 60*time.Second, func() bool {
			raw, err := exec.CommandContext(t.Context(), "journalctl", "--no-pager", "-o", "cat", "_SYSTEMD_INVOCATION_ID="+original[1]).CombinedOutput()
			if err != nil {
				t.Fatal(err)
			}
			return installedServiceEvent(raw, "permission-accepted", role)
		})
	}
}

func journeyDumpJournal(t *testing.T) {
	t.Helper()
	journal, _ := exec.Command("journalctl", "--no-pager", "-u", "ardents-endpoint.service", "-n", "80", "-o", "cat").CombinedOutput()
	t.Logf("participant journal:\n%s", journal)
	workers, _ := exec.Command("bash", "-c", "journalctl --no-pager -o cat _SYSTEMD_UNIT='ardents-text-*' -n 40 2>/dev/null; systemctl list-units --all 'ardents-text-*' --plain --no-legend").CombinedOutput()
	t.Logf("worker journal/units:\n%s", workers)
}

func journeyCopyIntoGuest(t *testing.T, source, destination string) {
	t.Helper()
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, raw, 0555); err != nil {
		t.Fatal(err)
	}
}

func journeyPrebuildBinaries(t *testing.T, repoRoot string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	next := filepath.Join(dir, "ardents-next")
	build := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", next, "./cmd/ardents-next")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("prebuild consumer: %v: %s", err, out)
	}
	text := filepath.Join(dir, "ardents-text")
	worker := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", text, "./cmd/ardents-text")
	worker.Dir = repoRoot
	worker.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := worker.CombinedOutput(); err != nil {
		t.Fatalf("prebuild worker: %v: %s", err, out)
	}
	return next, text
}

func journeyCompileParticipant(t *testing.T, repoRoot string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "journey.test")
	cmd := exec.CommandContext(t.Context(), "go", "test", "-c", "-tags=endpoint_pair_native,text_worker_installed",
		"-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", path, "./cmd/ardents-next")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile journey participant: %v: %s", err, out)
	}
	return path
}
