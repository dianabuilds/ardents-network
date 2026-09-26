package node

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

const eventSchema = "ardents-node-event-v1"

// Event is one bounded external observation of Node lifecycle state.
type Event struct {
	Elapsed          time.Duration           `json:"elapsed,omitempty"`
	Hosting          *resource.HostingSample `json:"hosting,omitempty"`
	Schema           string                  `json:"schema"`
	Kind             string                  `json:"kind"`
	State            string                  `json:"state"`
	At               time.Time               `json:"at"`
	Epoch            uint64                  `json:"epoch,omitempty"`
	Generation       string                  `json:"generation,omitempty"`
	Assignment       string                  `json:"assignment,omitempty"`
	CarrierProfile   string                  `json:"carrier_profile,omitempty"`
	AssignmentDigest [32]byte                `json:"assignment_digest,omitempty"`
	Reason           string                  `json:"reason,omitempty"`
	Resource         *resource.Sample        `json:"resource,omitempty"`
}

// Result describes the observed terminal lifecycle outcome. FAILED may report
// cleanup that did not complete or could not be proven inside its bound.
type Result struct {
	State            string
	Epoch            uint64
	Assignment       string
	CarrierProfile   string
	AssignmentDigest [32]byte
	Reason           string
}
