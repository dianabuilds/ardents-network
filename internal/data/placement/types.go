package placement

import (
	"io"
	"time"

	model "ardents/internal/data/model"
)

const (
	ReservationAccepted           = "accepted"
	ReservationRejected           = "rejected"
	CommitmentActive              = "active"
	CommitmentStale               = "stale"
	CommitmentCorrupt             = "corrupt"
	CommitmentRevoked             = "revoked"
	CommitmentExpired             = "expired"
	ReplicaProtocolVersion uint32 = 1
	MaxInlineReplicaBytes  int64  = 64*1024 + 16

	ReasonQuota       = "quota_refused"
	ReasonUntrusted   = "peer_untrusted"
	ReasonCapability  = "capability_denied"
	ReasonPolicy      = "policy_denied"
	ReasonLease       = "lease_invalid"
	ReasonUnsupported = "transfer_unsupported"
	ReasonObservation = "observation_unavailable"
	ReasonExisting    = "replica_already_observed"
)

type ReservationOffer struct {
	OperationID     string
	ProtocolVersion uint32
	IntentVersion   uint64
	BlobID          string
	CID             string
	EncryptedSize   int64
	RequestedLease  time.Duration
	ExpiresAt       time.Time
	Nonce           string
}

type PeerAuthorization struct {
	PeerID          string
	Authenticated   bool
	Trusted         bool
	CapabilityValid bool
	PolicyAllowed   bool
}

type ReservationResult struct {
	OperationID string
	Status      string
	Reason      string
	Token       string
	ExpiresAt   time.Time
}

type CommitRequest struct {
	OperationID    string
	Token          string
	Blob           model.Blob
	Ciphertext     []byte
	LeaseExpiresAt time.Time
}

type Commitment struct {
	OperationID    string
	IntentVersion  uint64
	BlobID         string
	CID            string
	PeerID         string
	Size           int64
	State          string
	HealthReason   string
	LeaseStartsAt  time.Time
	LastObservedAt time.Time
	LeaseExpiresAt time.Time
}

type ReceiverConfig struct {
	NodeID   string
	MaxBytes int64
	Now      func() time.Time
	Random   io.Reader
	Store    func(model.Blob, []byte, time.Time) error
}

type Candidate struct {
	NodeID          string
	FailureDomain   string
	Trusted         bool
	CapabilityValid bool
	PolicyAllowed   bool
	Usable          bool
	CapacityBytes   int64
	ObservedAt      time.Time
	DenialReason    string
}

type SelectionRequest struct {
	OwnerNodeID   string
	EncryptedSize int64
	Count         int
	Now           time.Time
	ExcludedNodes map[string]bool
}

type Denial struct {
	NodeID string
	Reason string
}

type SelectionDecision struct {
	Selected []Candidate
	Denials  []Denial
}

type Capacity struct {
	NodeID        string
	FreeBytes     int64
	ReservedBytes int64
	UsedBytes     int64
	ObservedAt    time.Time
}

func (d SelectionDecision) SelectedNodeIDs() []string {
	out := make([]string, 0, len(d.Selected))
	for _, candidate := range d.Selected {
		out = append(out, candidate.NodeID)
	}
	return out
}
