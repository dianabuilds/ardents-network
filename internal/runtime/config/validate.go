package config

import (
	"fmt"
	"net"
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
