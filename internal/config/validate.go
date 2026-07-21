package config

import (
	networkapi "ardents/internal/network"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

func Validate(doc Document) error {
	if doc.APIVersion != Version {
		return fmt.Errorf("unsupported api_version %q: expected %q", doc.APIVersion, Version)
	}
	if strings.TrimSpace(doc.Node.Name) == "" {
		return fmt.Errorf("node.name is required")
	}
	if strings.TrimSpace(doc.Node.DataDir) == "" {
		return fmt.Errorf("node.data_dir is required")
	}
	if _, _, err := net.SplitHostPort(doc.API.ListenAddress); err != nil {
		return fmt.Errorf("api.listen_address: %w", err)
	}
	if err := validateAPI(doc.API); err != nil {
		return err
	}
	if err := validateNetwork(doc); err != nil {
		return err
	}
	if err := validatePrivacy(doc.Privacy); err != nil {
		return err
	}
	if err := validateWorkloads(doc); err != nil {
		return err
	}
	if err := validateData(doc.Data); err != nil {
		return err
	}
	if err := validatePolicy(doc); err != nil {
		return err
	}
	return validateObservability(doc)
}

func validateAPI(cfg APIConfig) error {
	if strings.TrimSpace(cfg.OperatorSubject) == "" {
		return fmt.Errorf("api.operator_subject is required")
	}
	seen := make(map[string]struct{}, len(cfg.Capabilities))
	for _, capability := range cfg.Capabilities {
		capability = strings.TrimSpace(capability)
		if capability == "" || capability == "*" || capability == "admin" {
			return fmt.Errorf("api.capabilities must contain explicit action names")
		}
		if _, duplicate := seen[capability]; duplicate {
			return fmt.Errorf("api.capabilities contains duplicate %q", capability)
		}
		seen[capability] = struct{}{}
	}
	if cfg.CredentialExpiresAt != "" {
		if _, err := time.Parse(time.RFC3339, cfg.CredentialExpiresAt); err != nil {
			return fmt.Errorf("api.credential_expires_at must be RFC3339: %w", err)
		}
	}
	return nil
}

func validatePrivacy(cfg PrivacyConfig) error {
	if !cfg.Required {
		if cfg.CapabilityStore != "" || cfg.CapabilityStoreKeyFile != "" || cfg.ReplayKeyFile != "" ||
			cfg.Subject != "" || len(cfg.TrustedIssuers) > 0 || cfg.Discovery.Reference != "" ||
			cfg.Discovery.ReplayPath != "" || cfg.Data.Reference != "" || cfg.Data.ReplayPath != "" {
			return fmt.Errorf("privacy material requires privacy.required=true")
		}
		return nil
	}
	required := []struct {
		path  string
		value string
	}{
		{"privacy.capability_store", cfg.CapabilityStore},
		{"privacy.capability_store_key_file", cfg.CapabilityStoreKeyFile},
		{"privacy.replay_key_file", cfg.ReplayKeyFile},
		{"privacy.subject", cfg.Subject},
		{"privacy.discovery.reference", cfg.Discovery.Reference},
		{"privacy.discovery.replay_path", cfg.Discovery.ReplayPath},
		{"privacy.data.reference", cfg.Data.Reference},
		{"privacy.data.replay_path", cfg.Data.ReplayPath},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required when privacy.required=true", field.path)
		}
	}
	if len(cfg.TrustedIssuers) == 0 {
		return fmt.Errorf("privacy.trusted_issuers is required when privacy.required=true")
	}
	if cfg.Discovery.Reference == cfg.Data.Reference {
		return fmt.Errorf("privacy discovery and data references must be distinct")
	}
	if cfg.Discovery.ReplayPath == cfg.Data.ReplayPath {
		return fmt.Errorf("privacy discovery and data replay paths must be distinct")
	}
	return nil
}

func validateData(cfg DataConfig) error {
	if cfg.DesiredReplicas < 1 {
		return fmt.Errorf("data.desired_replicas must be at least 1")
	}
	if cfg.MinimumReplicas < 1 || cfg.MinimumReplicas > cfg.DesiredReplicas {
		return fmt.Errorf("data.minimum_replicas must be between 1 and desired_replicas")
	}
	if cfg.MaxRelayBytes < 0 || cfg.MaxReplicaBytes < 0 || cfg.MaxLocalBytes < 0 {
		return fmt.Errorf("data storage limits cannot be negative")
	}
	for path, raw := range map[string]string{
		"data.default_local_retention": cfg.DefaultLocalRetention,
		"data.default_relay_retention": cfg.DefaultRelayRetention,
	} {
		if raw != "" {
			if duration, err := time.ParseDuration(raw); err != nil || duration < 0 {
				return fmt.Errorf("%s must be a non-negative duration", path)
			}
		}
	}
	return nil
}

