package epoch

import (
	"crypto/ed25519"
	"errors"
	"time"
)

// Policy contains the installed authority and predecessor state used for one
// deterministic Network Epoch decision. Verify reads it synchronously and
// returns owned copies of canonical byte slices; a zero Policy is invalid.
type Policy struct {
	NetworkID            [32]byte
	Authorities          map[[32]byte]ed25519.PublicKey
	Threshold            int
	Profile              string
	Now                  time.Time
	MaterializationIndex uint32
	Previous             *Snapshot
}

// Snapshot is the immutable, complete Epoch/View result consumed atomically by
// Network State. The broad value keeps the authenticated identity, validity,
// commitments, record, and assignment from being observed out of generation.
type Snapshot struct {
	Generation                       string
	NetworkID                        [32]byte
	Epoch                            uint64
	Digest                           [32]byte
	PreviousDigest                   [32]byte
	EpochValidFrom                   time.Time
	ValidUntil                       time.Time
	Profile                          string
	ViewRoot                         [32]byte
	ViewLength                       uint32
	RejectedRoot                     [32]byte
	RejectedLength                   uint32
	RecordPresent                    bool
	NodeID                           [32]byte
	NodePublicKey                    [32]byte
	RecordGeneration                 uint64
	RecordValidFrom                  time.Time
	RecordValidUntil                 time.Time
	DeclaredFamily                   string
	RecordEndpoint                   string
	CarrierProfile                   string
	Capacity                         uint16
	Assignment                       string
	AssignmentDigest                 [32]byte
	DestinationResolutionNodeID      [32]byte
	DestinationResolutionProfile     [maximumDestinationResolutionProfileBytes]byte
	DestinationResolutionProfileSize uint16
	TransitIssuanceNodeID            [32]byte
	TransitIssuanceProfile           [maximumTransitIssuanceProfileBytes]byte
	TransitIssuanceProfileSize       uint16
}

// Decision retains the canonical bytes needed to persist and redistribute one
// verified result. Its slices are owned immutable copies and preserve canonical
// input order. A zero Decision has not been verified.
type Decision struct {
	EpochBytes []byte
	Inputs     [][]byte
	Header     Header
	Snapshot   Snapshot
	Candidates []Candidate

	epoch      epochEnvelope
	accepted   []nodeRecord
	rejections []rejection
}

// Candidate keeps every authenticated fact for one accepted Node
// Record together; its fields cannot drift across parallel indexes.
type Candidate struct {
	NodeID, KeyID, PublicKey, FamilyID, RecordDigest [32]byte
	DomainProof                                      []byte
	Family, Endpoint, CarrierProfile, Domain         string
	Capacity                                         uint16
	RecordGeneration                                 uint64
	ValidFrom, ValidUntil, AssignmentNotAfter        time.Time
}

// Verify authenticates one exact Epoch/View decision and its encoded
// materializations.
func Verify(policy Policy, epochBytes []byte, inputs, encodedMaterials [][]byte, requireMaterials bool) (Decision, error) {
	decision, err := Authenticate(policy, epochBytes, inputs, encodedMaterials, requireMaterials)
	if err != nil {
		return Decision{}, err
	}
	if err := verifyEpochChain(policy.Previous, decision.epoch); err != nil {
		return Decision{}, err
	}
	return decision, nil
}

// Authenticate verifies the signed document, committed View and proofs without
// deciding whether it extends accepted history. State must reconcile all
// authenticated observations before selecting or publishing a generation.
func Authenticate(policy Policy, epochBytes []byte, inputs, encodedMaterials [][]byte, requireMaterials bool) (Decision, error) {
	materials, err := decodeMaterializations(encodedMaterials)
	if err != nil {
		return Decision{}, err
	}
	return verifyEpochCandidate(policy, epochBytes, inputs, materials, requireMaterials)
}

// Materialization returns one canonical inclusion proof for resource.
func (decision Decision) Materialization(index uint32) ([]byte, error) {
	if index >= uint32(len(decision.accepted)) {
		return nil, errors.New("requested materialization index is unavailable")
	}
	values := make([][]byte, len(decision.accepted))
	for position := range decision.accepted {
		values[position] = decision.accepted[position].raw
	}
	material := materialization{
		epochDigest: decision.epoch.digest,
		index:       index,
		record:      append([]byte(nil), values[index]...),
		siblings:    epochCommitmentProof(values, int(index), emptyViewTag),
	}
	return encodeMaterialization(material), nil
}

// VerifyMaterials checks proofs for an already verified decision without
// accepting a second or successor Epoch.
func (decision Decision) VerifyMaterials(encoded [][]byte) error {
	materials, err := decodeMaterializations(encoded)
	if err != nil {
		return err
	}
	return verifyMaterializations(decision.epoch, decision.accepted, materials, true)
}
