package epoch

import "crypto/ed25519"

const materializationRule = "ardents-namespace-materialization-v1"

// MaterializationPolicy is the installed Network Epoch trust root for current Namespace
// materializations. Keys are copied by Open.
type MaterializationPolicy struct {
	Network     [32]byte
	Rule        string
	Authorities map[[32]byte]ed25519.PublicKey
	Threshold   int
}

// Epoch is the non-record portion of one current Namespace materialization.
// RecordRoot and RecordLength are derived by Commit from the complete input.
type Epoch struct {
	Number           uint64
	Digest           [32]byte
	CutoffOffset     int64
	TransitionRoot   [32]byte
	TransitionLength uint32
	RejectionRoot    [32]byte
	RejectionLength  uint32
}

// Store owns one exclusive bounded durable naming-state root.
type Store struct {
	root   *namespaceRoot
	policy MaterializationPolicy
}

type statement struct {
	network          [32]byte
	epoch            uint64
	epochDigest      [32]byte
	rule             string
	cutoff           int64
	recordRoot       [32]byte
	recordLength     uint32
	transitionRoot   [32]byte
	transitionLength uint32
	rejectionRoot    [32]byte
	rejectionLength  uint32
}

type attestedStatement struct {
	statement  statement
	signerIDs  [][32]byte
	signatures [][]byte
}

type resolutionLeaf struct {
	schema       uint16
	signedRecord []byte
	lineageRoot  [32]byte
	lineageCount uint8
	state        byte
	notAfter     int64
}

type snapshot struct {
	attested attestedStatement
	records  [][]byte
	leaves   [][]byte
	pending  uint64
}
