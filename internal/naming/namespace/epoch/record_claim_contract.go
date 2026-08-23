package epoch

import (
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/claim"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
)

type Record = record.Record
type Binding = record.Binding
type RecordSigner = record.RecordSigner
type RecordSigningRequest = record.RecordSigningRequest
type ClaimWinner = claim.ClaimWinner
type Policy = record.Policy

var EncodeRecord = record.EncodeRecord
var DecodeRecord = record.DecodeRecord
var VerifyRecord = record.VerifyRecord
