// Package publication owns local node and service advertisement lifecycle.
// It does not own workload execution, discovery intake, or carrier lifecycle.
package publication

import (
	"context"
	"time"

	"ardents/internal/discovery"
	"ardents/internal/hosting"
	"ardents/internal/workload/execution"
)

type LocalPresenceSnapshot struct {
	Published              bool      `json:"published,omitempty"`
	State                  string    `json:"state,omitempty"`
	Reason                 string    `json:"reason,omitempty"`
	RecordID               string    `json:"record_id,omitempty"`
	PublishedAt            time.Time `json:"published_at"`
	ExpiresAt              time.Time `json:"expires_at"`
	OperatorActionRequired bool      `json:"operator_action_required,omitempty"`
}

type Snapshot struct {
	Workloads       []execution.Status
	Discovery       []discovery.Entry
	DiscoveryState  string
	DiscoveryReason string
}

type Coordinator interface {
	LocalPresenceSnapshotLocked() LocalPresenceSnapshot
	ServicePublicationStatusLocked(string) hosting.PublicationSnapshot
	HostedServiceSnapshotLocked(string) (hosting.ServiceStatusSnapshot, error)
	SyncDesiredLocked(context.Context) error
	SyncLocalDesiredLocked() error
	CaptureWorkloadPublicationSnapshotLocked() Snapshot
	RollbackWorkloadMutationLocked(context.Context, string, error, Snapshot) error
	RefreshNetworkPublicationLocked(context.Context) error
	WithdrawNetworkPublicationLocked(context.Context) error
	ClearRollbackLocked()
	HandleSyncError(error) error
	EffectiveWorkloadStatus(execution.Status) execution.Status
}
