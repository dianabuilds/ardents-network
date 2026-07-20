package api

import "time"

type ReplicaIntentSnapshot struct {
	ID             string        `json:"id"`
	RootManifestID string        `json:"root_manifest_id"`
	Version        uint64        `json:"version"`
	DesiredCopies  int           `json:"desired_copies"`
	MinimumCopies  int           `json:"minimum_copies"`
	LeaseDuration  time.Duration `json:"lease_duration"`
	RenewalHorizon time.Duration `json:"renewal_horizon"`
	Retention      string        `json:"retention"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	ExpiresAt      time.Time     `json:"expires_at,omitempty"`
}

type AvailabilitySnapshot struct {
	RootManifestID  string    `json:"root_manifest_id"`
	IntentID        string    `json:"intent_id"`
	IntentVersion   uint64    `json:"intent_version"`
	State           string    `json:"state"`
	Reason          string    `json:"reason"`
	DesiredCopies   int       `json:"desired_copies"`
	MinimumCopies   int       `json:"minimum_copies"`
	ValidCopies     int       `json:"valid_copies"`
	CurrentLeases   int       `json:"current_leases"`
	StaleCopies     int       `json:"stale_copies"`
	ExpiredCopies   int       `json:"expired_copies"`
	CorruptCopies   int       `json:"corrupt_copies"`
	PendingRepairs  int       `json:"pending_repairs"`
	ObservedAt      time.Time `json:"observed_at"`
	NextLeaseExpiry time.Time `json:"next_lease_expiry,omitempty"`
}

type RepairSnapshot struct {
	ID                string     `json:"id"`
	IntentID          string     `json:"intent_id"`
	IntentVersion     uint64     `json:"intent_version"`
	RootManifestID    string     `json:"root_manifest_id"`
	BlobID            string     `json:"blob_id"`
	MissingOrdinal    int        `json:"missing_ordinal"`
	State             string     `json:"state"`
	Attempts          int        `json:"attempts"`
	PostLeaseAttempts int        `json:"post_lease_attempts"`
	StartedAt         time.Time  `json:"started_at"`
	LossEligibleAt    time.Time  `json:"loss_eligible_at"`
	DeadlineAt        time.Time  `json:"deadline_at"`
	NextAttemptAt     time.Time  `json:"next_attempt_at,omitempty"`
	LastAttemptAt     time.Time  `json:"last_attempt_at,omitempty"`
	Reason            string     `json:"reason,omitempty"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
}
