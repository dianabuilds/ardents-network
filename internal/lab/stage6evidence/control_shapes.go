package stage6evidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameresolution"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

type controlExchangeEvidence struct {
	Isolation [32]byte
	Admission namespace.Proof
	Envelope  []byte
	Operation controlOperation
	Result    controlExecutionResult
}

type controlOperation struct {
	Kind               string   `json:"kind"`
	OperationDigest    [32]byte `json:"operation_digest"`
	Network            [32]byte `json:"network,omitempty"`
	Nonce              [32]byte `json:"nonce,omitempty"`
	Deadline           int64    `json:"deadline,omitempty"`
	Name               string   `json:"name"`
	ParentName         string   `json:"parent_name,omitempty"`
	Generation         uint64   `json:"generation,omitempty"`
	ExpectedRevision   uint64   `json:"expected_revision,omitempty"`
	ParentGeneration   uint64   `json:"parent_generation,omitempty"`
	ParentRevision     uint64   `json:"parent_revision,omitempty"`
	ChildGeneration    uint64   `json:"child_generation,omitempty"`
	Authority          [32]byte `json:"authority,omitempty"`
	SuccessorAuthority [32]byte `json:"successor_authority,omitempty"`
	Target             [32]byte `json:"target,omitempty"`
	LeaseNotAfter      int64    `json:"lease_not_after,omitempty"`
	RecordNotAfter     int64    `json:"record_not_after,omitempty"`
	PolicyNotBefore    int64    `json:"policy_not_before,omitempty"`
	RecoveryNotBefore  int64    `json:"recovery_not_before,omitempty"`
	PolicyID           [32]byte `json:"policy_id,omitempty"`
	RecoveryStep       string   `json:"recovery_step,omitempty"`
	OrderingProof      []byte   `json:"ordering_proof,omitempty"`
	AuthorityProof     []byte   `json:"authority_proof,omitempty"`
	RecoveryPolicy     []byte   `json:"recovery_policy,omitempty"`
	RecoveryProof      []byte   `json:"recovery_proof,omitempty"`
}

type controlExecutionResult struct {
	Class      string
	Generation uint64
	Revision   uint64
	State      []byte
}

func controlShapeDigest(operation controlOperation) [32]byte {
	operation.OperationDigest = [32]byte{}
	operation.Network, operation.Nonce, operation.Deadline = [32]byte{}, [32]byte{}, 0
	raw, _ := json.Marshal(operation)
	return sha256.Sum256(append([]byte("ardents-name-control-operation-v1\x00"), raw...))
}

func executeControlOperations(fixture resolutionFixture, gate *namespace.Admission,
	operations []controlOperation, now time.Time,
) ([][32]byte, []controlExecutionResult, error) {
	isolations := make([][32]byte, len(operations))
	results := make([]controlExecutionResult, len(operations))
	for index, operation := range operations {
		isolations[index] = sha256.Sum256([]byte("ardents-stage-6-control-isolation-v1\x00" + operation.Kind))
		challenge, err := gate.Issue(now.UnixMilli(), controlSurface(operation.Kind), operation.OperationDigest,
			isolations[index], fixture.selection.Deadline.UnixMilli(), [16]byte{byte(index + 21)})
		if err != nil {
			return nil, nil, err
		}
		selection := nameresolution.Selection{At: now, Deadline: fixture.selection.Deadline,
			RelayNodeID: fixture.selection.RelayNodeID, GatewayNodeID: fixture.selection.GatewayNodeID,
			AdmissionChallenge: challenge}
		client, err := nameresolution.OpenControl(fixture.view, selection, fixture.profile(), isolations[index],
			fixture.relay.Client().Transport.(*http.Transport))
		if err != nil {
			return nil, nil, err
		}
		raw, encodeErr := json.Marshal(operation)
		if encodeErr != nil {
			return nil, nil, encodeErr
		}
		result, executeErr := client.Execute(context.Background(), raw, now)
		if executeErr != nil || result.Class != "submitted" {
			return nil, nil, fmt.Errorf("private control shape %s was not admitted: %w", operation.Kind, executeErr)
		}
		results[index] = controlExecutionResult{Class: result.Class}
	}
	return isolations, results, nil
}

func controlSurface(kind string) string {
	if kind == "claim" {
		return "root-claim"
	}
	if kind == "policy" || kind == "recovery" {
		return "policy-recovery"
	}
	return "renewal-update"
}

func cleanControlEnvelope(envelope []byte, isolation [32]byte, operation controlOperation) bool {
	for _, forbidden := range [][]byte{[]byte(operation.Name), []byte(operation.ParentName), isolation[:],
		operation.PolicyID[:], []byte(operation.RecoveryStep), operation.OrderingProof, operation.AuthorityProof,
		operation.RecoveryPolicy, operation.RecoveryProof} {
		if len(forbidden) > 0 && bytes.Contains(envelope, forbidden) {
			return false
		}
	}
	return len(envelope) > 0
}
