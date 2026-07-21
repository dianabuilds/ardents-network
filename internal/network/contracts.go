package network

import (
	"context"
)

type RouteRecord struct {
	Subject   string
	Service   string
	Mode      string
	Endpoints []string
}

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
	BuildCandidates(RouteRecord, bool) []Candidate
	PublishRelayEnvelope(context.Context, Envelope) error
	SubscribeRelayEnvelopes(context.Context, string, ...string) (<-chan Envelope, error)
	PublishLightpushEnvelope(context.Context, string, Envelope) error
	SubscribeFilterEnvelopes(context.Context, []string, string) (<-chan Envelope, error)
	FetchEnvelopes(context.Context, []string, string) ([]Envelope, error)
	ProfileSnapshot() Snapshot
	HealthSignals() HealthSignals
	ReachabilitySnapshot() ReachabilitySnapshot
	AbuseSnapshot() AbuseSnapshot
	SetReachabilityObserver(func())
}
