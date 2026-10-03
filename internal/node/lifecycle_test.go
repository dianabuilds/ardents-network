package node

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"errors"
	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"io"
	"math/big"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPreparedCancellationRetainsTerminalEventFailure(t *testing.T) {
	outputErr := errors.New("terminal event output failed")
	var observed Event
	config := runtimeConfig{Config: Config{Emit: func(_ context.Context, event Event) error {
		observed = event
		return outputErr
	}}, now: func() time.Time { return time.Unix(100, 0).UTC() }}
	machine := stateMachine{current: statePrepared}
	result, err := terminalWithoutDuty(config, &machine, state.NodeDuty{Assignment: "closed_issuer"}, context.Canceled)
	if result.State != stateNames[stateFailed] || observed.State != stateNames[stateFailed] ||
		!errors.Is(err, context.Canceled) || !errors.Is(err, outputErr) {
		t.Fatalf("prepared cancellation = %+v, event %+v, %v", result, observed, err)
	}
}

func TestWithdrawDoesNotPublishSuccessWhenRoleDrainFails(t *testing.T) {
	cleanupErr := errors.New("injected role drain failure")
	var events []Event
	config := runtimeConfig{Config: Config{Emit: func(_ context.Context, event Event) error {
		events = append(events, event)
		return nil
	}}, now: func() time.Time { return time.Unix(100, 0).UTC() }}
	machine := stateMachine{current: stateReady}
	server := &dutyHandle{Stop: func() {}, Drain: func(context.Context) error { return cleanupErr }}
	result, err := withdraw(config, &machine, server, state.NodeDuty{Assignment: "rendezvous"}, "test withdrawal")
	if !errors.Is(err, cleanupErr) || result.State == stateNames[stateWithdrawn] {
		t.Fatalf("withdraw result = %+v, %v", result, err)
	}
	for _, event := range events {
		if event.State == stateNames[stateWithdrawn] {
			t.Fatal("WITHDRAWN was published after unknown role cleanup")
		}
	}
}

const (
	testProbeHeaderBytes  = 4 + 1 + 6*32 + 2
	testProbePayloadBytes = 32
	testLifecycleWait     = 15 * time.Second
)

var testProbeProfile = sha256.Sum256([]byte("h3-role-probe-v1"))

type lifecycleFixture struct {
	config      Config
	snapshot    state.NodeDuty
	serverRoots *x509.CertPool
	client      tls.Certificate
	serverName  string
	mu          sync.RWMutex
}

type issuedCertificate struct {
	certificate tls.Certificate
	public      ed25519.PublicKey
	private     ed25519.PrivateKey
	leaf        *x509.Certificate
	pem         []byte
}

