package api

import (
	"context"

	discovery "ardents/internal/discovery"
	hostingapi "ardents/internal/hosting/api"
	nodeapi "ardents/internal/node/api"
	"ardents/internal/workload/observedstate"
)

type Snapshot struct {
	Workloads       []observedstate.Status
	Discovery       []discovery.Entry
	DiscoveryState  string
	DiscoveryReason string
}

type Service interface {
	LocalPresenceSnapshotLocked() nodeapi.LocalPresenceSnapshot
	ServicePublicationStatusLocked(string) hostingapi.PublicationStatusSnapshot
	HostedServiceSnapshotLocked(string) (hostingapi.HostedServiceStatusSnapshot, error)
	SyncDesiredLocked(context.Context) error
	SyncLocalDesiredLocked() error
	CaptureWorkloadPublicationSnapshotLocked() Snapshot
	RollbackWorkloadMutationLocked(context.Context, string, error, Snapshot) error
	RefreshNetworkPublicationLocked(context.Context) error
	WithdrawNetworkPublicationLocked(context.Context) error
	ClearRollbackLocked()
}
