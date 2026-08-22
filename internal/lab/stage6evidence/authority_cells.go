package stage6evidence

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameauthority"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

type recoveryTrace struct {
	Network          string   `json:"network"`
	Name             string   `json:"name"`
	Generation       uint64   `json:"generation"`
	PolicyRevision   uint64   `json:"policy_revision"`
	CurrentAuthority string   `json:"current_authority"`
	Threshold        uint8    `json:"threshold"`
	Participants     []string `json:"participants"`
	DelayMillis      int64    `json:"delay_millis"`
	PolicyDigest     string   `json:"policy_digest"`
	OperationID      string   `json:"operation_id"`
	Successor        string   `json:"successor"`
	StartedAt        int64    `json:"started_at"`
	CompletesAt      int64    `json:"completes_at"`
	Signatures       []string `json:"signatures"`
}

func runAuthorityCell(trace *traceRecord) error {
	if trace.Cell == "B3" {
		return runRecoveryCell(trace)
	}
	network := [32]byte{7}
	oldKey, successor := evidenceKey("old-authority"), evidenceKey("successor-authority")
	record := namespace.Record{Name: "alice", Generation: 1, Revision: 2,
		Lease: "active", Consistency: "current", Recovery: "stable", Target: [32]byte{1},
		Authority:      hex.EncodeToString(oldKey.Public().(ed25519.PublicKey)),
		LeaseExpiresAt: 1_000_000, GraceExpiresAt: 1_100_000, Continuity: 1}
	kind := "rotate"
	if trace.Cell == "B1" {
		kind = "transfer"
	}
	op := namespace.Op{Kind: kind, Name: record.Name, Authority: record.Authority,
		SuccessorAuthority: hex.EncodeToString(successor.Public().(ed25519.PublicKey)),
		ExpectedGeneration: record.Generation, ExpectedRevision: record.Revision}
	signature, err := nameauthority.SignTransition(network, record, op, oldKey)
	if err != nil {
		return err
	}
	admission, proof, err := transitionAdmission(network, record, op, "renewal-update", 1)
	if err != nil {
		return err
	}
	changed, err := nameauthority.ApplyAdmittedTransition(admission, proof, 100_000,
		proof.Challenge.OperationDigest, network, record, op, signature, 100, namespace.Policy{})
	if err != nil {
		return err
	}
	if trace.Cell == "B5" {
		replay := op
		replay.ExpectedRevision = changed.Revision
		otherAdmission, otherProof, admissionErr := transitionAdmission(network, changed, replay, "renewal-update", 2)
		if admissionErr != nil {
			return admissionErr
		}
		if _, replayErr := nameauthority.ApplyAdmittedTransition(otherAdmission, otherProof, 100_000,
			otherProof.Challenge.OperationDigest, network, changed, replay, signature, 101, namespace.Policy{}); replayErr == nil {
			return errors.New("predecessor transition replay was accepted")
		}
		trace.Fields = []string{"stale-proof"}
	}
	trace.Auxiliary = signature
	trace.Fields = append(trace.Fields, hex.EncodeToString(network[:]), op.SuccessorAuthority, kind)
	return setRecordTrace(trace, []namespace.Record{record}, []namespace.Record{changed}, []int64{100}, nil)
}

