package config

import (
	"encoding/json"
	"path/filepath"

	identitytrust "ardents/internal/identity/trust"
	workloadregistry "ardents/internal/workload/registry"
)

type Document struct {
	APIVersion           string                     `json:"api_version"`
	Node                 NodeConfig                 `json:"node"`
	API                  APIConfig                  `json:"api"`
	ApplicationInterface ApplicationInterfaceConfig `json:"application_interface"`
	Authority            AuthorityConfig            `json:"authority"`
	Trust                TrustConfig                `json:"trust"`
	Network              NetworkConfig              `json:"network"`
	Privacy              PrivacyConfig              `json:"privacy"`
	Workloads            WorkloadsConfig            `json:"workloads"`
	Services             []ServiceConfig            `json:"services"`
	Data                 DataConfig                 `json:"data"`
	Policy               PolicyConfig               `json:"policy"`
	Logging              LoggingConfig              `json:"logging"`
	Observability        ObservabilityConfig        `json:"observability"`
	Diagnostics          DiagnosticsConfig          `json:"diagnostics"`
}

type TrustConfig struct {
	Principals []TrustedPrincipalConfig `json:"principals"`
}

type TrustedPrincipalConfig struct {
	Principal string                  `json:"principal"`
	PublicKey string                  `json:"public_key"`
	Purposes  []identitytrust.Purpose `json:"purposes"`
}

type ApplicationInterfaceConfig struct {
	Enabled    bool   `json:"enabled"`
	SocketPath string `json:"socket_path,omitempty"`
}

type AuthorityConfig struct {
	Enabled                  bool   `json:"enabled"`
	StorePath                string `json:"store_path"`
	StoreKeyFile             string `json:"store_key_file"`
	SignerFile               string `json:"signer_file"`
	CheckpointRepositoryPath string `json:"checkpoint_repository_path"`
}

type NodeConfig struct {
	Name    string `json:"name"`
	Profile string `json:"profile"`
	DataDir string `json:"data_dir"`
}

type APIConfig struct {
	SocketPath string `json:"socket_path"`
}

type PrivacyConfig struct {
	Required                 bool                 `json:"required"`
	DeliveryEnabled          bool                 `json:"delivery_enabled"`
	ChannelGrantStore        string               `json:"channel_grant_store"`
	ChannelGrantStoreKeyFile string               `json:"channel_grant_store_key_file"`
	ReplayKeyFile            string               `json:"replay_key_file"`
	Subject                  string               `json:"subject"`
	Discovery                PrivacyChannelConfig `json:"discovery"`
	Data                     PrivacyChannelConfig `json:"data"`
}

type PrivacyChannelConfig struct {
	Reference  string `json:"reference"`
	ReplayPath string `json:"replay_path"`
}

type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

type ObservabilityConfig struct {
	ListenAddress string `json:"listen_address"`
	TokenFile     string `json:"token_file"`
}

type DiagnosticsConfig struct {
	MaxEvents   int    `json:"max_events"`
	DetailLevel string `json:"detail_level"`
}

type ServiceConfig struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Owner          string   `json:"owner"`
	Mode           string   `json:"mode"`
	Endpoints      []string `json:"endpoints"`
	ProbeEndpoints []string `json:"probe_endpoints"`
}

type NetworkConfig struct {
	TransportProfile        string        `json:"transport_profile"`
	BindAddress             string        `json:"bind_address"`
	ListenPort              int           `json:"listen_port"`
	StorePath               string        `json:"store_path"`
	PrivateKeyPath          string        `json:"private_key_path"`
	BootstrapPeers          []string      `json:"bootstrap_peers"`
	ReachabilityMode        string        `json:"reachability_mode"`
	AdvertiseAddresses      []string      `json:"advertise_addresses"`
	DNSDiscoveryURLs        []string      `json:"dns_discovery_urls"`
	DNSDiscoveryNameServer  string        `json:"dns_discovery_nameserver"`
	DiscoveryRefreshSeconds int           `json:"discovery_refresh_seconds"`
	WSS                     WSSConfig     `json:"wss"`
	Limits                  NetworkLimits `json:"limits"`
}

type WSSConfig struct {
	Port             int    `json:"port"`
	CertificateFile  string `json:"certificate_file"`
	PrivateKeyFile   string `json:"private_key_file"`
	CAFile           string `json:"ca_file"`
	AdvertiseAddress string `json:"advertise_address"`
}

