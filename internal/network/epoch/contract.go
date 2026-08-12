package epoch

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch/merkle"
)

// Policy contains the installed authority and predecessor state used for one
// deterministic Network Epoch decision.
type Policy struct {
	NetworkID            [32]byte
	Authorities          map[[32]byte]ed25519.PublicKey
	Threshold            int
	Now                  time.Time
	MaterializationIndex uint32
	Previous             *Snapshot
}

// Snapshot is the immutable Epoch/View result consumed by Network State.
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
// verified result. Its slices are owned immutable copies.
type Decision struct {
	EpochBytes []byte
	Inputs     [][]byte
	Snapshot   Snapshot
	Identities [][32]byte
	Families   []string
	Endpoints  []string

	epoch      epochEnvelope
	accepted   []nodeRecord
	rejections []rejection
}

type materialization struct {
	epochDigest [32]byte
	index       uint32
	record      []byte
	siblings    [][32]byte
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

func snapshotFor(value epochEnvelope) Snapshot {
	return Snapshot{
		Generation:     value.digestString(),
		NetworkID:      value.networkID,
		Epoch:          value.number,
		Digest:         value.digest,
		PreviousDigest: value.previous,
		EpochValidFrom: value.validFrom,
		ValidUntil:     value.validUntil,
		Profile:        epochProfile,
		ViewRoot:       value.viewRoot,
		ViewLength:     value.viewLength,
		RejectedRoot:   value.rejectedRoot,
		RejectedLength: value.rejectedLength,
	}
}

func (value epochEnvelope) digestString() string {
	const hexadecimal = "0123456789abcdef"
	encoded := make([]byte, len(value.digest)*2)
	for index, current := range value.digest {
		encoded[index*2] = hexadecimal[current>>4]
		encoded[index*2+1] = hexadecimal[current&15]
	}
	return string(encoded)
}

func decodeMaterializations(encoded [][]byte) ([]materialization, error) {
	if len(encoded) > 64 {
		return nil, errors.New("materialization count exceeds 64")
	}
	materials := make([]materialization, len(encoded))
	for index, raw := range encoded {
		if len(raw) == 0 || len(raw) > 35<<10 {
			return nil, errors.New("materialization framing length is invalid")
		}
		value, err := decodeMaterialization(raw)
		if err != nil {
			return nil, err
		}
		materials[index] = value
	}
	return materials, nil
}

func decodeMaterialization(raw []byte) (materialization, error) {
	d := newDecoder(raw)
	digest, err := d.bytes(32)
	if err != nil {
		return materialization{}, err
	}
	var value materialization
	copy(value.epochDigest[:], digest)
	if value.index, err = d.uint32(); err != nil {
		return materialization{}, err
	}
	record, err := lengthBytes(&d, maximumRecordBytes)
	if err != nil {
		return materialization{}, err
	}
	value.record = append([]byte(nil), record...)
	count, err := d.uint16()
	if err != nil || count > 64 {
		return materialization{}, errors.New("materialization proof count is invalid")
	}
	for range int(count) {
		sibling, readErr := d.bytes(32)
		if readErr != nil {
			return materialization{}, readErr
		}
		var digest [32]byte
		copy(digest[:], sibling)
		value.siblings = append(value.siblings, digest)
	}
	if !d.done() {
		return materialization{}, errors.New("materialization has trailing bytes")
	}
	return value, nil
}

func encodeMaterialization(value materialization) []byte {
	buffer := new(bytes.Buffer)
	buffer.Write(value.epochDigest[:])
	_ = binary.Write(buffer, binary.BigEndian, value.index)
	writeLengthBytes(buffer, value.record)
	_ = binary.Write(buffer, binary.BigEndian, uint16(len(value.siblings)))
	for _, sibling := range value.siblings {
		buffer.Write(sibling[:])
	}
	return buffer.Bytes()
}

func lengthBytes(d *decoder, maximum int) ([]byte, error) {
	length, err := d.uint32()
	if err != nil || length == 0 || length > uint32(maximum) {
		return nil, errors.New("materialization member length is invalid")
	}
	return d.bytes(int(length))
}

func writeLengthBytes(buffer *bytes.Buffer, raw []byte) {
	_ = binary.Write(buffer, binary.BigEndian, uint32(len(raw)))
	buffer.Write(raw)
}
