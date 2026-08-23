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
	"io"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

const (
	testProbeHeaderBytes  = 4 + 1 + 6*32 + 2
	testProbePayloadBytes = 32
	testLifecycleWait     = 15 * time.Second
)

var testProbeProfile = sha256.Sum256([]byte("h3-role-probe-v1"))

type lifecycleFixture struct {
	config      Config
	snapshot    dutyFacts
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
	fixture.config.Current = func() (DutyView, error) {
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
	fixture.config.Current = func() (DutyView, error) {
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
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
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
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
	fixture.config.Emit = func(context.Context, Event) error { return nil }
	result, err := Run(context.Background(), fixture.config)
	if err == nil || result.State != "FAILED" {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
	if connection, dialErr := net.DialTimeout("tcp", fixture.config.Probe.ListenAddress, 50*time.Millisecond); dialErr == nil {
		_ = connection.Close()
		t.Fatal("failed Node opened its listener")
	}
}

func TestPreparedNodeFailsWhenRecordDisappears(t *testing.T) {
	fixture := newLifecycleFixture(t)
	fixture.snapshot.ProbeCapacity = 0
	events := make(chan Event, 8)
	fixture.config.Current = func() (DutyView, error) {
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
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
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
	snapshot := dutyFacts{Generation: "generation-1", NetworkID: [32]byte{1}, Epoch: 1,
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
		LocalRoleStateRoot: t.TempDir(),
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

func encodeProbeRequest(snapshot dutyFacts, nonce [32]byte, payload []byte) []byte {
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
	deadline := time.After(testLifecycleWait)
	for {
		select {
		case event := <-events:
			if event.State == state {
				return
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
