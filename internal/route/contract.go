package route

import (
	"crypto/tls"
	"io"
	"time"
)

const observationSchema = "ardents-h3-route-observation-v1"

// Actor is one role-local Route process configuration. Cross-role fields are
// rejected: a Node never receives the complete Plan or Publisher credential.
type Actor struct {
	ManifestDigest                                [32]byte
	NetworkID                                     [32]byte
	EpochDigest                                   [32]byte
	Role                                          string
	NodeID                                        [32]byte
	ListenAddress                                 string
	Certificate                                   tls.Certificate
	UpstreamPin                                   [32]byte
	NextNodeID                                    [32]byte
	NextAddress                                   string
	NextPin                                       [32]byte
	Plan                                          Plan
	PublisherPin                                  [32]byte
	ClientCertificate                             tls.Certificate
	ServiceCertificate                            tls.Certificate
	Stream                                        io.ReadWriteCloser
	AcknowledgementSocket, AcknowledgementKeyFile string
	Deadline                                      time.Duration
}

// Evidence is bounded role-local evidence from one process.
type Evidence struct {
	Schema              string     `json:"schema"`
	Kind                string     `json:"kind"`
	Role                string     `json:"role"`
	PID                 int        `json:"pid"`
	RuntimeID           string     `json:"runtime_id"`
	SourceID            string     `json:"source_id"`
	BuildDigest         [32]byte   `json:"build_digest"`
	ManifestDigest      [32]byte   `json:"manifest_digest"`
	NetworkID           [32]byte   `json:"network_id,omitempty"`
	Generation          string     `json:"generation,omitempty"`
	Epoch               uint64     `json:"epoch,omitempty"`
	EpochDigest         [32]byte   `json:"epoch_digest,omitempty"`
	Profile             string     `json:"profile,omitempty"`
	ViewRoot            [32]byte   `json:"view_root,omitempty"`
	SelectionSeed       [32]byte   `json:"selection_seed,omitempty"`
	SelectionAt         int64      `json:"selection_at,omitempty"`
	ExcludedIdentities  [][32]byte `json:"excluded_identities,omitempty"`
	ExcludedFamilies    []string   `json:"excluded_families,omitempty"`
	ExcludedDomains     []string   `json:"excluded_domains,omitempty"`
	NodeID              [32]byte   `json:"node_id,omitempty"`
	PreviousPin         [32]byte   `json:"previous_pin,omitempty"`
	NextNodeID          [32]byte   `json:"next_node_id,omitempty"`
	OpaqueBytes         uint64     `json:"opaque_bytes,omitempty"`
	OpaqueDigest        [32]byte   `json:"opaque_digest,omitempty"`
	ReverseOpaqueBytes  uint64     `json:"reverse_opaque_bytes,omitempty"`
	ReverseOpaqueDigest [32]byte   `json:"reverse_opaque_digest,omitempty"`
	CanaryLength        uint32     `json:"canary_length,omitempty"`
	CanaryDigest        [32]byte   `json:"canary_digest,omitempty"`
	Canary              []byte     `json:"canary,omitempty"`
	Positions           []Position `json:"positions,omitempty"`
	PeerAuthenticated   bool       `json:"peer_authenticated"`
	DeadlineMillis      uint32     `json:"deadline_millis"`
	Cancelled           bool       `json:"cancelled"`
	Cleanup             bool       `json:"cleanup"`
	Terminal            string     `json:"terminal"`
	Error               string     `json:"error,omitempty"`
}
