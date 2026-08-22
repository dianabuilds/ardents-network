package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameauthority"
	"github.com/dianabuilds/ardents-network/internal/namelease"
	"github.com/dianabuilds/ardents-network/internal/nameresolution"
	"github.com/dianabuilds/ardents-network/internal/namestore"
	"github.com/dianabuilds/ardents-network/internal/naming"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestResolveCommandRunsPrivateResolution(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	network := [32]byte{9}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	record := namelease.Record{Name: "alice", Generation: 1, Revision: 1,
		Lease: "active", Consistency: "current", Recovery: "stable",
		Authority: hex.EncodeToString(public), Target: [32]byte{1},
		LeaseExpiresAt: now.Add(time.Hour).Unix(), GraceExpiresAt: now.Add(2 * time.Hour).Unix()}
	signed, err := nameauthority.SignRecord(network, record, private)
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
	recordStore, materialization := commandRecordStore(t, network, signed)
	gatewayState, err := nameresolution.BindGatewayState(recordStore, materialization, 1, [32]byte{1}, admission, nil)
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := nameresolution.NewGateway(nameresolution.GatewayConfig{
		NodeID: [32]byte{2}, Family: "gateway-family", Domain: "rendezvous",
		AssignmentNotAfter: now.Add(time.Minute), MaximumPending: 8,
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
	relayServer := httptest.NewUnstartedServer(relay.Handler())
	relayServer.StartTLS()
	t.Cleanup(relayServer.Close)

	view := commandResolutionView(t, network, now, relayServer.URL, gatewayServer.URL, gatewayPublic)
	bindCommandMaterialization(&view, materialization)
	isolation := [32]byte{1}
	digest := commandResolutionAdmissionDigest(t, network, "alice", now.Add(15*time.Second).UnixNano())
	challenge, err := admission.Issue(now.UnixMilli(), "resolution", digest, isolation,
		now.Add(15*time.Second).UnixMilli(), [16]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	input := resolutionInput{Schema: resolutionInputSchema, StateRoot: t.TempDir(),
		NetworkID: hex.EncodeToString(network[:]), AuthorityPublic: commandPolicyPublic(materialization),
		AuthorityThreshold: materialization.Threshold, AcceptedProfile: "h3-route-tracer-v1",
		SelectionAt: now.Format(time.RFC3339Nano), Deadline: now.Add(15 * time.Second).Format(time.RFC3339Nano),
		RelayNodeID: fixedNodeID(1), GatewayNodeID: fixedNodeID(2),
		ConnectionRendezvousNodeID: fixedNodeID(3), GatewayProfile: gateway.Profile(),
		AdmissionChallenge: challenge}
	planRaw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(t.TempDir(), "resolution.json")
	if err := os.WriteFile(planPath, planRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	contextHex := hex.EncodeToString(append([]byte{1}, make([]byte, 31)...))
	var output bytes.Buffer
	load := func(state.Config) (state.Snapshot, error) { return view, nil }
	transport := relayServer.Client().Transport.(*http.Transport)
	if err := runWithRuntime([]string{"resolve", planPath, "alice", contextHex}, &output, transport, load); err != nil {
		t.Fatalf("resolve command: %v", err)
	}
	var receipt struct {
		Schema        string `json:"schema"`
		Class         string `json:"class"`
		Name          string `json:"name"`
		Generation    uint64 `json:"generation"`
		Revision      uint64 `json:"revision"`
		Target        string `json:"target"`
		RecordSHA256  string `json:"record_sha256"`
		BindingSHA256 string `json:"binding_sha256"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &receipt); err != nil ||
		receipt.Schema != "ardents-name-resolution-result-v1" || receipt.Class != "resolved" ||
		receipt.Name != "alice" || receipt.Generation != 1 || receipt.Revision != 1 ||
		receipt.Target != "0100000000000000000000000000000000000000000000000000000000000000" ||
		len(receipt.RecordSHA256) != 64 || len(receipt.BindingSHA256) != 64 {
		t.Fatalf("receipt=%+v output=%q err=%v", receipt, output.String(), err)
	}
	input.StateRoot = t.TempDir()
	planRaw, err = json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, planRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runWithTransport([]string{"resolve", planPath, "alice", contextHex}, &bytes.Buffer{}, transport); err == nil {
		t.Fatal("resolve accepted an input without recovered authenticated Network State")
	}
}

func commandRecordStore(t *testing.T, network [32]byte, signed ...[]byte) (*namestore.Store, namestore.Policy) {
	t.Helper()
	policy, signers := commandMaterializationPolicy(network)
	store, err := namestore.Open(t.TempDir(), policy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	epoch := namestore.Epoch{Number: 1, Digest: [32]byte{1}, CutoffOffset: 1,
		TransitionRoot: sha256.Sum256([]byte("transitions")), TransitionLength: 1,
		RejectionRoot: sha256.Sum256([]byte("rejections"))}
	if err := store.Commit(epoch, signed, func(transcript []byte) ([][32]byte, [][]byte, error) {
		ids, signatures := make([][32]byte, 2), make([][]byte, 2)
		for index, private := range signers[:2] {
			ids[index] = sha256.Sum256(private.Public().(ed25519.PublicKey))
			signatures[index] = ed25519.Sign(private, transcript)
		}
		if bytes.Compare(ids[0][:], ids[1][:]) > 0 {
			ids[0], ids[1], signatures[0], signatures[1] = ids[1], ids[0], signatures[1], signatures[0]
		}
		return ids, signatures, nil
	}); err != nil {
		t.Fatal(err)
	}
	return store, policy
}

func commandMaterializationPolicy(network [32]byte) (namestore.Policy, []ed25519.PrivateKey) {
	policy := namestore.Policy{Network: network, Rule: "ardents-namespace-materialization-v1",
		Authorities: make(map[[32]byte]ed25519.PublicKey), Threshold: 2}
	var signers []ed25519.PrivateKey
	for index := 0; index < 3; index++ {
		seed := sha256.Sum256([]byte("command-namespace-" + string(rune('0'+index))))
		private := ed25519.NewKeyFromSeed(seed[:])
		public := private.Public().(ed25519.PublicKey)
		policy.Authorities[sha256.Sum256(public)] = public
		signers = append(signers, private)
	}
	return policy, signers
}

func commandPolicyIDs(policy namestore.Policy) [][32]byte {
	ids := make([][32]byte, 0, len(policy.Authorities))
	for id := range policy.Authorities {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
	return ids
}

func commandPolicyPublic(policy namestore.Policy) []string {
	ids, values := commandPolicyIDs(policy), make([]string, 0, len(policy.Authorities))
	for _, id := range ids {
		values = append(values, hex.EncodeToString(policy.Authorities[id]))
	}
	return values
}

func bindCommandMaterialization(view *state.Snapshot, policy namestore.Policy) {
	ids := commandPolicyIDs(policy)
	view.EpochAuthorityCount, view.EpochThreshold = uint8(len(ids)), uint8(policy.Threshold)
	for index, id := range ids {
		view.EpochAuthorityIDs[index] = id
		copy(view.EpochAuthorityKeys[index][:], policy.Authorities[id])
	}
}

func commandResolutionAdmissionDigest(t *testing.T, network [32]byte, raw string, deadline int64) [32]byte {
	t.Helper()
	name, err := naming.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := naming.EncodeWire(name)
	if err != nil {
		t.Fatal(err)
	}
	transcript := []byte("ardents-name-resolution-operation-v1\x00")
	transcript = append(transcript, network[:]...)
	transcript = binary.BigEndian.AppendUint64(transcript, uint64(deadline))
	transcript = binary.BigEndian.AppendUint16(transcript, uint16(len(wire)))
	return sha256.Sum256(append(transcript, wire...))
}

func fixedNodeID(marker byte) string {
	return hex.EncodeToString(append([]byte{marker}, make([]byte, 31)...))
}

func commandResolutionView(t *testing.T, network [32]byte, now time.Time, relayURL, gatewayURL string,
	gatewayPublic ed25519.PublicKey) state.Snapshot {
	t.Helper()
	view := state.Snapshot{Generation: "generation-a", NetworkID: network, Epoch: 1,
		Digest: [32]byte{1}, ValidUntil: now.Add(time.Hour), Profile: "h3-route-tracer-v1",
		ViewRoot: [32]byte{2}, Freshness: "fresh", CandidateCount: 3}
	values := []struct {
		id                       byte
		family, domain, endpoint string
	}{
		{1, "relay-family", "initiator", commandEndpoint(t, relayURL)},
		{2, "gateway-family", "rendezvous", commandEndpoint(t, gatewayURL)},
		{3, "connection-family", "rendezvous", "127.0.0.1:7443"},
	}
	for index, value := range values {
		view.Candidates[index].NodeID = [32]byte{value.id}
		view.Candidates[index].PublicKey = [32]byte{value.id, 1}
		view.Candidates[index].KeyID = [32]byte{value.id, 2}
		view.Candidates[index].Family = value.family
		view.Candidates[index].Endpoint = value.endpoint
		view.Candidates[index].Capacity = 1
		view.Candidates[index].Domain = value.domain
		view.Candidates[index].ValidFrom = now.Add(-time.Minute)
		view.Candidates[index].ValidUntil = now.Add(time.Hour)
		view.Candidates[index].AssignmentNotAfter = now.Add(time.Minute)
	}
	copy(view.Candidates[1].PublicKey[:], gatewayPublic)
	return view
}

func commandEndpoint(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Host
}
