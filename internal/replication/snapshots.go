package replication

import "time"

const (
	ReplicaCommitmentActive  = "active"
	ReplicaCommitmentCorrupt = "corrupt"

	ReplicaReasonQuota      = "quota_refused"
	ReplicaReasonCapability = "capability_denied"
)

type ReplicaCommitment struct {
	OperationID    string    `json:"operation_id"`
	IntentVersion  uint64    `json:"intent_version"`
	BlobID         string    `json:"blob_id"`
	CID            string    `json:"cid"`
	PeerID         string    `json:"peer_id"`
	Size           int64     `json:"size"`
	State          string    `json:"state"`
	HealthReason   string    `json:"health_reason,omitempty"`
	LeaseStartsAt  time.Time `json:"lease_starts_at"`
	LastObservedAt time.Time `json:"last_observed_at"`
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
}

type ReplicaPlacementSnapshot struct {
	Reserved    int64                        `json:"reserved"`
	Used        int64                        `json:"used"`
	Commitments map[string]ReplicaCommitment `json:"commitments"`
}

type ReplicaPlacementDenial struct {
	NodeID string `json:"node_id"`
	Reason string `json:"reason"`
}

type ReplicaPlacementDecision struct {
	Selected []string                 `json:"selected"`
	Denials  []ReplicaPlacementDenial `json:"denials"`
}

func (d ReplicaPlacementDecision) SelectedNodeIDs() []string {
	return append([]string(nil), d.Selected...)
}

type ReplicaPlacementOutcome struct {
	Decision    ReplicaPlacementDecision `json:"decision"`
	Commitments []ReplicaCommitment      `json:"commitments"`
}
