package placement

import (
	"io"
	"time"

	model "ardents/internal/content/catalog"
	identityprincipal "ardents/internal/identity/principal"
)

const (
	ReservationAccepted           = "accepted"
	ReservationRejected           = "rejected"
	CommitmentActive              = "active"
	CommitmentStale               = "stale"
	CommitmentCorrupt             = "corrupt"
	CommitmentRevoked             = "revoked"
	CommitmentExpired             = "expired"
	ReplicaProtocolVersion uint32 = 2
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
	OperationID      string                 `json:"operation_id"`
	ProtocolVersion  uint32                 `json:"protocol_version"`
	IntentVersion    uint64                 `json:"intent_version"`
	ContentReference model.ContentReference `json:"content_reference"`
	EncryptedSize    int64                  `json:"encrypted_size"`
	RequestedLease   time.Duration          `json:"requested_lease"`
	ExpiresAt        time.Time              `json:"expires_at"`
	Nonce            string                 `json:"nonce"`
}

type PeerAuthorization struct {
	NodePrincipal   identityprincipal.ID
	Authenticated   bool
	Trusted         bool
	CapabilityValid bool
	PolicyAllowed   bool
}

type ReservationResult struct {
	OperationID string    `json:"operation_id"`
	Status      string    `json:"status"`
	Reason      string    `json:"reason,omitempty"`
	Token       string    `json:"token,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type CommitRequest struct {
	OperationID    string
	Token          string
	Blob           model.Blob
	Ciphertext     []byte
	LeaseExpiresAt time.Time
}

type Commitment struct {
	OperationID      string                 `json:"operation_id"`
	IntentVersion    uint64                 `json:"intent_version"`
	ContentReference model.ContentReference `json:"content_reference"`
	TargetNode       identityprincipal.ID   `json:"target_node"`
	Size             int64                  `json:"size"`
	State            string                 `json:"state"`
	HealthReason     string                 `json:"health_reason,omitempty"`
	LeaseStartsAt    time.Time              `json:"lease_starts_at"`
	LastObservedAt   time.Time              `json:"last_observed_at"`
	LeaseExpiresAt   time.Time              `json:"lease_expires_at"`
}

type ReceiverConfig struct {
	NodePrincipal identityprincipal.ID
	MaxBytes      int64
	Now           func() time.Time
	Random        io.Reader
	Store         func(model.Blob, []byte, time.Time) error
}

type Candidate struct {
	NodePrincipal   identityprincipal.ID
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
	OwnerPrincipal identityprincipal.ID
	EncryptedSize  int64
	Count          int
	Now            time.Time
	ExcludedNodes  map[identityprincipal.ID]bool
}

type Denial struct {
	NodePrincipal identityprincipal.ID
	Reason        string
}

type SelectionDecision struct {
	Selected []Candidate
	Denials  []Denial
}

type Capacity struct {
	NodePrincipal identityprincipal.ID `json:"node_principal"`
	FreeBytes     int64                `json:"free_bytes"`
	ReservedBytes int64                `json:"reserved_bytes"`
	UsedBytes     int64                `json:"used_bytes"`
	ObservedAt    time.Time            `json:"observed_at"`
}

func (d SelectionDecision) SelectedNodePrincipals() []identityprincipal.ID {
	out := make([]identityprincipal.ID, 0, len(d.Selected))
	for _, candidate := range d.Selected {
		out = append(out, candidate.NodePrincipal)
	}
	return out
}
