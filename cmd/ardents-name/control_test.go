package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameauthority"
	"github.com/dianabuilds/ardents-network/internal/namelease"
	"github.com/dianabuilds/ardents-network/internal/nameresolution"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type commandControlOperation struct {
	Kind               string   `json:"kind"`
	OperationDigest    [32]byte `json:"operation_digest"`
	Network            [32]byte `json:"network"`
	Nonce              [32]byte `json:"nonce"`
	Deadline           int64    `json:"deadline"`
	Name               string   `json:"name"`
	ParentName         string   `json:"parent_name"`
	Generation         uint64   `json:"generation"`
	ExpectedRevision   uint64   `json:"expected_revision"`
	ParentGeneration   uint64   `json:"parent_generation"`
	ParentRevision     uint64   `json:"parent_revision"`
	ChildGeneration    uint64   `json:"child_generation"`
	Authority          [32]byte `json:"authority"`
	SuccessorAuthority [32]byte `json:"successor_authority"`
	Target             [32]byte `json:"target"`
	LeaseNotAfter      int64    `json:"lease_not_after"`
	RecordNotAfter     int64    `json:"record_not_after"`
	PolicyNotBefore    int64    `json:"policy_not_before"`
	RecoveryNotBefore  int64    `json:"recovery_not_before"`
	PolicyID           [32]byte `json:"policy_id"`
	RecoveryStep       string   `json:"recovery_step"`
	OrderingProof      []byte   `json:"ordering_proof"`
	AuthorityProof     []byte   `json:"authority_proof"`
	RecoveryPolicy     []byte   `json:"recovery_policy"`
	RecoveryProof      []byte   `json:"recovery_proof"`
}

func TestControlCommandExecutesEveryPrivateControlShape(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	network := [32]byte{41}
	namePublic, namePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	current := namelease.Record{Name: "alice", Generation: 1, Revision: 1, Lease: "active",
		Consistency: "current", Recovery: "stable", Authority: hex.EncodeToString(namePublic),
		LeaseExpiresAt: now.Add(time.Hour).Unix(), GraceExpiresAt: now.Add(2 * time.Hour).Unix()}
	signed, err := nameauthority.SignRecord(network, current, namePrivate)
	if err != nil {
		t.Fatal(err)
	}
	store, materialization := commandRecordStore(t, network, signed)
	bootSecret := [32]byte{43}
	admission, err := namespace.NewAdmission([32]byte{2}, network, 1, bootSecret)
	if err != nil {
		t.Fatal(err)
	}
	gatewayPublic, gatewayPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	gatewayState, err := nameresolution.BindGatewayState(store, materialization, 1, [32]byte{1}, admission,
		commandControlAuthority{})
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
	gatewayServer := httptest.NewTLSServer(gateway.Handler())
	t.Cleanup(gatewayServer.Close)
	relay, err := nameresolution.NewRelay(gatewayServer.URL, gatewayServer.Client())
	if err != nil {
		t.Fatal(err)
	}
	relayServer := httptest.NewTLSServer(relay.Handler())
	t.Cleanup(relayServer.Close)

	view := commandResolutionView(t, network, now, relayServer.URL, gatewayServer.URL, gatewayPublic)
	bindCommandMaterialization(&view, materialization)
	transport := relayServer.Client().Transport.(*http.Transport)
	load := func(state.Config) (state.Snapshot, error) { return view, nil }
	for index, operation := range commandControlOperations(now) {
		t.Run(operation.Kind, func(t *testing.T) {
			digest := commandControlDigest(t, operation)
			operation.OperationDigest = digest
			isolation := [32]byte{byte(50 + index)}
			challenge, issueErr := admission.Issue(now.UnixMilli(), commandControlSurface(operation.Kind), digest,
				isolation, now.Add(15*time.Second).UnixMilli(), [16]byte{byte(70 + index)})
			if issueErr != nil {
				t.Fatal(issueErr)
			}
			input := resolutionInput{Schema: controlInputSchema, StateRoot: t.TempDir(),
				NetworkID: hex.EncodeToString(network[:]), AuthorityPublic: commandPolicyPublic(materialization),
				AuthorityThreshold: materialization.Threshold, AcceptedProfile: "h3-route-tracer-v1",
				SelectionAt: now.Format(time.RFC3339Nano), Deadline: now.Add(15 * time.Second).Format(time.RFC3339Nano),
				RelayNodeID: fixedNodeID(1), GatewayNodeID: fixedNodeID(2),
				ConnectionRendezvousNodeID: fixedNodeID(0), GatewayProfile: gateway.Profile(), AdmissionChallenge: challenge}
			inputPath := writeCommandJSON(t, "control-input.json", input)
			operationPath := writeCommandJSON(t, "control-operation.json", operation)
			var output bytes.Buffer
			if runErr := runWithRuntime([]string{"control", inputPath, operationPath, hex.EncodeToString(isolation[:])},
				&output, transport, load); runErr != nil {
				t.Fatalf("control command: %v", runErr)
			}
			var result struct {
				Schema     string `json:"schema"`
				Kind       string `json:"kind"`
				Name       string `json:"name"`
				Class      string `json:"class"`
				Generation uint64 `json:"generation"`
				Revision   uint64 `json:"revision"`
				State      []byte `json:"state"`
			}
			wantState := []byte("accepted-" + operation.Kind)
			if decodeErr := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &result); decodeErr != nil ||
				result.Schema != "ardents-name-control-result-v1" || result.Kind != operation.Kind ||
				result.Name != operation.Name || result.Class != "accepted" ||
				result.Generation != 1 || result.Revision != 2 || !bytes.Equal(result.State, wantState) {
				t.Fatalf("result=%+v output=%q err=%v", result, output.String(), decodeErr)
			}
		})
	}
}

