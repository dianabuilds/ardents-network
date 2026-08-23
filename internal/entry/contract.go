package entry

import (
	"context"
	"net"
	"sync"
	"time"
)

const profileID = "ardents-interactive-route-v1"

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

// Class is the closed result of one Invite import.
type Class string

const (
	Accepted            Class = "accepted"
	AlreadyPresent      Class = "already-present"
	Invalid             Class = "invalid"
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

// Attempt identifies one endpoint-owned use of the bounded Entry set. Entry
// does not derive this identity from a Route, Target, or carrier protocol.
// Deadline is an absolute caller bound that Entry can only shorten.
type Attempt struct {
	ID       [32]byte
	Deadline time.Time
}

// CandidateOpener opens one State-derived adjacent candidate. On an open
// error, cleanupComplete states whether the opener has fully disposed of any
// carrier state it created. A successful result must contain both a
// connection and its cleanup function. Entry owns the order and persistence
// of calls; the opener owns the TCP/TLS implementation.
type CandidateOpener func(context.Context, Candidate, time.Time) (connection net.Conn, cleanup func() error, cleanupComplete bool, err error)

type owner struct {
	mu      sync.Mutex
	root    string
	lease   rootLease
	config  Config
	state   durableState
	current string
	closed  bool
	failed  error
}
