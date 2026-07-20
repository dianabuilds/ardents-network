package api

import (
	"context"

	discovery "ardents/internal/discovery"
	networkprivacy "ardents/internal/network/privacy"
)

type Service interface {
	Start(context.Context) error
	Stop(context.Context) error
	State() string
	Reason() string
	Endpoints() []string
	PeerCount() int
	RelayPeerCount(string) int
	SetBootstrapNodes([]string)
	SetBootstrapObserver(func(BootstrapDialReport))
	BootstrapStatus() BootstrapStatus
	BuildCandidates(discovery.Record, bool) []Candidate
	PublishRelayEnvelope(context.Context, Envelope) error
	SubscribeRelayEnvelopes(context.Context, string, ...string) (<-chan Envelope, error)
	PublishPrivateEnvelope(context.Context, networkprivacy.SealedEnvelope) error
	SubscribePrivateEnvelopes(context.Context, string) (<-chan networkprivacy.SealedEnvelope, error)
	PublishPrivateLightpush(context.Context, string, networkprivacy.SealedEnvelope) error
	SubscribePrivateFilter(context.Context, []string, string) (<-chan networkprivacy.SealedEnvelope, error)
	FetchPrivateEnvelopes(context.Context, []string, string) ([]networkprivacy.SealedEnvelope, error)
	ProfileSnapshot() Snapshot
	HealthSignals() HealthSignals
	ReachabilitySnapshot() ReachabilitySnapshot
	AbuseSnapshot() AbuseSnapshot
	SetReachabilityObserver(func(ReachabilitySnapshot))
}
