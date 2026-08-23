package admission

import "sync"

type profile struct {
	Surface         string
	WorkBits        uint8
	MaximumSpent    int
	MaximumInFlight int
}

// Challenge is one short-lived Node-, operation-, and Isolation-bound input.
type Challenge struct {
	Node              [32]byte
	Network           [32]byte
	Epoch             uint64
	Surface           string
	OperationDigest   [32]byte
	IsolationBinding  [32]byte
	IssuedAt          int64
	ExpiresAt         int64
	Nonce             [16]byte
	WorkBits          uint8
	AuthenticationTag [32]byte
}

// Proof adds the one-use SHA-256 work nonce to an authenticated Challenge.
type Proof struct {
	Challenge Challenge
	WorkNonce uint64
}

// Digest returns the canonical replay identity bound to this proof.
func (proof Proof) Digest() [32]byte { return challengeDigest(proof.Challenge) }

// Admission owns one boot generation's finite spent and in-flight state.
type Admission struct {
	node, network [32]byte
	epoch         uint64
	bootSecret    [32]byte
	profiles      []profile
	surfaces      map[string]*admissionSurface
}

// Network returns the Network identity fixed when this gate was opened.
func (admission *Admission) Network() [32]byte {
	if admission == nil {
		return [32]byte{}
	}
	return admission.network
}

// Epoch returns the Epoch fixed when this gate was opened.
func (admission *Admission) Epoch() uint64 {
	if admission == nil {
		return 0
	}
	return admission.epoch
}

// AcceptsNode reports whether node owns this boot-scoped gate.
func (admission *Admission) AcceptsNode(node [32]byte) bool {
	return admission != nil && node != [32]byte{} && admission.node == node
}

// Matches reports whether this gate is fixed to the supplied Namespace context.
func (admission *Admission) Matches(network [32]byte, epoch uint64) bool {
	return admission != nil && network != [32]byte{} && epoch != 0 &&
		admission.network == network && admission.epoch == epoch
}

type admissionSurface struct {
	mu         sync.Mutex
	spent      map[[32]byte]int64
	nextExpiry int64
	inflight   chan struct{}
}
