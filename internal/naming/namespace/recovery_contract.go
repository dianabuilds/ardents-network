package namespace

import "time"

const maximumPolicyProofBytes = 2 << 10

// RecoveryPolicy is one effective generation-scoped Recovery Authority set.
type RecoveryPolicy struct {
	Network          [32]byte
	Name             string
	Generation       uint64
	Revision         uint64
	CurrentAuthority [32]byte
	Threshold        uint8
	Participants     [][32]byte
	Delay            time.Duration
}

// Signature is one visible participant authorization.
type Signature struct {
	Signer [32]byte
	Bytes  []byte
}

// RecoveryProof binds one initiation or cancellation to fixed time boundaries.
type RecoveryProof struct {
	Operation    string
	PolicyDigest [32]byte
	OperationID  [32]byte
	Successor    [32]byte
	StartedAt    int64
	CompletesAt  int64
	Signatures   []Signature
}

// Authorization is the bounded fact consumed by the Name lifecycle module.
type Authorization struct {
	Operation      string
	PolicyDigest   [32]byte
	PolicyRevision uint64
	OperationID    [32]byte
	Successor      [32]byte
	StartedAt      int64
	CompletesAt    int64
	ValidSigners   uint8
	seal           [32]byte
}

// Verified reports whether this immutable value came from Authorize and has
// not been field-mutated by a later caller.
func (authorization Authorization) Verified() bool {
	return authorization.seal != [32]byte{} && authorization.seal == authorizationSeal(authorization)
}