type NetworkLimits struct {
	MaxMessageBytes         int   `json:"max_message_bytes"`
	MaxPeerConnections      int   `json:"max_peer_connections"`
	MaxConnectionsPerIP     int   `json:"max_connections_per_ip"`
	MaxConcurrentOperations int   `json:"max_concurrent_operations"`
	OperationRate           int   `json:"operation_rate"`
	OperationBurst          int   `json:"operation_burst"`
	MaxFilterSubscribers    int   `json:"max_filter_subscribers"`
	MaxStoreResults         int   `json:"max_store_results"`
	StoreMaxMessages        int   `json:"store_max_messages"`
	StoreMaxAgeSeconds      int   `json:"store_max_age_seconds"`
	StoreMaxBytes           int64 `json:"store_max_bytes"`
}

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
	MaxWorkloads                    int                                    `json:"max_workloads"`
	AllowedPolicyRefs               []string                               `json:"allowed_policy_refs"`
	DeniedWorkloadRequirements      []workloadregistry.WorkloadRequirement `json:"denied_workload_requirements"`
	DisableServicePublication       bool                                   `json:"disable_service_publication"`
	DisableNetworkPublishedServices bool                                   `json:"disable_network_published_services"`
	DeniedServiceTypes              []string                               `json:"denied_service_types"`
	DisableUntrustedRouteUse        bool                                   `json:"disable_untrusted_route_use"`
	DeniedRouteSchemes              []string                               `json:"denied_route_schemes"`
	DisablePrivateChannelGrantUse   bool                                   `json:"disable_private_channel_grant_use"`
	DisableRealmAuthorityCreation   bool                                   `json:"disable_realm_authority_creation"`
	DisableRealmChannelDelivery     bool                                   `json:"disable_realm_channel_delivery"`
	DeniedChannelGrantScopes        []string                               `json:"denied_channel_grant_scopes"`
	DisableLocalBlobRetention       bool                                   `json:"disable_local_blob_retention"`
	DisableRelayBlobRetention       bool                                   `json:"disable_relay_blob_retention"`
	DisableBlobPinning              bool                                   `json:"disable_blob_pinning"`
	DisablePeerBlobReserving        bool                                   `json:"disable_peer_blob_reserving"`
	AllowPinRelayRetainedBlobs      bool                                   `json:"allow_pin_relay_retained_blobs"`
	AllowReservingRelayBlobs        bool                                   `json:"allow_reserving_relay_blobs"`
	MaxLocalRetention               string                                 `json:"max_local_retention"`
	MaxRelayRetention               string                                 `json:"max_relay_retention"`
}

type WorkloadsConfig struct {
	Executor            string         `json:"executor"`
	AllowedRegistries   []string       `json:"allowed_registries"`
	AllowedPolicyRefs   []string       `json:"allowed_policy_refs"`
	TrustedRuntime      string         `json:"trusted_runtime"`
	UntrustedRuntime    string         `json:"untrusted_runtime"`
	AllowedIngressHosts []string       `json:"allowed_ingress_hosts"`
	IngressBindAddress  string         `json:"ingress_bind_address"`
	IngressProxyImage   string         `json:"ingress_proxy_image"`
	Initial             []WorkloadSpec `json:"initial"`
}

type WorkloadSpec struct {
	ID            string                                 `json:"id"`
	Kind          string                                 `json:"kind"`
	Owner         string                                 `json:"owner"`
	Config        string                                 `json:"config"`
	Desired       string                                 `json:"desired"`
	Requirements  []workloadregistry.WorkloadRequirement `json:"requirements"`
	PolicyRef     string                                 `json:"policy_ref"`
	RestartPolicy string                                 `json:"restart_policy"`
	Services      []ServiceConfig                        `json:"services"`
}

func applyContextDefaults(raw []byte, doc *Document) {
	var root map[string]json.RawMessage
	if json.Unmarshal(raw, &root) != nil {
		return
	}
	if !nestedFieldPresent(root, "node", "data_dir") {
		doc.Node.DataDir = filepath.Join("var", doc.Node.Name)
	}
	if !nestedFieldPresent(root, "network", "store_path") {
		doc.Network.StorePath = filepath.Join(doc.Node.DataDir, "waku-store.db")
	}
}

func nestedFieldPresent(root map[string]json.RawMessage, section, field string) bool {
	raw, ok := root[section]
	if !ok {
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return false
	}
	_, ok = fields[field]
	return ok
}
