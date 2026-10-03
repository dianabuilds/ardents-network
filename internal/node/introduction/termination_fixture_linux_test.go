//go:build linux

package introduction

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"math/big"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	admissionissuer "github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	introductioncapsule "github.com/dianabuilds/ardents-network/internal/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

// This fixture supplies already-authenticated State projections and offline
// permission authority. It runs the production shared listener, Node handler,
// inner TLS, token issuer, receiving spend ledger and Hosting. It is not installed qualification.
// The only transport seam pauses a particular actual Outer CLOSE write.
type terminationGate struct {
	lane            atomic.Uint32
	physical        atomic.Bool
	physicalEntered chan struct{}
	physicalResume  chan struct{}
	physicalOnce    sync.Once
	physicalFailure error
	reply           atomic.Bool
	tlsClose        atomic.Bool
	entered         chan struct{}
	resume          chan struct{}
	once            sync.Once
	hit             sync.Once
	failure         error
}

func (gate *terminationGate) release() { gate.once.Do(func() { close(gate.resume) }) }
func (gate *terminationGate) releasePhysical() {
	gate.physicalOnce.Do(func() { close(gate.physicalResume) })
}

type terminationConnection struct {
	closeOnce sync.Once
	closeErr  error
	net.Conn
	gate *terminationGate
}

func (connection *terminationConnection) Write(raw []byte) (int, error) {
	if len(raw) >= ardp.HeaderSize && ardp.ValidHeader(raw[:ardp.HeaderSize]) && (raw[6] == ardp.KindClose && !connection.gate.reply.Load() && !connection.gate.tlsClose.Load() || raw[6] == ardp.KindBytes && (connection.gate.reply.Load() || connection.gate.tlsClose.Load() && len(raw) == ardp.HeaderSize+24)) && binary.BigEndian.Uint32(raw[8:12]) == connection.gate.lane.Load() {
		connection.gate.hit.Do(func() { close(connection.gate.entered) })
		<-connection.gate.resume
		if connection.gate.failure != nil {
			return 0, connection.gate.failure
		}
	}
	return connection.Conn.Write(raw)
}

func (connection *terminationConnection) Close() error {
	connection.closeOnce.Do(func() {
		if connection.gate.physical.Load() {
			close(connection.gate.physicalEntered)
			<-connection.gate.physicalResume
		}
		connection.closeErr = errors.Join(connection.Conn.Close(), connection.gate.physicalFailure)
	})
	return connection.closeErr
}

type terminationListener struct {
	carrier.ClosedSharedCarrierListener
	gate *terminationGate
}

func (listener *terminationListener) Accept(ctx context.Context, timeout time.Duration) (carrier.ClosedSharedCarrier, error) {
	accepted, err := listener.ClosedSharedCarrierListener.Accept(ctx, timeout)
	if err == nil {
		accepted.Connection = &terminationConnection{Conn: accepted.Connection, gate: listener.gate}
	}
	return accepted, err
}

type terminationFixture struct {
	expectCleanupFailure bool
	server               *Server
	gate                 *terminationGate
	hosting              *resource.Hosting
	released             atomic.Uint32
	releaseDone          chan struct{}
	peer                 carrier.Carrier
	writer               sync.Mutex
	pipesMu              sync.Mutex
	pipes                map[uint32]net.Conn
	readerDone           chan struct{}
	profile              state.ClosedProfileView
	public               [32]byte
	controls             [][]byte
	submissions          [][]byte
	operation            []byte
	deadline             time.Time
	workers              sync.WaitGroup
	nonce                [32]byte
}

