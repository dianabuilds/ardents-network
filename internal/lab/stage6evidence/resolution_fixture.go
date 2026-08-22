package stage6evidence

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameresolution"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type resolutionFixture struct {
	now             time.Time
	view            state.Snapshot
	selection       nameresolution.Selection
	gateway         *httptest.Server
	relay           *httptest.Server
	gate            *namespace.Admission
	profile         func() nameresolution.GatewayProfile
	observations    func() (uint32, uint32, uint32, uint32)
	roleEvidence    func() ([]resolutionGatewayView, []resolutionRelayView)
	control         func() (uint32, uint32, uint32)
	capture         *resolutionCapture
	store           *namespace.Store
	materialization namespaceFixture
	records         [][]byte
	transitions     [][]byte
	claimOrder      namespace.ClaimOrder
	claimProof      namespace.ClaimProof
	root            string
}

func newResolutionFixture(control ...interface {
	Apply([]byte, namespace.Proof) (string, uint64, uint64, []byte)
},
) (resolutionFixture, error) {
	value := resolutionFixture{now: time.Unix(1_800_000_000, 0).UTC()}
	network, authority := [32]byte{9}, evidenceKey("resolution-authority")
	materialization := newNamespaceFixture(network)
	value.materialization = materialization
	claimRecord := namespace.Record{Name: "alice", Generation: 1, Revision: 1, Lease: "active",
		Consistency: "current", Recovery: "stable", Authority: hex.EncodeToString(authority.Public().(ed25519.PublicKey)),
		LeaseExpiresAt: value.now.Add(time.Hour).Unix(),
		GraceExpiresAt: value.now.Add(2 * time.Hour).Unix(), Continuity: 1}
	currentRecord := claimRecord
	currentRecord.Revision, currentRecord.Target = 2, [32]byte{1}
	signedClaim, err := namespace.SignRecord(network, claimRecord, authority)
	if err != nil {
		return value, err
	}
	signed, err := namespace.SignRecord(network, currentRecord, authority)
	if err != nil {
		return value, err
	}
	value.records = [][]byte{append([]byte(nil), signed...)}
	value.transitions = [][]byte{append([]byte(nil), signedClaim...), append([]byte(nil), signed...)}
	first := evidenceClaimFor(network, 1, 0, [32]byte{1}, authority)
	second := evidenceClaimFor(network, 1, 1, [32]byte{2}, evidenceKey("resolution-claim-second"))
	value.claimOrder, value.claimProof = evidenceClaimSetFor(network, 1, []namespace.Claim{first, second})
	value.root, err = os.MkdirTemp("", "ardents-stage6-resolution-")
	if err != nil {
		return value, err
	}
	value.store, err = namespace.Open(value.root, materialization.policy)
	if err != nil {
		value.close()
		return value, err
	}
	epoch := namespace.Epoch{Number: 1, Digest: [32]byte{1}, CutoffOffset: value.claimProof.CutoffOffset,
		TransitionRoot: namespaceTransitionRoot(value.transitions), TransitionLength: uint32(len(value.transitions)),
		RejectionRoot: value.claimProof.RejectionRoot, RejectionLength: value.claimProof.RejectionLength}
	if err = value.store.Commit(epoch, value.records, materialization.attest); err != nil {
		value.close()
		return value, err
	}
	gatewayIdentity := evidenceKey("resolution-gateway")
	value.gate, err = namespace.NewAdmission([32]byte{2}, network, 1, [32]byte{6})
	if err != nil {
		value.close()
		return value, err
	}
	var controlAuthority interface {
		Apply([]byte, namespace.Proof) (string, uint64, uint64, []byte)
	}
	if len(control) == 1 {
		controlAuthority = control[0]
	}
	gatewayState, err := nameresolution.BindGatewayState(value.store, materialization.policy, 1, [32]byte{1},
		value.gate, controlAuthority)
	if err != nil {
		value.close()
		return value, err
	}
	gateway, err := nameresolution.NewGateway(nameresolution.GatewayConfig{NodeID: [32]byte{2},
		Family: "gateway-family", Domain: "rendezvous", AssignmentNotAfter: value.now.Add(time.Minute),
		MaximumPending: 16, IdentityKey: gatewayIdentity,
		Clock: func() time.Time { return value.now }, State: gatewayState})
	if err != nil {
		value.close()
		return value, err
	}
	value.gateway = httptest.NewTLSServer(gateway.Handler())
	relay, err := nameresolution.NewRelay(value.gateway.URL, value.gateway.Client())
	if err != nil {
		value.close()
		return value, err
	}
	value.capture = &resolutionCapture{handler: relay.Handler()}
	value.relay = httptest.NewTLSServer(value.capture)
	value.view = resolutionSnapshot(network, value.now, value.relay.URL, value.gateway.URL,
		gatewayIdentity.Public().(ed25519.PublicKey))
	materialization.bind(&value.view)
	value.selection = nameresolution.Selection{At: value.now, Deadline: value.now.Add(15 * time.Second),
		RelayNodeID: [32]byte{1}, GatewayNodeID: [32]byte{2}, ConnectionRendezvousNodeID: [32]byte{3}}
	value.profile = gateway.Profile
	value.observations = func() (uint32, uint32, uint32, uint32) {
		relayRequests, _, _ := relay.Observation()
		gatewayRequests, resolved, rejected := gateway.Observation()
		return relayRequests, gatewayRequests, resolved, rejected
	}
	value.roleEvidence = func() ([]resolutionGatewayView, []resolutionRelayView) {
		gatewayEvidence := gateway.RoleEvidence()
		gatewayViews := make([]resolutionGatewayView, len(gatewayEvidence))
		for index, view := range gatewayEvidence {
			gatewayViews[index] = resolutionGatewayView{Operation: view.Operation, Name: view.Name,
				Network: view.Network, Nonce: view.Nonce, Target: view.Target, Deadline: view.Deadline,
				Result: view.Result, Generation: view.Generation, Revision: view.Revision}
		}
		relayEvidence := relay.RoleEvidence()
		relayViews := make([]resolutionRelayView, len(relayEvidence))
		for index, view := range relayEvidence {
			relayViews[index] = resolutionRelayView{Origin: view.Origin, Gateway: view.Gateway,
				Request: view.Request, Response: view.Response, RequestBytes: view.RequestBytes,
				ResponseBytes: view.ResponseBytes, KeyID: view.KeyID, Deadline: view.Deadline}
		}
		return gatewayViews, relayViews
	}
	value.control = gateway.ControlObservation
	return value, nil
}

