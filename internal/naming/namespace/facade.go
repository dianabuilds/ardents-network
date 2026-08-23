package namespace

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace/admission"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/authority"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/claim"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/epoch"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/recovery"
)

// The aliases preserve the C4 compatibility surface while callers migrate to
// the cohesive nested Namespace modules.
type Admission = admission.Admission
type Challenge = admission.Challenge
type Proof = admission.Proof
type Claim = claim.Claim
type ClaimCommitment = claim.ClaimCommitment
type ClaimOrder = claim.ClaimOrder
type ClaimProof = claim.ClaimProof
type ClaimWinner = claim.ClaimWinner
type EpochClaimInput = claim.EpochClaimInput
type Authorization = recovery.Authorization
type RecoveryPolicy = recovery.RecoveryPolicy
type RecoveryProof = recovery.RecoveryProof
type Signature = recovery.Signature
type Binding = record.Binding
type Op = record.Op
type Policy = record.Policy
type Record = record.Record
type RecordSigner = record.RecordSigner
type RecordSigningRequest = record.RecordSigningRequest
type Epoch = epoch.Epoch
type EpochInstallation = epoch.EpochInstallation
type MaterializationPolicy = epoch.MaterializationPolicy
type Store = epoch.Store
type Submission = authority.Submission

type control interface {
	Submit(Submission, Proof) string
}

type evidenceControl interface {
	ApplyEvidence([]byte, Proof) (string, uint64, uint64, []byte)
}

var (
	NewAdmission            = admission.NewAdmission
	CommitmentFor           = claim.CommitmentFor
	RevealTranscript        = claim.RevealTranscript
	StatementTranscript     = claim.StatementTranscript
	CanonicalProof          = claim.CanonicalProof
	AdmitClaimCommitment    = claim.AdmitClaimCommitment
	OpenClaimWinner         = claim.OpenClaimWinner
	EncodeRecord            = record.EncodeRecord
	DecodeRecord            = record.DecodeRecord
	SignRecord              = record.SignRecord
	VerifyRecord            = record.VerifyRecord
	ResolveBindingLegacy    = record.ResolveBindingLegacy
	ApplyLegacy             = record.ApplyLegacy
	ApplyAtLegacy           = record.ApplyAtLegacy
	Open                    = epoch.Open
	VerifyLegacy            = epoch.VerifyLegacy
	VerifyBinding           = epoch.VerifyBinding
	OpenSubmission          = authority.OpenSubmission
	SignTransition          = authority.SignTransition
	TransitionDigest        = authority.TransitionDigest
	ApplyAdmittedTransition = authority.ApplyAdmittedTransition
)

func NewEvidenceControl(network [32]byte, gate *Admission, order ClaimOrder,
	records []Record, clock func() time.Time, policy Policy,
) (evidenceControl, error) {
	return authority.NewEvidenceControl(network, gate, order, records, clock, policy)
}

func OpenControl(store *Store, gate *Admission, order ClaimOrder,
	clock func() time.Time, policy Policy,
) (control, error) {
	return authority.OpenControl(store, gate, order, clock, policy)
}
