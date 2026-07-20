package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	transport "ardents/internal/network/api"
	runtimeinfra "ardents/internal/runtime/process"
	workloadcontroller "ardents/internal/workload/controller"
	"ardents/internal/workload/execution"
)

const (
	apiTokenEnv                 = "ARDENTS_API_TOKEN"
	apiTokenFileEnv             = "ARDENTS_API_TOKEN_FILE"
	nodeNameEnv                 = "ARDENTS_NODE_NAME"
	dataDirEnv                  = "ARDENTS_DATA_DIR"
	storePathEnv                = "ARDENTS_WAKU_STORE_PATH"
	bootstrapPeersEnv           = "ARDENTS_BOOTSTRAP_PEERS"
	trustAnchorsEnv             = "ARDENTS_TRUST_ANCHORS"
	transportProfileEnv         = "ARDENTS_TRANSPORT_PROFILE"
	transportPortEnv            = "ARDENTS_TRANSPORT_PORT"
	nodeProfileEnv              = "ARDENTS_NODE_PROFILE"
	wssPortEnv                  = "ARDENTS_WSS_PORT"
	wssCertPathEnv              = "ARDENTS_WSS_CERT_PATH"
	wssKeyPathEnv               = "ARDENTS_WSS_KEY_PATH"
	wssCAPathEnv                = "ARDENTS_WSS_CA_PATH"
	wssAdvertiseEnv             = "ARDENTS_WSS_ADVERTISE_ADDRESS"
	dnsDiscoveryURLsEnv         = "ARDENTS_DNS_DISCOVERY_URLS"
	dnsNameServerEnv            = "ARDENTS_DNS_DISCOVERY_NAMESERVER"
	reachabilityModeEnv         = "ARDENTS_REACHABILITY_MODE"
	advertiseAddrsEnv           = "ARDENTS_ADVERTISE_ADDRESSES"
	maxMessageBytesEnv          = "ARDENTS_MAX_NETWORK_MESSAGE_BYTES"
	maxPeerConnsEnv             = "ARDENTS_MAX_PEER_CONNECTIONS"
	maxConnsPerIPEnv            = "ARDENTS_MAX_CONNECTIONS_PER_IP"
	maxNetworkOpsEnv            = "ARDENTS_MAX_NETWORK_CONCURRENCY"
	networkRateEnv              = "ARDENTS_NETWORK_OPERATION_RATE"
	networkBurstEnv             = "ARDENTS_NETWORK_OPERATION_BURST"
	maxFilterSubsEnv            = "ARDENTS_MAX_FILTER_SUBSCRIBERS"
	maxStoreResultsEnv          = "ARDENTS_MAX_STORE_RESULTS"
	workloadExecutorEnv         = "ARDENTS_WORKLOAD_EXECUTOR"
	workloadRegistriesEnv       = "ARDENTS_WORKLOAD_ALLOWED_REGISTRIES"
	workloadPolicyRefsEnv       = "ARDENTS_WORKLOAD_ALLOWED_POLICY_REFS"
	workloadTrustedRuntimeEnv   = "ARDENTS_WORKLOAD_TRUSTED_RUNTIME"
	workloadUntrustedRuntimeEnv = "ARDENTS_WORKLOAD_UNTRUSTED_RUNTIME"
	workloadIngressHostsEnv     = "ARDENTS_WORKLOAD_INGRESS_HOSTS"
	workloadIngressBindEnv      = "ARDENTS_WORKLOAD_INGRESS_BIND_ADDRESS"
	workloadIngressProxyEnv     = "ARDENTS_WORKLOAD_INGRESS_PROXY_IMAGE"
	operatorConfigFileEnv       = "ARDENTS_CONFIG_FILE"
)

type runtimeConfig struct {
	ListenAddr         string
	ObservabilityAddr  string
	ObservabilityToken string
	APIToken           string
	APISubject         string
	APICapabilities    []string
	APICredentialEnd   time.Time
	Node               runtimeinfra.Config
}

