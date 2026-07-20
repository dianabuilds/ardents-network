package config

const Version = "ardents.config/v1"

func Defaults() Document {
	return Document{
		APIVersion: Version,
		Node:       NodeConfig{Name: "ardents", Profile: "service_node", DataDir: "var/ardents"},
		API:        APIConfig{ListenAddress: "127.0.0.1:8080", OperatorSubject: "ardd-local-api"},
		Network: NetworkConfig{
			TransportProfile: "tcp_only", BindAddress: "0.0.0.0",
			StorePath: "var/ardents/waku-store.db", ReachabilityMode: "private_lan",
			DiscoveryRefreshSeconds: 30,
			Limits: NetworkLimits{
				MaxMessageBytes: 143360, MaxPeerConnections: 64, MaxConnectionsPerIP: 4,
				MaxConcurrentOperations: 16, OperationRate: 20, OperationBurst: 40,
				MaxFilterSubscribers: 32, MaxStoreResults: 128,
			},
		},
		Workloads:     WorkloadsConfig{Executor: "docker"},
		Data:          DataConfig{MaxReplicaBytes: 1 << 30, DesiredReplicas: 3, MinimumReplicas: 2},
		Logging:       LoggingConfig{Level: "info", Format: "json"},
		Observability: ObservabilityConfig{ListenAddress: "127.0.0.1:9090"},
		Diagnostics:   DiagnosticsConfig{MaxEvents: 1000, DetailLevel: "standard"},
	}
}
