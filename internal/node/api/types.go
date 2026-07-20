package api

import (
	diagapi "ardents/internal/diagnostics/api"
	"time"
)

type LifecycleTransitionSnapshot struct {
	From string    `json:"from,omitempty"`
	To   string    `json:"to,omitempty"`
	At   time.Time `json:"at,omitempty"`
}

type CapabilitiesSnapshot struct {
	Version  string          `json:"version,omitempty"`
	Services []string        `json:"services,omitempty"`
	Features map[string]bool `json:"features,omitempty"`
}

type PartSnapshot struct {
	State  string `json:"state,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type LifecycleSnapshot struct {
	Current        string                        `json:"current,omitempty"`
	Previous       string                        `json:"previous,omitempty"`
	EnteredAt      time.Time                     `json:"entered_at,omitempty"`
	TransitionedAt time.Time                     `json:"transitioned_at,omitempty"`
	Transitions    []LifecycleTransitionSnapshot `json:"transitions,omitempty"`
}

type NodeSnapshot struct {
	Name      string            `json:"name,omitempty"`
	State     string            `json:"state,omitempty"`
	Ready     bool              `json:"ready,omitempty"`
	Reason    string            `json:"reason,omitempty"`
	Lifecycle LifecycleSnapshot `json:"lifecycle,omitempty"`
}

type BootSnapshot struct {
	Joined bool     `json:"joined,omitempty"`
	State  string   `json:"state,omitempty"`
	Reason string   `json:"reason,omitempty"`
	Source []string `json:"source,omitempty"`
}

type IdentitySnapshot struct {
	State     string `json:"state,omitempty"`
	Principal string `json:"principal,omitempty"`
	Device    string `json:"device,omitempty"`
	PublicKey string `json:"public_key,omitempty"`
	Source    string `json:"source,omitempty"`
}

type TrustSnapshot struct {
	State   string `json:"state,omitempty"`
	Outcome string `json:"outcome,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Valid   bool   `json:"valid,omitempty"`
	Trusted bool   `json:"trusted,omitempty"`
	Usable  bool   `json:"usable,omitempty"`
}

