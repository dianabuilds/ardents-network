package authority

import (
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/admission"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/claim"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/epoch"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/recovery"
)

type Admission = admission.Admission
type Proof = admission.Proof
type ClaimOrder = claim.ClaimOrder
type ClaimProof = claim.ClaimProof
type ClaimWinner = claim.ClaimWinner
type Record = record.Record
type Policy = record.Policy
type Op = record.Op
type RecordSigningRequest = record.RecordSigningRequest
type RecoveryPolicy = recovery.RecoveryPolicy
type RecoveryProof = recovery.RecoveryProof
type Authorization = recovery.Authorization
type Signature = recovery.Signature
type Claim = claim.Claim

type Store = epoch.Store
type Epoch = epoch.Epoch
type MaterializationPolicy = epoch.MaterializationPolicy
type PendingEntry = epoch.PendingEntry

var CanonicalProof = claim.CanonicalProof
var OpenClaimWinner = claim.OpenClaimWinner
var EncodeRecord = record.EncodeRecord
var DecodeRecord = record.DecodeRecord
var VerifyRecord = record.VerifyRecord
var ApplyLegacy = record.ApplyLegacy
var ApplyAtLegacy = record.ApplyAtLegacy
var Validate = record.Validate
var ValidateParents = record.ValidateParents
var AuthorityKey = record.AuthorityKey
var SignRecord = record.SignRecord
var NewAdmission = admission.NewAdmission
var Open = epoch.Open
var CommitmentFor = claim.CommitmentFor
var RevealTranscript = claim.RevealTranscript
var StatementTranscript = claim.StatementTranscript
