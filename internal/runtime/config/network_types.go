package config

type NetworkConfig struct {
	TransportProfile        string        `json:"transport_profile"`
	BindAddress             string        `json:"bind_address"`
	ListenPort              int           `json:"listen_port"`
	StorePath               string        `json:"store_path"`
	PrivateKeyPath          string        `json:"private_key_path"`
	BootstrapPeers          []string      `json:"bootstrap_peers"`
	TrustAnchors            []string      `json:"trust_anchors"`
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
	MaxMessageBytes         int `json:"max_message_bytes"`
	MaxPeerConnections      int `json:"max_peer_connections"`
	MaxConnectionsPerIP     int `json:"max_connections_per_ip"`
	MaxConcurrentOperations int `json:"max_concurrent_operations"`
	OperationRate           int `json:"operation_rate"`
	OperationBurst          int `json:"operation_burst"`
	MaxFilterSubscribers    int `json:"max_filter_subscribers"`
	MaxStoreResults         int `json:"max_store_results"`
}
