package route

import (
	"crypto/tls"
	"time"
)

const observationSchema = "ardents-h3-route-observation-v1"

// Actor is one role-local Route process configuration. Cross-role fields are
// rejected: a Node never receives the complete Plan or Publisher credential.
type Actor struct {
	NetworkID          [32]byte
	EpochDigest        [32]byte
	Role               string
	NodeID             [32]byte
	ListenAddress      string
	Certificate        tls.Certificate
	UpstreamPin        [32]byte
	NextNodeID         [32]byte
	NextAddress        string
	NextPin            [32]byte
	Plan               Plan
	PublisherPin       [32]byte
	Canary             []byte
	ClientCertificate  tls.Certificate
	ServiceCertificate tls.Certificate
	Deadline           time.Duration
}

// Evidence is bounded role-local evidence from one process.
type Evidence struct {
	Schema       string     `json:"schema"`
	Kind         string     `json:"kind"`
	Role         string     `json:"role"`
	PID          int        `json:"pid"`
	NetworkID    [32]byte   `json:"network_id,omitempty"`
	Generation   string     `json:"generation,omitempty"`
	Epoch        uint64     `json:"epoch,omitempty"`
	EpochDigest  [32]byte   `json:"epoch_digest,omitempty"`
	NodeID       [32]byte   `json:"node_id,omitempty"`
	PreviousPin  [32]byte   `json:"previous_pin,omitempty"`
	NextNodeID   [32]byte   `json:"next_node_id,omitempty"`
	OpaqueBytes  uint64     `json:"opaque_bytes,omitempty"`
	OpaqueDigest [32]byte   `json:"opaque_digest,omitempty"`
	CanaryLength uint32     `json:"canary_length,omitempty"`
	CanaryDigest [32]byte   `json:"canary_digest,omitempty"`
	Canary       []byte     `json:"canary,omitempty"`
	Positions    []Position `json:"positions,omitempty"`
	Error        string     `json:"error,omitempty"`
}