func terminalRequest(raw []byte, nonce [32]byte) []byte {
	body := make([]byte, 16<<10)
	body[0] = 1
	copy(body[1:33], nonce[:])
	copy(body[33:], raw)
	return body
}
func (fixture *terminationFixture) write(frame ardp.Frame) error {
	fixture.writer.Lock()
	defer fixture.writer.Unlock()
	return ardp.WriteFrame(fixture.peer, frame)
}
func (fixture *terminationFixture) observe(t *testing.T) uint64 {
	t.Helper()
	got, err := fixture.hosting.Observe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return got.ReservedBytes
}
func newTerminationFixture(t *testing.T, transport carrier.CarrierProfile, failure error) *terminationFixture {
	t.Helper()
	fixture := &terminationFixture{gate: &terminationGate{entered: make(chan struct{}), resume: make(chan struct{}), physicalEntered: make(chan struct{}), physicalResume: make(chan struct{}), failure: failure}, releaseDone: make(chan struct{}, 8), pipes: make(map[uint32]net.Conn), readerDone: make(chan struct{})}
	fixture.gate.lane.Store(1)
	// Tokens and permissions expire at an hour boundary. Start with enough
	// real wall-clock authority for setup, the ten-second child and cleanup;
	// waiting near the boundary neither skips a case nor changes its clock.
	now := time.Now().UTC()
	if remaining := now.Truncate(time.Hour).Add(time.Hour).Sub(now); remaining < 30*time.Second {
		select {
		case <-time.After(remaining):
		case <-t.Context().Done():
			t.Fatal("cancelled while waiting for a complete token window")
		}
	}
	now = time.Now().UTC().Truncate(time.Second)
	window := now.Truncate(time.Hour)
	fixture.deadline = now.Add(10 * time.Second)
	certificate, public := nodeCertificate(t, 220, "introduction-termination")
	fixture.public = public
	peerCertificate, peerPublic := nodeCertificate(t, 221, "introduction-peer")
	_, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	profile := state.ClosedProfileView{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, Digest: [32]byte{4}, IssuerNodeID: [32]byte{5}, IssuerDutyGeneration: 1, Epoch: 1, NotBefore: window, NotAfter: window.Add(time.Hour)}
	copy(profile.IssuanceAuthorityKey[:], signer.Public().(ed25519.PublicKey))
	root := filepath.Join(t.TempDir(), "issuer")
	receipt, err := admissionissuer.InitializeClosedIssuerRoot(admissionissuer.ClosedIssuerRootConfig{Root: root, NetworkID: profile.NetworkID, NodeID: profile.IssuerNodeID, IdentityKey: certificate.PrivateKey.(ed25519.PrivateKey), NotBefore: profile.NotBefore, NotAfter: profile.NotAfter, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := admission.DecodeClosedIssuerProfile(receipt.Profile, ed25519.PublicKey(public[:]))
	if err != nil {
		t.Fatal(err)
	}
	profile.TokenKeyCount = uint8(len(inventory.Keys))
	for i, key := range inventory.Keys {
		profile.TokenKeys[i].WindowStart = key.WindowStart
		profile.TokenKeys[i].Class = uint8(key.Class)
		copy(profile.TokenKeys[i].SPKI[:], key.SPKI)
	}
	fixture.profile = profile
	issuer, err := admissionissuer.OpenClosedTokenIssuer(admissionissuer.ClosedTokenIssuerConfig{Root: root, NetworkID: profile.NetworkID, CurrentProfile: func() (state.ClosedProfileView, bool) { return profile, true }, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := issuer.Close(); err != nil {
			t.Error(err)
		}
	})
	_, holder, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	permission := admission.Permission{NetworkID: profile.NetworkID, IssuerNodeID: profile.IssuerNodeID, DutyGeneration: 1, PermissionID: [32]byte{8}, NotBefore: profile.NotBefore, NotAfter: profile.NotAfter, Maxima: [3]uint32{8, 0, 8}}
	copy(permission.HolderKey[:], holder.Public().(ed25519.PublicKey))
	copy(permission.Signature[:], ed25519.Sign(signer, admission.PermissionTranscript(permission)))
	challenge := token.ClosedTokenContext{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, IssuerNodeID: profile.IssuerNodeID, ReceiverNodeID: [32]byte{12}, ReceiverDutyGeneration: 1, Class: 3, WindowStart: window}
	prepare := func(contexts []token.ClosedTokenContext) *token.PendingClosedTokenBatch {
		p, err := token.PrepareClosedTokenBatch(token.ClosedTokenBatchConfig{Profile: profile, Contexts: contexts, Permission: permission, HolderKey: holder, Now: now})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(p.Discard)
		return p
	}
	bootstrap := prepare([]token.ClosedTokenContext{challenge, challenge, challenge})
	initialNonce := [32]byte{10}
	result, err := issuer.IssueTerminalOperation(terminalRequest(bootstrap.Request(), initialNonce))
	if err != nil {
		t.Fatal(err)
	}
	fixture.controls, err = bootstrap.FinalizeTerminalOperation(initialNonce, result)
	if err != nil {
		t.Fatal(err)
	}
	challenge.Class = 1
	pendingSubmission := prepare([]token.ClosedTokenContext{challenge})
	submissionNonce := [32]byte{12}
	submissionResult, err := issuer.IssueTerminalOperation(terminalRequest(pendingSubmission.Request(), submissionNonce))
	if err != nil {
		t.Fatal(err)
	}
	fixture.submissions, err = pendingSubmission.FinalizeTerminalOperation(submissionNonce, submissionResult)
	if err != nil {
		t.Fatal(err)
	}
	fixture.nonce = [32]byte{11}
	view := state.ClosedRouteView{Profile: profile, NodeCount: 1}
	view.Nodes[0] = state.ClosedRouteNodeView{NodeID: [32]byte{12}, RecordDigest: [32]byte{6}, DutyGeneration: 1, RoleDomain: 4, Subrole: 3}
	source := authority.Source{CurrentRoute: func() (state.ClosedRouteView, error) { return view, nil }, CurrentProfile: func() (state.ClosedProfileView, bool) { return profile, true }}
	snapshot := state.NodeDuty{Generation: hex.EncodeToString(profile.StateGeneration[:]), NetworkID: profile.NetworkID, Digest: profile.StateDigest, Epoch: 1, EpochValidFrom: window, ValidUntil: profile.NotAfter, RecordValidUntil: profile.NotAfter, Profile: carrier.ClosedRouteProfile, Fresh: true, NodeID: [32]byte{12}, RecordGeneration: 1}
	hostRoot := filepath.Join(t.TempDir(), "hosting")
	err = resource.InitializeHosting(hostRoot, resource.HostingPolicy{Provider: "fixture", Start: window, End: window.Add(time.Hour), Unit: "GiB", Quantity: 1, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	host, err := hosting.Open(hostRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := host.Close(); err != nil {
			t.Error(err)
		}
	})
	fixture.hosting, err = resource.OpenHosting(hostRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := fixture.hosting.Close(); err != nil {
			t.Error(err)
		}
	})
	spends, err := spending.Open(t.TempDir(), spending.Binding{NetworkID: profile.NetworkID, ProfileDigest: profile.Digest, ReceiverNodeID: [32]byte{12}, ReceiverDutyGeneration: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = spends.Close() })
	limits, err := route.NewClosedDutyLimits(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	config := Config{Authority: source, CurrentDuty: func() (state.NodeDuty, error) { return snapshot, nil }, Now: time.Now, VerifyAdmission: func(receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
		verify := hosting.ControlAdmissionVerifier(source, time.Now, receiver, host)
		return func(input route.ClosedAdmissionVerification) (route.ClosedAdmissionApproval, error) {
			approval, err := verify(input)
			if err == nil {
				release := approval.Release
				approval.Release = func() error { err := release(); fixture.released.Add(1); fixture.releaseDone <- struct{}{}; return err }
			}
			return approval, err
		}
	}}
	// Reserve then release a local address; the actual selected listener binds it.
	address, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := address.Addr().String()
	if err := address.Close(); err != nil {
		t.Fatal(err)
	}
	shared, err := carrier.ListenClosedSharedCarrier(transport, endpoint, certificate, func(key [32]byte) bool { return key == peerPublic }, 4)
	if err != nil {
		t.Fatal(err)
	}
	receiver, ok := source.Receiver(snapshot, ardp.PurposeIntroduction, time.Now())
	if !ok {
		t.Fatal("unavailable real receiver fixture")
	}
	ctx, cancel := context.WithCancel(context.Background())
	floor, err := spends.IntroductionSlots()
	if err != nil {
		cancel()
		shared.Close()
		t.Fatal(err)
	}
	fixture.server = &Server{config: config, receiver: receiver, certificate: certificate, listener: &terminationListener{ClosedSharedCarrierListener: shared, gate: fixture.gate}, slots: make(map[[32]byte]*closedIntroductionSlot), slotFloor: floor, spends: spends, limits: limits, capacity: make(chan struct{}, 4), cancel: cancel, done: make(chan error, 1), drained: make(chan struct{})}
	fixture.operation, err = terminal.EncodeRegistrationRequest(terminal.RegistrationRequest{Nonce: [32]byte{31}, Slot: [32]byte{32}, Revision: 1, Expiry: fixture.deadline.Add(-time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	go fixture.server.run(ctx)
	t.Cleanup(func() {
		fixture.gate.release()
		fixture.gate.releasePhysical()
		_ = fixture.server.stop()
		if fixture.peer != nil {
			_ = fixture.peer.Close()
		}
		fixture.pipesMu.Lock()
		for _, pipe := range fixture.pipes {
			_ = pipe.Close()
		}
		fixture.pipesMu.Unlock()
		fixture.workers.Wait()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := fixture.drain(ctx, 3*time.Second); err != nil && failure == nil && !fixture.expectCleanupFailure {
			t.Error(err)
		}
	})
	fixture.peer, err = carrier.OpenClosedNodeCarrier(t.Context(), carrier.ClosedNodeCarrierRequest{CarrierProfile: transport, Endpoint: endpoint, Certificate: peerCertificate, ExpectedPeerKey: public, Deadline: now.Add(10 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	hello := fixture.hello(0)
	hello.Purpose = ardp.PurposeForwarding
	body, err := ardp.EncodeHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.write(ardp.Frame{Kind: ardp.KindHello, Body: body}); err != nil {
		t.Fatal(err)
	}
	accepted, err := ardp.ReadFrame(fixture.peer)
	if err != nil || accepted.Kind != ardp.KindAccept {
		t.Fatalf("outer acceptance: %v / %v", accepted.Kind, err)
	}
	fixture.workers.Go(fixture.read)
	return fixture
}
func (fixture *terminationFixture) hello(id uint32) ardp.Hello {
	p := fixture.profile
	return ardp.Hello{NetworkID: p.NetworkID, StateGeneration: p.StateGeneration, StateDigest: p.StateDigest, ProfileDigest: p.Digest, RecipientNodeID: [32]byte{12}, RecipientDutyGeneration: 1, Purpose: ardp.PurposeIntroduction, ChannelNonce: [32]byte{byte(id + 30)}, Deadline: fixture.deadline}
}
func (fixture *terminationFixture) read() {
	defer close(fixture.readerDone)
	defer func() {
		fixture.pipesMu.Lock()
		defer fixture.pipesMu.Unlock()
		for _, pipe := range fixture.pipes {
			_ = pipe.Close()
		}
	}()
	for {
		frame, err := ardp.ReadFrame(fixture.peer)
		if err != nil {
			return
		}
		fixture.pipesMu.Lock()
		pipe := fixture.pipes[frame.Lane]
		fixture.pipesMu.Unlock()
		if pipe == nil {
			continue
		}
		switch frame.Kind {
		case ardp.KindBytes:
			if _, err := pipe.Write(frame.Body); err != nil {
				return
			}
		case ardp.KindClose:
			_ = pipe.Close()
		}
	}
}
func (fixture *terminationFixture) open(t *testing.T, id uint32, control []byte) net.Conn {
	return fixture.openFor(t, id, control, ardp.PurposeIntroduction, 3)
}
func (fixture *terminationFixture) openFor(t *testing.T, id uint32, control []byte, purpose ardp.Purpose, class byte) net.Conn {
	t.Helper()
	local, pump := net.Pipe()
	t.Cleanup(func() { _ = local.Close(); _ = pump.Close() })
	fixture.pipesMu.Lock()
	fixture.pipes[id] = pump
	fixture.pipesMu.Unlock()
	body, err := route.EncodeClosedNodeOpen(route.ClosedOpen{NextNodeID: [32]byte{12}, NextDutyGeneration: 1, Purpose: purpose, Deadline: fixture.hello(id).Deadline}, route.ClosedChildOrdinary)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.write(ardp.Frame{Kind: ardp.KindOpen, Lane: id, Body: body}); err != nil {
		t.Fatal(err)
	}
	fixture.workers.Go(func() {
		buffer := make([]byte, ardp.MaximumBodySize)
		for {
			n, err := pump.Read(buffer)
			if n > 0 {
				if writeErr := fixture.write(ardp.Frame{Kind: ardp.KindBytes, Lane: id, Body: buffer[:n]}); writeErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	})
	secured, err := carrier.OpenClosedRoleTLS(t.Context(), local, fixture.public, time.Now().Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	hello := fixture.hello(id)
	hello.Purpose = purpose
	helloBody, err := ardp.EncodeHello(hello)
	if err != nil {
		t.Fatal(err)
	}
	if err := ardp.WriteFrame(secured, ardp.Frame{Kind: ardp.KindHello, Body: helloBody}); err != nil {
		t.Fatal(err)
	}
	if err := ardp.WriteFrame(secured, ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{class}, control...)}); err != nil {
		t.Fatal(err)
	}
	accepted, err := ardp.ReadFrame(secured)
	if err != nil || accepted.Kind != ardp.KindAccept {
		t.Fatalf("inner acceptance: %v / %v", accepted.Kind, err)
	}
	return secured
}
func (fixture *terminationFixture) exchange(t *testing.T, connection net.Conn) {
	t.Helper()
	request := terminal.RegistrationRequest{Nonce: [32]byte{31}, Slot: [32]byte{32}, Revision: 1, Expiry: fixture.deadline.Add(-time.Second)}
	// Each successful sibling gets a different slot; retained withdrawn IDs stay unavailable.
	request.Slot[2] = byte(fixture.released.Load())
	fixture.registration(t, connection, request)
	request.Withdraw = true
	request.Expiry = time.Time{}
	request.Nonce = [32]byte{33}
	fixture.registration(t, connection, request)
	fixture.workers.Go(func() { _, _ = io.Copy(io.Discard, connection) })
}
func (fixture *terminationFixture) drain(ctx context.Context, timeout time.Duration) error {
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_ = fixture.server.stop()
	select {
	case <-fixture.server.drained:
		return fixture.server.drainErr
	case <-bounded.Done():
		return bounded.Err()
	}
}
func (fixture *terminationFixture) registration(t *testing.T, connection net.Conn, request terminal.RegistrationRequest) {
	t.Helper()
	body, err := terminal.EncodeRegistrationRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := ardp.WriteFrame(connection, ardp.Frame{Kind: ardp.KindOperation, Body: body}); err != nil {
		t.Fatal(err)
	}
	result, err := ardp.ReadFrame(connection)
	if err != nil || result.Kind != ardp.KindResult {
		t.Fatalf("registration result: %d %v", result.Kind, err)
	}
	status, proof, err := terminal.DecodeDescriptorResult(result.Body, request.Nonce)
	if err != nil || status != 0 || len(proof) != 0 {
		t.Fatalf("registration verdict: %d %v", status, err)
	}
}

func nodeCertificate(t *testing.T, serial int64, name string) (tls.Certificate, [32]byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	var identifier [32]byte
	copy(identifier[:], public)
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}, identifier
}

// An Introduction only forwards these opaque capsule bytes; decryption belongs
// to the publisher and is outside this reservation-lifetime regression.
func (fixture *terminationFixture) submission(t *testing.T, registration, connection net.Conn, request terminal.RegistrationRequest) {
	t.Helper()
	nonce := [32]byte{91}
	capsule := introductioncapsule.Capsule{Slot: request.Slot, Revision: request.Revision, Expiry: request.Expiry, DeliveryNonce: [32]byte{92}, Encapsulation: [32]byte{93}, Ciphertext: make([]byte, 360)}
	body, err := introductioncapsule.EncodeSubmission(nonce, capsule)
	if err != nil {
		t.Fatal(err)
	}
	if err := ardp.WriteFrame(connection, ardp.Frame{Kind: ardp.KindOperation, Body: body}); err != nil {
		t.Fatal(err)
	}
	delivery, err := ardp.ReadFrame(registration)
	if err != nil || delivery.Kind != ardp.KindOperation || delivery.Lane != 2 {
		t.Fatalf("delivery: %d/%d %v", delivery.Kind, delivery.Lane, err)
	}
	forwardedNonce, forwarded, err := introductioncapsule.DecodeSubmission(delivery.Body)
	if err != nil || forwardedNonce == nonce || forwarded.Slot != request.Slot {
		t.Fatal("invalid forwarded capsule")
	}
	result, err := terminal.EncodeDescriptorResult(forwardedNonce, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ardp.WriteFrame(registration, ardp.Frame{Kind: ardp.KindResult, Lane: delivery.Lane, Body: result}); err != nil {
		t.Fatal(err)
	}
	closed, err := ardp.ReadFrame(registration)
	if err != nil || closed.Kind != ardp.KindClose || closed.Lane != delivery.Lane {
		t.Fatalf("delivery close: %v", err)
	}
	response, err := ardp.ReadFrame(connection)
	if err != nil || response.Kind != ardp.KindResult {
		t.Fatalf("submission reply: %v", err)
	}
	status, proof, err := terminal.DecodeDescriptorResult(response.Body, nonce)
	if err != nil || status != 0 || len(proof) != 0 {
		t.Fatalf("submission result: %d %v", status, err)
	}
	fixture.workers.Go(func() { _, _ = io.Copy(io.Discard, connection) })
}
