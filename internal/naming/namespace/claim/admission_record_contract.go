package claim

import (
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/admission"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
)

type Admission = admission.Admission
type Proof = admission.Proof
type Record = record.Record
type Policy = record.Policy
type Op = record.Op
type RecordSigner = record.RecordSigner

var ApplyAtLegacy = record.ApplyAtLegacy
var SignWith = record.SignWith
