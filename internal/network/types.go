package network

import (
	"ardents/internal/network/routing"
	"time"
)

const BindAddressEnv = "ARDENTS_TRANSPORT_BIND_ADDRESS"

type Config struct {
	NodeProfile            NodeProfile
	StorePath              string
	PrivateKeyPath         string
	BindAddress            string
	ListenPort             int
	Profile                Profile
	WSSPort                int
	WSSCertPath            string
	WSSKeyPath             string
	WSSCAPath              string
	WSSAdvertiseAddress    string
	DNSDiscoveryURLs       []string
	DNSDiscoveryNameServer string
	ReachabilityMode       ReachabilityMode
	AdvertiseAddresses     []string
	Limits                 Limits
}

type Limits struct {
	MaxMessageBytes         int
	MaxPeerConnections      int
	MaxConnectionsPerIP     int
	MaxConcurrentOperations int
	OperationRate           int
	OperationBurst          int
	MaxFilterSubscribers    int
	MaxStoreResults         int
	StoreMaxMessages        int
	StoreMaxAgeSeconds      int
	StoreMaxBytes           int64
}

type StoreRetention struct {
	MaxMessages int
	MaxAge      time.Duration
	MaxBytes    int64
}

type AbuseSnapshot struct {
	State                   string
	Reason                  string
	RateLimitedOperations   uint64
	BackpressuredOperations uint64
	OversizedMessages       uint64
	BannedProviders         int
	StoreEnabled            bool
	StoreState              string
	StoreMessages           int
	StoreCapacityMessages   int
	StoreCapacityBytes      int64
	StoreFileBytes          int64
	StoreUsageRatio         float64
	Limits                  Limits
}

type HealthSignals struct {
	NodeProfile          NodeProfile
	ServiceState         string
	ServiceReason        string
	BootstrapSourceCount int
	BootstrapStatus      BootstrapStatus
	EndpointCount        int
	UsableEndpointCount  int
	PeerCount            int
	RelayPeerCount       int
	FilterPeerCount      int
	LightpushPeerCount   int
	StorePeerCount       int
	Reachability         ReachabilitySnapshot
}

type Candidate = routing.Candidate

// Transport-prefixed names remain the canonical operator vocabulary while the
// shorter names are used inside the network module.
