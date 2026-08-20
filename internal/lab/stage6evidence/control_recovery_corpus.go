package stage6evidence

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameauthority"
	"github.com/dianabuilds/ardents-network/internal/namelease"
	"github.com/dianabuilds/ardents-network/internal/namerecovery"
)

func recoveryControlOperations(network [32]byte, now time.Time, records []namelease.Record,
	keys map[string]ed25519.PrivateKey,
) ([]controlOperation, error) {
	policyAdd, err := policyControlOperation(network, now, records, keys, "policy-name", false)
	if err != nil {
		return nil, err
	}
	policyDisable, err := policyControlOperation(network, now, records, keys, "policy-disable-name", true)
	if err != nil {
		return nil, err
	}
	initiate, err := recoveryControlOperation(network, now, records, keys, "recovery-name", "initiate")
	if err != nil {
		return nil, err
	}
	cancel, err := recoveryControlOperation(network, now, records, keys, "recovery-cancel-name", "cancel")
	if err != nil {
		return nil, err
	}
	complete, err := recoveryControlOperation(network, now, records, keys, "recovery-complete-name", "complete")
	if err != nil {
		return nil, err
	}
	resume, err := recoveryResumeOperation(network, now, records, keys)
	if err != nil {
		return nil, err
	}
	return []controlOperation{policyAdd, policyDisable, initiate, cancel, complete, resume}, nil
}

func policyControlOperation(network [32]byte, now time.Time, records []namelease.Record,
	keys map[string]ed25519.PrivateKey, name string, disable bool,
) (controlOperation, error) {
	index := recordIndex(records, name)
	record := records[index]
	policy, _ := controlRecoveryPolicy(network, record, keys[name])
	if disable {
		record.RecoveryPolicy, record.RecoveryPolicyRev = policy.Digest(), policy.Revision
		record.RecoveryPolicyDelay = policy.Delay.Milliseconds()
		records[index] = record
	}
	op := namelease.Op{Kind: "schedule-recovery-policy", Name: name, Authority: record.Authority,
		ExpectedGeneration: 1, ExpectedRevision: 1, PolicyRevision: 1, PolicyDelay: policy.Delay,
		PolicyActivatesAt: now.Add(policy.Delay).UnixMilli()}
	var policyRaw []byte
	if disable {
		op.PolicyRevision = 2
	} else {
		op.PolicyDigest = policy.Digest()
		policyRaw, _ = json.Marshal(policy)
	}
	signature, err := nameauthority.SignTransition(network, record, op, keys[name])
	return controlOperation{Kind: "policy", Name: name, Generation: 1, ExpectedRevision: 1,
		PolicyNotBefore: op.PolicyActivatesAt, RecoveryPolicy: policyRaw, AuthorityProof: signature}, err
}

func recoveryControlOperation(network [32]byte, now time.Time, records []namelease.Record,
	keys map[string]ed25519.PrivateKey, name, step string,
) (controlOperation, error) {
	index := recordIndex(records, name)
	record := records[index]
	policy, signers := controlRecoveryPolicy(network, record, keys[name])
	record.RecoveryPolicy, record.RecoveryPolicyRev = policy.Digest(), policy.Revision
	record.RecoveryPolicyDelay = policy.Delay.Milliseconds()
	started := now
	proofOperation := "initiate"
	if step == "cancel" {
		started, proofOperation = now.Add(-time.Second), "cancel"
	}
	if step == "complete" {
		started = now.Add(-policy.Delay)
	}
	proof := signedRecoveryProof(policy, signers, proofOperation, name, started)
	if step == "cancel" || step == "complete" {
		record.Recovery = "recovery-pending"
		record.RecoveryOperation, record.RecoverySuccessor = proof.OperationID, proof.Successor
		record.RecoveryStartedAt, record.RecoveryExpiresAt = proof.StartedAt, proof.CompletesAt
	}
	records[index] = record
	envelope, err := json.Marshal(struct {
		Policy namerecovery.RecoveryPolicy `json:"policy"`
		Proof  namerecovery.Proof          `json:"proof"`
	}{policy, proof})
	return controlOperation{Kind: "recovery", Name: name, Generation: 1, ExpectedRevision: 1,
		PolicyID: policy.Digest(), RecoveryStep: step, RecoveryNotBefore: now.UnixMilli(), RecoveryProof: envelope}, err
}

func recoveryResumeOperation(network [32]byte, now time.Time, records []namelease.Record,
	keys map[string]ed25519.PrivateKey,
) (controlOperation, error) {
	name := "recovery-resume-name"
	index := recordIndex(records, name)
	record := records[index]
	policy, _ := controlRecoveryPolicy(network, record, keys[name])
	successor := evidenceKey("control-recovery-resume-successor")
	record.RecoveryPolicy, record.RecoveryPolicyRev = policy.Digest(), policy.Revision
	record.RecoveryPolicyDelay = policy.Delay.Milliseconds()
	record.Authority = hex.EncodeToString(successor.Public().(ed25519.PublicKey))
	record.Target, record.Consistency = [32]byte{}, "unavailable"
	records[index] = record
	op := namelease.Op{Kind: "resume-recovery", Name: name, Authority: record.Authority,
		ExpectedGeneration: 1, ExpectedRevision: 1, Target: [32]byte{24}}
	signature, err := nameauthority.SignTransition(network, record, op, successor)
	return controlOperation{Kind: "recovery", Name: name, Generation: 1, ExpectedRevision: 1,
		PolicyID: policy.Digest(), RecoveryStep: "resume", RecoveryNotBefore: now.UnixMilli(),
		Target: op.Target, AuthorityProof: signature}, err
}

func signedRecoveryProof(policy namerecovery.RecoveryPolicy, signers []ed25519.PrivateKey,
	operation, label string, started time.Time,
) namerecovery.Proof {
	proof := namerecovery.Proof{Operation: operation, PolicyDigest: policy.Digest(),
		OperationID: sha256.Sum256([]byte("control-" + label)),
		Successor:   publicBytes(evidenceKey("control-" + label + "-successor")),
		StartedAt:   started.UnixMilli(), CompletesAt: started.Add(policy.Delay).UnixMilli()}
	for _, signer := range signers {
		proof.Signatures = append(proof.Signatures, namerecovery.Signature{Signer: publicBytes(signer),
			Bytes: ed25519.Sign(signer, policy.Transcript(proof))})
	}
	return proof
}
