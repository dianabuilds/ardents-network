package node

import (
	"context"
	"crypto/ed25519"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node/probe"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

const eventSchema = "ardents-h3-node-event-v1"

// Facts are the authenticated inputs required to decide one Node duty. They do
// not expose Network State persistence, source, retry, or pending metadata.
type Facts struct {
	Generation       string
	NetworkID        [32]byte
	Epoch            uint64
	Digest           [32]byte
	EpochValidFrom   time.Time
	ValidUntil       time.Time
	Profile          string
	Conflicting      bool
	RecordPresent    bool
	NodeID           [32]byte
	NodePublicKey    [32]byte
	RecordValidFrom  time.Time
	RecordValidUntil time.Time
	ProbeEndpoint    string
	ProbeCapacity    uint16
	Assignment       string
	AssignmentDigest [32]byte
	Fresh            bool
}

// Config binds one local identity, authenticated duty facts, and private role-probe listener.
type Config struct {
	NetworkID       [32]byte
	NodeID          [32]byte
	IdentityKey     ed25519.PrivateKey
	Current         func() (Facts, error)
	Probe           probe.Config
	PollInterval    time.Duration
	Quarantine      time.Duration
	ResourceProfile string
	Now             func() time.Time
	CheckPlacement  func() error
	// Emit must honor ctx cancellation and return before its deadline.
	Emit func(context.Context, Event) error
}

// Event is one bounded external observation of Node lifecycle state.
type Event struct {
	Schema           string    `json:"schema"`
	Kind             string    `json:"kind"`
	State            string    `json:"state"`
	At               time.Time `json:"at"`
	Epoch            uint64    `json:"epoch,omitempty"`
	Generation       string    `json:"generation,omitempty"`
	Assignment       string    `json:"assignment,omitempty"`
	AssignmentDigest [32]byte  `json:"assignment_digest,omitempty"`
	Reason           string    `json:"reason,omitempty"`
}

// Result describes the terminal state after the listener and accepted work are gone.
type Result struct {
	State            string
	Epoch            uint64
	Assignment       string
	AssignmentDigest [32]byte
	Reason           string
}

type runtimeConfig struct {
	Config
	now      func() time.Time
	probe    *probe.Plan
	pressure *resource.Guard
}
