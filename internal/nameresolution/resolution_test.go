package nameresolution_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameresolution"
	"github.com/dianabuilds/ardents-network/internal/naming"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/openpcc/ohttp"
)

func TestResolveSeparatesRelayAndGatewayViews(t *testing.T) {
	t.Parallel()
	fixture := newResolutionFixture(t)
	contexts := [][32]byte{{1}, {2}}
	for index, isolation := range contexts {
		selection := fixture.admitted(t, fixture.selection, "alice", isolation, byte(index+1))
		resolver, err := nameresolution.Open(fixture.view, selection, fixture.gatewayProfile(), isolation,
			relayTransport(fixture.relayServer))
		if err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			identities, families, rendezvous := resolver.ConnectionExclusions()
			if len(identities) != 2 || identities[0] != fixture.selection.RelayNodeID ||
				identities[1] != fixture.selection.GatewayNodeID || len(families) != 2 ||
				families[0] != "relay-family" || families[1] != "gateway-family" ||
				rendezvous != fixture.selection.ConnectionRendezvousNodeID {
				t.Fatalf("connection exclusions=%v/%v rendezvous=%x", identities, families, rendezvous)
			}
		}
		result, resolveErr := resolver.Resolve(context.Background(), "alice", fixture.now)
		if resolveErr != nil {
			t.Fatalf("Resolve: %v", resolveErr)
		}
		if result.Class != "resolved" || result.Record.Target != ([32]byte{1}) ||
			result.Binding.Target != result.Record.Target || result.Binding.Commitment == ([32]byte{}) {
			t.Fatalf("result = %+v", result)
		}
		again, againErr := resolver.Resolve(context.Background(), "alice", fixture.now)
		if againErr == nil || again.Class != "local authorization or policy denial" {
			t.Fatalf("single-use result=%+v err=%v", again, againErr)
		}
	}
	envelopes, origins := fixture.relayEvidence()
	if len(envelopes) != 2 || len(envelopes[0]) != len(envelopes[1]) || bytes.Equal(envelopes[0], envelopes[1]) {
		t.Fatalf("OHTTP envelopes are not fixed and fresh: sizes=%d/%d", len(envelopes[0]), len(envelopes[1]))
	}
	for index, envelope := range envelopes {
		target := [32]byte{1}
		if bytes.Contains(envelope, []byte("alice")) || bytes.Contains(envelope, target[:]) ||
			bytes.Contains(envelope, contexts[index][:]) {
			t.Fatalf("Relay envelope %d contains a forbidden view", index)
		}
	}
	if len(origins) != 2 || origins[0] == origins[1] {
		t.Fatalf("Isolation Contexts reused Relay transport identity: %v", origins)
	}
	replay, err := http.NewRequest(http.MethodPost, fixture.relayServer.URL+"/ohttp", bytes.NewReader(envelopes[0]))
	if err != nil {
		t.Fatal(err)
	}
	replay.Header.Set("Content-Type", ohttp.RequestMediaType)
	replayResponse, err := fixture.relayServer.Client().Do(replay)
	if err != nil {
		t.Fatal(err)
	}
	_ = replayResponse.Body.Close()
	if fixture.gatewayRequests() != 2 || fixture.gatewayRejected() != 1 || fixture.relayRequests() != 3 {
		t.Fatal("role observations did not count the completed exchanges")
	}
}

func TestResolveFailsClosedOnRoleConflictAndTampering(t *testing.T) {
	t.Parallel()
	fixture := newResolutionFixture(t)
	conflict := fixture.selection
	conflict.ConnectionRendezvousNodeID = conflict.GatewayNodeID
	if _, err := nameresolution.Open(fixture.view, conflict, fixture.gatewayProfile(), [32]byte{3},
		relayTransport(fixture.relayServer)); err == nil {
		t.Fatal("Gateway was also accepted as the connection Rendezvous")
	}

	fixture.setTamper(true)
	selection := fixture.admitted(t, fixture.selection, "alice", [32]byte{3}, 3)
	resolver, err := nameresolution.Open(fixture.view, selection, fixture.gatewayProfile(), [32]byte{3},
		relayTransport(fixture.relayServer))
	if err != nil {
		t.Fatal(err)
	}
	result, err := resolver.Resolve(context.Background(), "alice", fixture.now)
	if err == nil || result.Class != "invalid naming evidence" || result.Record != (namespace.Record{}) {
		t.Fatalf("tampered response result=%+v err=%v", result, err)
	}
}