func (value *resolutionFixture) close() {
	if value.relay != nil {
		value.relay.Close()
	}
	if value.gateway != nil {
		value.gateway.Close()
	}
	if value.store != nil {
		_ = value.store.Close()
	}
	if value.root != "" {
		_ = os.RemoveAll(value.root)
	}
}

func (value resolutionFixture) admitted(name string, isolation [32]byte, nonce byte) (nameresolution.Selection, error) {
	wire := append([]byte{0, 1, byte(len(name))}, name...)
	out := append([]byte("ardents-name-resolution-operation-v1\x00"), value.view.NetworkID[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(value.selection.Deadline.UnixNano()))
	out = binary.BigEndian.AppendUint16(out, uint16(len(wire)))
	digest := sha256.Sum256(append(out, wire...))
	challenge, err := value.gate.Issue(value.selection.At.UnixMilli(), "resolution", digest, isolation,
		value.selection.Deadline.UnixMilli(), [16]byte{nonce})
	selection := value.selection
	selection.AdmissionChallenge = challenge
	return selection, err
}

func resolutionSnapshot(network [32]byte, now time.Time, relayURL, gatewayURL string, gatewayKey ed25519.PublicKey) state.Snapshot {
	view := state.Snapshot{Generation: "generation-a", NetworkID: network, Epoch: 1, Digest: [32]byte{1},
		ValidUntil: now.Add(time.Hour), Profile: "h3-route-tracer-v1", ViewRoot: [32]byte{2}, Freshness: "fresh", CandidateCount: 3}
	roles := []struct {
		id                       byte
		family, domain, endpoint string
	}{
		{1, "relay-family", "initiator", urlHost(relayURL)},
		{2, "gateway-family", "rendezvous", urlHost(gatewayURL)},
		{3, "connection-family", "rendezvous", "127.0.0.1:7443"}}
	for index, role := range roles {
		candidate := &view.Candidates[index]
		candidate.NodeID, candidate.PublicKey, candidate.KeyID = [32]byte{role.id}, [32]byte{role.id, 1}, [32]byte{role.id, 2}
		candidate.Family, candidate.Domain, candidate.Endpoint, candidate.Capacity = role.family, role.domain, role.endpoint, 1
		candidate.ValidFrom, candidate.ValidUntil = now.Add(-time.Minute), now.Add(time.Hour)
		candidate.AssignmentNotAfter = now.Add(time.Minute)
	}
	copy(view.Candidates[1].PublicKey[:], gatewayKey)
	return view
}

func urlHost(raw string) string { parsed, _ := url.Parse(raw); return parsed.Host }

type resolutionCapture struct {
	mu        sync.Mutex
	handler   http.Handler
	envelopes [][]byte
}

func (capture *resolutionCapture) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, "invalid", http.StatusBadRequest)
		return
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	capture.mu.Lock()
	capture.envelopes = append(capture.envelopes, append([]byte(nil), body...))
	capture.mu.Unlock()
	capture.handler.ServeHTTP(writer, request)
}
func (capture *resolutionCapture) evidence() [][]byte {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	out := make([][]byte, len(capture.envelopes))
	for index := range out {
		out[index] = append([]byte(nil), capture.envelopes[index]...)
	}
	return out
}