func validateObservability(doc Document) error {
	if doc.Logging.Level != "debug" && doc.Logging.Level != "info" && doc.Logging.Level != "warn" && doc.Logging.Level != "error" {
		return fmt.Errorf("logging.level: unsupported value %q", doc.Logging.Level)
	}
	if doc.Logging.Format != "json" && doc.Logging.Format != "text" {
		return fmt.Errorf("logging.format: unsupported value %q", doc.Logging.Format)
	}
	if err := validateObservabilityListen(doc.Observability); err != nil {
		return err
	}
	if strings.TrimSpace(doc.API.ListenAddress) == strings.TrimSpace(doc.Observability.ListenAddress) {
		return fmt.Errorf("observability.listen_address must differ from api.listen_address")
	}
	if doc.Diagnostics.MaxEvents < 100 || doc.Diagnostics.MaxEvents > 100000 {
		return fmt.Errorf("diagnostics.max_events must be between 100 and 100000")
	}
	if doc.Diagnostics.DetailLevel != "minimal" && doc.Diagnostics.DetailLevel != "standard" {
		return fmt.Errorf("diagnostics.detail_level: unsupported value %q", doc.Diagnostics.DetailLevel)
	}
	return nil
}

func validateObservabilityListen(cfg ObservabilityConfig) error {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(cfg.ListenAddress))
	if err != nil {
		return fmt.Errorf("observability.listen_address: invalid address")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("observability.listen_address: invalid port")
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("observability.listen_address must be loopback; secure remote exposure is configured by deployment")
	}
	return nil
}

func validateNetwork(doc Document) error {
	if strings.TrimSpace(doc.Network.StorePath) == "" {
		return fmt.Errorf("network.store_path is required")
	}
	nodeProfile := networkapi.NodeProfile(doc.Node.Profile)
	transportProfile := networkapi.Profile(doc.Network.TransportProfile)
	if _, err := networkapi.ResolveNodeProfile(nodeProfile); err != nil {
		return fmt.Errorf("node.profile: %w", err)
	}
	if _, err := networkapi.ResolveProfile(transportProfile); err != nil {
		return fmt.Errorf("network.transport_profile: %w", err)
	}
	if err := networkapi.ValidateNodeProfileTransport(nodeProfile, transportProfile); err != nil {
		return fmt.Errorf("network participation configuration: %w", err)
	}
	if doc.Node.Profile == string(networkapi.NodeProfileLocalDevelopment) &&
		doc.Network.BindAddress != "127.0.0.1" && doc.Network.BindAddress != "localhost" {
		return fmt.Errorf("local_development node profile requires a loopback bind address")
	}
	if doc.Network.DiscoveryRefreshSeconds < 1 || doc.Network.DiscoveryRefreshSeconds > 3600 {
		return fmt.Errorf("network.discovery_refresh_seconds must be between 1 and 3600")
	}
	return nil
}

func validatePolicy(doc Document) error {
	policy := doc.Policy
	if policy.MaxWorkloads < 0 {
		return fmt.Errorf("policy.max_workloads cannot be negative")
	}
	if policy.MaxWorkloads > 0 && len(doc.Workloads.Initial) > policy.MaxWorkloads {
		return fmt.Errorf("policy.max_workloads is lower than the initial workload count")
	}
	if len(policy.AllowedPolicyRefs) > 0 && len(doc.Workloads.AllowedPolicyRefs) > 0 &&
		!sameStrings(policy.AllowedPolicyRefs, doc.Workloads.AllowedPolicyRefs) {
		return fmt.Errorf("policy.allowed_policy_refs conflicts with workloads.allowed_policy_refs")
	}
	localMax, err := policyDuration("policy.max_local_retention", policy.MaxLocalRetention)
	if err != nil {
		return err
	}
	relayMax, err := policyDuration("policy.max_relay_retention", policy.MaxRelayRetention)
	if err != nil {
		return err
	}
	if err := validateRetentionDefault("data.default_local_retention", doc.Data.DefaultLocalRetention, localMax); err != nil {
		return err
	}
	if err := validateRetentionDefault("data.default_relay_retention", doc.Data.DefaultRelayRetention, relayMax); err != nil {
		return err
	}
	if policy.DisableBlobPinning && policy.AllowPinRelayRetainedBlobs {
		return fmt.Errorf("policy cannot allow relay pinning while blob pinning is disabled")
	}
	if policy.DisablePeerBlobReserving && policy.AllowReservingRelayBlobs {
		return fmt.Errorf("policy cannot allow relay reservations while peer reserving is disabled")
	}
	return nil
}

func policyDuration(path, raw string) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("%s must be a non-negative duration", path)
	}
	return value, nil
}

func validateRetentionDefault(path, raw string, ceiling time.Duration) error {
	if raw == "" || ceiling == 0 {
		return nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("%s must be a duration", path)
	}
	if value > ceiling {
		return fmt.Errorf("%s exceeds its policy ceiling", path)
	}
	return nil
}

func sameStrings(left, right []string) bool {
	leftCopy, rightCopy := append([]string(nil), left...), append([]string(nil), right...)
	slices.Sort(leftCopy)
	slices.Sort(rightCopy)
	return slices.Equal(leftCopy, rightCopy)
}

