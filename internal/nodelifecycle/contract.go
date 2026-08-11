package nodelifecycle

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"time"

	"github.com/dianabuilds/ardents-network/internal/networkstate"
)

const eventSchema = "ardents-h3-s1-node-event-v1"

// Config binds one local identity, immutable Network State reader, and private role-probe listener.
type Config struct {
	NetworkID      [32]byte
	NodeID         [32]byte
	IdentityKey    ed25519.PrivateKey
	Current        func() (networkstate.Snapshot, error)
	ListenAddress  string
	Certificate    tls.Certificate
	ClientRootPEM  []byte
	ClientKeyPins  [][32]byte
	PollInterval   time.Duration
	MaximumDuty    time.Duration
	DrainTimeout   time.Duration
	Quarantine     time.Duration
	Now            func() time.Time
	CheckPlacement func() error
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
	now func() time.Time
}
