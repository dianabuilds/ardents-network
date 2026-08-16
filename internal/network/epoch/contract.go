package epoch

import (
	"crypto/ed25519"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch/merkle"
)

// Policy contains the installed authority and predecessor state used for one
// deterministic Network Epoch decision. Verify copies authority keys and all
// returned byte slices; a zero Policy is invalid.
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
	Generation       string
	NetworkID        [32]byte
	Epoch            uint64
	Digest           [32]byte
	PreviousDigest   [32]byte
	EpochValidFrom   time.Time
	ValidUntil       time.Time
	Profile          string
	ViewRoot         [32]byte
	ViewLength       uint32
	RejectedRoot     [32]byte
	RejectedLength   uint32
	RecordPresent    bool
	NodeID           [32]byte
	NodePublicKey    [32]byte
	RecordGeneration uint64
	RecordValidFrom  time.Time
	RecordValidUntil time.Time
	DeclaredFamily   string
	ProbeEndpoint    string
	ProbeCapacity    uint16
	Assignment       string
	AssignmentDigest [32]byte
}

// Decision retains the canonical bytes needed to persist and redistribute one
// verified result. Its slices are owned immutable copies and preserve canonical
// input order. A zero Decision has not been verified.
type Decision struct {
	EpochBytes         []byte
	Inputs             [][]byte
	Snapshot           Snapshot
	NodeIDs            [][32]byte
	KeyIDs             [][32]byte
	PublicKeys         [][32]byte
	FamilyIDs          [][32]byte
	Families           []string
	RecordDigests      [][32]byte
	DomainProofs       [][]byte
	Endpoints          []string
	Capacities         []uint16
	Domains            []string
	ValidFrom          []time.Time
	ValidUntil         []time.Time
	AssignmentNotAfter []time.Time

	epoch      epochEnvelope
	accepted   []nodeRecord
	rejections []rejection
}

// Verify authenticates one exact Epoch/View decision and its encoded
// materializations.
func Verify(policy Policy, epochBytes []byte, inputs, encodedMaterials [][]byte, requireMaterials bool) (Decision, error) {
	materials, err := decodeMaterializations(encodedMaterials)
	if err != nil {
		return Decision{}, err
	}
	return verifyDecision(policy, policy.Previous, epochBytes, inputs, materials, requireMaterials)
}

// Inspect validates canonical Epoch framing and returns only authenticated-
// independent metadata needed to load bounded persisted input files.
func Inspect(raw []byte) (Snapshot, error) {
	parsed, err := parseEpoch(raw)
	if err != nil {
		return Snapshot{}, err
	}
	return snapshotFor(parsed), nil
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
		siblings:    merkle.Proof(values, int(index), emptyViewTag),
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