func loadRuntimeConfig() (runtimeConfig, error) {
	if path := strings.TrimSpace(os.Getenv(operatorConfigFileEnv)); path != "" {
		return loadOperatorRuntimeConfig(path)
	}
	token, err := loadAPIToken()
	if err != nil {
		return runtimeConfig{}, err
	}

	name := strings.TrimSpace(os.Getenv(nodeNameEnv))
	if name == "" {
		name = "ardd"
	}
	dataDir := strings.TrimSpace(os.Getenv(dataDirEnv))
	if dataDir == "" {
		dataDir = filepath.Join("var", name)
	}
	storePath := strings.TrimSpace(os.Getenv(storePathEnv))
	if storePath == "" {
		storePath = filepath.Join(dataDir, "waku-store.db")
	}

	cfg := baseRuntimeConfig(token, name, dataDir, storePath)
	if err := validateLocalAPIListenAddr(cfg.ListenAddr); err != nil {
		return runtimeConfig{}, err
	}
	if err := applyNetworkConfig(&cfg); err != nil {
		return runtimeConfig{}, err
	}
	if err := applyWorkloadExecutor(&cfg, name); err != nil {
		return runtimeConfig{}, err
	}
	return cfg, nil
}

func applyWorkloadExecutor(cfg *runtimeConfig, nodeID string) error {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv(workloadExecutorEnv)))
	if mode == "" {
		mode = "docker"
	}
	switch mode {
	case "docker":
		policyRefs := splitListEnv(workloadPolicyRefsEnv)
		cfg.Node.Policy.AllowedPolicyRefs = policyRefs
		executor, err := workloadcontroller.NewDockerExecutorWithConfig(execution.DockerExecutorConfig{
			NodeID: nodeID, AllowedRegistries: splitListEnv(workloadRegistriesEnv),
			AllowedPolicyRefs:   policyRefs,
			TrustedRuntime:      strings.TrimSpace(os.Getenv(workloadTrustedRuntimeEnv)),
			UntrustedRuntime:    strings.TrimSpace(os.Getenv(workloadUntrustedRuntimeEnv)),
			AllowedIngressHosts: splitListEnv(workloadIngressHostsEnv),
			IngressBindAddress:  strings.TrimSpace(os.Getenv(workloadIngressBindEnv)),
			IngressProxyImage:   strings.TrimSpace(os.Getenv(workloadIngressProxyEnv)),
		})
		if err != nil {
			return fmt.Errorf("%s=docker: %w", workloadExecutorEnv, err)
		}
		cfg.Node.WorkloadExecutor = executor
		return nil
	case "trusted-process":
		if transport.NormalizeNodeProfile(cfg.Node.NodeProfile) != transport.NodeProfileLocalDevelopment {
			return fmt.Errorf("%s=trusted-process requires local_development node profile", workloadExecutorEnv)
		}
		cfg.Node.WorkloadExecutor = workloadcontroller.NewLocalExecutor()
		return nil
	default:
		return fmt.Errorf("%s: unsupported mode %q", workloadExecutorEnv, mode)
	}
}

func applyNetworkConfig(cfg *runtimeConfig) error {
	if port, ok, err := intEnv(transportPortEnv); err != nil {
		return err
	} else if ok {
		cfg.Node.Transport.ListenPort = port
	}

	profile, ok, err := transportProfileEnvValue()
	if err != nil {
		return err
	}
	if ok {
		cfg.Node.Transport.Profile = profile
	}
	nodeProfile, ok, err := nodeProfileEnvValue()
	if err != nil {
		return err
	}
	if ok {
		cfg.Node.NodeProfile = nodeProfile
	}
	if port, ok, err := intEnv(wssPortEnv); err != nil {
		return err
	} else if ok {
		cfg.Node.Transport.WSSPort = port
	}
	cfg.Node.Transport.WSSCertPath = strings.TrimSpace(os.Getenv(wssCertPathEnv))
	cfg.Node.Transport.WSSKeyPath = strings.TrimSpace(os.Getenv(wssKeyPathEnv))
	cfg.Node.Transport.WSSCAPath = strings.TrimSpace(os.Getenv(wssCAPathEnv))
	cfg.Node.Transport.WSSAdvertiseAddress = strings.TrimSpace(os.Getenv(wssAdvertiseEnv))
	cfg.Node.Transport.DNSDiscoveryURLs = splitListEnv(dnsDiscoveryURLsEnv)
	cfg.Node.Transport.DNSDiscoveryNameServer = strings.TrimSpace(os.Getenv(dnsNameServerEnv))
	applyReachabilityConfig(cfg)
	if err := applyNetworkLimits(cfg); err != nil {
		return err
	}
	if err := runtimeinfra.ValidateConfig(cfg.Node); err != nil {
		return err
	}
	return nil
}