func validateWorkloads(doc Document) error {
	switch doc.Workloads.Executor {
	case "docker":
	case "trusted-process":
		if doc.Node.Profile != string(networkapi.NodeProfileLocalDevelopment) {
			return fmt.Errorf("workloads.executor=trusted-process requires local_development node profile")
		}
	default:
		return fmt.Errorf("workloads.executor: unsupported mode %q", doc.Workloads.Executor)
	}
	if err := validateServiceList("services", doc.Services, false, ""); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	allowedPolicyRefs := normalizedStrings(effectiveAllowedPolicyRefs(doc))
	deniedCapabilities := normalizedStrings(doc.Policy.DeniedCapabilities)
	for index, workload := range doc.Workloads.Initial {
		path := fmt.Sprintf("workloads.initial[%d]", index)
		if err := validateWorkload(path, workload); err != nil {
			return err
		}
		if err := validateWorkloadPolicy(path, workload, allowedPolicyRefs, deniedCapabilities); err != nil {
			return err
		}
		if _, duplicate := seen[workload.ID]; duplicate {
			return fmt.Errorf("%s.id is duplicated", path)
		}
		seen[workload.ID] = struct{}{}
		if err := validateServiceList(path+".services", workload.Services, true, workload.ID); err != nil {
			return err
		}
	}
	return nil
}

func validateWorkloadPolicy(path string, workload WorkloadSpec, allowed, denied map[string]struct{}) error {
	if workload.PolicyRef != "" {
		if _, ok := allowed[strings.ToLower(strings.TrimSpace(workload.PolicyRef))]; !ok {
			return fmt.Errorf("%s.policy_ref is not allowed", path)
		}
	}
	for _, capability := range workload.Capabilities {
		if _, ok := denied[strings.ToLower(strings.TrimSpace(capability))]; ok {
			return fmt.Errorf("%s.capabilities contains a policy-denied capability", path)
		}
	}
	return nil
}

func effectiveAllowedPolicyRefs(doc Document) []string {
	if len(doc.Policy.AllowedPolicyRefs) > 0 {
		return doc.Policy.AllowedPolicyRefs
	}
	return doc.Workloads.AllowedPolicyRefs
}

func normalizedStrings(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		if normalized := strings.ToLower(strings.TrimSpace(value)); normalized != "" {
			out[normalized] = struct{}{}
		}
	}
	return out
}

func validateWorkload(path string, workload WorkloadSpec) error {
	if strings.TrimSpace(workload.ID) == "" || strings.TrimSpace(workload.Owner) == "" {
		return fmt.Errorf("%s id and owner are required", path)
	}
	switch workload.Kind {
	case "service", "worker", "app", "adapter":
	default:
		return fmt.Errorf("%s.kind is unsupported", path)
	}
	switch workload.Desired {
	case "present", "running", "stopped", "disabled", "removed":
	default:
		return fmt.Errorf("%s.desired is unsupported", path)
	}
	if workload.Desired == "running" && strings.TrimSpace(workload.Config) == "" {
		return fmt.Errorf("%s.config is required when desired=running", path)
	}
	if workload.RestartPolicy != "" && workload.RestartPolicy != "on-failure" && workload.RestartPolicy != "never" {
		return fmt.Errorf("%s.restart_policy is unsupported", path)
	}
	return nil
}

func validateServiceList(path string, services []ServiceConfig, paired bool, expectedOwner string) error {
	seen := map[string]struct{}{}
	for index, service := range services {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		if strings.TrimSpace(service.ID) == "" || strings.TrimSpace(service.Type) == "" {
			return fmt.Errorf("%s id and type are required", itemPath)
		}
		if expectedOwner == "" && strings.TrimSpace(service.Owner) == "" {
			return fmt.Errorf("%s.owner is required", itemPath)
		}
		if expectedOwner != "" && service.Owner != "" && service.Owner != expectedOwner {
			return fmt.Errorf("%s.owner must match its workload", itemPath)
		}
		if _, duplicate := seen[service.ID]; duplicate {
			return fmt.Errorf("%s.id is duplicated", itemPath)
		}
		seen[service.ID] = struct{}{}
		if service.Mode != "NetworkPublished" && service.Mode != "LocalOnly" {
			return fmt.Errorf("%s.mode is unsupported", itemPath)
		}
		if len(service.Endpoints) == 0 {
			return fmt.Errorf("%s requires at least one endpoint", itemPath)
		}
		if paired && len(service.Endpoints) != len(service.ProbeEndpoints) {
			return fmt.Errorf("%s requires paired endpoint and probe sets", itemPath)
		}
		if !paired && len(service.ProbeEndpoints) > 0 && len(service.Endpoints) != len(service.ProbeEndpoints) {
			return fmt.Errorf("%s endpoint and probe sets must have equal length", itemPath)
		}
		for _, endpoint := range append(append([]string(nil), service.Endpoints...), service.ProbeEndpoints...) {
			parsed, err := url.Parse(endpoint)
			if err != nil || parsed.Scheme == "" {
				return fmt.Errorf("%s contains an invalid endpoint", itemPath)
			}
		}
	}
	return nil
}
