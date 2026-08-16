package route

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

const observationSchema = "ardents-h3-route-observation-v1"

// Actor is one role-local Route process configuration. Cross-role fields are
// rejected: a Node never receives the complete Plan or Publisher credential.
type Actor struct {
	ManifestDigest     [32]byte
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
	ClientCertificate  tls.Certificate
	ServiceCertificate tls.Certificate
	// Stream is caller-owned scoped Application/Endpoint IPC.
	Stream                                        io.ReadWriteCloser
	RawAttachment                                 bool
	AcknowledgementSocket, AcknowledgementKeyFile string
	IntroductionSetupSocket                       string
	IntroductionSetupPublic                       [32]byte
	IntroductionSetupPeer                         [32]byte
	IntroductionForwardSocket                     string
	IntroductionForwardPublic                     [32]byte
	IntroductionServicePublic                     [32]byte
	IntroductionSetupNode                         [32]byte
	Deadline                                      time.Duration
	Lifetime                                      time.Duration
	MaximumAttachments                            uint16
	AttachmentTarget                              uint16
	ResourceProfile                               string
	LocalRoleStateRoot                            string
	OpenEntry                                     func(context.Context, func(context.Context, net.Conn) (*tls.Conn, error)) (*tls.Conn, func() error, error)
	ResourceMeasure                               func() (resource.Sample, error)
	ResourceCheck                                 func() error
	PressureInterval                              time.Duration
}

// Evidence is bounded role-local evidence from one process.
type Evidence struct {
	Schema                   string            `json:"schema"`
	Kind                     string            `json:"kind"`
	Role                     string            `json:"role"`
	PID                      int               `json:"pid"`
	RuntimeID                string            `json:"runtime_id"`
	SourceID                 string            `json:"source_id"`
	BuildDigest              [32]byte          `json:"build_digest"`
	ManifestDigest           [32]byte          `json:"manifest_digest"`
	NetworkID                [32]byte          `json:"network_id,omitempty"`
	Generation               string            `json:"generation,omitempty"`
	Attachment               uint32            `json:"attachment,omitempty"`
	Epoch                    uint64            `json:"epoch,omitempty"`
	EpochDigest              [32]byte          `json:"epoch_digest,omitempty"`
	Profile                  string            `json:"profile,omitempty"`
	ViewRoot                 [32]byte          `json:"view_root,omitempty"`
	SelectionSeed            [32]byte          `json:"selection_seed,omitempty"`
	SelectionAt              int64             `json:"selection_at,omitempty"`
	ExcludedIdentities       [][32]byte        `json:"excluded_identities,omitempty"`
	ExcludedFamilies         []string          `json:"excluded_families,omitempty"`
	ExcludedDomains          []string          `json:"excluded_domains,omitempty"`
	NodeID                   [32]byte          `json:"node_id,omitempty"`
	PreviousPin              [32]byte          `json:"previous_pin,omitempty"`
	NextNodeID               [32]byte          `json:"next_node_id,omitempty"`
	OpaqueBytes              uint64            `json:"opaque_bytes,omitempty"`
	OpaqueDigest             [32]byte          `json:"opaque_digest,omitempty"`
	ReverseOpaqueBytes       uint64            `json:"reverse_opaque_bytes,omitempty"`
	ReverseOpaqueDigest      [32]byte          `json:"reverse_opaque_digest,omitempty"`
	AttachmentsCompleted     uint16            `json:"attachments_completed,omitempty"`
	AttachmentsRefused       uint16            `json:"attachments_refused,omitempty"`
	AttachmentsAbandoned     uint16            `json:"attachments_abandoned,omitempty"`
	CanaryLength             uint32            `json:"canary_length,omitempty"`
	CanaryDigest             [32]byte          `json:"canary_digest,omitempty"`
	Canary                   []byte            `json:"canary,omitempty"`
	IntroductionSetupReceipt [32]byte          `json:"introduction_setup_receipt,omitempty"`
	IntroductionSetup        introductionSetup `json:"introduction_setup,omitempty"`
	IntroductionOpaqueBytes  uint64            `json:"introduction_opaque_bytes,omitempty"`
	IntroductionOpaqueDigest [32]byte          `json:"introduction_opaque_digest,omitempty"`
	Positions                []Position        `json:"positions,omitempty"`
	PeerAuthenticated        bool              `json:"peer_authenticated"`
	DeadlineMillis           uint32            `json:"deadline_millis"`
	LifetimeMillis           uint32            `json:"lifetime_millis"`
	Cancelled                bool              `json:"cancelled"`
	Cleanup                  bool              `json:"cleanup"`
	Terminal                 string            `json:"terminal"`
	Error                    string            `json:"error,omitempty"`
	State                    string            `json:"state,omitempty"`
	Resource                 *resource.Sample  `json:"resource,omitempty"`
	ResourceMaximum          *resource.Sample  `json:"resource_maximum,omitempty"`
	ResourceSamples          uint32            `json:"resource_samples,omitempty"`
}

// introductionSetup is the retained public transcript of one mutually
// authenticated sealed setup; TLS keys and channel material are never retained.
type introductionSetup struct {
	ManifestDigest, NetworkID, EpochDigest, ViewRoot         [32]byte
	ProfileDigest, CapabilitiesDigest                        [32]byte
	IntroductionNode, RendezvousNode, RendezvousReachability [32]byte
	JoinHandle, EndpointHandshake, TranscriptContext, Reply  [32]byte
	ExpiresAtNanos                                           int64
}