func TestRunServesBoundProbeThenWithdrawsOnRecordRemoval(t *testing.T) {
	fixture := newLifecycleFixture(t)
	events := make(chan Event, 16)
	fixture.config.Current = func() (state.NodeDuty, error) {
		fixture.mu.RLock()
		defer fixture.mu.RUnlock()
		return fixture.snapshot, nil
	}
	fixture.config.Emit = func(_ context.Context, event Event) error { events <- event; return nil }
	result := make(chan Result, 1)
	errors := make(chan error, 1)
	go func() {
		value, err := Run(context.Background(), fixture.config)
		result <- value
		errors <- err
	}()
	waitForState(t, events, "READY")
	roles, err := localroles.Open(localroles.Config{Root: fixture.config.LocalRoleStateRoot, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	if conflict, err := roles.Conflict(fixture.snapshot.NodeID, sha256.Sum256([]byte(fixture.snapshot.DeclaredFamily))); err != nil || !conflict {
		t.Fatalf("READY Node local duty = %v, %v", conflict, err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
	connection := dialProbe(t, fixture)
	request := encodeProbeRequest(fixture.snapshot, [32]byte{9}, []byte("bounded work"))
	if _, err := connection.Write(request); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, testProbeHeaderBytes+sha256.Size)
	if _, err := io.ReadFull(connection, response); err != nil || string(response[:4]) != "ARNS" {
		t.Fatalf("probe response = %q, %v", response[:4], err)
	}
	_ = connection.Close()
	replay := dialProbe(t, fixture)
	if _, err := replay.Write(request); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(replay, response); err == nil {
		t.Fatal("replayed harness nonce was accepted")
	}
	_ = replay.Close()
	established := dialProbe(t, fixture)
	fixture.mu.Lock()
	fixture.snapshot.RecordPresent = false
	fixture.mu.Unlock()
	waitForState(t, events, "DRAINING")
	request = encodeProbeRequest(fixture.snapshot, [32]byte{10}, []byte("accepted before drain"))
	if _, err := established.Write(request); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(established, response); err != nil {
		t.Fatalf("established probe did not finish during drain: %v", err)
	}
	_ = established.Close()
	select {
	case value := <-result:
		if value.State != "WITHDRAWN" || value.Assignment != "domain-a" {
			t.Fatalf("terminal result = %+v", value)
		}
	case <-time.After(testLifecycleWait):
		t.Fatal("Node did not withdraw after record removal")
	}
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
	roles, err = localroles.Open(localroles.Config{Root: fixture.config.LocalRoleStateRoot, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	if conflict, err := roles.Conflict(fixture.snapshot.NodeID, sha256.Sum256([]byte(fixture.snapshot.DeclaredFamily))); err != nil || conflict {
		t.Fatalf("withdrawn Node local duty = %v, %v", conflict, err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
	states := drainStates(events)
	if len(states) < 1 || states[len(states)-1] != "WITHDRAWN" {
		t.Fatalf("terminal events = %v", states)
	}
	if connection, err := tls.Dial("tcp", fixture.config.Probe.ListenAddress, probeClientTLS(fixture)); err == nil {
		_ = connection.Close()
		t.Fatal("withdrawn Node still accepts new work")
	}
}

func TestDrainCancelsEstablishedProbeAtDeadline(t *testing.T) {
	fixture := newLifecycleFixture(t)
	fixture.config.Probe.DrainTimeout = 30 * time.Millisecond
	events := make(chan Event, 16)
	fixture.config.Current = func() (state.NodeDuty, error) {
		fixture.mu.RLock()
		defer fixture.mu.RUnlock()
		return fixture.snapshot, nil
	}
	fixture.config.Emit = func(_ context.Context, event Event) error { events <- event; return nil }
	result := make(chan Result, 1)
	go func() { value, _ := Run(context.Background(), fixture.config); result <- value }()
	waitForState(t, events, "READY")
	established := dialProbe(t, fixture)
	fixture.mu.Lock()
	fixture.snapshot.Fresh = false
	fixture.mu.Unlock()
	waitForState(t, events, "DRAINING")
	select {
	case terminal := <-result:
		if terminal.State != "WITHDRAWN" {
			t.Fatalf("terminal result = %+v", terminal)
		}
	case <-time.After(testLifecycleWait):
		t.Fatal("established probe outlived drain deadline")
	}
	if _, err := established.Write([]byte{1}); err == nil {
		buffer := make([]byte, 1)
		if _, err = established.Read(buffer); err == nil {
			t.Fatal("drain deadline left the established socket usable")
		}
	}
	_ = established.Close()
}

func TestProtectPreservesEstablishedWorkAndRejectsNewAdmission(t *testing.T) {
	fixture := newLifecycleFixture(t)
	events := make(chan Event, 32)
	fixture.config.Current = func() (state.NodeDuty, error) { return fixture.snapshot, nil }
	fixture.config.Emit = func(_ context.Context, event Event) error { events <- event; return nil }
	fixture.config.ResourceProfile = "h3-np1-v1"
	var protect atomic.Bool
	fixture.config.ResourceMeasure = func() (resource.Sample, error) {
		if !protect.Load() {
			return resource.Sample{}, nil
		}
		return resource.Sample{HighEvents: 1}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan Result, 1)
	go func() { value, _ := Run(ctx, fixture.config); result <- value }()
	waitForState(t, events, "READY")
	established := dialProbe(t, fixture)
	protect.Store(true)
	waitForState(t, events, "PROTECT")
	if connection, err := tls.Dial("tcp", fixture.config.Probe.ListenAddress, probeClientTLS(fixture)); err == nil {
		_ = connection.Close()
		t.Fatal("PROTECT accepted new expensive work")
	}
	request := encodeProbeRequest(fixture.snapshot, [32]byte{11}, []byte("established work survives"))
	if _, err := established.Write(request); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, testProbeHeaderBytes+sha256.Size)
	if _, err := io.ReadFull(established, response); err != nil {
		t.Fatalf("PROTECT interrupted established work: %v", err)
	}
	_ = established.Close()
	cancel()
	select {
	case terminal := <-result:
		if terminal.State != "WITHDRAWN" {
			t.Fatalf("terminal result = %+v", terminal)
		}
	case <-time.After(testLifecycleWait):
		t.Fatal("protected Node did not shut down")
	}
}

func TestRunFailsBeforeReadinessOnKeyMismatch(t *testing.T) {
	fixture := newLifecycleFixture(t)
	fixture.snapshot.NodePublicKey[0]++
	fixture.config.Current = func() (state.NodeDuty, error) { return fixture.snapshot, nil }
	events := make(chan Event, 4)
	fixture.config.Emit = func(_ context.Context, event Event) error { events <- event; return nil }
	result, err := Run(context.Background(), fixture.config)
	if err == nil || result.State != "FAILED" {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
	states := drainStates(events)
	for _, state := range states {
		if state == "READY" {
			t.Fatalf("key-mismatched Node reached READY: %v", states)
		}
	}
}

func TestRunReportsReadinessLossDuringQuarantine(t *testing.T) {
	fixture := newLifecycleFixture(t)
	fixture.config.Quarantine = 10 * time.Millisecond
	var calls atomic.Int32
	fixture.config.Current = func() (state.NodeDuty, error) {
		snapshot := fixture.snapshot
		if calls.Add(1) > 1 {
			snapshot.ProbeCapacity = 0
		}
		return snapshot, nil
	}
	fixture.config.Emit = func(context.Context, Event) error { return nil }
	result, err := Run(context.Background(), fixture.config)
	if err == nil || result.State != "FAILED" || !strings.Contains(result.Reason, "assignment lost readiness during quarantine: profile or deterministic assignment is inactive") {
		t.Fatalf("quarantine readiness result = %+v, %v", result, err)
	}
}

func TestPreparedNodeFailsWhenRecordDisappears(t *testing.T) {
	fixture := newLifecycleFixture(t)
	fixture.snapshot.ProbeCapacity = 0
	events := make(chan Event, 8)
	fixture.config.Current = func() (state.NodeDuty, error) {
		fixture.mu.RLock()
		defer fixture.mu.RUnlock()
		return fixture.snapshot, nil
	}
	fixture.config.Emit = func(_ context.Context, event Event) error { events <- event; return nil }
	result := make(chan Result, 1)
	go func() { value, _ := Run(context.Background(), fixture.config); result <- value }()
	waitForState(t, events, "PREPARED")
	fixture.mu.Lock()
	fixture.snapshot.RecordPresent = false
	fixture.mu.Unlock()
	select {
	case value := <-result:
		if value.State != "FAILED" {
			t.Fatalf("terminal result = %+v", value)
		}
	case <-time.After(testLifecycleWait):
		t.Fatal("PREPARED Node did not terminate after record removal")
	}
}

func TestResolveRejectsInvalidOrUnboundedClientTrust(t *testing.T) {
	fixture := newLifecycleFixture(t)
	fixture.config.Current = func() (state.NodeDuty, error) { return fixture.snapshot, nil }
	fixture.config.Emit = func(context.Context, Event) error { return nil }
	for _, roots := range [][]byte{[]byte("not PEM"), make([]byte, (64<<10)+1)} {
		config := fixture.config
		config.Probe.ClientRootPEM = roots
		if _, err := resolveConfig(config); err == nil {
			t.Fatal("invalid client trust was accepted")
		}
	}
}

func newLifecycleFixture(t *testing.T) *lifecycleFixture {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	identityPublic, identityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := createCertificate(t, nil, "test-root", true)
	server := createCertificate(t, &ca, "node.test", false)
	client := createCertificate(t, &ca, "harness.test", false)
	address := reserveAddress(t)
	snapshot := state.NodeDuty{Generation: "generation-1", NetworkID: [32]byte{1}, Epoch: 1,
		Digest: [32]byte{3}, EpochValidFrom: now.Add(-time.Hour), ValidUntil: now.Add(time.Hour),
		Profile: "h3-role-probe-v1", Fresh: true, RecordPresent: true, NodeID: [32]byte{2}, DeclaredFamily: "family-a",
		RecordValidFrom: now.Add(-time.Hour), RecordValidUntil: now.Add(time.Hour), ProbeEndpoint: address, ProbeCapacity: 4,
		Assignment: "domain-a", AssignmentDigest: [32]byte{4}}
	copy(snapshot.NodePublicKey[:], identityPublic)
	pinBytes, err := x509.MarshalPKIXPublicKey(client.public)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(ca.pem)
	fixture := &lifecycleFixture{snapshot: snapshot, serverRoots: roots, client: client.certificate, serverName: "node.test"}
	fixture.config = Config{NetworkID: snapshot.NetworkID, NodeID: snapshot.NodeID, IdentityKey: identityPrivate,
		LocalRoleStateRoot: localRoleStateRoot(t),
		Probe: ProbeConfig{ListenAddress: address, Certificate: server.certificate, ClientRootPEM: ca.pem,
			ClientKeyPins: [][32]byte{sha256.Sum256(pinBytes)}, MaximumDuty: 2 * time.Second, DrainTimeout: time.Second},
		PollInterval: 10 * time.Millisecond, Quarantine: time.Millisecond,
		CheckPlacement: func() error { return nil }}
	return fixture
}

func createCertificate(t *testing.T, parent *issuedCertificate, name string, authority bool) issuedCertificate {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name},
		DNSNames: []string{name}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: authority, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature}
	issuer, issuerKey := template, private
	if authority {
		template.KeyUsage |= x509.KeyUsageCertSign
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
		issuer, issuerKey = parent.leaf, parent.private
	}
	raw, err := x509.CreateCertificate(rand.Reader, template, issuer, public, issuerKey)
	if err != nil {
		t.Fatal(err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw})
	if parent != nil {
		certificatePEM = append(certificatePEM, parent.pem...)
	}
	keyRaw, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyRaw})
	certificate, err := tls.X509KeyPair(certificatePEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	return issuedCertificate{certificate: certificate, public: public, private: private, leaf: leaf, pem: certificatePEM}
}

func reserveAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func dialProbe(t *testing.T, fixture *lifecycleFixture) *tls.Conn {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		connection, err := tls.Dial("tcp", fixture.config.Probe.ListenAddress, probeClientTLS(fixture))
		if err == nil {
			return connection
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func probeClientTLS(fixture *lifecycleFixture) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, RootCAs: fixture.serverRoots,
		ServerName: fixture.serverName, Certificates: []tls.Certificate{fixture.client}, SessionTicketsDisabled: true}
}

func encodeProbeRequest(snapshot state.NodeDuty, nonce [32]byte, payload []byte) []byte {
	request := make([]byte, testProbeHeaderBytes+testProbePayloadBytes)
	copy(request, "ARNP")
	request[4] = 1
	offset := 5
	for _, value := range [][32]byte{snapshot.NetworkID, testProbeProfile, snapshot.Digest, snapshot.NodeID, snapshot.AssignmentDigest, nonce} {
		copy(request[offset:offset+32], value[:])
		offset += 32
	}
	binary.BigEndian.PutUint16(request[offset:], testProbePayloadBytes)
	digest := sha256.Sum256(payload)
	copy(request[testProbeHeaderBytes:], digest[:])
	return request
}

func waitForState(t *testing.T, events <-chan Event, state string) {
	t.Helper()
	_ = waitForStateEvent(t, events, state)
}

func waitForStateEvent(t *testing.T, events <-chan Event, state string) Event {
	t.Helper()
	deadline := time.After(testLifecycleWait)
	for {
		select {
		case event := <-events:
			if event.State == state {
				return event
			}
		case <-deadline:
			t.Fatalf("Node did not reach %s", state)
		}
	}
}

func drainStates(events <-chan Event) []string {
	var states []string
	for {
		select {
		case event := <-events:
			states = append(states, event.State)
		default:
			return states
		}
	}
}

type hostingLifetimeTestHost struct {
	closed atomic.Int32
	err    error
}

func (host *hostingLifetimeTestHost) Sample(context.Context, time.Duration) (hostingbudget.Sample, error) {
	return hostingbudget.Sample{}, nil
}

func (host *hostingLifetimeTestHost) Reserve(context.Context, hostingbudget.Traffic, hostingbudget.Traffic, time.Time) (hosting.Reservation, error) {
	return nil, nil
}

func (host *hostingLifetimeTestHost) Close() error {
	host.closed.Add(1)
	return host.err
}

func TestWithdrawTransfersHostingCloseToUnjoinedLateChild(t *testing.T) {
	host := &hostingLifetimeTestHost{}
	config := runtimeConfig{Config: Config{Emit: func(context.Context, Event) error { return nil }},
		now: func() time.Time { return time.Unix(100, 0).UTC() }}
	var released atomic.Int32
	config.cleanup = &dutyCleanup{host: hosting.NewLifetime(host), retained: true, release: func() error {
		released.Add(1)
		return nil
	}}
	machine := stateMachine{current: stateReady}
	joined := make(chan struct{})
	server := &dutyHandle{Stop: func() {}, Joined: joined,
		Drain: func(context.Context) error { return context.DeadlineExceeded }}
	result, err := withdraw(config, &machine, server, state.NodeDuty{Assignment: "rendezvous"}, "test withdrawal")
	if !errors.Is(err, context.DeadlineExceeded) || result.State == stateNames[stateWithdrawn] {
		t.Fatalf("withdraw result = %+v, %v", result, err)
	}
	// The deferred Run-level close must transfer, not close: a late child can
	// still release its Hosting reservation against the shared handle (F-62).
	if err := config.cleanup.Close(); err != nil {
		t.Fatalf("transferred Hosting close failed: %v", err)
	}
	if host.closed.Load() != 0 {
		t.Fatal("shared Hosting closed before the late child joined")
	}
	if released.Load() != 0 {
		t.Fatal("local role was released before the late child joined")
	}
	close(joined)
	deadline := time.After(time.Second)
	for host.closed.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("shared Hosting did not close after the late child joined")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if released.Load() != 0 {
		t.Fatal("failed withdrawal released its local role before expiry")
	}
}

func TestWithdrawPublishesSuccessOnlyAfterProcessCleanup(t *testing.T) {
	for _, selected := range []struct {
		name       string
		hostErr    error
		releaseErr error
	}{
		{name: "complete cleanup"},
		{name: "host close", hostErr: errors.New("host close failed")},
		{name: "local role removal", releaseErr: errors.New("role removal failed")},
	} {
		t.Run(selected.name, func(t *testing.T) {
			host := &hostingLifetimeTestHost{err: selected.hostErr}
			var events []Event
			var released bool
			config := runtimeConfig{Config: Config{Emit: func(_ context.Context, event Event) error {
				if event.State == "WITHDRAWN" && (host.closed.Load() != 1 || !released) {
					t.Error("WITHDRAWN was emitted before Hosting close and local role removal")
				}
				events = append(events, event)
				return nil
			}}, now: func() time.Time { return time.Unix(100, 0).UTC() }}
			config.cleanup = &dutyCleanup{host: hosting.NewLifetime(host), retained: true, release: func() error {
				released = true
				return selected.releaseErr
			}}
			machine := stateMachine{current: stateReady}
			server := &dutyHandle{Stop: func() {}, Drain: func(context.Context) error { return nil }}
			result, err := withdraw(config, &machine, server, state.NodeDuty{Assignment: "rendezvous"}, "test withdrawal")
			want := selected.hostErr
			if want == nil {
				want = selected.releaseErr
			}
			if want == nil && (result.State != "WITHDRAWN" || err != nil) ||
				want != nil && (result.State != "FAILED" || !errors.Is(err, want)) {
				t.Fatalf("withdraw result = %+v, %v", result, err)
			}
			if host.closed.Load() != 1 || released != (selected.hostErr == nil) {
				t.Fatalf("cleanup order: host closes = %d, role released = %v", host.closed.Load(), released)
			}
			for _, event := range events {
				if want != nil && event.State == "WITHDRAWN" {
					t.Fatal("WITHDRAWN published before complete process cleanup")
				}
			}
		})
	}
}

func TestWithdrawRejectsUnjoinedDrainResult(t *testing.T) {
	host := &hostingLifetimeTestHost{}
	joined := make(chan struct{})
	var released bool
	var events []Event
	config := runtimeConfig{Config: Config{Emit: func(_ context.Context, event Event) error {
		events = append(events, event)
		return nil
	}}, now: func() time.Time { return time.Unix(100, 0).UTC() }}
	config.cleanup = &dutyCleanup{host: hosting.NewLifetime(host), retained: true, release: func() error {
		released = true
		return nil
	}}
	machine := stateMachine{current: stateReady}
	server := &dutyHandle{Joined: joined, Stop: func() {}, Drain: func(context.Context) error { return nil }}
	result, err := withdraw(config, &machine, server, state.NodeDuty{Assignment: "rendezvous"}, "test withdrawal")
	if result.State != "FAILED" || err == nil {
		t.Fatalf("unjoined drain result = %+v, %v", result, err)
	}
	if err := config.cleanup.Close(); err != nil || released || host.closed.Load() != 0 {
		t.Fatalf("unjoined resources were released: %v, %v, %d", err, released, host.closed.Load())
	}
	for _, event := range events {
		if event.State == "WITHDRAWN" {
			t.Fatal("unjoined role was reported withdrawn")
		}
	}
	close(joined)
	deadline := time.After(time.Second)
	for host.closed.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("shared Hosting did not close after the unjoined role finished")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func TestUnjoinedRoleKeepsDurableLocalConflict(t *testing.T) {
	fixture := newLifecycleFixture(t)
	config := runtimeConfig{Config: fixture.config, now: time.Now}
	if err := retainLocalDuty(config, fixture.snapshot, "live"); err != nil {
		t.Fatal(err)
	}
	joined := make(chan struct{})
	config.cleanup = &dutyCleanup{retained: true, release: func() error { return releaseLocalDuty(config) }}
	config.cleanup.deferUntil(joined)
	if err := config.cleanup.Close(); err != nil {
		t.Fatal(err)
	}
	roles, err := localroles.Open(localroles.Config{Root: config.LocalRoleStateRoot, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	conflict, checkErr := roles.Conflict(fixture.snapshot.NodeID, sha256.Sum256([]byte(fixture.snapshot.DeclaredFamily)))
	closeErr := roles.Close()
	if checkErr != nil || closeErr != nil || !conflict {
		t.Fatalf("local duty after unknown join = %v, check %v, close %v", conflict, checkErr, closeErr)
	}
	close(joined)
}
