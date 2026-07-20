package api

import (
	"context"

	discoveryapi "ardents/internal/discovery/api"
)

type RuntimeService interface {
	Start(context.Context) error
	Stop(context.Context) error
	Snapshot() Snapshot
	Subscribe(context.Context) <-chan Event
	GetNodeRuntime() NodeRuntimeSnapshot
	GetNetworkStatus() NetworkStatusSnapshot
	GetDiscoveryStatus() DiscoveryStatusSnapshot
	GetLocalPresence() LocalPresenceSnapshot
	ListPeers() []PeerSnapshot
	ListRouteCandidates(ListRouteCandidatesQuery) ([]discoveryapi.RouteCandidateSnapshot, discoveryapi.RouteSnapshot, error)
	Capabilities() CapabilitiesSnapshot
}