func TestSelectRejectsEveryInvalidRoleBinding(t *testing.T) {
	t.Parallel()
	fixture := newResolutionFixture(t)
	tests := []struct {
		name   string
		mutate func(*state.Snapshot, *nameresolution.Selection)
	}{
		{"wrong relay domain", func(view *state.Snapshot, _ *nameresolution.Selection) {
			view.Candidates[0].Domain = "rendezvous"
		}},
		{"same family", func(view *state.Snapshot, _ *nameresolution.Selection) {
			view.Candidates[1].Family = view.Candidates[0].Family
		}},
		{"stale assignment", func(view *state.Snapshot, _ *nameresolution.Selection) {
			view.Candidates[1].AssignmentNotAfter = fixture.now
		}},
		{"excluded family", func(_ *state.Snapshot, selection *nameresolution.Selection) {
			selection.ExcludedFamilies = []string{"gateway-family"}
		}},
		{"missing role", func(view *state.Snapshot, _ *nameresolution.Selection) {
			view.CandidateCount = 2
		}},
		{"oversized deadline", func(_ *state.Snapshot, selection *nameresolution.Selection) {
			selection.Deadline = selection.At.Add(16 * time.Second)
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			view, selection := fixture.view, fixture.selection
			test.mutate(&view, &selection)
			if _, err := nameresolution.Open(view, selection, fixture.gatewayProfile(), [32]byte{8},
				relayTransport(fixture.relayServer)); err == nil {
				t.Fatal("invalid role binding was accepted")
			}
		})
	}
	profile := fixture.gatewayProfile()
	profile.KeyConfig = append([]byte(nil), profile.KeyConfig...)
	profile.KeyConfig[len(profile.KeyConfig)-1] ^= 1
	profile.KeyConfigDigest = sha256.Sum256(profile.KeyConfig)
	if _, err := nameresolution.Open(fixture.view, fixture.selection, profile, [32]byte{8},
		relayTransport(fixture.relayServer)); err == nil {
		t.Fatal("forged Gateway key configuration was accepted")
	}
}

