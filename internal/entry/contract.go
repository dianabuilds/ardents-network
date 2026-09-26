package entry

import (
	"crypto/tls"
	"sync"
	"time"
)

const profileID = "ardents-interactive-route-v2"

// Candidate is one bounded, authenticated adjacent Entry fact projected by
// the State adapter. Entry never discovers or mutates this data.
type Candidate struct {
	NodeID, PublicKey, KeyID, FamilyID        [32]byte
	RecordDigest, DomainProofDigest           [32]byte
	Endpoint                                  string
	Capacity                                  uint16
	Domain                                    string
	ValidFrom, ValidUntil, AssignmentNotAfter time.Time
}

// View is the authenticated State projection required to validate one Invite.
// Callers construct it from their owned State view; Entry has no State import.
type View struct {
	NetworkID  [32]byte
	Epoch      uint64
	Digest     [32]byte
	Profile    string
	Fresh      bool
	Candidates []Candidate
}

// Config provides only opaque current State and local-duty facts. Inputs are
// copied by Open and no caller-owned State root is retained by Entry.
type Config struct {
	Root          string
	Current       func() (View, error)
	Conflict      func([32]byte, [32]byte) (bool, error)
	Clock         func() time.Time
	TimeConfident func() bool
}

// Verification supplies the current facts needed to validate one decoded
// Invite. It has no durable Entry root or User identity.
type Verification struct {
	Current       func() (View, error)
	Conflict      func([32]byte, [32]byte) (bool, error)
	Clock         func() time.Time
	TimeConfident func() bool
}

// Class is the closed result of one Invite import.
type Class string

const (
	Accepted            Class = "accepted"
	AlreadyPresent      Class = "already-present"
	Invalid             Class = "invalid"
	WrongRecipient      Class = "wrong-recipient"
	Incompatible        Class = "incompatible"
	WrongDomain         Class = "wrong-domain"
	ConflictingRole     Class = "conflicting-role"
	SetFull             Class = "set-full"
	ReplacementRejected Class = "replacement-rejected"
	Expired             Class = "expired"
	Replay              Class = "replay"
)

// Result is the bounded classification of one import attempt.
type Result struct {
	Class      Class    `json:"class"`
	InviteID   [32]byte `json:"invite_id"`
	Slot       uint8    `json:"slot"`
	Generation uint8    `json:"generation"`
}

type owner struct {
	mu       sync.Mutex
	root     string
	lease    rootLease
	config   Config
	state    durableState
	current  string
	closing  bool
	closed   bool
	closeErr error
	failed   error

	// closeDone is closed once the terminal Close result is recorded.
	closeDone chan struct{}
	// recipient is the owner-local TLS identity retained for Invite
	// recipient binding; the retired attachment execution machinery
	// (ADR-0095) was its only certificate consumer.
	recipient tls.Certificate
}
