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
	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/publication"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
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
// privacy qualification. The cross-host controller also checks Descriptor
// refresh. QUIC pre-effect refusal and full containment remain separate. The
// transitional guarded-start/ACK/recovery orchestration is neither exercised nor
// retired (ADR-0125 D6).

const (
	journeyRootEnv    = "ARDENTS_JOURNEY_ROOT"
	journeyRoleEnv    = "ARDENTS_JOURNEY_ROLE"
	journeyNonceEnv   = "ARDENTS_JOURNEY_NONCE"
	journeyAddressEnv = "ARDENTS_JOURNEY_ADDRESS"
)

type journeyParticipant struct {
	base       string
	original   [3]string
	principals [3]string
	stop       func()
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
	deadline := time.Now().Add(6 * time.Minute)
	if testDeadline, ok := t.Deadline(); ok {
		deadline = testDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		t.Fatal("bound journey participant channel", err)
	}
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
		var publisherProcess *exec.Cmd
		var publisherOutput, publisherDiagnostic bytes.Buffer
		var publisherDone chan error
		defer func() {
			if publisherProcess != nil {
				_ = publisherProcess.Process.Kill()
				<-publisherDone
			}
		}()
		installedPairSend(t, conn, installedPairMessage{Operation: "ready", Original: original, Principals: participant.principals})
		for {
			command := installedPairReceive(t, conn)
			if command.Operation != "restart" && command.Operation != "stream-result" && installedStartupIdentity(t) != original {
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
					response.Body, err = document.Read(ctx, stream)
				}
				cancel()
				if err != nil {
					t.Fatal("journey installed reader exchange", err)
				}
			case "stream-publish":
				if role != "publisher" || publisherProcess != nil {
					t.Fatal("invalid byte-stream publication command")
				}
				socket := filepath.Join(participant.base, "administration", "socket")
				process := exec.CommandContext(t.Context(), os.Getenv("ARDENTS_PREBUILT_NEXT"), "stream", "publish", socket)
				process.Stdin = bytes.NewReader(command.Body)
				process.Stdout, process.Stderr = &publisherOutput, &publisherDiagnostic
				if err := process.Start(); err != nil {
					t.Fatal(err)
				}
				publisherProcess = process
				publisherDone = make(chan error, 1)
				go func(done chan<- error) { done <- process.Wait() }(publisherDone)
				journeyWait(t, 55*time.Second, func() bool {
					select {
					case err := <-publisherDone:
						publisherProcess = nil
						t.Fatalf("stream publisher stopped before readiness: %v: %s", err, &publisherDiagnostic)
					default:
					}
					ctx, cancel := context.WithTimeout(t.Context(), time.Second)
					defer cancel()
					link, err := administration.RequestPublishedLink(ctx, socket)
					response.Link = link
					return err == nil && link != ""
				})
			case "stream-connect":
				if role != "reader" {
					t.Fatal("invalid byte-stream connection command")
				}
				path := filepath.Join(t.TempDir(), "target-link")
				if err := os.WriteFile(path, []byte(command.Link), 0600); err != nil {
					t.Fatal(err)
				}
				socket := filepath.Join(participant.base, "application", "reader")
				process := exec.CommandContext(t.Context(), os.Getenv("ARDENTS_PREBUILT_NEXT"), "stream", "connect", socket, "--link-file", path)
				process.Stdin = bytes.NewReader(command.Body)
				var diagnostic bytes.Buffer
				process.Stderr = &diagnostic
				output, err := process.Output()
				if err != nil {
					journeyDumpJournal(t)
					response.Failure = fmt.Sprintf("stream connect: %v: %s", err, &diagnostic)
					t.Log(response.Failure)
				}
				response.Body = output
			case "stream-result":
				if role != "publisher" || publisherProcess == nil {
					t.Fatal("invalid byte-stream result command")
				}
				err := <-publisherDone
				publisherProcess = nil
				if err != nil {
					journeyDumpJournal(t)
					response.Failure = fmt.Sprintf("stream publish: %v: %s", err, &publisherDiagnostic)
					t.Log(response.Failure)
				} else {
					response.Original = installedStartupIdentity(t)
				}
				response.Body = publisherOutput.Bytes()
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
				original = journeyRestartContinuity(t, participant, original, approve)
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

// TestNativeEndpointRealHostPublication proves platform-led publication on ONE
// real dual-role Endpoint installed directly on the admitted host manager (no
// container). It is the real-host de-risk for the finite blind-token batch
// deadline that the shared nspawn VPS could not meet: a single manager publishes
// a snapshot through real REGISTER/Store ACK, obtains the canonical accepting
// Link and withdraws, all within the product publication deadlines. The
// cross-Endpoint read is deliberately not exercised here: the receiving fixture
// requires a separate reader participant (pair topology), so the full
// publish/link/read journey belongs to the two-manager qualification. Component
// evidence with fixture authority; not the two-manager privacy qualification.
func TestNativeEndpointRealHostPublication(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("real-host publication qualification requires a root driver")
	}
	carrier := string(transport.ClosedCarrierTCP)
	f, _ := installedServiceNetwork(t, carrier)
	network := installedSignedNetwork(t, f)
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	tag := hex.EncodeToString(nonce[:6])
	base := filepath.Join("/var/lib/ardents", "rh-"+tag)
	etcDir := filepath.Join("/etc/ardents", "rh-"+tag)
	// The real host manager retains no fixture state between runs: remove the
	// account, units, fixed resources and roots so a repeat (-count=2) provisions
	// fresh, exactly as the nspawn guests are terminated per run.
	t.Cleanup(func() {
		_ = exec.Command("systemctl", "stop", "ardents-endpoint.service", "ardents-text-reader.socket", "ardents-text-publisher.socket").Run()
		_ = exec.Command("pkill", "-9", "-f", "ardents-linux-amd64").Run()
		_ = exec.Command("pkill", "-9", "-f", "ardents-text").Run()
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
		_ = exec.Command("bash", "-c", "systemctl list-units --all 'ardents-text-*' --plain --no-legend | awk '{print $1}' | xargs -r systemctl reset-failed").Run()
		_ = exec.Command("userdel", "-f", "ardents-endpoint").Run()
		_ = exec.Command("groupdel", "ardents-endpoint").Run()
		_ = os.RemoveAll(base)
		_ = os.RemoveAll(etcDir)
	})
	approve := func(approvalBase, role string, request []byte) []byte {
		return installedServiceApproval(t, approvalBase, role, request, f)
	}
	body := []byte("one platform-led request on a real admitted host manager\n")
	exercise := func(p journeyParticipant) {
		socket := filepath.Join(p.base, "administration", "socket")
		ctx, cancel := context.WithTimeout(t.Context(), administration.SnapshotTimeout)
		_, err := administration.RequestSnapshot(ctx, socket, body)
		cancel()
		if err != nil {
			journeyDumpJournal(t)
			t.Fatal("real-host installed Snapshot", err)
		}
		ctx, cancel = context.WithTimeout(t.Context(), 15*time.Second)
		link, err := administration.RequestPublishedLink(ctx, socket)
		cancel()
		if err != nil || link == "" {
			t.Fatal("real-host committed Link", err)
		}
		if _, _, err := publication.ParseTargetLink(link); err != nil {
			t.Fatal("real-host canonical Link", err)
		}
		ctx, cancel = context.WithTimeout(t.Context(), 15*time.Second)
		_, err = administration.Request(ctx, socket, administration.Withdraw)
		cancel()
		if err != nil {
			t.Fatal("real-host withdrawal", err)
		}
		ctx, cancel = context.WithTimeout(t.Context(), 15*time.Second)
		link, err = administration.RequestPublishedLink(ctx, socket)
		cancel()
		if err == nil || link != "" {
			t.Fatal("withdrawn real-host publisher still exposes a Link")
		}
		t.Log("real-host platform-led publish/link/withdraw completed within the finite publication deadlines")
	}
	runPlatformLedJourneyParticipant(t, base, etcDir, network, approve, exercise)
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
		_, acceptErr := seed.AcceptEpoch(ctx, network.Epoch, network.Inputs, network.Materials)
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
	bundle, initialProgram = platformNativeDistribution(t, bundle, filepath.Join(base, "initial-distribution"))
	// Advance the signed metadata while retaining the exact staged release and descriptor.
	candidateBundle, _, _ := signedConsumerRelease(t, "linux-amd64", true, true, 1, resources, rootAuthority, 2, 1)
	candidateBundle, _ = platformNativeDistribution(t, candidateBundle, filepath.Join(base, "candidate-distribution"))
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
	// Two Endpoints require distinct declared principals as well as distinct
	// roots/accounts/invocations. The network-facing authorities (Instance,
	// holder, permission, recipient) are already fresh per participant; replace
	// the shared local Broker/Connection/Administration identity placeholders
	// with this participant's own random declarations.
	var principals [3]string
	for i, placeholder := range []string{strings.Repeat("03", 32), strings.Repeat("04", 32), strings.Repeat("09", 32)} {
		var identity [32]byte
		if _, err := rand.Read(identity[:]); err != nil {
			t.Fatal(err)
		}
		principals[i] = hex.EncodeToString(identity[:])
		if strings.Count(request, quoted(placeholder)) != 1 {
			t.Fatal("journey declaration identity fixture changed")
		}
		request = strings.Replace(request, quoted(placeholder), quoted(principals[i]), 1)
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
	uid, gid := stagePlatformNativeEndpoint(t, requestPath, initialProgram, pin, candidateBundle, keyPath)
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
	installPlatformNativeUnit(t, requestPath, candidateBundle, program)
	original := journeyStartPlatformLed(t)
	installedServiceInstance(t, program, credential, base, network.NetworkID)
	journeyGrantPermissions(t, base, original, uint64(uid), uint64(gid), approve)
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
	exercise(journeyParticipant{base: base, original: original, principals: principals, stop: stop})
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
	cmd := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", path, "github.com/dianabuilds/ardents-network/internal/application/document/worker")
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
// pair's restart-refusal). The restarted invocation erased its volatile holder and
// retired the prior permission handover, so the external fixture authority
// re-approves its fresh request before readiness, exactly as the original
// bootstrap. It returns the new original invocation.
func journeyRestartContinuity(t *testing.T, participant journeyParticipant, previous [3]string, approve func(base, role string, request []byte) []byte) [3]string {
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
		journeyDumpJournal(t)
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
	journeyGrantPermissions(t, participant.base, current, uint64(participantUID(t)), uint64(participantGID(t)), approve)
	journeyWaitServiceReady(t, participant.base, current, participantUID(t), participantGID(t))
	t.Logf("platform-led restart reached Service readiness; new InvocationID=%s, selection retained", current[1])
	return current
}

// journeyRestartExchange sends the restart fixture command and serves the two
// interleaved fresh permission requests the restarted participant sends over the
// channel before it reports its new original invocation. The fixture authority
// re-approves each role exactly once; the signing key never leaves the controller.
func journeyRestartExchange(t *testing.T, conn net.Conn, approve func(role string, request []byte) []byte) installedPairMessage {
	t.Helper()
	installedPairSend(t, conn, installedPairMessage{Operation: "restart"})
	approved := make(map[string]bool)
	for {
		message := installedPairReceive(t, conn)
		switch message.Operation {
		case "permission":
			if (message.Role != "reader" && message.Role != "publisher") || approved[message.Role] {
				t.Fatal("unexpected restart approval request")
			}
			response := approve(message.Role, message.Body)
			approved[message.Role] = true
			installedPairSend(t, conn, installedPairMessage{Operation: "permission", Role: message.Role, Body: response})
		case "restart":
			if !approved["reader"] || !approved["publisher"] {
				t.Fatal("platform-led restart omitted independent permission re-admission")
			}
			return message
		default:
			t.Fatal("unexpected journey message during restart exchange")
		}
	}
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
	runNativeEndpointPlatformLedJourney(t, false)
}

func TestNativeInstalledEndpointByteStream(t *testing.T) {
	runNativeEndpointPlatformLedJourney(t, true)
}

func runNativeEndpointPlatformLedJourney(t *testing.T, byteStream bool) {
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
	var declared [2][3]string
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
				declared[i] = request.Principals
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
	publishOperation := "publish"
	if byteStream {
		publishOperation = "stream-publish"
		body = bytes.Repeat([]byte{0, 255, 128, 7}, 8192)
	}
	publication := exchange(0, installedPairMessage{Operation: publishOperation, Body: body})
	if publication.Link == "" {
		t.Fatal("installed publisher supplied no Link")
	}
	identities[1] = startAdmit(1, "reader")
	if identities[0][1] == identities[1][1] {
		t.Fatal("participants share a manager invocation")
	}
	for j := range declared[0] {
		if declared[0][j] == "" || declared[0][j] == declared[1][j] {
			t.Fatal("participants share a declared principal")
		}
	}
	if byteStream {
		sent := bytes.Repeat([]byte{129, 0, 254, 3, 5}, 5000)
		read := exchange(1, installedPairMessage{Operation: "stream-connect", Link: publication.Link, Body: sent})
		received := exchange(0, installedPairMessage{Operation: "stream-result"})
		if read.Failure != "" || received.Failure != "" {
			t.Fatalf("installed byte stream stopped (received %d/%d bytes): reader %s; publisher %s", len(read.Body), len(received.Body), read.Failure, received.Failure)
		}
		if read.Original != identities[1] || received.Original != identities[0] ||
			!bytes.Equal(read.Body, body) || !bytes.Equal(received.Body, sent) {
			t.Fatal("installed byte-stream exchange changed bytes or original Endpoint identities")
		}
		for i := range peers {
			exchange(i, installedPairMessage{Operation: "finish"})
			err := commands[i].Wait()
			joined[i] = true
			if err != nil {
				t.Fatalf("byte-stream participant did not join: %v: %s", err, &outputs[i])
			}
		}
		t.Log("two independently admitted installed Endpoints exchanged byte-equal data in both directions via the public CLI")
		return
	}
	read := exchange(1, installedPairMessage{Operation: "read", Link: publication.Link})
	if !bytes.Equal(read.Body, body) {
		t.Fatal("installed reader returned different Application bytes")
	}
	exchange(0, installedPairMessage{Operation: "withdraw"})
	for i := range peers {
		restartRoot := filepath.Join(t.TempDir(), "restart-"+[]string{"publisher", "reader"}[i])
		if err := os.MkdirAll(restartRoot, 0700); err != nil {
			t.Fatal(err)
		}
		response := journeyRestartExchange(t, peers[i], func(role string, request []byte) []byte {
			return installedServiceApproval(t, restartRoot, role, request, f)
		})
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
	worker := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", text, "./internal/application/document/worker")
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
	cmd := exec.CommandContext(t.Context(), "go", "test", "-c", "-tags=endpoint_pair_native,document_worker_installed",
		"-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", path, "./cmd/ardents-next")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile journey participant: %v: %s", err, out)
	}
	return path
}

const (
	crossHostBindEnv      = "ARDENTS_XH_BIND_HOST"
	crossHostPublisherEnv = "ARDENTS_XH_PUBLISHER_SSH"
	crossHostKeyEnv       = "ARDENTS_XH_SSH_KEY"
)

// TestNativeEndpointCrossHostJourney is the cross-host controller for the
// two-Endpoint platform-led text journey (#391 S2.2). It runs as root on the
// reader/controller host and keeps the shared fixture route network and signing
// authority there: routeFixtureBindHost is lifted from loopback to the
// controller's reachable address BEFORE the fixture network is created, so the
// signed epoch advertises receivers that the remote publisher can dial (the
// Endpoint runtime only opens outbound role connections; no participant listens
// for cross-host traffic). The publisher participant runs on a second real
// admitted host manager over SSH; the reader participant runs locally. Both
// reuse the unchanged platform-led participant and receive only public signed
// Network material and actual permission requests/responses over the fixture
// channel. The controller approves each permission with the fixture authority
// and coordinates publish → accepting Link → byte-equal read → withdraw →
// platform-led restart continuity on both hosts (new InvocationID, retained
// immutable selection, fresh permission re-bootstrap and Service readiness), then
// joins both participants and restores both hosts to their pre-run state. A
// restart erases the volatile holder and retires the prior permission handover;
// the fixture authority re-approves the fresh invocation's request. Restart
// proves readiness continuity, not publication continuity.
//
// Environment: ARDENTS_XH_BIND_HOST (controller address reachable from the
// publisher; must be a local interface address), ARDENTS_XH_PUBLISHER_SSH (SSH
// target such as root@host) and optional ARDENTS_XH_SSH_KEY (private key for
// that target). Missing privileges, tools, residual installed state or an
// unreachable fixture path are invalid environments, never a passing skip.
//
// Component/acceptance evidence with fixture Release/Network authority for two
// independently admitted real host managers; NOT the whole-host two-participant
// privacy qualification. The transitional guarded-start/ACK/recovery
// orchestration is neither exercised nor retired (ADR-0125 D6).
func TestNativeEndpointCrossHostJourney(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("cross-host journey qualification requires a root driver on the controller host")
	}
	bindHost, publisherSSH := os.Getenv(crossHostBindEnv), os.Getenv(crossHostPublisherEnv)
	if bindHost == "" || publisherSSH == "" {
		t.Fatalf("invalid environment: %s and %s required", crossHostBindEnv, crossHostPublisherEnv)
	}
	if !crossHostLocalAddress(bindHost) {
		t.Fatalf("invalid environment: %s is not a local controller address", bindHost)
	}
	for _, tool := range []string{"ssh", "scp"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("invalid environment: %s unavailable: %v", tool, err)
		}
	}
	crossHostVerifyReachability(t, bindHost, publisherSSH)
	crossHostRequireAbsentEndpoint(t, publisherSSH)

	previousBind := routeFixtureBindHost
	routeFixtureBindHost = bindHost
	t.Cleanup(func() { routeFixtureBindHost = previousBind })

	carrier := string(transport.ClosedCarrierTCP)
	f, store := installedServiceNetwork(t, carrier)
	network := installedSignedNetwork(t, f)
	for _, node := range f.spec.Nodes {
		t.Logf("fixture route node %d endpoint %s", node.NodeID[0], node.Endpoint)
	}
	listener, err := net.Listen("tcp", bindHost+":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	stopListener := context.AfterFunc(t.Context(), func() { _ = listener.Close() })
	defer stopListener()

	repoRoot := pairRepoRoot(t)
	participantBinary, nextBinary, textBinary := journeyCrossHostBinaries(t, repoRoot)
	const participantTimeout = 12 * time.Minute
	// The existing refresh oracle needs the original token hour throughout the
	// refresh and signed-predecessor overlap. Check after the cold build.
	if time.Until(time.Now().UTC().Truncate(time.Hour).Add(time.Hour)) < 16*time.Minute {
		t.Fatal("invalid environment: cross-host refresh needs sixteen minutes in the original token hour")
	}
	var runNonce [32]byte
	if _, err := rand.Read(runNonce[:]); err != nil {
		t.Fatal(err)
	}
	remoteDir := "/root/xh-" + hex.EncodeToString(runNonce[:6])
	crossHostRun(t, t.Context(), publisherSSH, "mkdir -p "+remoteDir+"\n")
	crossHostCopy(t, publisherSSH, participantBinary, remoteDir+"/journey.test")
	crossHostCopy(t, publisherSSH, nextBinary, remoteDir+"/ardents-next")
	crossHostCopy(t, publisherSSH, textBinary, remoteDir+"/ardents-text")
	crossHostRun(t, t.Context(), publisherSSH, "chmod 0555 "+remoteDir+"/journey.test "+remoteDir+"/ardents-next "+remoteDir+"/ardents-text\n")

	var peers [2]net.Conn
	var commands [2]*exec.Cmd
	var outputs [2]bytes.Buffer
	var joined [2]bool
	var bases, etcDirs, nonces [2]string
	t.Cleanup(func() {
		for _, peer := range peers {
			if peer != nil {
				_ = peer.Close()
			}
		}
		for i := range commands {
			if commands[i] == nil {
				continue
			}
			// Restore each real host even when the journey failed mid-exchange;
			// the participant then observes the stopped service and joins. The
			// test context is already canceled in cleanup, so the restoration
			// runs on its own finite background bound.
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			script := crossHostCleanupScript(bases[i], etcDirs[i])
			var cleanup *exec.Cmd
			if i == 0 {
				cleanup = exec.CommandContext(ctx, "ssh", append(crossHostSSHArgs(), publisherSSH, "bash", "-s")...)
			} else {
				cleanup = exec.CommandContext(ctx, "bash", "-s")
			}
			cleanup.Stdin = strings.NewReader(script)
			if out, err := cleanup.CombinedOutput(); err != nil {
				t.Errorf("journey host cleanup: %v: %s", err, out)
			}
			cancel()
			if i == 0 {
				removeCtx, removeCancel := context.WithTimeout(context.Background(), 30*time.Second)
				remove := exec.CommandContext(removeCtx, "ssh", append(crossHostSSHArgs(), publisherSSH, "bash", "-s")...)
				remove.Stdin = strings.NewReader("rm -rf " + remoteDir + "\n")
				if out, err := remove.CombinedOutput(); err != nil {
					t.Errorf("journey publisher artifact cleanup: %v: %s", err, out)
				}
				removeCancel()
			}
			if !joined[i] {
				if err := commands[i].Wait(); err != nil {
					t.Errorf("journey participant cleanup: %v\n%s", err, outputs[i].String())
				}
			}
		}
	})

	startParticipant := func(i int, role string, remote bool) {
		var nonce [32]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			t.Fatal(err)
		}
		nonces[i] = hex.EncodeToString(nonce[:])
		// Keep the mutable root short: the application/administration Unix socket
		// paths below it must fit the kernel sun_path bound (108 bytes).
		bases[i] = filepath.Join("/var/lib/ardents", "xh-"+hex.EncodeToString(nonce[:6]))
		etcDirs[i] = filepath.Join("/etc/ardents", "journey-"+nonces[i])
		address := listener.Addr().String()
		if remote {
			script := fmt.Sprintf("exec env PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"+
				" ARDENTS_PREBUILT_NEXT=%s/ardents-next ARDENTS_PREBUILT_TEXT=%s/ardents-text"+
				" %s=%s %s=%s %s=%s %s=%s"+
				" %s/journey.test -test.run='^TestNativeEndpointPlatformLedJourneyParticipant$' -test.v -test.timeout=%s\n",
				remoteDir, remoteDir,
				journeyRootEnv, bases[i], journeyRoleEnv, role, journeyNonceEnv, nonces[i], journeyAddressEnv, address,
				remoteDir, participantTimeout)
			command := exec.CommandContext(t.Context(), "ssh", append(crossHostSSHArgs(), publisherSSH, "bash", "-s")...)
			command.Stdin = strings.NewReader(script)
			command.Stdout, command.Stderr = &outputs[i], &outputs[i]
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			commands[i] = command
			return
		}
		command := exec.CommandContext(t.Context(), participantBinary,
			"-test.run=^TestNativeEndpointPlatformLedJourneyParticipant$", "-test.v", "-test.timeout="+participantTimeout.String())
		command.Env = append(os.Environ(),
			"ARDENTS_PREBUILT_NEXT="+nextBinary,
			"ARDENTS_PREBUILT_TEXT="+textBinary,
			journeyRootEnv+"="+bases[i],
			journeyRoleEnv+"="+role,
			journeyNonceEnv+"="+nonces[i],
			journeyAddressEnv+"="+address)
		command.Stdout, command.Stderr = &outputs[i], &outputs[i]
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		commands[i] = command
		t.Logf("journey participant %d (%s) started pid=%d address=%s root=%s", i, role, command.Process.Pid, listener.Addr().String(), bases[i])
	}
	roles := [2]string{"publisher", "reader"}
	// Both participants launch concurrently; accept arrival order is not
	// guaranteed, so bind each connection to its participant by the original
	// fixture nonce and role, then send the public signed Network to both
	// immediately so provisioning and platform-led launch proceed concurrently
	// on the two hosts. Permission approvals are served serially afterwards.
	bindParticipants := func() {
		for range peers {
			if tcp, ok := listener.(*net.TCPListener); ok {
				_ = tcp.SetDeadline(time.Now().Add(180 * time.Second))
			}
			conn, err := listener.Accept()
			if err != nil {
				alive := [2]string{}
				for i, command := range commands {
					if command != nil && command.Process != nil {
						alive[i] = fmt.Sprintf("pid=%d kill0=%v", command.Process.Pid, syscall.Kill(command.Process.Pid, 0))
					}
				}
				t.Fatalf("journey participants did not connect: %v (%s; %s)\n%s\n%s", err, alive[0], alive[1], outputs[0].String(), outputs[1].String())
			}
			if err := conn.SetDeadline(time.Now().Add(participantTimeout)); err != nil {
				_ = conn.Close()
				t.Fatal("bound cross-host journey channel", err)
			}
			hello := installedPairReceive(t, conn)
			matched := -1
			for j := range nonces {
				if hello.Operation == "hello" && hello.Role == roles[j] && hello.Nonce == nonces[j] {
					matched = j
				}
			}
			if matched < 0 || peers[matched] != nil {
				t.Fatal("foreign journey fixture client")
			}
			peers[matched] = conn
			t.Logf("journey channel bound participant %d (%s) from %s", matched, roles[matched], conn.RemoteAddr())
		}
		for i := range peers {
			installedPairSend(t, peers[i], installedPairMessage{Operation: "network", Network: &network})
		}
	}
	var declared [2][3]string
	admit := func(i int) [3]string {
		conn := peers[i]
		approvalRoot := filepath.Join(t.TempDir(), roles[i])
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
				declared[i] = request.Principals
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

	// Admit both participants before the exchange: the publisher runs on its own
	// dedicated host manager, so the reader's provisioning on the controller host
	// does not contend with the finite blind-token publication issuance. First
	// prove the original publication, then observe a real automatic refresh and
	// read through the same Link while both original invocations remain live.
	body := []byte("one platform-led request across two real admitted host managers\n")
	identities := [2][3]string{}
	startParticipant(0, "publisher", true)
	startParticipant(1, "reader", false)
	bindParticipants()
	identities[0] = admit(0)
	identities[1] = admit(1)
	if identities[0][1] == identities[1][1] {
		t.Fatal("participants share a manager invocation")
	}
	for j := range declared[0] {
		if declared[0][j] == "" || declared[0][j] == declared[1][j] {
			t.Fatal("participants share a declared principal")
		}
	}
	publishStarted := time.Now()
	published := exchange(0, installedPairMessage{Operation: "publish", Body: body})
	if published.Link == "" || published.Original != identities[0] {
		t.Fatal("installed publisher supplied no Link for its original invocation")
	}
	first := installedPairStoredProof(t, store, f, published.Link)
	read := func() {
		result := exchange(1, installedPairMessage{Operation: "read", Link: published.Link})
		if !bytes.Equal(result.Body, body) || result.Original != identities[1] {
			t.Fatal("installed reader changed Application bytes or invocation")
		}
	}
	read()
	installedPairAwaitRefresh(t, store, f, published.Link, first, publishStarted)
	replacement := installedPairStoredProof(t, store, f, published.Link)
	// Store commit precedes ACK. Wait through the predecessor overlap, then
	// challenge the old recipient while its signed proof is still valid.
	overlap := time.NewTimer(62 * time.Second)
	defer overlap.Stop()
	select {
	case <-t.Context().Done():
		t.Fatal("cross-host predecessor retirement observation interrupted", t.Context().Err())
	case <-overlap.C:
	}
	if err := first.Current(time.Now()); err != nil {
		t.Fatal("original signed Descriptor expired before predecessor refusal", err)
	}
	installedPublisherSubmitNonce(t, f, first, false)
	if current := installedPairStoredProof(t, store, f, published.Link); current.Digest() != replacement.Digest() {
		t.Fatal("cross-host predecessor retirement changed the accepting replacement")
	}
	read()
	if withdrawn := exchange(0, installedPairMessage{Operation: "withdraw"}); withdrawn.Original != identities[0] {
		t.Fatal("refreshed publication was withdrawn by a different invocation")
	}
	t.Log("cross-host Descriptor refresh, signed predecessor refusal and unchanged document verified")
	// Platform-led restart continuity on both real hosts: each participant stops
	// and starts its Endpoint under a new InvocationID with the same immutable
	// selection and retained durable floors. The fresh invocation erased its
	// volatile holder and retired the prior permission handover, so the fixture
	// authority re-approves its fresh request and the participant reaches Service
	// readiness again. The proven publish/read/withdraw exchange precedes restart;
	// restart proves readiness continuity, not publication continuity
	// (endpoint-service-runtime owner). Repeatable (-count=2).
	for i := range peers {
		restartRoot := filepath.Join(t.TempDir(), "restart-"+roles[i])
		if err := os.MkdirAll(restartRoot, 0700); err != nil {
			t.Fatal(err)
		}
		response := journeyRestartExchange(t, peers[i], func(role string, request []byte) []byte {
			return installedServiceApproval(t, restartRoot, role, request, f)
		})
		if response.Original[1] == identities[i][1] {
			t.Fatal("platform-led restart reused the original invocation")
		}
		identities[i] = response.Original
		_ = peers[i].Close()
		peers[i] = nil
		waitErr := commands[i].Wait()
		joined[i] = true
		t.Log(outputs[i].String())
		if waitErr != nil {
			t.Fatal("journey participant did not join", waitErr)
		}
	}
	t.Log("two real admitted host managers: platform-led publish/link, exact cross-host read, owning withdrawal and restart continuity completed")
}

// crossHostSSHArgs keeps the publisher transport non-interactive: a missing key
// or host trust fails fast as an invalid environment instead of prompting.
func crossHostSSHArgs() []string {
	args := []string{"-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new", "-o", "ConnectTimeout=15"}
	if key := os.Getenv(crossHostKeyEnv); key != "" {
		args = append(args, "-i", key)
	}
	return args
}

func crossHostLocalAddress(host string) bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.String() == host {
			return true
		}
	}
	return false
}

