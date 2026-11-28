//go:build linux && endpoint_pair_native

package endpoint_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/publication"
	"github.com/dianabuilds/ardents-network/internal/domain/route/transport"
	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"
)

// This native qualification proves the two-Endpoint platform-led document journey
// between TWO separately admitted Ubuntu/systemd managers (#391 S2.2), building
// on the S2.1 nspawn launch foundation. The controller keeps one shared fixture
// route network (Forwarding/Descriptor Store/Introduction/JOIN/Responder/Issuer
// receivers on the shared host loopback) and the signing authority; each guest
// independently provisions and platform-led-launches its own dual-role Endpoint
// (no root ACK), authenticates the public signed Network through its own State,
// and obtains separately approved permissions. The publisher guest publishes a
// snapshot through real REGISTER/Store ACK and exposes an accepting Link; the
// reader guest resolves the Descriptor, joins and reads the exact document bytes through
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
	network := endpointfixture.BuildSignedNetwork(t, f.spec)
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
	exercise := func(p endpointfixture.JourneyParticipant) {
		socket := filepath.Join(p.Base, "administration", "socket")
		path := filepath.Join(t.TempDir(), "snapshot-input")
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), administration.SnapshotTimeout)
		err := administration.PublishLocal(ctx, socket, path)
		cancel()
		if err != nil {
			endpointfixture.DumpJourneyJournal(t)
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
	endpointfixture.RunJourneyParticipant(t, commandBuild.Binary(t), base, etcDir, network, approve, exercise)
}

// TestNativeEndpointPlatformLedJourney is the outer controller. It provisions two
// separately admitted managers (S2.1 nspawn foundation), keeps the shared fixture
// route network and signing authority, and coordinates the publish/link/read/
// withdraw/restart journey over the local fixture channel.
func TestNativeEndpointPlatformLedJourney(t *testing.T) {
	runNativeEndpointPlatformLedJourney(t, false, nil, false)
}

func TestNativeInstalledEndpointByteStream(t *testing.T) {
	runNativeEndpointPlatformLedJourney(t, true, nil, false)
}

// Both installed drivers allow the original ten-minute publication to report
// its own outcome, plus setup and joined cleanup outside that product lifetime.
const fullInstalledJourneyParticipantTimeout = 12 * time.Minute

func TestNativeEndpointSegmentedInstalledPair(t *testing.T) {
	payload := int64(admission.ForwardClass.ByteLimit()) + (1 << 20)
	t.Run("bidirectional", func(t *testing.T) {
		streams := [2]endpointfixture.JourneyStream{
			{Bytes: payload, Seed: 1},
			{Bytes: payload, Seed: 2, BytesPerSecond: 512 << 10},
		}
		runNativeEndpointPlatformLedJourney(t, true, &streams, false)
	})
	t.Run("published-binary-file", func(t *testing.T) {
		streams := [2]endpointfixture.JourneyStream{{Bytes: payload, Seed: 3}, {}}
		runNativeEndpointPlatformLedJourney(t, true, &streams, true)
	})
}

