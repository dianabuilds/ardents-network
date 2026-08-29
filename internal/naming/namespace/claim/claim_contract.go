package claim

import (
	"crypto/ed25519"
	"sync"
)

const claimOrderRule = "ardents-name-claim-order-v1"

// ClaimOrder is the authenticated Epoch policy for one bounded verification.
type ClaimOrder struct {
	Network       [32]byte
	Rule          string
	MinimumEpoch  uint64
	MaximumClaims uint32
	Authorities   map[[32]byte]ed25519.PublicKey
	Threshold     int
}

// Claim is one revealed root-name commitment input.
type Claim struct {
	Ordinal         uint32
	Name            string
	Secret          [32]byte
	Authority       [32]byte
	Commitment      [32]byte
	AdmissionDigest [32]byte
	InputPath       [][32]byte
	Signature       [64]byte
}

// ClaimProof carries one Name's reveals plus the threshold-authenticated Epoch close
// and their input/materialization inclusion paths. AlternateSets contain only
// close fields and signatures.
type ClaimProof struct {
	Network                [32]byte
	Epoch                  uint64
	Rule                   string
	CutoffOffset           int64
	InputRoot              [32]byte
	InputLength            uint32
	MaterializationRoot    [32]byte
	MaterializationLength  uint32
	RejectionRoot          [32]byte
	RejectionLength        uint32
	MaterializationOrdinal uint32
	MaterializationPath    [][32]byte
	Claims                 []Claim
	SignerIDs              [][32]byte
	Signatures             [][]byte
	AlternateSets          []ClaimProof
}

// ClaimWinner is one verified, process-local result of an authenticated claim
// Epoch close. Its fields are private so only OpenClaimWinner can create a
// materializable root claim; the signed ClaimProof remains the interoperable
// Epoch evidence under R-042.
type ClaimWinner struct {
	value *claimWinner
}

// ClaimCommitment is an opaque, locally admitted R-042 commit input. Only
// AdmitClaimCommitment can create it after consuming the root-claim proof.
type ClaimCommitment struct {
	network     [32]byte
	revealEpoch uint64
	commitment  [32]byte
	admission   [32]byte
}

// EpochClaimInput is one opaque canonical commit input for the authenticated
// Epoch log. It contains no Name, Authority, Secret, or local proof state.
type EpochClaimInput struct {
	raw        [64]byte
	commitment [32]byte
}

type claimWinner struct {
	mu        sync.Mutex
	network   [32]byte
	name      string
	authority [32]byte
	ordinal   uint32
	epoch     uint64
	close     [32]byte
	consumed  bool
}

type result struct {
	Outcome         string
	WinnerOrdinal   uint32
	LoserOrdinals   []uint32
	OperationDigest [32]byte
}