// crossHostRun executes one stdin script on the publisher host. The script
// travels on stdin (bash -s), never in the SSH command line, so fixture cleanup
// patterns cannot match the transporting shell itself.
func crossHostRun(t *testing.T, ctx context.Context, publisherSSH, script string) []byte {
	t.Helper()
	cmd := exec.CommandContext(ctx, "ssh", append(crossHostSSHArgs(), publisherSSH, "bash", "-s")...)
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("publisher host command: %v: %s", err, out)
	}
	return out
}

func crossHostCopy(t *testing.T, publisherSSH, local, remote string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "scp", append(crossHostSSHArgs(), local, publisherSSH+":"+remote)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("scp %s: %v: %s", remote, err, out)
	}
}

// crossHostVerifyReachability proves the exact fixture path the remote
// participant will use: one ephemeral controller listener on the advertised
// address, dialed from the publisher host. The website's fixed ports are never
// touched.
func crossHostVerifyReachability(t *testing.T, bindHost, publisherSSH string) {
	t.Helper()
	listener, err := net.Listen("tcp", bindHost+":0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan error, 1)
	go func() {
		if tcp, ok := listener.(*net.TCPListener); ok {
			_ = tcp.SetDeadline(time.Now().Add(30 * time.Second))
		}
		conn, err := listener.Accept()
		if err != nil {
			accepted <- err
			return
		}
		defer conn.Close()
		var buf [16]byte
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		n, err := conn.Read(buf[:])
		if err != nil || string(buf[:n]) != "reachable" {
			accepted <- fmt.Errorf("unexpected reachability probe %q: %v", buf[:n], err)
			return
		}
		accepted <- nil
	}()
	script := fmt.Sprintf("timeout 10 bash -c 'exec 3<>/dev/tcp/%s/%s && printf reachable >&3'\n", bindHost, port)
	crossHostRun(t, t.Context(), publisherSSH, script)
	if err := <-accepted; err != nil {
		t.Fatalf("invalid environment: publisher cannot reach the controller fixture path: %v", err)
	}
}

// crossHostRequireAbsentEndpoint refuses residual installed state on either
// host: provisioning preflight requires an absent account and unit, and a
// foreign residual must not be silently overwritten.
func crossHostRequireAbsentEndpoint(t *testing.T, publisherSSH string) {
	t.Helper()
	script := "if getent passwd ardents-endpoint >/dev/null 2>&1 || test -e /etc/systemd/system/ardents-endpoint.service; then echo 'ardents-endpoint residual present on publisher host'; exit 1; fi\n"
	crossHostRun(t, t.Context(), publisherSSH, script)
	if _, err := user.Lookup("ardents-endpoint"); err == nil {
		t.Fatal("invalid environment: ardents-endpoint account residual on the controller host")
	}
	if _, err := os.Lstat("/etc/systemd/system/ardents-endpoint.service"); err == nil {
		t.Fatal("invalid environment: ardents-endpoint unit residual on the controller host")
	}
}

// journeyCrossHostBinaries compiles the participant and prebuilds the canonical
// consumer and text worker as STATIC executables, so the controller-host build
// runs unchanged on both admitted platform releases without a glibc
// forward-compatibility assumption across hosts.
func journeyCrossHostBinaries(t *testing.T, repoRoot string) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	build := func(name, pkg string, args ...string) string {
		path := filepath.Join(dir, name)
		full := append([]string{}, args...)
		full = append(full, "-o", path, pkg)
		cmd := exec.CommandContext(t.Context(), "go", full...)
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cross-host build %s: %v: %s", name, err, out)
		}
		return path
	}
	participant := build("journey.test", "./cmd/ardents-next", "test", "-c", "-tags=endpoint_pair_native,document_worker_installed", "-trimpath", "-buildvcs=false", "-ldflags=-w")
	next := build("ardents-next", "./cmd/ardents-next", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w")
	text := build("ardents-text", "./internal/application/document/worker", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w")
	return participant, next, text
}

// crossHostCleanupScript mirrors the single-real-host publication cleanup: the
// account, units, fixed resources and roots are namespaced Ardents artifacts
// only. It runs via bash -s so the pkill patterns cannot match the transporting
// shell, and never touches the website's containers or fixed ports. Account and
// group removal use absolute /usr/sbin paths so they do not depend on the
// caller's PATH containing /usr/sbin, with a bounded wait for the killed
// endpoint/worker processes to exit and a bounded removal retry, making the
// joined cleanup reliable across -count repeats.
func crossHostCleanupScript(base, etcDir string) string {
	return `systemctl stop ardents-endpoint.service ardents-text-reader.socket ardents-text-publisher.socket 2>/dev/null
pkill -9 -f ardents-linux-amd64 2>/dev/null
pkill -9 -f ardents-text 2>/dev/null
for i in $(seq 1 20); do if ! pgrep -f ardents-linux-amd64 >/dev/null 2>&1 && ! pgrep -f ardents-text >/dev/null 2>&1; then break; fi; sleep 0.5; done
rm -f /etc/systemd/system/ardents-endpoint.service /etc/systemd/system/ardents-text-reader@.service /etc/systemd/system/ardents-text-publisher@.service /etc/systemd/system/ardents-text-reader.socket /etc/systemd/system/ardents-text-publisher.socket /usr/share/polkit-1/rules.d/50-ardents-text.rules /usr/lib/tmpfiles.d/ardents-text.conf /etc/ardents/text-worker-artifact.json
rm -rf /usr/lib/ardents/text-worker-root /run/ardents-text
systemctl daemon-reload
systemctl reset-failed ardents-endpoint.service 2>/dev/null
systemctl list-units --all 'ardents-text-*' --plain --no-legend | awk '{print $1}' | xargs -r systemctl reset-failed
for i in $(seq 1 20); do /usr/sbin/userdel -f ardents-endpoint 2>/dev/null; getent passwd ardents-endpoint >/dev/null 2>&1 || break; sleep 0.5; done
for i in $(seq 1 20); do /usr/sbin/groupdel ardents-endpoint 2>/dev/null; getent group ardents-endpoint >/dev/null 2>&1 || break; sleep 0.5; done
rm -rf ` + base + ` ` + etcDir + `
echo host-cleanup-done
`
}
