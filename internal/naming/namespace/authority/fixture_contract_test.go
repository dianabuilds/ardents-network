package authority

import (
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/admission"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/claim"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/epoch"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/recovery"
)

// Fixture aliases keep tests concise; production authority code imports the
// owning modules directly and exposes none of these compatibility names.
type Admission = admission.Admission
type Proof = admission.Proof
type ClaimOrder = claim.ClaimOrder
type ClaimProof = claim.ClaimProof
type ClaimWinner = claim.ClaimWinner
type Claim = claim.Claim
type Record = record.Record
type Policy = record.Policy
type Op = record.Op
type RecordSigningRequest = record.RecordSigningRequest
type RecoveryPolicy = recovery.RecoveryPolicy
type RecoveryProof = recovery.RecoveryProof
type Signature = recovery.Signature
type Store = epoch.Store
type Epoch = epoch.Epoch
type MaterializationPolicy = epoch.MaterializationPolicy

var NewAdmission = admission.NewAdmission
var SignRecord = record.SignRecord
var DecodeRecord = record.DecodeRecord
var ApplyLegacy = record.ApplyLegacy
var Open = epoch.Open
var CommitmentFor = claim.CommitmentFor
var RevealTranscript = claim.RevealTranscript
var StatementTranscript = claim.StatementTranscript
var OpenClaimWinner = claim.OpenClaimWinner
