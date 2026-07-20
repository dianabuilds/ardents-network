package config

type DataConfig struct {
	DefaultLocalRetention string `json:"default_local_retention"`
	DefaultRelayRetention string `json:"default_relay_retention"`
	MaxRelayBytes         int64  `json:"max_relay_bytes"`
	MaxReplicaBytes       int64  `json:"max_replica_bytes"`
	MaxLocalBytes         int64  `json:"max_local_bytes"`
	DesiredReplicas       int    `json:"desired_replicas"`
	MinimumReplicas       int    `json:"minimum_replicas"`
}

type PolicyConfig struct {
	MaxWorkloads                    int      `json:"max_workloads"`
	AllowedPolicyRefs               []string `json:"allowed_policy_refs"`
	DeniedCapabilities              []string `json:"denied_capabilities"`
	DisableServicePublication       bool     `json:"disable_service_publication"`
	DisableNetworkPublishedServices bool     `json:"disable_network_published_services"`
	DeniedServiceTypes              []string `json:"denied_service_types"`
	DisableUntrustedRouteUse        bool     `json:"disable_untrusted_route_use"`
	DeniedRouteSchemes              []string `json:"denied_route_schemes"`
	DisablePrivateCapabilityUse     bool     `json:"disable_private_capability_use"`
	DeniedCapabilityScopes          []string `json:"denied_capability_scopes"`
	DisableLocalBlobRetention       bool     `json:"disable_local_blob_retention"`
	DisableRelayBlobRetention       bool     `json:"disable_relay_blob_retention"`
	DisableBlobPinning              bool     `json:"disable_blob_pinning"`
	DisablePeerBlobReserving        bool     `json:"disable_peer_blob_reserving"`
	AllowPinRelayRetainedBlobs      bool     `json:"allow_pin_relay_retained_blobs"`
	AllowReservingRelayBlobs        bool     `json:"allow_reserving_relay_blobs"`
	MaxLocalRetention               string   `json:"max_local_retention"`
	MaxRelayRetention               string   `json:"max_relay_retention"`
}