func runNativeEndpointPlatformLedJourney(t *testing.T, byteStream bool, streams *[2]endpointfixture.JourneyStream, filePublication bool) {
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
	repoRoot := endpointfixture.RepositoryRoot(t)
	goRoot := endpointfixture.GoEnv(t, "GOROOT")
	modCache := endpointfixture.GoEnv(t, "GOMODCACHE")
	buildCache := endpointfixture.GoEnv(t, "GOCACHE")
	base := endpointfixture.EnsurePairBaseRootfs(t)
	carrier := string(transport.ClosedCarrierTCP)
	var spends *installedRecoverySpends
	if streams != nil {
		spends = &installedRecoverySpends{}
	}
	f, _ := installedServiceNetwork(t, carrier, &installedReceiverEvidence{recovery: spends})
	network := endpointfixture.BuildSignedNetwork(t, f.spec)
	guestBinary := journeyCompileParticipant(t, repoRoot)
	// Prebuild the canonical consumer and static worker on the host so the guests
	// do not repeat Go builds under shared-host load; the publication deadlines
	// are finite and must not be weakened to absorb build contention.
	nextBinary, workerBinary := journeyPrebuildBinaries(t, repoRoot)
	participantTimeout := 6 * time.Minute
	if streams != nil {
		participantTimeout = fullInstalledJourneyParticipantTimeout
		if time.Until(time.Now().UTC().Truncate(time.Hour).Add(time.Hour)) < 16*time.Minute {
			t.Fatal("invalid environment: full installed transfer needs sixteen minutes in the original token hour")
		}
	}
	runID := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	var guests [2]*endpointfixture.PairGuest
	for i, label := range []string{"pub", "read"} {
		guests[i] = endpointfixture.BootPairGuest(t, base, runID, label, goRoot, repoRoot, modCache, buildCache)
	}
	endpointfixture.VerifyDistinctManagers(t, guests[:])
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
	exchange := func(index int, command endpointfixture.PairMessage) endpointfixture.PairMessage {
		return endpointfixture.ExchangeJourneyCommand(t, peers[index], index, command)
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
		journeyCopyIntoGuest(t, guestBinary, filepath.Join(guest.Rootfs(), "root", "journey.test"))
		journeyCopyIntoGuest(t, nextBinary, filepath.Join(guest.Rootfs(), "root", "ardents-next"))
		journeyCopyIntoGuest(t, workerBinary, filepath.Join(guest.Rootfs(), "root", "ardents-document-worker"))
		// Keep the mutable root short: the application/administration Unix socket
		// paths below it must fit the kernel sun_path bound (108 bytes).
		guestRoot := filepath.Join("/var/lib/ardents", "journey-"+hex.EncodeToString(nonce[:6]))
		script := fmt.Sprintf("cd %s/tests/e2e/endpointinstalled && /root/journey.test -test.run='^TestNativeEndpointPlatformLedJourneyParticipant$' -test.v -test.timeout=%s", repoRoot, participantTimeout)
		command := exec.Command("systemd-run", "-M", guest.Name(), "--pipe", "--quiet",
			"--setenv=PATH=/usr/local/go/bin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"--setenv=GOTOOLCHAIN=local",
			"--setenv=GOMODCACHE="+modCache,
			"--setenv=GOCACHE="+buildCache,
			"--setenv=GOFLAGS=-buildvcs=false",
			"--setenv=ARDENTS_PREBUILT_NEXT=/root/ardents-next",
			"--setenv=ARDENTS_PREBUILT_DOCUMENT_WORKER=/root/ardents-document-worker",
			"--setenv="+endpointfixture.JourneyRootEnv+"="+guestRoot,
			"--setenv="+endpointfixture.JourneyRoleEnv+"="+role,
			"--setenv="+endpointfixture.JourneyNonceEnv+"="+hex.EncodeToString(nonce[:]),
			"--setenv="+endpointfixture.JourneyAddressEnv+"="+listener.Addr().String(),
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
		_ = conn.SetDeadline(time.Now().Add(participantTimeout))
		hello := endpointfixture.ReceivePairMessage(t, conn)
		if hello.Operation != "hello" || hello.Role != role || hello.Nonce != hex.EncodeToString(nonce[:]) {
			t.Fatal("foreign journey fixture client")
		}
		endpointfixture.SendPairMessage(t, conn, endpointfixture.PairMessage{Operation: "network", Network: &network})
		approvalRoot := filepath.Join(t.TempDir(), role)
		if err := os.MkdirAll(approvalRoot, 0700); err != nil {
			t.Fatal(err)
		}
		admitted := endpointfixture.AdmitJourneyParticipant(t, conn, func(role string, request []byte) []byte {
			return installedServiceApproval(t, approvalRoot, role, request, f)
		})
		declared[i] = admitted.Principals
		return admitted.Original
	}
	body := bytes.Repeat([]byte("one platform-led document across independently admitted managers\n"), 600)
	identities := [2][3]string{}
	identities[0] = startAdmit(0, "publisher")
	publishOperation := "publish"
	if byteStream {
		publishOperation = "stream-publish"
		body = bytes.Repeat([]byte{0, 255, 128, 7}, 8192)
	}
	publish := endpointfixture.PairMessage{Operation: publishOperation, Body: body}
	if streams != nil {
		publish.Body, publish.Stream = nil, &streams[0]
		if filePublication {
			publish.Operation = "file-publish"
		}
	}
	publication := exchange(0, publish)
	if publication.Link == "" {
		t.Fatal("installed publisher supplied no Link")
	}
	identities[1] = startAdmit(1, "reader")
	if identities[0][1] == identities[1][1] {
		t.Fatal("participants share a manager invocation")
	}
	for j := range declared[0] {
		if declared[0][j] == "" || declared[1][j] == "" || declared[0][j] == declared[1][j] {
			t.Fatal("participants share a declared principal")
		}
	}
	if byteStream {
		sent := bytes.Repeat([]byte{129, 0, 254, 3, 5}, 5000)
		request := endpointfixture.PairMessage{Operation: "stream-connect", Link: publication.Link, Body: sent}
		if streams != nil {
			request.Body, request.Stream = nil, &streams[1]
		}
		var before [2]uint64
		if streams != nil {
			before = sampleTransferRSS(t, exchange, identities)
		}
		started := time.Now()
		read := exchange(1, request)
		results := []endpointfixture.PairMessage{read}
		if filePublication {
			if withdrawn := exchange(0, endpointfixture.PairMessage{Operation: "withdraw"}); withdrawn.Original != identities[0] {
				t.Fatal("file was withdrawn by a different invocation")
			}
		} else {
			results = append(results, exchange(0, endpointfixture.PairMessage{Operation: "stream-result"}))
		}
		for i, result := range results {
			if result.Failure != "" || result.Original != identities[1-i] {
				var received int64
				if result.Proof != nil {
					received = result.Proof.Bytes
				}
				t.Fatalf("installed stream direction %d failed or changed invocation after %d bytes: %s", i, received, result.Failure)
			}
			if streams == nil {
				if !bytes.Equal(result.Body, [][]byte{body, sent}[i]) {
					t.Fatal("installed byte-stream exchange changed bytes")
				}
				continue
			}
			expected := endpointfixture.NewJourneyStreamOutput()
			if _, err := io.Copy(expected, streams[i].Reader()); err != nil {
				t.Fatal(err)
			}
			if result.Proof == nil || *result.Proof != expected.Proof() || len(result.Body) != 0 {
				t.Fatalf("installed stream direction %d differs: observed %+v, expected %+v", i, result.Proof, expected.Proof())
			}
		}
		if duration := time.Since(started); streams != nil && !filePublication && duration <= 60*time.Second {
			t.Fatalf("installed local-pair stream completed within the old Reader window: %s", duration)
		}
		if streams != nil {
			logTransferRSS(t, before, sampleTransferRSS(t, exchange, identities))
		}
		for i := range peers {
			exchange(i, endpointfixture.PairMessage{Operation: "finish"})
			err := commands[i].Wait()
			joined[i] = true
			if err != nil {
				t.Fatalf("byte-stream participant did not join: %v: %s", err, &outputs[i])
			}
		}
		if spends != nil {
			requireTransferAccounting(t, spends, streams[0].Bytes)
		}
		t.Log("two independently admitted installed Endpoints exchanged byte-equal data in both directions via the public CLI")
		return
	}
	read := exchange(1, endpointfixture.PairMessage{Operation: "read", Link: publication.Link})
	if !bytes.Equal(read.Body, body) {
		t.Fatal("installed reader returned different Application bytes")
	}
	exchange(0, endpointfixture.PairMessage{Operation: "withdraw"})
	for i := range peers {
		restartRoot := filepath.Join(t.TempDir(), "restart-"+[]string{"publisher", "reader"}[i])
		if err := os.MkdirAll(restartRoot, 0700); err != nil {
			t.Fatal(err)
		}
		response := endpointfixture.RestartJourneyParticipant(t, peers[i], func(role string, request []byte) []byte {
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
	t.Log("two independent platform-led admissions, one actual AAI3 document exchange, exact response, owning withdrawal and restart continuity completed")
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
	workerPath := filepath.Join(dir, "ardents-document-worker")
	worker := exec.CommandContext(t.Context(), "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", workerPath, "./internal/application/document/worker")
	worker.Dir = repoRoot
	worker.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := worker.CombinedOutput(); err != nil {
		t.Fatalf("prebuild worker: %v: %s", err, out)
	}
	return next, workerPath
}

func journeyCompileParticipant(t *testing.T, repoRoot string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "journey.test")
	cmd := exec.CommandContext(t.Context(), "go", "test", "-c", "-tags=endpoint_pair_native,document_worker_installed",
		"-trimpath", "-buildvcs=false", "-ldflags=-w", "-o", path, "./tests/e2e/endpointinstalled")
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
// two-Endpoint platform-led document journey (#391 S2.2). It runs as root on the
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
	runNativeEndpointCrossHostJourney(t, nil, false)
}

func TestNativeInstalledEndpointByteStreamContinuation(t *testing.T) {
	bytesPerDirection := int64(admission.ForwardClass.ByteLimit()) + (1 << 20)
	streams := [2]endpointfixture.JourneyStream{
		{Bytes: bytesPerDirection, Seed: 1},
		{Bytes: bytesPerDirection, Seed: 2, BytesPerSecond: 512 << 10},
	}
	runNativeEndpointCrossHostJourney(t, &streams, false)
}

func TestNativeInstalledEndpointBinaryFileContinuation(t *testing.T) {
	streams := [2]endpointfixture.JourneyStream{
		{Bytes: int64(admission.ForwardClass.ByteLimit()) + (1 << 20), Seed: 3},
		{},
	}
	runNativeEndpointCrossHostJourney(t, &streams, true)
}

func runNativeEndpointCrossHostJourney(t *testing.T, streams *[2]endpointfixture.JourneyStream, filePublication bool) {
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
	var spends *installedRecoverySpends
	if streams != nil {
		spends = &installedRecoverySpends{}
	}
	f, store := installedServiceNetwork(t, carrier, &installedReceiverEvidence{recovery: spends})
	network := endpointfixture.BuildSignedNetwork(t, f.spec)
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

	repoRoot := endpointfixture.RepositoryRoot(t)
	participantBinary, nextBinary, workerBinary := journeyCrossHostBinaries(t, repoRoot)
	const participantTimeout = fullInstalledJourneyParticipantTimeout
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
	crossHostCopy(t, publisherSSH, workerBinary, remoteDir+"/ardents-document-worker")
	crossHostRun(t, t.Context(), publisherSSH, "chmod 0555 "+remoteDir+"/journey.test "+remoteDir+"/ardents-next "+remoteDir+"/ardents-document-worker\n")

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
				" ARDENTS_PREBUILT_NEXT=%s/ardents-next ARDENTS_PREBUILT_DOCUMENT_WORKER=%s/ardents-document-worker"+
				" %s=%s %s=%s %s=%s %s=%s"+
				" %s/journey.test -test.run='^TestNativeEndpointPlatformLedJourneyParticipant$' -test.v -test.timeout=%s\n",
				remoteDir, remoteDir,
				endpointfixture.JourneyRootEnv, bases[i], endpointfixture.JourneyRoleEnv, role, endpointfixture.JourneyNonceEnv, nonces[i], endpointfixture.JourneyAddressEnv, address,
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
			"ARDENTS_PREBUILT_DOCUMENT_WORKER="+workerBinary,
			endpointfixture.JourneyRootEnv+"="+bases[i],
			endpointfixture.JourneyRoleEnv+"="+role,
			endpointfixture.JourneyNonceEnv+"="+nonces[i],
			endpointfixture.JourneyAddressEnv+"="+address)
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
			hello := endpointfixture.ReceivePairMessage(t, conn)
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
			endpointfixture.SendPairMessage(t, peers[i], endpointfixture.PairMessage{Operation: "network", Network: &network})
		}
	}
	var declared [2][3]string
	admit := func(i int) [3]string {
		conn := peers[i]
		approvalRoot := filepath.Join(t.TempDir(), roles[i])
		if err := os.MkdirAll(approvalRoot, 0700); err != nil {
			t.Fatal(err)
		}
		admitted := endpointfixture.AdmitJourneyParticipant(t, conn, func(role string, request []byte) []byte {
			return installedServiceApproval(t, approvalRoot, role, request, f)
		})
		declared[i] = admitted.Principals
		return admitted.Original
	}
	exchange := func(index int, command endpointfixture.PairMessage) endpointfixture.PairMessage {
		return endpointfixture.ExchangeJourneyCommand(t, peers[index], index, command)
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
		if declared[0][j] == "" || declared[1][j] == "" || declared[0][j] == declared[1][j] {
			t.Fatal("participants share a declared principal")
		}
	}
	if streams != nil {

		before := sampleTransferRSS(t, exchange, identities)
		publicationCommand := "stream-publish"
		if filePublication {
			publicationCommand = "file-publish"
		}
		published := exchange(0, endpointfixture.PairMessage{Operation: publicationCommand, Stream: &streams[0]})
		if published.Link == "" || published.Original != identities[0] {
			t.Fatal("installed stream publisher supplied no Link for its original invocation")
		}
		started := time.Now()
		read := exchange(1, endpointfixture.PairMessage{Operation: "stream-connect", Link: published.Link, Stream: &streams[1]})
		results := []endpointfixture.PairMessage{read}
		if filePublication {
			if withdrawn := exchange(0, endpointfixture.PairMessage{Operation: "withdraw"}); withdrawn.Original != identities[0] {
				t.Fatal("file was withdrawn by a different invocation")
			}
		} else {
			results = append(results, exchange(0, endpointfixture.PairMessage{Operation: "stream-result"}))
		}
		for i, result := range results {
			if result.Failure != "" || result.Original != identities[1-i] {
				t.Fatalf("installed stream direction %d failed or changed invocation: %s", i, result.Failure)
			}
			expected := endpointfixture.NewJourneyStreamOutput()
			if _, err := io.Copy(expected, streams[i].Reader()); err != nil {
				t.Fatal(err)
			}
			if result.Proof == nil || *result.Proof != expected.Proof() || len(result.Body) != 0 {
				t.Fatalf("installed stream direction %d differs: observed %+v, expected %+v", i, result.Proof, expected.Proof())
			}
		}
		if duration := time.Since(started); !filePublication && duration <= 60*time.Second {
			t.Fatalf("installed stream completed within the old Reader window: %s", duration)
		}
		logTransferRSS(t, before, sampleTransferRSS(t, exchange, identities))
		for i := range peers {
			exchange(i, endpointfixture.PairMessage{Operation: "finish"})
			_ = peers[i].Close()
			peers[i] = nil
			waitErr := commands[i].Wait()
			joined[i] = true
			t.Log(outputs[i].String())
			if waitErr != nil {
				t.Fatal("installed stream participant did not join", waitErr)
			}
		}
		requireTransferAccounting(t, spends, streams[0].Bytes)
		t.Logf("two real admitted host managers verified %d published bytes and %d reply bytes", streams[0].Bytes, streams[1].Bytes)
		return
	}
	publishStarted := time.Now()
	published := exchange(0, endpointfixture.PairMessage{Operation: "publish", Body: body})
	if published.Link == "" || published.Original != identities[0] {
		t.Fatal("installed publisher supplied no Link for its original invocation")
	}
	first := endpointfixture.InstalledPairStoredProof(t, store, f.profile.NetworkID, f.profile.Digest, published.Link)
	read := func() {
		result := exchange(1, endpointfixture.PairMessage{Operation: "read", Link: published.Link})
		if !bytes.Equal(result.Body, body) || result.Original != identities[1] {
			t.Fatal("installed reader changed Application bytes or invocation")
		}
	}
	read()
	endpointfixture.InstalledPairAwaitRefresh(t, store, f.profile.NetworkID, f.profile.Digest, published.Link, first, publishStarted)
	replacement := endpointfixture.InstalledPairStoredProof(t, store, f.profile.NetworkID, f.profile.Digest, published.Link)
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
	if current := endpointfixture.InstalledPairStoredProof(t, store, f.profile.NetworkID, f.profile.Digest, published.Link); current.Digest() != replacement.Digest() {
		t.Fatal("cross-host predecessor retirement changed the accepting replacement")
	}
	read()
	if withdrawn := exchange(0, endpointfixture.PairMessage{Operation: "withdraw"}); withdrawn.Original != identities[0] {
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
		response := endpointfixture.RestartJourneyParticipant(t, peers[i], func(role string, request []byte) []byte {
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

func sampleTransferRSS(t *testing.T, exchange func(int, endpointfixture.PairMessage) endpointfixture.PairMessage, identities [2][3]string) [2]uint64 {
	t.Helper()
	var peaks [2]uint64
	for i := range peaks {
		result := exchange(i, endpointfixture.PairMessage{Operation: "resources"})
		if result.Original != identities[i] || result.PeakRSSKiB == 0 {
			t.Fatal("peak resident memory sample lost the original Endpoint")
		}
		peaks[i] = result.PeakRSSKiB
	}
	return peaks
}

func logTransferRSS(t *testing.T, before, after [2]uint64) {
	t.Helper()
	for i, peak := range after {
		if peak < before[i] {
			t.Fatal("Endpoint peak resident memory regressed")
		}
		t.Logf("Endpoint %d peak RSS KiB before=%d after=%d growth=%d", i, before[i], peak, peak-before[i])
	}
}

// Each segmented transfer requires two fresh Rendezvous admissions per
// Connection. Related controls can add traffic but cannot reuse a token or role
// channel. Check the genuine Receiving and Hosting owners after both users join.
func requireTransferAccounting(t *testing.T, proof *installedRecoverySpends, payload int64) {
	t.Helper()
	limit := int64(endpointstream.ByteStreamSegmentDirectionalLimit)
	minimum := 2 * ((payload + limit - 1) / limit)
	actual := proof.joinAccepted.Load()
	if actual < minimum || actual%2 != 0 {
		t.Fatalf("accepted %d JOIN roles, need at least %d complete fresh pairs", actual, minimum)
	}
	if err := proof.presentations.Check(int(actual)); err != nil {
		t.Fatal(err)
	}
	proof.requireJoinBudgetReleased(t, t.Context())
	observation, err := proof.joinBudget.Observe(t.Context())
	if err != nil || observation.UsedBytes == 0 {
		t.Fatalf("JOIN Hosting usage was not retained: %+v, %v", observation, err)
	}
	t.Logf("fresh JOIN admissions=%d, JOIN Hosting used bytes=%d, retained reserve bytes=%d",
		actual, observation.UsedBytes, observation.ReservedBytes)
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
// consumer and document worker as STATIC executables, so the controller-host build
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
	participant := build("journey.test", "./tests/e2e/endpointinstalled", "test", "-c", "-tags=endpoint_pair_native,document_worker_installed", "-trimpath", "-buildvcs=false", "-ldflags=-w")
	next := build("ardents-next", "./cmd/ardents-next", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w")
	workerPath := build("ardents-document-worker", "./internal/application/document/worker", "build", "-trimpath", "-buildvcs=false", "-ldflags=-w")
	return participant, next, workerPath
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
