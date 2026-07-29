package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	identitytrust "ardents/internal/identity/trust"
	networkapi "ardents/internal/network"
	workloadregistry "ardents/internal/workload/registry"
)

var immutableImageReferencePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*@sha256:[0-9a-f]{64}$`)

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
	if image := strings.TrimSpace(doc.Node.ImageReference); image != "" &&
		!immutableImageReferencePattern.MatchString(image) {
		return fmt.Errorf("node.image_reference must be an immutable canonical sha256 image reference")
	}
	if err := validateAPI(doc.API); err != nil {
		return err
	}
	if err := validateApplicationInterface(doc.ApplicationInterface); err != nil {
		return err
	}
	if err := validateAuthority(doc.Authority, doc.Node.DataDir); err != nil {
		return err
	}
	if doc.ApplicationInterface.Enabled {
		if doc.ApplicationInterface.SocketPath == doc.API.SocketPath {
			return fmt.Errorf("application_interface.socket_path must differ from api.socket_path")
		}
	}
	if err := validateNetwork(doc); err != nil {
		return err
	}
	if err := validateTrust(doc.Trust); err != nil {
		return err
	}
	if err := validatePrivacy(doc.Privacy, doc.Trust); err != nil {
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

func validateAuthority(cfg AuthorityConfig, nodeDataDir string) error {
	values := []struct {
		name, value string
	}{
		{"authority.store_path", cfg.StorePath},
		{"authority.store_key_file", cfg.StoreKeyFile},
		{"authority.signer_file", cfg.SignerFile},
		{"authority.checkpoint_repository_path", cfg.CheckpointRepositoryPath},
	}
	optionalValues := []struct {
		name, value string
	}{
		{"authority.successor_signer_file", cfg.SuccessorSignerFile},
	}
	if !cfg.Enabled {
		if cfg.RecoveryOnly {
			return fmt.Errorf("authority.recovery_only requires authority.enabled=true")
		}
		for _, field := range values {
			if field.value != "" {
				return fmt.Errorf("%s requires authority.enabled=true", field.name)
			}
		}
		for _, field := range optionalValues {
			if field.value != "" {
				return fmt.Errorf("%s requires authority.enabled=true", field.name)
			}
		}
		return nil
	}
	for _, field := range values {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required when authority.enabled=true", field.name)
		}
		if !filepath.IsAbs(field.value) {
			return fmt.Errorf("%s must be absolute", field.name)
		}
	}
	for _, field := range optionalValues {
		if strings.TrimSpace(field.value) == "" {
			continue
		}
		if !filepath.IsAbs(field.value) {
			return fmt.Errorf("%s must be absolute", field.name)
		}
		values = append(values, field)
	}
	clean := make(map[string]string, len(values))
	for _, field := range values {
		clean[field.name] = filepath.Clean(field.value)
	}
	if clean["authority.store_path"] == clean["authority.store_key_file"] ||
		clean["authority.store_path"] == clean["authority.signer_file"] ||
		clean["authority.store_key_file"] == clean["authority.signer_file"] ||
		(clean["authority.successor_signer_file"] != "" &&
			(clean["authority.successor_signer_file"] == clean["authority.store_path"] ||
				clean["authority.successor_signer_file"] == clean["authority.store_key_file"] ||
				clean["authority.successor_signer_file"] == clean["authority.signer_file"])) {
		return fmt.Errorf("authority store and signer inputs must be distinct")
	}
	checkpoint := clean["authority.checkpoint_repository_path"]
	for _, protected := range []struct {
		name, path string
	}{
		{"node.data_dir", filepath.Clean(nodeDataDir)},
		{"authority.store_path", clean["authority.store_path"]},
		{"authority.store_key_file", clean["authority.store_key_file"]},
		{"authority.signer_file", clean["authority.signer_file"]},
		{"authority.successor_signer_file", clean["authority.successor_signer_file"]},
	} {
		if protected.path == "" {
			continue
		}
		if pathContains(protected.path, checkpoint) || pathContains(checkpoint, protected.path) {
			return fmt.Errorf("authority.checkpoint_repository_path must be outside %s", protected.name)
		}
	}
	return nil
}

func pathContains(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil || relative == "." {
		return err == nil
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func validateApplicationInterface(cfg ApplicationInterfaceConfig) error {
	if !cfg.Enabled {
		return nil
	}
	return validateSocketPath("application_interface.socket_path", cfg.SocketPath)
}

func validateAPI(cfg APIConfig) error {
	return validateSocketPath("api.socket_path", cfg.SocketPath)
}

func validateSocketPath(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", field)
	}
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("%s contains an invalid character", field)
	}
	if !filepath.IsAbs(value) && !strings.HasPrefix(value, "/") && !strings.HasPrefix(value, `\`) {
		return fmt.Errorf("%s must be absolute", field)
	}
	return nil
}

func validatePrivacy(cfg PrivacyConfig, trust TrustConfig) error {
	if !cfg.Required && !cfg.DeliveryEnabled {
		if cfg.ChannelGrantStore != "" || cfg.ChannelGrantStoreKeyFile != "" || cfg.ReplayKeyFile != "" ||
			cfg.Subject != "" || cfg.Discovery.Reference != "" ||
			cfg.Discovery.ReplayPath != "" || cfg.Data.Reference != "" || cfg.Data.ReplayPath != "" {
			return fmt.Errorf("privacy material requires privacy.required=true or privacy.delivery_enabled=true")
		}
		return nil
	}
	deliveryRequired := []struct {
		path  string
		value string
	}{
		{"privacy.channel_grant_store", cfg.ChannelGrantStore},
		{"privacy.channel_grant_store_key_file", cfg.ChannelGrantStoreKeyFile},
		{"privacy.subject", cfg.Subject},
	}
	for _, field := range deliveryRequired {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required when privacy delivery is enabled", field.path)
		}
	}
	if !trustHasPurpose(trust, identitytrust.PurposeChannelIssue) {
		return fmt.Errorf("trust.principals requires at least one channel.issue purpose when privacy delivery is enabled")
	}
	if !cfg.Required {
		return nil
	}
	required := []struct {
		path  string
		value string
	}{
		{"privacy.replay_key_file", cfg.ReplayKeyFile},
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
	if cfg.Discovery.Reference == cfg.Data.Reference {
		return fmt.Errorf("privacy discovery and data references must be distinct")
	}
	sameStore, err := sameReplayStore(cfg.Discovery.ReplayPath, cfg.Data.ReplayPath)
	if err != nil {
		return fmt.Errorf("privacy replay path identity is unavailable: %w", err)
	}
	if sameStore {
		return fmt.Errorf("privacy discovery and data replay paths resolve to the same physical store")
	}
	return nil
}

func validateTrust(cfg TrustConfig) error {
	entries := make([]identitytrust.Entry, 0, len(cfg.Principals))
	for index, entry := range cfg.Principals {
		path := fmt.Sprintf("trust.principals[%d]", index)
		public, err := base64.StdEncoding.DecodeString(entry.PublicKey)
		if err != nil || len(public) != ed25519.PublicKeySize || base64.StdEncoding.EncodeToString(public) != entry.PublicKey {
			return fmt.Errorf("%s.public_key is invalid", path)
		}
		trusted := identitytrust.Entry{
			Principal: entry.Principal,
			PublicKey: ed25519.PublicKey(public),
			Purposes:  entry.Purposes,
		}
		if _, err := identitytrust.NewRegistry([]identitytrust.Entry{trusted}); err != nil {
			return fmt.Errorf("%s is invalid: %w", path, err)
		}
		entries = append(entries, trusted)
	}
	if _, err := identitytrust.NewRegistry(entries); err != nil {
		return fmt.Errorf("trust.principals is invalid: %w", err)
	}
	return nil
}

func trustHasPurpose(cfg TrustConfig, purpose identitytrust.Purpose) bool {
	for _, entry := range cfg.Principals {
		if slices.Contains(entry.Purposes, purpose) {
			return true
		}
	}
	return false
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
	persistentStore := nodeProfile == networkapi.NodeProfileServiceNode ||
		nodeProfile == networkapi.NodeProfileLocalDevelopment
	if doc.Network.Limits.StoreMaxMessages < 0 {
		return fmt.Errorf("network.limits.store_max_messages cannot be negative")
	}
	if doc.Network.Limits.StoreMaxAgeSeconds < 0 {
		return fmt.Errorf("network.limits.store_max_age_seconds cannot be negative")
	}
	if doc.Network.Limits.StoreMaxBytes < 0 {
		return fmt.Errorf("network.limits.store_max_bytes cannot be negative")
	}
	if persistentStore && doc.Network.Limits.StoreMaxMessages < 1 {
		return fmt.Errorf("network.limits.store_max_messages must be finite and positive for persistent Store profiles")
	}
	if persistentStore && doc.Network.Limits.StoreMaxAgeSeconds < 1 {
		return fmt.Errorf("network.limits.store_max_age_seconds must be finite and positive for persistent Store profiles")
	}
	if persistentStore && doc.Network.Limits.StoreMaxBytes < 1 {
		return fmt.Errorf("network.limits.store_max_bytes must be finite and positive for persistent Store profiles")
	}
	if doc.Network.Limits.StoreMaxBytes > 0 && doc.Network.Limits.StoreMaxBytes < 4<<20 {
		return fmt.Errorf("network.limits.store_max_bytes must be at least 4194304")
	}
	if doc.Network.Limits.StoreMaxMessages > 10_000_000 {
		return fmt.Errorf("network.limits.store_max_messages exceeds the supported bound")
	}
	if doc.Network.Limits.StoreMaxAgeSeconds > 10*365*24*60*60 {
		return fmt.Errorf("network.limits.store_max_age_seconds exceeds the supported bound")
	}
	if doc.Network.Limits.StoreMaxBytes > 1<<40 {
		return fmt.Errorf("network.limits.store_max_bytes exceeds the supported bound")
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
	disabled := false
	switch doc.Workloads.Executor {
	case "disabled":
		disabled = true
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
	deniedRequirements := workloadRequirementSet(doc.Policy.DeniedWorkloadRequirements)
	if err := validateWorkloadRequirements("policy.denied_workload_requirements", doc.Policy.DeniedWorkloadRequirements); err != nil {
		return err
	}
	for index, workload := range doc.Workloads.Initial {
		path := fmt.Sprintf("workloads.initial[%d]", index)
		if err := validateWorkload(path, workload); err != nil {
			return err
		}
		if err := validateWorkloadPolicy(path, workload, allowedPolicyRefs, deniedRequirements); err != nil {
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
	if disabled && len(doc.Workloads.Initial) > 0 {
		return fmt.Errorf("workloads.initial requires an enabled workload executor")
	}
	return nil
}

func validateWorkloadPolicy(path string, workload WorkloadSpec, allowed map[string]struct{}, denied map[workloadregistry.WorkloadRequirement]struct{}) error {
	if workload.PolicyRef != "" {
		if _, ok := allowed[strings.ToLower(strings.TrimSpace(workload.PolicyRef))]; !ok {
			return fmt.Errorf("%s.policy_ref is not allowed", path)
		}
	}
	for _, requirement := range workload.Requirements {
		if _, ok := denied[requirement]; ok {
			return fmt.Errorf("%s.requirements contains a policy-denied workload requirement", path)
		}
	}
	return nil
}

func workloadRequirementSet(values []workloadregistry.WorkloadRequirement) map[workloadregistry.WorkloadRequirement]struct{} {
	out := make(map[workloadregistry.WorkloadRequirement]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func validateWorkloadRequirements(path string, values []workloadregistry.WorkloadRequirement) error {
	if err := workloadregistry.ValidateWorkloadRequirements(values); err != nil {
		return fmt.Errorf("%s: %w", path, err)
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
	if err := validateWorkloadRequirements(path+".requirements", workload.Requirements); err != nil {
		return err
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
