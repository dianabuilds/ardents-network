package namespace

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

// Admission owns one boot generation's finite spent and in-flight state.
type Admission struct {
	node, network [32]byte
	epoch         uint64
	bootSecret    [32]byte
	profiles      []profile
	surfaces      map[string]*admissionSurface
}

type admissionSurface struct {
	mu         sync.Mutex
	spent      map[[32]byte]int64
	nextExpiry int64
	inflight   chan struct{}
}