func TestResolveContactsOnlyTheSelectedRelayAndReturnsABoundedError(t *testing.T) {
	t.Parallel()
	fixture := newResolutionFixture(t)
	var mu sync.Mutex
	var contacts []string
	transport := &http.Transport{DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
		mu.Lock()
		contacts = append(contacts, address)
		mu.Unlock()
		return nil, errors.New("dial included secret peer 192.0.2.1")
	}}
	selection := fixture.admitted(t, fixture.selection, "alice", [32]byte{5}, 5)
	resolver, err := nameresolution.Open(fixture.view, selection, fixture.gatewayProfile(), [32]byte{5}, transport)
	if err != nil {
		t.Fatal(err)
	}
	result, err := resolver.Resolve(context.Background(), "alice", fixture.now)
	if err == nil || result.Class != "private resolution unavailable" || err.Error() != result.Class {
		t.Fatalf("bounded result=%+v err=%v", result, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(contacts) != 1 || contacts[0] != fixture.view.Candidates[0].Endpoint {
		t.Fatalf("resolution contacts=%v", contacts)
	}
}

func TestResolveDoesNotExposeUnboundOrUnknownNames(t *testing.T) {
	t.Parallel()
	fixture := newResolutionFixture(t)
	selection := fixture.admitted(t, fixture.selection, "missing", [32]byte{4}, 4)
	resolver, err := nameresolution.Open(fixture.view, selection, fixture.gatewayProfile(), [32]byte{4},
		relayTransport(fixture.relayServer))
	if err != nil {
		t.Fatal(err)
	}
	result, err := resolver.Resolve(context.Background(), "missing", fixture.now)
	if err == nil || result.Class != "private resolution unavailable" || result.Record != (namespace.Record{}) {
		t.Fatalf("missing name result=%+v err=%v", result, err)
	}
	if _, err := nameresolution.Open(fixture.view, fixture.selection, fixture.gatewayProfile(), [32]byte{},
		relayTransport(fixture.relayServer)); err == nil {
		t.Fatal("zero Isolation Context was accepted")
	}
}

type resolutionFixture struct {
	now             time.Time
	view            state.Snapshot
	selection       nameresolution.Selection
	relayServer     *httptest.Server
	gatewayProfile  func() nameresolution.GatewayProfile
	gatewayRequests func() uint32
	gatewayRejected func() uint32
	relayRequests   func() uint32
	relayEvidence   func() ([][]byte, []string)
	roleEvidence    func() ([]gatewayRoleView, []relayRoleView)
	setTamper       func(bool)
	admission       *namespace.Admission
}

func newResolutionFixture(t *testing.T) resolutionFixture {
	return newResolutionFixtureWithControl(t, nil)
}

func newResolutionFixtureWithControl(t *testing.T, control interface {
	Apply([]byte, namespace.Proof) (string, uint64, uint64, []byte)
},
) resolutionFixture {
	t.Helper()
	now := time.Unix(1_800_000_000, 0).UTC()
	network := [32]byte{9}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	record := namespace.Record{Name: "alice", Generation: 1, Revision: 1,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(public), Target: [32]byte{1},
		LeaseExpiresAt: now.Add(time.Hour).Unix(), GraceExpiresAt: now.Add(2 * time.Hour).Unix(),
		RecordNotAfter: now.Add(30 * time.Minute).UnixMilli()}
	signed, err := namespace.SignRecord(network, record, private)
	if err != nil {
		t.Fatal(err)
	}
	gatewayPublic, gatewayPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	admission, err := namespace.NewAdmission([32]byte{2}, network, 1, [32]byte{6})
	if err != nil {
		t.Fatal(err)
	}
	store, materialization := resolutionRecordStore(t, network, signed)
	gatewayState, err := nameresolution.BindGatewayState(store, materialization.policy, 1, [32]byte{1}, admission, control)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := nameresolution.NewGateway(nameresolution.GatewayConfig{
		NodeID: [32]byte{2}, Family: "gateway-family", Domain: "rendezvous",
		AssignmentNotAfter: now.Add(time.Minute), MaximumPending: 16,
		IdentityKey: gatewayPrivate, Clock: func() time.Time { return now }, State: gatewayState})
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewUnstartedServer(gateway.Handler())
	gatewayServer.StartTLS()
	t.Cleanup(gatewayServer.Close)

	relay, err := nameresolution.NewRelay(gatewayServer.URL, gatewayServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	capture := &relayCapture{handler: relay.Handler()}
	relayServer := httptest.NewUnstartedServer(capture)
	relayServer.StartTLS()
	t.Cleanup(relayServer.Close)

	view := resolutionView(t, network, now, relayServer.URL, gatewayServer.URL, gatewayPublic)
	bindNamespacePolicy(&view, materialization.policy)
	selection := nameresolution.Selection{At: now, Deadline: now.Add(15 * time.Second),
		RelayNodeID: [32]byte{1}, GatewayNodeID: [32]byte{2}, ConnectionRendezvousNodeID: [32]byte{3}}
	return resolutionFixture{now: now, view: view, selection: selection, relayServer: relayServer, admission: admission,
		gatewayProfile:  gateway.Profile,
		gatewayRequests: func() uint32 { requests, _, _ := gateway.Observation(); return requests },
		gatewayRejected: func() uint32 { _, _, rejected := gateway.Observation(); return rejected },
		relayRequests:   func() uint32 { requests, _, _ := relay.Observation(); return requests },
		roleEvidence: func() ([]gatewayRoleView, []relayRoleView) {
			gatewayEvidence := gateway.RoleEvidence()
			gatewayViews := make([]gatewayRoleView, len(gatewayEvidence))
			for index, view := range gatewayEvidence {
				gatewayViews[index] = gatewayRoleView{operation: view.Operation, name: view.Name, result: view.Result,
					network: view.Network, nonce: view.Nonce, target: view.Target, deadline: view.Deadline,
					generation: view.Generation, revision: view.Revision}
			}
			relayEvidence := relay.RoleEvidence()
			relayViews := make([]relayRoleView, len(relayEvidence))
			for index, view := range relayEvidence {
				relayViews[index] = relayRoleView{origin: view.Origin, gateway: view.Gateway,
					request: view.Request, response: view.Response, requestBytes: view.RequestBytes,
					responseBytes: view.ResponseBytes, keyID: view.KeyID}
			}
			return gatewayViews, relayViews
		},
		relayEvidence: capture.evidence, setTamper: capture.setTamper}
}

func resolutionRecordStore(t *testing.T, network [32]byte, signed ...[]byte) (*namespace.Store, namespaceFixture) {
	t.Helper()
	materialization := testNamespaceFixture(network, "resolution-namespace")
	store, err := namespace.Open(t.TempDir(), materialization.policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	materialization.commit(t, store, 1, signed)
	return store, materialization
}

func (fixture resolutionFixture) admitted(t *testing.T, selection nameresolution.Selection,
	name string, isolation [32]byte, nonce byte,
) nameresolution.Selection {
	t.Helper()
	digest := testResolutionAdmissionDigest(t, fixture.view.NetworkID, name, selection.Deadline.UnixNano())
	challenge, err := fixture.admission.Issue(selection.At.UnixMilli(), "resolution", digest, isolation,
		selection.Deadline.UnixMilli(), [16]byte{nonce})
	if err != nil {
		t.Fatal(err)
	}
	selection.AdmissionChallenge = challenge
	return selection
}

func testResolutionAdmissionDigest(t *testing.T, network [32]byte, raw string, deadline int64) [32]byte {
	t.Helper()
	name, err := naming.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := naming.EncodeWire(name)
	if err != nil {
		t.Fatal(err)
	}
	out := []byte("ardents-name-resolution-operation-v1\x00")
	out = append(out, network[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(deadline))
	out = binary.BigEndian.AppendUint16(out, uint16(len(wire)))
	return sha256.Sum256(append(out, wire...))
}

func resolutionView(t *testing.T, network [32]byte, now time.Time, relayURL, gatewayURL string,
	gatewayPublic ed25519.PublicKey) state.Snapshot {
	t.Helper()
	view := state.Snapshot{Generation: "generation-a", NetworkID: network, Epoch: 1,
		Digest: [32]byte{1}, ValidUntil: now.Add(time.Hour), Profile: "h3-route-tracer-v1",
		ViewRoot: [32]byte{2}, Freshness: "fresh", CandidateCount: 3}
	roles := []struct {
		id       byte
		family   string
		domain   string
		endpoint string
	}{
		{1, "relay-family", "initiator", endpointOf(t, relayURL)},
		{2, "gateway-family", "rendezvous", endpointOf(t, gatewayURL)},
		{3, "route-rendezvous-family", "rendezvous", "127.0.0.1:7443"},
	}
	for index, role := range roles {
		view.Candidates[index].NodeID = [32]byte{role.id}
		view.Candidates[index].PublicKey = [32]byte{role.id, 1}
		view.Candidates[index].KeyID = [32]byte{role.id, 2}
		view.Candidates[index].Family = role.family
		view.Candidates[index].Endpoint = role.endpoint
		view.Candidates[index].Capacity = 1
		view.Candidates[index].Domain = role.domain
		view.Candidates[index].ValidFrom = now.Add(-time.Minute)
		view.Candidates[index].ValidUntil = now.Add(time.Hour)
		view.Candidates[index].AssignmentNotAfter = now.Add(time.Minute)
	}
	copy(view.Candidates[1].PublicKey[:], gatewayPublic)
	return view
}

func endpointOf(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Host
}

func relayTransport(server *httptest.Server) *http.Transport {
	return server.Client().Transport.(*http.Transport)
}

type relayCapture struct {
	mu        sync.Mutex
	handler   http.Handler
	envelopes [][]byte
	origins   []string
	tamper    bool
}

func (capture *relayCapture) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, "invalid body", http.StatusBadRequest)
		return
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	capture.handler.ServeHTTP(recorder, request)
	capture.mu.Lock()
	capture.envelopes = append(capture.envelopes, append([]byte(nil), body...))
	capture.origins = append(capture.origins, request.RemoteAddr)
	tamper := capture.tamper
	capture.mu.Unlock()
	for key, values := range recorder.Header() {
		writer.Header()[key] = append([]string(nil), values...)
	}
	responseBody := recorder.Body.Bytes()
	if tamper && len(responseBody) > 0 {
		responseBody = append([]byte(nil), responseBody...)
		responseBody[len(responseBody)-1] ^= 1
	}
	writer.WriteHeader(recorder.Code)
	_, _ = writer.Write(responseBody)
}

func (capture *relayCapture) evidence() ([][]byte, []string) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	envelopes := make([][]byte, len(capture.envelopes))
	for index := range capture.envelopes {
		envelopes[index] = append([]byte(nil), capture.envelopes[index]...)
	}
	return envelopes, append([]string(nil), capture.origins...)
}

func (capture *relayCapture) setTamper(value bool) {
	capture.mu.Lock()
	capture.tamper = value
	capture.mu.Unlock()
}