type DiscoverySnapshot struct {
	State     string `json:"state,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Records   int    `json:"records,omitempty"`
	LocalNode string `json:"local_node,omitempty"`
	Services  int    `json:"services,omitempty"`
}

type TransportSnapshot struct {
	Profile             string   `json:"profile,omitempty"`
	Mode                string   `json:"mode,omitempty"`
	Health              string   `json:"health,omitempty"`
	ActiveFamilies      []string `json:"active_families,omitempty"`
	SuppressedFamilies  []string `json:"suppressed_families,omitempty"`
	SwitchReason        string   `json:"switch_reason,omitempty"`
	SwitchAutomatic     bool     `json:"switch_automatic,omitempty"`
	ReducedCapabilities []string `json:"reduced_capabilities,omitempty"`
	ActiveCapabilities  []string `json:"active_capabilities,omitempty"`
	RecoveryState       string   `json:"recovery_state,omitempty"`
}

type WorkloadStateSnapshot struct {
	State   string `json:"state,omitempty"`
	Desired int    `json:"desired,omitempty"`
	Active  int    `json:"active,omitempty"`
}

type StoreSnapshot struct {
	Authority int `json:"authority,omitempty"`
	Cached    int `json:"cached,omitempty"`
	Derived   int `json:"derived,omitempty"`
	Pinned    int `json:"pinned,omitempty"`
}

type NodeRuntimeSnapshot struct {
	Node     NodeSnapshot           `json:"node"`
	Boot     BootSnapshot           `json:"boot"`
	Identity IdentitySnapshot       `json:"identity"`
	Health   diagapi.HealthSnapshot `json:"health"`
}

type NetworkStatusSnapshot struct {
	State                   string    `json:"state,omitempty"`
	Reason                  string    `json:"reason,omitempty"`
	Joined                  bool      `json:"joined,omitempty"`
	Reachable               bool      `json:"reachable,omitempty"`
	ReachabilityMode        string    `json:"reachability_mode,omitempty"`
	ReachabilityState       string    `json:"reachability_state,omitempty"`
	ReachabilityReason      string    `json:"reachability_reason,omitempty"`
	ReachabilityObservedAt  time.Time `json:"reachability_observed_at,omitempty"`
	ActiveProfile           string    `json:"active_profile,omitempty"`
	ActiveMode              string    `json:"active_mode,omitempty"`
	ReducedCapabilities     []string  `json:"reduced_capabilities,omitempty"`
	ActiveCapabilities      []string  `json:"active_capabilities,omitempty"`
	AbuseState              string    `json:"abuse_state,omitempty"`
	AbuseReason             string    `json:"abuse_reason,omitempty"`
	RateLimitedOperations   uint64    `json:"rate_limited_operations,omitempty"`
	BackpressuredOperations uint64    `json:"backpressured_operations,omitempty"`
	OversizedMessages       uint64    `json:"oversized_messages,omitempty"`
	BannedProviders         int       `json:"banned_providers,omitempty"`
	LastTransitionAt        time.Time `json:"last_transition_at,omitempty"`
	PrivacyProfile          string    `json:"privacy_profile,omitempty"`
	PrivacyState            string    `json:"privacy_state,omitempty"`
	PrivacySwitchReason     string    `json:"privacy_switch_reason,omitempty"`
	PrivacyRecoveryState    string    `json:"privacy_recovery_state,omitempty"`
	PrivacyErrors           []string  `json:"privacy_error_categories,omitempty"`
	NodeProfile             string    `json:"node_profile,omitempty"`
}

type DiscoveryStatusSnapshot struct {
	State           string    `json:"state,omitempty"`
	Reason          string    `json:"reason,omitempty"`
	LocalRecords    int       `json:"local_records,omitempty"`
	RemoteRecords   int       `json:"remote_records,omitempty"`
	TrustedRecords  int       `json:"trusted_records,omitempty"`
	RejectedRecords int       `json:"rejected_records,omitempty"`
	StaleRecords    int       `json:"stale_records,omitempty"`
	LastPublishAt   time.Time `json:"last_publish_at,omitempty"`
	LastRefreshAt   time.Time `json:"last_refresh_at,omitempty"`
}

type LocalPresenceSnapshot struct {
	Published              bool      `json:"published,omitempty"`
	State                  string    `json:"state,omitempty"`
	Reason                 string    `json:"reason,omitempty"`
	RecordID               string    `json:"record_id,omitempty"`
	PublishedAt            time.Time `json:"published_at,omitempty"`
	ExpiresAt              time.Time `json:"expires_at,omitempty"`
	OperatorActionRequired bool      `json:"operator_action_required,omitempty"`
}

type PeerSnapshot struct {
	NodeID       string        `json:"node_id,omitempty"`
	DeviceID     string        `json:"device_id,omitempty"`
	Addresses    []string      `json:"addresses,omitempty"`
	Trust        TrustSnapshot `json:"trust"`
	Reachability string        `json:"reachability,omitempty"`
	Source       string        `json:"source,omitempty"`
	LastSeenAt   time.Time     `json:"last_seen_at,omitempty"`
	State        string        `json:"state,omitempty"`
	Reason       string        `json:"reason,omitempty"`
}

type Snapshot struct {
	Node      NodeSnapshot          `json:"node"`
	Boot      BootSnapshot          `json:"boot"`
	Ident     IdentitySnapshot      `json:"ident"`
	Trust     TrustSnapshot         `json:"trust"`
	Disco     DiscoverySnapshot     `json:"disco"`
	Trans     PartSnapshot          `json:"trans"`
	Transport *TransportSnapshot    `json:"transport,omitempty"`
	Route     PartSnapshot          `json:"route"`
	Object    PartSnapshot          `json:"object"`
	Blob      PartSnapshot          `json:"blob"`
	Policy    PartSnapshot          `json:"policy"`
	Workload  WorkloadStateSnapshot `json:"workload"`
	Store     StoreSnapshot         `json:"store"`
	Diag      diagapi.DiagSnapshot  `json:"diag"`
}

type Event struct {
	Seq   int64          `json:"seq,omitempty"`
	Time  time.Time      `json:"time,omitempty"`
	Topic string         `json:"topic,omitempty"`
	Data  map[string]any `json:"data,omitempty"`
}
