package main

import (
	"fmt"
	"time"

	networkapi "ardents/internal/network/api"
	runtimeconfig "ardents/internal/runtime/config"
	runtimeprocess "ardents/internal/runtime/process"
	connect "ardents/internal/transport/connectrpc"
	workloadcontroller "ardents/internal/workload/controller"
	"ardents/internal/workload/execution"
)

func runtimeConfigFromDocument(doc runtimeconfig.Document, token string) (runtimeConfig, error) {
	executor, err := operatorWorkloadExecutor(doc)
	if err != nil {
		return runtimeConfig{}, err
	}
	data, err := operatorDataConfig(doc.Data)
	if err != nil {
		return runtimeConfig{}, err
	}
	data.Dir = doc.Node.DataDir
	credential, err := operatorCredential(doc.API, token)
	if err != nil {
		return runtimeConfig{}, err
	}
	observabilityToken, err := operatorObservabilityToken(doc.Observability.TokenFile)
	if err != nil {
		return runtimeConfig{}, err
	}
	cfg := runtimeConfig{
		ListenAddr: doc.API.ListenAddress, ObservabilityAddr: doc.Observability.ListenAddress,
		ObservabilityToken: observabilityToken,
		APIToken:           token,
		APISubject:         credential.SubjectID, APICapabilities: credential.Capabilities, APICredentialEnd: credential.ExpiresAt,
		Node: operatorNodeConfig(doc, data, executor),
	}
	privacy, dataPrivacy, policyService, err := operatorPrivacyChannels(doc, cfg.Node.Policy)
	if err != nil {
		return runtimeConfig{}, err
	}
	cfg.Node.Privacy = privacy
	cfg.Node.DataPrivacy = dataPrivacy
	cfg.Node.PolicyService = policyService
	if err := validateLocalAPIListenAddr(cfg.ListenAddr); err != nil {
		return runtimeConfig{}, err
	}
	if err := runtimeprocess.ValidateConfig(cfg.Node); err != nil {
		return runtimeConfig{}, err
	}
	return cfg, nil
}

func operatorNodeConfig(doc runtimeconfig.Document, data runtimeprocess.DataConfig, executor workloadcontroller.Executor) runtimeprocess.Config {
	return runtimeprocess.Config{
		Name: doc.Node.Name, NodeProfile: networkapi.NodeProfile(doc.Node.Profile),
		Boot:  runtimeprocess.BootConfig{Sources: cloneStrings(doc.Network.BootstrapPeers)},
		Trust: runtimeprocess.TrustConfig{Anchors: cloneStrings(doc.Network.TrustAnchors)},
		Data:  data, Transport: operatorTransportConfig(doc.Network),
		Policy: operatorPolicyConfig(doc), Service: operatorServiceConfigs(doc.Services),
		Workload:                 operatorWorkloadConfigs(doc.Workloads.Initial),
		DiscoveryRefreshInterval: time.Duration(doc.Network.DiscoveryRefreshSeconds) * time.Second,
		WorkloadExecutor:         executor,
	}
}

func operatorCredential(api runtimeconfig.APIConfig, token string) (connect.AuthConfig, error) {
	expiresAt, err := parseCredentialExpiry(api.CredentialExpiresAt)
	if err != nil {
		return connect.AuthConfig{}, err
	}
	capabilities := cloneStrings(api.Capabilities)
	if len(capabilities) == 0 {
		capabilities = localAdminCapabilities()
	}
	credential := connect.AuthConfig{
		Token: token, SubjectID: api.OperatorSubject, Capabilities: capabilities, ExpiresAt: expiresAt,
	}
	if err := credential.Validate(); err != nil {
		return connect.AuthConfig{}, err
	}
	return credential, nil
}

func parseCredentialExpiry(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("api.credential_expires_at: %w", err)
	}
	return value, nil
}

func operatorWorkloadExecutor(doc runtimeconfig.Document) (workloadcontroller.Executor, error) {
	if doc.Workloads.Executor == "trusted-process" {
		return workloadcontroller.NewLocalExecutor(), nil
	}
	return workloadcontroller.NewDockerExecutorWithConfig(execution.DockerExecutorConfig{
		NodeID: doc.Node.Name, AllowedRegistries: cloneStrings(doc.Workloads.AllowedRegistries),
		AllowedPolicyRefs: cloneStrings(operatorAllowedPolicyRefs(doc)),
		TrustedRuntime:    doc.Workloads.TrustedRuntime, UntrustedRuntime: doc.Workloads.UntrustedRuntime,
		AllowedIngressHosts: cloneStrings(doc.Workloads.AllowedIngressHosts),
		IngressBindAddress:  doc.Workloads.IngressBindAddress, IngressProxyImage: doc.Workloads.IngressProxyImage,
	})
}

func operatorAllowedPolicyRefs(doc runtimeconfig.Document) []string {
	if len(doc.Policy.AllowedPolicyRefs) > 0 {
		return doc.Policy.AllowedPolicyRefs
	}
	return doc.Workloads.AllowedPolicyRefs
}

func operatorDataConfig(in runtimeconfig.DataConfig) (runtimeprocess.DataConfig, error) {
	local, err := parseOptionalDuration("data.default_local_retention", in.DefaultLocalRetention)
	if err != nil {
		return runtimeprocess.DataConfig{}, err
	}
	relay, err := parseOptionalDuration("data.default_relay_retention", in.DefaultRelayRetention)
	if err != nil {
		return runtimeprocess.DataConfig{}, err
	}
	return runtimeprocess.DataConfig{
		DefaultLocalRetentionTTL: local, DefaultRelayRetentionTTL: relay,
		MaxRelayRetentionBytes: in.MaxRelayBytes, MaxReplicaRetentionBytes: in.MaxReplicaBytes,
		MaxLocalStorageBytes:   in.MaxLocalBytes,
		DefaultDesiredReplicas: in.DesiredReplicas,
		DefaultMinimumReplicas: in.MinimumReplicas,
	}, nil
}

func parseOptionalDuration(path, raw string) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	return value, nil
}

func cloneStrings(in []string) []string {
	return append([]string(nil), in...)
}
