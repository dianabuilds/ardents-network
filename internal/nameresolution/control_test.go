package nameresolution_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"sync"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/nameadmission"
	"github.com/dianabuilds/ardents-network/internal/nameresolution"
)

type testControlOperation struct {
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

func TestControlUsesTheResolutionOHTTPBoundaryAndExposesOnlyTheAuthorityView(t *testing.T) {
	t.Parallel()
	authority := &capturingControlAuthority{class: "accepted", generation: 4, revision: 9,
		state: []byte("accepted-state")}
	fixture := newResolutionFixtureWithControl(t, authority)
	isolation := [32]byte{41}
	operation := testControlOperation{Kind: "renew", Name: "alice", Generation: 4,
		ExpectedRevision: 8, LeaseNotAfter: fixture.now.Add(2 * 60 * 60 * 1e9).UnixMilli(),
		AuthorityProof: []byte("authority-proof-forbidden-at-relay")}
	operation.OperationDigest = testControlDigest(t, operation)
	digest := operation.OperationDigest
	challenge, err := fixture.admission.Issue(fixture.now.UnixMilli(), "renewal-update", digest, isolation,
		fixture.selection.Deadline.UnixMilli(), [16]byte{91})
	if err != nil {
		t.Fatal(err)
	}
	selection := nameresolution.Selection{At: fixture.now, Deadline: fixture.selection.Deadline,
		RelayNodeID: fixture.selection.RelayNodeID, GatewayNodeID: fixture.selection.GatewayNodeID,
		AdmissionChallenge: challenge}
	client, err := nameresolution.OpenControl(fixture.view, selection, fixture.gatewayProfile(), isolation,
		relayTransport(fixture.relayServer))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(operation)
	result, err := client.Execute(context.Background(), raw, fixture.now)
	if err != nil || result.Class != "accepted" || result.Generation != 4 || result.Revision != 9 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	observed, proof := authority.observation()
	if observed.Kind != operation.Kind || observed.Name != operation.Name || observed.Network != [32]byte{} ||
		observed.Nonce != [32]byte{} || observed.Deadline != 0 ||
		proof.Challenge.OperationDigest != digest {
		t.Fatalf("authority view=%+v proof=%+v", observed, proof)
	}
	envelopes, _ := fixture.relayEvidence()
	if len(envelopes) != 1 || bytes.Contains(envelopes[0], []byte("alice")) ||
		bytes.Contains(envelopes[0], isolation[:]) || bytes.Contains(envelopes[0], operation.AuthorityProof) {
		t.Fatal("Relay envelope exposed a forbidden control field")
	}
}

func TestControlRejectsFieldsForbiddenForTheSelectedOperation(t *testing.T) {
	t.Parallel()
	authority := &capturingControlAuthority{class: "accepted", generation: 1, revision: 1,
		state: []byte("accepted-state")}
	fixture := newResolutionFixtureWithControl(t, authority)
	base := testControlOperation{Kind: "renew", Name: "alice", Generation: 1,
		ExpectedRevision: 2, LeaseNotAfter: 3, AuthorityProof: []byte{1}}
	base.OperationDigest = testControlDigest(t, base)
	mutations := []func(*testControlOperation){
		func(value *testControlOperation) { value.ParentName = "root" },
		func(value *testControlOperation) { value.Target = [32]byte{1} },
		func(value *testControlOperation) { value.RecoveryProof = []byte{1} },
		func(value *testControlOperation) { value.Network = [32]byte{1} },
		func(value *testControlOperation) { value.Nonce = [32]byte{1} },
		func(value *testControlOperation) { value.Deadline = 1 },
	}
	for index, mutate := range mutations {
		value := base
		mutate(&value)
		isolation := [32]byte{byte(index + 21)}
		challenge, err := fixture.admission.Issue(fixture.now.UnixMilli(), "renewal-update", base.OperationDigest,
			isolation, fixture.selection.Deadline.UnixMilli(), [16]byte{byte(index + 31)})
		if err != nil {
			t.Fatal(err)
		}
		selection := nameresolution.Selection{At: fixture.now, Deadline: fixture.selection.Deadline,
			RelayNodeID: fixture.selection.RelayNodeID, GatewayNodeID: fixture.selection.GatewayNodeID,
			AdmissionChallenge: challenge}
		client, err := nameresolution.OpenControl(fixture.view, selection, fixture.gatewayProfile(), isolation,
			relayTransport(fixture.relayServer))
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(value)
		if _, err := client.Execute(context.Background(), raw, fixture.now); err == nil {
			t.Fatal("forbidden control field was accepted")
		}
	}
}

func TestControlCarriesEveryFrozenOperationShape(t *testing.T) {
	t.Parallel()
	authority := &capturingControlAuthority{class: "accepted", generation: 1, revision: 1,
		state: []byte("accepted-state")}
	fixture := newResolutionFixtureWithControl(t, authority)
	operations := []testControlOperation{
		{Kind: "claim", Name: "alice", Generation: 1, Authority: [32]byte{1}, LeaseNotAfter: 1, OrderingProof: []byte{1}},
		{Kind: "renew", Name: "alice", Generation: 1, ExpectedRevision: 1, LeaseNotAfter: 1, AuthorityProof: []byte{1}},
		{Kind: "record", Name: "alice", Generation: 1, ExpectedRevision: 1, Target: [32]byte{1}, RecordNotAfter: 1, AuthorityProof: []byte{1}},
		{Kind: "release", Name: "alice", Generation: 1, ExpectedRevision: 1, AuthorityProof: []byte{1}},
		{Kind: "transfer", Name: "alice", Generation: 1, ExpectedRevision: 1, SuccessorAuthority: [32]byte{2}, AuthorityProof: []byte{1}},
		{Kind: "delegate", Name: "leaf.root", ParentName: "root", ParentGeneration: 1, ParentRevision: 1,
			ChildGeneration: 1, Authority: [32]byte{3}, LeaseNotAfter: 1, AuthorityProof: []byte{1}},
		{Kind: "policy", Name: "alice", Generation: 1, ExpectedRevision: 1, PolicyNotBefore: 1,
			RecoveryPolicy: []byte{1}, AuthorityProof: []byte{1}},
		{Kind: "recovery", Name: "alice", Generation: 1, ExpectedRevision: 1, PolicyID: [32]byte{1},
			RecoveryStep: "initiate", RecoveryNotBefore: 1, RecoveryProof: []byte{1}},
	}
	for index := range operations {
		operation := &operations[index]
		operation.OperationDigest = testControlDigest(t, *operation)
		surface := "renewal-update"
		if operation.Kind == "claim" {
			surface = "root-claim"
		}
		if operation.Kind == "policy" || operation.Kind == "recovery" {
			surface = "policy-recovery"
		}
		isolation := [32]byte{byte(index + 1)}
		challenge, err := fixture.admission.Issue(fixture.now.UnixMilli(), surface, operation.OperationDigest,
			isolation, fixture.selection.Deadline.UnixMilli(), [16]byte{byte(index + 1)})
		if err != nil {
			t.Fatal(err)
		}
		selection := nameresolution.Selection{At: fixture.now, Deadline: fixture.selection.Deadline,
			RelayNodeID: fixture.selection.RelayNodeID, GatewayNodeID: fixture.selection.GatewayNodeID,
			AdmissionChallenge: challenge}
		client, err := nameresolution.OpenControl(fixture.view, selection, fixture.gatewayProfile(), isolation,
			relayTransport(fixture.relayServer))
		if err != nil {
			t.Fatalf("%s OpenControl: %v", operation.Kind, err)
		}
		raw, _ := json.Marshal(operation)
		if _, err := client.Execute(context.Background(), raw, fixture.now); err != nil {
			t.Fatalf("%s Execute: %v", operation.Kind, err)
		}
	}
}

func testControlDigest(t *testing.T, operation testControlOperation) [32]byte {
	t.Helper()
	operation.OperationDigest = [32]byte{}
	operation.Network, operation.Nonce, operation.Deadline = [32]byte{}, [32]byte{}, 0
	raw, err := json.Marshal(operation)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(append([]byte("ardents-name-control-operation-v1\x00"), raw...))
}

type capturingControlAuthority struct {
	mu                   sync.Mutex
	operation            testControlOperation
	proof                nameadmission.Proof
	class                string
	generation, revision uint64
	state                []byte
}

func (authority *capturingControlAuthority) Apply(raw []byte, proof nameadmission.Proof,
) (string, uint64, uint64, []byte) {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	var operation testControlOperation
	if err := json.Unmarshal(raw, &operation); err != nil {
		return "denied", 0, 0, nil
	}
	authority.operation, authority.proof = operation, proof
	return authority.class, authority.generation, authority.revision, append([]byte(nil), authority.state...)
}

func (authority *capturingControlAuthority) observation() (testControlOperation, nameadmission.Proof) {
	authority.mu.Lock()
	defer authority.mu.Unlock()
	return authority.operation, authority.proof
}