type commandControlAuthority struct{}

func (commandControlAuthority) Apply(raw []byte, _ namespace.Proof) (string, uint64, uint64, []byte) {
	var operation commandControlOperation
	if json.Unmarshal(raw, &operation) != nil {
		return "denied", 0, 0, nil
	}
	return "accepted", 1, 2, []byte("accepted-" + operation.Kind)
}

func commandControlDigest(t *testing.T, operation commandControlOperation) [32]byte {
	t.Helper()
	operation.OperationDigest = [32]byte{}
	operation.Network, operation.Nonce, operation.Deadline = [32]byte{}, [32]byte{}, 0
	raw, err := json.Marshal(operation)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(append([]byte("ardents-name-control-operation-v1\x00"), raw...))
}

func commandControlOperations(now time.Time) []commandControlOperation {
	proof := []byte("proof")
	return []commandControlOperation{
		{Kind: "claim", Name: "claim-root", Generation: 1, Authority: [32]byte{1},
			LeaseNotAfter: now.Add(time.Hour).Unix(), OrderingProof: proof},
		{Kind: "renew", Name: "alice", Generation: 1, ExpectedRevision: 1,
			LeaseNotAfter: now.Add(time.Hour).Unix(), AuthorityProof: proof},
		{Kind: "record", Name: "alice", Generation: 1, ExpectedRevision: 1, Target: [32]byte{44},
			RecordNotAfter: now.Add(time.Hour).Unix(), AuthorityProof: proof},
		{Kind: "release", Name: "alice", Generation: 1, ExpectedRevision: 1, AuthorityProof: proof},
		{Kind: "transfer", Name: "alice", Generation: 1, ExpectedRevision: 1,
			SuccessorAuthority: [32]byte{2}, AuthorityProof: proof},
		{Kind: "delegate", Name: "leaf.root", ParentName: "root", ParentGeneration: 1,
			ParentRevision: 1, ChildGeneration: 1, Authority: [32]byte{3},
			LeaseNotAfter: now.Add(time.Hour).Unix(), AuthorityProof: proof},
		{Kind: "policy", Name: "alice", Generation: 1, ExpectedRevision: 1,
			PolicyNotBefore: now.Add(time.Minute).Unix(), RecoveryPolicy: proof, AuthorityProof: proof},
		{Kind: "recovery", Name: "alice", Generation: 1, ExpectedRevision: 1, PolicyID: [32]byte{1},
			RecoveryStep: "initiate", RecoveryNotBefore: now.Add(time.Minute).Unix(), RecoveryProof: proof},
	}
}

func commandControlSurface(kind string) string {
	if kind == "claim" {
		return "root-claim"
	}
	if kind == "policy" || kind == "recovery" {
		return "policy-recovery"
	}
	return "renewal-update"
}

func writeCommandJSON(t *testing.T, name string, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err = os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