func applyNetworkLimits(cfg *runtimeConfig) error {
	bindings := []struct {
		name   string
		target *int
	}{
		{maxMessageBytesEnv, &cfg.Node.Transport.Limits.MaxMessageBytes},
		{maxPeerConnsEnv, &cfg.Node.Transport.Limits.MaxPeerConnections},
		{maxConnsPerIPEnv, &cfg.Node.Transport.Limits.MaxConnectionsPerIP},
		{maxNetworkOpsEnv, &cfg.Node.Transport.Limits.MaxConcurrentOperations},
		{networkRateEnv, &cfg.Node.Transport.Limits.OperationRate},
		{networkBurstEnv, &cfg.Node.Transport.Limits.OperationBurst},
		{maxFilterSubsEnv, &cfg.Node.Transport.Limits.MaxFilterSubscribers},
		{maxStoreResultsEnv, &cfg.Node.Transport.Limits.MaxStoreResults},
	}
	for _, binding := range bindings {
		value, ok, err := nonNegativeIntEnv(binding.name)
		if err != nil {
			return err
		}
		if ok {
			*binding.target = value
		}
	}
	return nil
}

func nonNegativeIntEnv(name string) (int, bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, fmt.Errorf("%s: parse int: %w", name, err)
	}
	if value < 0 {
		return 0, false, fmt.Errorf("%s: value cannot be negative", name)
	}
	return value, true, nil
}

func applyReachabilityConfig(cfg *runtimeConfig) {
	cfg.Node.Transport.ReachabilityMode = transport.ReachabilityMode(strings.TrimSpace(os.Getenv(reachabilityModeEnv)))
	cfg.Node.Transport.AdvertiseAddresses = splitListEnv(advertiseAddrsEnv)
	if cfg.Node.Transport.ReachabilityMode == "" {
		if transport.NormalizeNodeProfile(cfg.Node.NodeProfile) == transport.NodeProfileLocalDevelopment {
			cfg.Node.Transport.ReachabilityMode = transport.ReachabilityLocalOnly
		} else if transport.NormalizeNodeProfile(cfg.Node.NodeProfile) == transport.NodeProfileConstrainedClient {
			cfg.Node.Transport.ReachabilityMode = transport.ReachabilityOutboundOnly
		} else {
			cfg.Node.Transport.ReachabilityMode = transport.ReachabilityPrivateLAN
		}
	}
}

func baseRuntimeConfig(token, name, dataDir, storePath string) runtimeConfig {
	return runtimeConfig{
		ListenAddr:        listenAddr(),
		ObservabilityAddr: "127.0.0.1:9090",
		APIToken:          token,
		APISubject:        "ardd-local-api",
		APICapabilities:   localAdminCapabilities(),
		Node: runtimeinfra.Config{
			Name:        name,
			NodeProfile: transport.NodeProfileServiceNode,
			Boot:        runtimeinfra.BootConfig{Sources: splitListEnv(bootstrapPeersEnv)},
			Trust:       runtimeinfra.TrustConfig{Anchors: splitListEnv(trustAnchorsEnv)},
			Data:        runtimeinfra.DataConfig{Dir: dataDir},
			Transport: runtimeinfra.TransportConfig{
				StorePath:   storePath,
				BindAddress: strings.TrimSpace(os.Getenv(transport.BindAddressEnv)),
			},
		},
	}
}

func transportProfileEnvValue() (transport.TransportProfile, bool, error) {
	rawProfile := strings.TrimSpace(os.Getenv(transportProfileEnv))
	if rawProfile == "" {
		return "", false, nil
	}
	profile := transport.TransportProfile(rawProfile)
	if _, err := transport.ResolveProfile(profile); err != nil {
		return "", false, fmt.Errorf("%s: %w", transportProfileEnv, err)
	}
	return profile, true, nil
}

func nodeProfileEnvValue() (transport.NodeProfile, bool, error) {
	rawProfile := strings.TrimSpace(os.Getenv(nodeProfileEnv))
	if rawProfile == "" {
		return "", false, nil
	}
	profile := transport.NodeProfile(rawProfile)
	if _, err := transport.ResolveNodeProfile(profile); err != nil {
		return "", false, fmt.Errorf("%s: %w", nodeProfileEnv, err)
	}
	return profile, true, nil
}

func splitListEnv(name string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}
	return splitList(raw)
}

func splitList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func intEnv(name string) (int, bool, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, fmt.Errorf("%s: parse int: %w", name, err)
	}
	if value < 0 || value > 65535 {
		return 0, false, fmt.Errorf("%s: port must be between 0 and 65535", name)
	}
	return value, true, nil
}