func runRecoveryCell(trace *traceRecord) error {
	network, currentKey := [32]byte{7}, evidenceKey("recovery-current")
	policy := namespace.RecoveryPolicy{Network: network, Name: "alice", Generation: 1,
		Revision: 1, Threshold: 2, Delay: 72 * time.Hour}
	copy(policy.CurrentAuthority[:], currentKey.Public().(ed25519.PublicKey))
	signers := []ed25519.PrivateKey{evidenceKey("recovery-1"), evidenceKey("recovery-2")}
	sort.Slice(signers, func(i, j int) bool {
		return bytes.Compare(signers[i].Public().(ed25519.PublicKey), signers[j].Public().(ed25519.PublicKey)) < 0
	})
	for _, signer := range signers {
		var participant [32]byte
		copy(participant[:], signer.Public().(ed25519.PublicKey))
		policy.Participants = append(policy.Participants, participant)
	}
	successorKey := evidenceKey("recovery-successor")
	var successor [32]byte
	copy(successor[:], successorKey.Public().(ed25519.PublicKey))
	record := namespace.Record{Name: "alice", Generation: 1, Revision: 3,
		Lease: "active", Consistency: "current", Recovery: "stable", Target: [32]byte{1},
		Authority:      hex.EncodeToString(currentKey.Public().(ed25519.PublicKey)),
		LeaseExpiresAt: 1_000_000, GraceExpiresAt: 1_100_000, Continuity: 1,
		RecoveryPolicy: policy.Digest(), RecoveryPolicyRev: 1, RecoveryPolicyDelay: policy.Delay.Milliseconds()}
	recoveryProof := namespace.RecoveryProof{Operation: "initiate", PolicyDigest: policy.Digest(),
		OperationID: sha256.Sum256([]byte("stage6-recovery-operation")), Successor: successor,
		StartedAt: 100_000, CompletesAt: 100_000 + policy.Delay.Milliseconds()}
	for _, signer := range signers {
		var id [32]byte
		copy(id[:], signer.Public().(ed25519.PublicKey))
		recoveryProof.Signatures = append(recoveryProof.Signatures, namespace.Signature{Signer: id,
			Bytes: ed25519.Sign(signer, policy.Transcript(recoveryProof))})
	}
	authorization, err := policy.Authorize(recoveryProof)
	if err != nil {
		return err
	}
	start := namespace.Op{Kind: "start-recovery", Name: record.Name, ExpectedGeneration: 1,
		ExpectedRevision: record.Revision, RecoveryAuthorization: authorization}
	startAdmission, startProof, err := transitionAdmission(network, record, start, "policy-recovery", 3)
	if err != nil {
		return err
	}
	pending, err := nameauthority.ApplyAdmittedTransition(startAdmission, startProof, 100_000,
		startProof.Challenge.OperationDigest, network, record, start, nil, 100, namespace.Policy{})
	if err != nil {
		return err
	}
	complete := namespace.Op{Kind: "complete-recovery", Name: pending.Name, ExpectedGeneration: 1,
		ExpectedRevision: pending.Revision, RecoveryAuthorization: authorization}
	completeAdmission, completeProof, err := transitionAdmission(network, pending, complete, "policy-recovery", 4)
	if err != nil {
		return err
	}
	completed, err := nameauthority.ApplyAdmittedTransition(completeAdmission, completeProof, 100_000,
		completeProof.Challenge.OperationDigest, network, pending, complete, nil,
		recoveryProof.CompletesAt/1_000, namespace.Policy{})
	if err != nil {
		return err
	}
	resume := namespace.Op{Kind: "resume-recovery", Name: completed.Name, Authority: completed.Authority,
		ExpectedGeneration: 1, ExpectedRevision: completed.Revision, Target: [32]byte{9}}
	resumeSignature, err := nameauthority.SignTransition(network, completed, resume, successorKey)
	if err != nil {
		return err
	}
	resumeAdmission, resumeProof, err := transitionAdmission(network, completed, resume, "policy-recovery", 5)
	if err != nil {
		return err
	}
	resumed, err := nameauthority.ApplyAdmittedTransition(resumeAdmission, resumeProof, 100_000,
		resumeProof.Challenge.OperationDigest, network, completed, resume, resumeSignature,
		recoveryProof.CompletesAt/1_000+1, namespace.Policy{})
	if err != nil {
		return err
	}
	policyDigest := policy.Digest()
	evidence := recoveryTrace{Network: hex.EncodeToString(network[:]), Name: policy.Name,
		Generation: 1, PolicyRevision: 1, CurrentAuthority: record.Authority, Threshold: policy.Threshold,
		DelayMillis: policy.Delay.Milliseconds(), PolicyDigest: hex.EncodeToString(policyDigest[:]),
		OperationID: hex.EncodeToString(recoveryProof.OperationID[:]), Successor: hex.EncodeToString(successor[:]),
		StartedAt: recoveryProof.StartedAt, CompletesAt: recoveryProof.CompletesAt}
	for index, participant := range policy.Participants {
		evidence.Participants = append(evidence.Participants, hex.EncodeToString(participant[:]))
		evidence.Signatures = append(evidence.Signatures, hex.EncodeToString(recoveryProof.Signatures[index].Bytes))
	}
	trace.Auxiliary, err = json.Marshal(evidence)
	if err != nil {
		return err
	}
	return setRecordTrace(trace, []namespace.Record{record},
		[]namespace.Record{pending, completed, resumed}, []int64{recoveryProof.StartedAt, recoveryProof.CompletesAt}, nil)
}

func transitionAdmission(network [32]byte, record namespace.Record, op namespace.Op,
	surface string, nonce byte,
) (*namespace.Admission, namespace.Proof, error) {
	digest, err := nameauthority.TransitionDigest(network, record, op)
	if err != nil {
		return nil, namespace.Proof{}, err
	}
	admission, err := namespace.NewAdmission([32]byte{9}, network, 1, [32]byte{8, nonce})
	if err != nil {
		return nil, namespace.Proof{}, err
	}
	challenge, err := admission.Issue(100_000, surface, digest, [32]byte{nonce}, 110_000, [16]byte{nonce})
	if err != nil {
		return nil, namespace.Proof{}, err
	}
	proof, _ := challenge.Solve()
	return admission, proof, nil
}

func evidenceKey(label string) ed25519.PrivateKey {
	seed := sha256.Sum256([]byte(label))
	return ed25519.NewKeyFromSeed(seed[:])
}
