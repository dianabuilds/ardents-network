// Package availability owns replica intents, availability snapshots, and repair records.
// It does not own placement commitments or payload storage.
package availability

import (
	"time"

	"ardents/internal/content/catalog"
	"ardents/internal/identity/principal"
)

type ReplicaIntent struct {
	ID                string        `json:"id"`
	RootManifestOwner principal.ID  `json:"root_manifest_owner"`
	RootManifestID    string        `json:"root_manifest_id"`
	Version           uint64        `json:"version"`
	DesiredCopies     int           `json:"desired_copies"`
	MinimumCopies     int           `json:"minimum_copies"`
	LeaseDuration     time.Duration `json:"lease_duration"`
	RenewalHorizon    time.Duration `json:"renewal_horizon"`
	Retention         string        `json:"retention"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
	ExpiresAt         time.Time     `json:"expires_at"`
}

type RepairRecord struct {
	ID                string                   `json:"id"`
	IntentID          string                   `json:"intent_id"`
	IntentVersion     uint64                   `json:"intent_version"`
	RootManifestOwner principal.ID             `json:"root_manifest_owner"`
	RootManifestID    string                   `json:"root_manifest_id"`
	ContentReference  catalog.ContentReference `json:"content_reference"`
	MissingOrdinal    int                      `json:"missing_ordinal"`
	State             string                   `json:"state"`
	Attempts          int                      `json:"attempts"`
	PostLeaseAttempts int                      `json:"post_lease_attempts"`
	StartedAt         time.Time                `json:"started_at"`
	LossEligibleAt    time.Time                `json:"loss_eligible_at"`
	DeadlineAt        time.Time                `json:"deadline_at"`
	NextAttemptAt     time.Time                `json:"next_attempt_at"`
	LastAttemptAt     time.Time                `json:"last_attempt_at"`
	Reason            string                   `json:"reason,omitempty"`
	FinishedAt        *time.Time               `json:"finished_at,omitempty"`
}

type Snapshot struct {
	RootManifestOwner principal.ID `json:"root_manifest_owner"`
	RootManifestID    string       `json:"root_manifest_id"`
	IntentID          string       `json:"intent_id"`
	IntentVersion     uint64       `json:"intent_version"`
	State             string       `json:"state"`
	Reason            string       `json:"reason"`
	DesiredCopies     int          `json:"desired_copies"`
	MinimumCopies     int          `json:"minimum_copies"`
	ValidCopies       int          `json:"valid_copies"`
	CurrentLeases     int          `json:"current_leases"`
	StaleCopies       int          `json:"stale_copies"`
	ExpiredCopies     int          `json:"expired_copies"`
	CorruptCopies     int          `json:"corrupt_copies"`
	PendingRepairs    int          `json:"pending_repairs"`
	ObservedAt        time.Time    `json:"observed_at"`
	NextLeaseExpiry   time.Time    `json:"next_lease_expiry"`
}

type ReconcileResult struct {
	Snapshot   Snapshot       `json:"snapshot"`
	DueRepairs []RepairRecord `json:"due_repairs"`
}

type State struct {
	Intents   map[string]ReplicaIntent `json:"intents"`
	Snapshots map[string]Snapshot      `json:"snapshots"`
	Repairs   map[string]RepairRecord  `json:"repairs"`
}

func NewState() State {
	return State{
		Intents: map[string]ReplicaIntent{}, Snapshots: map[string]Snapshot{}, Repairs: map[string]RepairRecord{},
	}
}

func Normalize(state State) State {
	if state.Intents == nil {
		state.Intents = map[string]ReplicaIntent{}
	}
	if state.Snapshots == nil {
		state.Snapshots = map[string]Snapshot{}
	}
	if state.Repairs == nil {
		state.Repairs = map[string]RepairRecord{}
	}
	return state
}
