package authority

import (
	"crypto/ed25519"
	"encoding/json"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/epoch"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
)

func TestControlPreparesOneCustodyDerivedSubmissionWithoutMutatingState(t *testing.T) {
	now, network := time.Unix(1_800_001_000, 0).UTC(), [32]byte{12}
	key := deterministicControlKey("custody-derived-control")
	current := controlTestRecord("alice", key, now)
	policy := Policy{DefaultLeaseDuration: 2 * time.Hour, DefaultGraceDuration: time.Hour}
	control := &control{network: network, records: map[string]Record{current.Name: current},
		clock: func() time.Time { return now }, policy: policy, store: new(epoch.Store)}
	operation := controlOperation{Kind: "renew", SigningMode: custodyDerivedSigningMode, Name: current.Name,
		Generation: current.Generation, ExpectedRevision: current.Revision,
		LeaseNotAfter: now.Add(policy.DefaultLeaseDuration).UnixMilli()}
	operation.OperationDigest = canonicalControlDigest(operation)
	raw, err := json.Marshal(operation)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := OpenIntent(raw)
	if err != nil {
		t.Fatalf("open custody intent: %v", err)
	}
	signer := pairedTestSigner{private: key}
	submission, err := control.Prepare(intent, &signer)
	if err != nil {
		t.Fatalf("prepare custody submission: %v", err)
	}
	if signer.transitions != 1 || signer.records != 1 {
		t.Fatalf("unexpected preparation signer calls: transitions=%d records=%d", signer.transitions, signer.records)
	}
	if !submission.MatchesSignatures(signer.transition, signer.successor) {
		prepared, decodeErr := decodeControlOperation(submission.Canonical())
		t.Fatalf("prepared submission did not retain paired signatures: err=%v transition=%t successor=%t", decodeErr,
			decodeErr == nil && string(prepared.AuthorityProof) == string(signer.transition),
			decodeErr == nil && string(prepared.SuccessorRecord) == string(signer.successor))
	}
	operation, err = decodeControlOperation(submission.Canonical())
	if err != nil || operation.OperationDigest != intent.Digest() {
		t.Fatalf("prepared submission=%+v err=%v", operation, err)
	}
	updated, err := record.VerifyRecord(network, operation.SuccessorRecord)
	if err != nil || updated.Generation != current.Generation || updated.Revision != current.Revision+1 {
		t.Fatalf("prepared successor=%+v err=%v", updated, err)
	}
	if control.records[current.Name] != current {
		t.Fatal("preparation mutated the control state before admission and submission")
	}
	forged := operation
	forged.AuthorityProof = []byte{1}
	if forged.OperationDigest = canonicalControlDigest(forged); forged.OperationDigest != intent.Digest() {
		t.Fatal("custody-derived digest included the generated signature")
	}
}

type pairedTestSigner struct {
	private     ed25519.PrivateKey
	transition  []byte
	successor   []byte
	transitions int
	records     int
}

func (signer *pairedTestSigner) SignTransition(request TransitionSigningRequest) ([]byte, error) {
	signer.transitions++
	signer.transition = ed25519.Sign(signer.private, request.Transcript())
	return append([]byte(nil), signer.transition...), nil
}

func (signer *pairedTestSigner) SignRecord(request record.RecordSigningRequest) ([]byte, error) {
	signer.records++
	signer.successor = ed25519.Sign(signer.private, request.Transcript())
	return append([]byte(nil), signer.successor...), nil
}
