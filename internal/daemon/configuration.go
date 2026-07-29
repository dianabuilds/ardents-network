package daemon

import (
	"ardents/internal/channeldelivery"
	runtimeconfig "ardents/internal/config"
	"ardents/internal/diagnostics"
	"ardents/internal/discovery"
	"ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
	identitykeyring "ardents/internal/identity/keyring"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"
	networkprivacy "ardents/internal/messaging"
	networkapi "ardents/internal/network"
	networkwaku "ardents/internal/network/waku"
	apppolicy "ardents/internal/policy"
	workloaddocker "ardents/internal/workload/docker"
	workloadcontroller "ardents/internal/workload/execution"
	workloadregistry "ardents/internal/workload/registry"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"
)

type runtimeConfig struct {
	SocketPath            string
	ObservabilityAddr     string
	ObservabilityToken    string
	ApplicationEnabled    bool
	ApplicationSocketPath string
	Authority             runtimeconfig.AuthorityConfig
	ChannelDelivery       *channeldelivery.Service
	Node                  Config
}

func loadRuntimeConfig() (runtimeConfig, error) {
	path := runtimeconfig.OperatorFile()
	if path == "" {
		return runtimeConfig{}, fmt.Errorf("ARDENTS_CONFIG_FILE is required")
	}
	return loadOperatorRuntimeConfig(path)
}

func ValidateConfig(cfg Config) error {
	nodeProfile := networkapi.NormalizeNodeProfile(cfg.NodeProfile)
	transportProfile := networkapi.NormalizeProfile(cfg.Transport.Profile)
	if err := networkapi.ValidateNodeProfileTransport(nodeProfile, transportProfile); err != nil {
		return fmt.Errorf("network participation configuration: %w", err)
	}
	if nodeProfile == networkapi.NodeProfileLocalDevelopment && !isLoopbackBind(networkapi.ResolveBindAddress(cfg.Transport.BindAddress)) {
		return fmt.Errorf("network participation configuration: node profile %q requires a loopback bind address", nodeProfile)
	}
	if nodeProfile == networkapi.NodeProfileLocalDevelopment && len(cfg.Transport.DNSDiscoveryURLs) > 0 {
		return fmt.Errorf("network participation configuration: node profile %q does not allow network DNS discovery", nodeProfile)
	}
	mode, err := validateParticipationReachability(cfg, nodeProfile)
	if err != nil {
		return err
	}
	transportConfig := networkapi.Config{
		NodeProfile:            nodeProfile,
		Profile:                cfg.Transport.Profile,
		WSSPort:                cfg.Transport.WSSPort,
		WSSCertPath:            cfg.Transport.WSSCertPath,
		WSSKeyPath:             cfg.Transport.WSSKeyPath,
		WSSCAPath:              cfg.Transport.WSSCAPath,
		WSSAdvertiseAddress:    cfg.Transport.WSSAdvertiseAddress,
		DNSDiscoveryURLs:       append([]string(nil), cfg.Transport.DNSDiscoveryURLs...),
		DNSDiscoveryNameServer: cfg.Transport.DNSDiscoveryNameServer,
		ReachabilityMode:       mode,
		AdvertiseAddresses:     append([]string(nil), cfg.Transport.AdvertiseAddresses...),
		Limits:                 cfg.Transport.Limits,
	}
	if err := networkwaku.ValidateConfig(transportConfig, time.Now()); err != nil {
		return fmt.Errorf("network participation configuration: %w", err)
	}
	return nil
}

func validateParticipationReachability(cfg Config, nodeProfile networkapi.NodeProfile) (networkapi.ReachabilityMode, error) {
	mode := networkapi.ReachabilityModeForProfile(cfg.Transport.ReachabilityMode, nodeProfile)
	if nodeProfile == networkapi.NodeProfileLocalDevelopment && mode != networkapi.ReachabilityLocalOnly {
		return mode, fmt.Errorf("network participation configuration: node profile %q requires reachability mode %q", nodeProfile, networkapi.ReachabilityLocalOnly)
	}
	if nodeProfile == networkapi.NodeProfileConstrainedClient && mode != networkapi.ReachabilityOutboundOnly {
		return mode, fmt.Errorf("network participation configuration: node profile %q requires reachability mode %q", nodeProfile, networkapi.ReachabilityOutboundOnly)
	}
	if nodeProfile == networkapi.NodeProfileServiceNode && mode == networkapi.ReachabilityLocalOnly {
		return mode, fmt.Errorf("network participation configuration: node profile %q does not allow reachability mode %q", nodeProfile, mode)
	}
	if mode == networkapi.ReachabilityPublicDirect && isLoopbackBind(networkapi.ResolveBindAddress(cfg.Transport.BindAddress)) {
		return mode, fmt.Errorf("network participation configuration: public direct reachability requires a non-loopback bind address")
	}
	return mode, nil
}

func isLoopbackBind(address string) bool {
	if strings.EqualFold(strings.TrimSpace(address), "localhost") {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(address))
	return ip != nil && ip.IsLoopback()
}

type Config struct {
	Name                     string
	ImageReference           string
	NodeProfile              networkapi.NodeProfile
	Boot                     BootConfig
	Data                     DataConfig
	Trust                    TrustConfig
	Transport                TransportConfig
	Policy                   PolicyConfig
	Service                  []ServiceConfig
	Workload                 []WorkloadConfig
	DiscoveryRefreshInterval time.Duration
	Privacy                  *networkprivacy.Channel
	DataPrivacy              *networkprivacy.Channel
	WorkloadExecutor         workloadcontroller.Executor
	OperatorConfig           *runtimeconfig.Manager
	PolicyService            *apppolicy.Service
}

type BootConfig struct {
	Sources []string
}

type TrustConfig struct {
	Registry *identitytrust.Registry
}

type DataConfig struct {
	Dir                      string
	DefaultLocalRetentionTTL time.Duration
	DefaultRelayRetentionTTL time.Duration
	MaxRelayRetentionBytes   int64
	MaxReplicaRetentionBytes int64
	MaxLocalStorageBytes     int64
	DefaultDesiredReplicas   int
	DefaultMinimumReplicas   int
}

type TransportConfig struct {
	NodeProfile            networkapi.NodeProfile
	StorePath              string
	PrivateKeyPath         string
	BindAddress            string
	ListenPort             int
	Profile                networkapi.Profile
	WSSPort                int
	WSSCertPath            string
	WSSKeyPath             string
	WSSCAPath              string
	WSSAdvertiseAddress    string
	DNSDiscoveryURLs       []string
	DNSDiscoveryNameServer string
	ReachabilityMode       networkapi.ReachabilityMode
	AdvertiseAddresses     []string
	Limits                 networkapi.Limits
}

type PolicyConfig struct {
	MaxWorkloads                    int
	AllowedPolicyRefs               []string
	DeniedWorkloadRequirements      []workloadregistry.WorkloadRequirement
	DisableServicePublication       bool
	DisableNetworkPublishedServices bool
	DeniedServiceTypes              []string
	DisableUntrustedRouteUse        bool
	DeniedRouteSchemes              []string
	DisablePrivateChannelGrantUse   bool
	DisableRealmAuthorityCreation   bool
	DisableRealmAuthorityRecovery   bool
	DisableRealmChannelDelivery     bool
	DisableRealmChannelRotation     bool
	DisableRealmChannelMembership   bool
	DeniedChannelGrantScopes        []string
	DisableLocalBlobRetention       bool
	DisableRelayBlobRetention       bool
	DisableBlobPinning              bool
	DisablePeerBlobReserving        bool
	AllowPinRelayRetainedBlobs      bool
	AllowReservingRelayBlobs        bool
	MaxLocalRetentionTTL            time.Duration
	MaxRelayRetentionTTL            time.Duration
}

type ServiceConfig struct {
	ID             string
	Type           string
	Owner          string
	Mode           string
	Endpoints      []string
	ProbeEndpoints []string
}

type WorkloadConfig struct {
	ID            string
	Kind          string
	Owner         string
	Config        string
	Desired       string
	Requirements  []workloadregistry.WorkloadRequirement
	PolicyRef     string
	RestartPolicy string
	Services      []ServiceConfig
}

func defaultNodeName(name string) string {
	if name == "" {
		return "ardents"
	}
	return name
}

func defaultDataDir(name, dir string) string {
	if dir == "" {
		return filepath.Join("var", name)
	}
	return dir
}

func policyConfigFromOperator(in runtimeconfig.PolicyConfig) PolicyConfig {
	return PolicyConfig{
		MaxWorkloads: in.MaxWorkloads, AllowedPolicyRefs: cloneStrings(in.AllowedPolicyRefs),
		DeniedWorkloadRequirements:      append([]workloadregistry.WorkloadRequirement(nil), in.DeniedWorkloadRequirements...),
		DisableServicePublication:       in.DisableServicePublication,
		DisableNetworkPublishedServices: in.DisableNetworkPublishedServices,
		DeniedServiceTypes:              cloneStrings(in.DeniedServiceTypes),
		DisableUntrustedRouteUse:        in.DisableUntrustedRouteUse,
		DeniedRouteSchemes:              cloneStrings(in.DeniedRouteSchemes),
		DisablePrivateChannelGrantUse:   in.DisablePrivateChannelGrantUse,
		DisableRealmAuthorityCreation:   in.DisableRealmAuthorityCreation,
		DisableRealmAuthorityRecovery:   in.DisableRealmAuthorityRecovery,
		DisableRealmChannelDelivery:     in.DisableRealmChannelDelivery,
		DisableRealmChannelRotation:     in.DisableRealmChannelRotation,
		DisableRealmChannelMembership:   in.DisableRealmChannelMembership,
		DeniedChannelGrantScopes:        cloneStrings(in.DeniedChannelGrantScopes),
		DisableLocalBlobRetention:       in.DisableLocalBlobRetention,
		DisableRelayBlobRetention:       in.DisableRelayBlobRetention,
		DisableBlobPinning:              in.DisableBlobPinning,
		DisablePeerBlobReserving:        in.DisablePeerBlobReserving,
		AllowPinRelayRetainedBlobs:      in.AllowPinRelayRetainedBlobs,
		AllowReservingRelayBlobs:        in.AllowReservingRelayBlobs,
		MaxLocalRetentionTTL:            parseOperatorDuration(in.MaxLocalRetention),
		MaxRelayRetentionTTL:            parseOperatorDuration(in.MaxRelayRetention),
	}
}

func parseOperatorDuration(raw string) time.Duration {
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0
	}
	return value
}

func (n *Node) initOperatorConfig() {
	if n.cfg.OperatorConfig == nil {
		return
	}
	if err := n.cfg.OperatorConfig.RegisterTrustChangeClassifier(discoveryOnlyTrustChange); err != nil {
		panic(fmt.Sprintf("register trust change classifier: %v", err))
	}
	if err := n.cfg.OperatorConfig.RegisterApplier(&nodeConfigApplier{node: n}); err != nil {
		panic(fmt.Sprintf("register node configuration applier: %v", err))
	}
}

func discoveryOnlyTrustChange(active, candidate runtimeconfig.TrustConfig) bool {
	if reflect.DeepEqual(active, candidate) {
		return false
	}
	return reflect.DeepEqual(nonDiscoveryTrust(active), nonDiscoveryTrust(candidate))
}

func nonDiscoveryTrust(config runtimeconfig.TrustConfig) runtimeconfig.TrustConfig {
	projected := runtimeconfig.TrustConfig{
		Principals: make([]runtimeconfig.TrustedPrincipalConfig, 0, len(config.Principals)),
	}
	for _, principal := range config.Principals {
		entry := runtimeconfig.TrustedPrincipalConfig{
			Principal: principal.Principal,
			PublicKey: principal.PublicKey,
		}
		for _, purpose := range principal.Purposes {
			if purpose != identitytrust.PurposeDiscoveryPublish {
				entry.Purposes = append(entry.Purposes, purpose)
			}
		}
		if len(entry.Purposes) > 0 {
			projected.Principals = append(projected.Principals, entry)
		}
	}
	return projected
}

func (n *Node) GetEffectiveConfig() runtimeconfig.EffectiveSnapshot {
	if n.cfg.OperatorConfig == nil {
		return runtimeconfig.EffectiveSnapshot{}
	}
	return n.cfg.OperatorConfig.Snapshot()
}

func (n *Node) ReloadConfig(ctx context.Context) runtimeconfig.ReloadResult {
	if n.cfg.OperatorConfig == nil {
		return runtimeconfig.ReloadResult{
			Outcome: runtimeconfig.OutcomeRejectedInvalid,
			Reason:  "operator configuration source is unavailable",
		}
	}
	result := n.cfg.OperatorConfig.Reload(ctx)
	n.mu.Lock()
	if result.Outcome == runtimeconfig.OutcomeRollbackFailed {
		n.recordConfigRollbackFailureLocked(result.Reason)
	}
	n.publishLocked("config.reload", map[string]any{
		"outcome": string(result.Outcome), "active_generation": result.ActiveGeneration,
		"candidate_generation": result.CandidateGeneration,
	})
	n.mu.Unlock()
	return result
}

func (n *Node) recordConfigRollbackFailureLocked(detail string) {
	reason := &diagnostics.Reason{
		Code: "config.reload.rollback_failed", Domain: "configuration",
		Summary: "operator configuration rollback failed", Detail: detail,
		Impact:                 "runtime owners may have mixed effective configuration",
		Recovery:               "restart node from the last validated configuration",
		OperatorActionRequired: true, Resource: n.cfg.Name,
	}
	n.diag.SetSubsystem("configuration", diagnostics.HealthDegraded, reason)
	if n.life.State() == diagnostics.Ready || n.life.State() == diagnostics.Initializing {
		if err := n.life.Move(diagnostics.Degraded); err != nil {
			n.diag.RecordEvent("configuration", "lifecycle_transition_failed", n.cfg.Name, err.Error(), "config.reload.lifecycle_failed", nil)
		}
	}
	n.diag.RecordEvent("configuration", "rollback_failed", n.cfg.Name, reason.Summary, reason.Code, nil)
}

type nodeConfigApplier struct {
	node                 *Node
	trustTransaction     *discovery.TrustTransaction
	discoveryTrustChange bool
}

func (a *nodeConfigApplier) Prepare(
	_ context.Context,
	previous runtimeconfig.Document,
	next runtimeconfig.Document,
) error {
	a.trustTransaction = nil
	a.discoveryTrustChange = !reflect.DeepEqual(previous.Trust, next.Trust)
	return nil
}

func (a *nodeConfigApplier) Apply(
	_ context.Context,
	_ runtimeconfig.Document,
	next runtimeconfig.Document,
) error {
	configuredTrust, effectiveTrust, err := a.node.operatorDiscoveryTrust(next)
	if err != nil {
		return err
	}
	a.node.mu.Lock()
	if a.discoveryTrustChange {
		a.trustTransaction, err = a.node.disco.BeginTrustRegistry(effectiveTrust)
		if err != nil {
			a.node.mu.Unlock()
			return err
		}
	}
	a.node.commitOperatorDocumentLocked(next, configuredTrust)
	a.node.mu.Unlock()
	return nil
}

func (a *nodeConfigApplier) Rollback(_ context.Context, previous runtimeconfig.Document) error {
	configuredTrust, effectiveTrust, err := a.node.operatorDiscoveryTrust(previous)
	if err != nil {
		return err
	}
	if a.trustTransaction != nil {
		if err := a.trustTransaction.Rollback(effectiveTrust); err != nil {
			return fmt.Errorf("restore discovery trust and truth: %w", err)
		}
	}
	a.node.mu.Lock()
	defer a.node.mu.Unlock()
	a.node.commitOperatorDocumentLocked(previous, configuredTrust)
	a.trustTransaction = nil
	return nil
}

func (a *nodeConfigApplier) Commit(context.Context) {
	if a.trustTransaction != nil {
		a.trustTransaction.Commit()
	}
	a.trustTransaction = nil
}

func (n *Node) operatorDiscoveryTrust(
	doc runtimeconfig.Document,
) (*identitytrust.Registry, *identitytrust.Registry, error) {
	configuredTrust, err := operatorTrustRegistry(doc.Trust)
	if err != nil {
		return nil, nil, err
	}
	local := n.ident.NodeSummary()
	if local.Principal == "" {
		return configuredTrust, configuredTrust, nil
	}
	effectiveTrust, err := trustRegistryWithLocalPrincipal(
		configuredTrust,
		local.Principal,
		local.PublicKey,
	)
	if err != nil {
		return nil, nil, err
	}
	return configuredTrust, effectiveTrust, nil
}

func (n *Node) commitOperatorDocumentLocked(
	doc runtimeconfig.Document,
	configuredTrust *identitytrust.Registry,
) {
	n.cfg.Trust.Registry = configuredTrust
	if n.runtimeMgr != nil {
		n.runtimeMgr.configuredTrust = configuredTrust
	}
	policy := policyConfigFromOperator(doc.Policy)
	n.cfg.Policy = policy
	n.policyLive.Reconfigure(runtimePolicyConfig(policy))
	n.cfg.DiscoveryRefreshInterval = time.Duration(doc.Network.DiscoveryRefreshSeconds) * time.Second
	n.diag.SetMaxEvents(doc.Diagnostics.MaxEvents)
	n.diag.SetDetailLevel(doc.Diagnostics.DetailLevel)
	n.replaceDiscoveryRefreshLocked(n.refreshDiscoveryPublication)
}

func loadOperatorRuntimeConfig(path string) (runtimeConfig, error) {
	doc, err := runtimeconfig.Load(path)
	if err != nil {
		return runtimeConfig{}, err
	}
	cfg, err := runtimeConfigFromDocument(normalizeOperatorDocumentPaths(doc))
	if err != nil {
		return runtimeConfig{}, err
	}
	manager, err := newOperatorConfigManager(path, doc)
	if err != nil {
		return runtimeConfig{}, err
	}
	cfg.Node.OperatorConfig = manager
	return cfg, nil
}

func newOperatorConfigManager(path string, doc runtimeconfig.Document) (*runtimeconfig.Manager, error) {
	manager, err := runtimeconfig.NewManager(path, doc, configureOperatorLogging(doc.Logging))
	if err != nil {
		return nil, err
	}
	if err := registerOperatorCandidateValidator(manager); err != nil {
		return nil, err
	}
	return manager, nil
}

func registerOperatorCandidateValidator(manager *runtimeconfig.Manager) error {
	return manager.RegisterValidator(func(candidate runtimeconfig.Document) error {
		_, err := runtimeConfigFromDocument(normalizeOperatorDocumentPaths(candidate))
		return err
	})
}

func runtimeConfigFromDocument(doc runtimeconfig.Document) (runtimeConfig, error) {
	executor, err := operatorWorkloadExecutor(doc)
	if err != nil {
		return runtimeConfig{}, err
	}
	data, err := operatorDataConfig(doc.Data)
	if err != nil {
		return runtimeConfig{}, err
	}
	data.Dir = doc.Node.DataDir
	observabilityToken, err := runtimeconfig.ObservabilityToken(doc.Observability.TokenFile)
	if err != nil {
		return runtimeConfig{}, err
	}
	trustedPrincipals, err := operatorTrustRegistry(doc.Trust)
	if err != nil {
		return runtimeConfig{}, err
	}
	cfg := runtimeConfig{
		SocketPath: doc.API.SocketPath, ObservabilityAddr: doc.Observability.ListenAddress,
		ObservabilityToken:    observabilityToken,
		ApplicationEnabled:    doc.ApplicationInterface.Enabled,
		ApplicationSocketPath: doc.ApplicationInterface.SocketPath,
		Authority:             doc.Authority,
		Node:                  operatorNodeConfig(doc, data, executor, trustedPrincipals),
	}
	privacy, dataPrivacy, policyService, channelDelivery, err := operatorPrivacyChannels(
		doc, cfg.Node.Policy, trustedPrincipals,
	)
	if err != nil {
		return runtimeConfig{}, err
	}
	cfg.Node.Privacy = privacy
	cfg.Node.DataPrivacy = dataPrivacy
	cfg.Node.PolicyService = policyService
	cfg.ChannelDelivery = channelDelivery
	if err := ValidateConfig(cfg.Node); err != nil {
		return runtimeConfig{}, err
	}
	return cfg, nil
}

func normalizeOperatorDocumentPaths(doc runtimeconfig.Document) runtimeconfig.Document {
	doc.Node.DataDir = filepath.Clean(doc.Node.DataDir)
	if doc.Network.StorePath != "" {
		doc.Network.StorePath = filepath.Clean(doc.Network.StorePath)
	}
	return doc
}

func operatorNodeConfig(doc runtimeconfig.Document, data DataConfig, executor workloadcontroller.Executor, trustedPrincipals *identitytrust.Registry) Config {
	return Config{
		Name: doc.Node.Name, ImageReference: doc.Node.ImageReference, NodeProfile: networkapi.NodeProfile(doc.Node.Profile),
		Boot:  BootConfig{Sources: cloneStrings(doc.Network.BootstrapPeers)},
		Trust: TrustConfig{Registry: trustedPrincipals},
		Data:  data, Transport: operatorTransportConfig(doc.Network),
		Policy: operatorPolicyConfig(doc), Service: operatorServiceConfigs(doc.Services),
		Workload:                 operatorWorkloadConfigs(doc.Workloads.Initial),
		DiscoveryRefreshInterval: time.Duration(doc.Network.DiscoveryRefreshSeconds) * time.Second,
		WorkloadExecutor:         executor,
	}
}

func operatorWorkloadExecutor(doc runtimeconfig.Document) (workloadcontroller.Executor, error) {
	switch doc.Workloads.Executor {
	case "disabled":
		return workloadcontroller.NewDisabledExecutor(), nil
	case "trusted-process":
		return workloadcontroller.NewLocalExecutor(), nil
	case "docker":
		return workloaddocker.NewExecutor(workloaddocker.ExecutorConfig{
			NodeID: doc.Node.Name, AllowedRegistries: cloneStrings(doc.Workloads.AllowedRegistries),
			AllowedPolicyRefs: cloneStrings(operatorAllowedPolicyRefs(doc)),
			TrustedRuntime:    doc.Workloads.TrustedRuntime, UntrustedRuntime: doc.Workloads.UntrustedRuntime,
			AllowedIngressHosts: cloneStrings(doc.Workloads.AllowedIngressHosts),
			IngressBindAddress:  doc.Workloads.IngressBindAddress, IngressProxyImage: doc.Workloads.IngressProxyImage,
		})
	default:
		return nil, fmt.Errorf("unsupported workload executor %q", doc.Workloads.Executor)
	}
}

func operatorAllowedPolicyRefs(doc runtimeconfig.Document) []string {
	if len(doc.Policy.AllowedPolicyRefs) > 0 {
		return doc.Policy.AllowedPolicyRefs
	}
	return doc.Workloads.AllowedPolicyRefs
}

func operatorDataConfig(in runtimeconfig.DataConfig) (DataConfig, error) {
	local, err := parseOptionalDuration("data.default_local_retention", in.DefaultLocalRetention)
	if err != nil {
		return DataConfig{}, err
	}
	relay, err := parseOptionalDuration("data.default_relay_retention", in.DefaultRelayRetention)
	if err != nil {
		return DataConfig{}, err
	}
	return DataConfig{
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

func operatorTransportConfig(in runtimeconfig.NetworkConfig) TransportConfig {
	return TransportConfig{
		StorePath: in.StorePath, PrivateKeyPath: in.PrivateKeyPath, BindAddress: in.BindAddress,
		ListenPort: in.ListenPort, Profile: networkapi.Profile(in.TransportProfile),
		WSSPort: in.WSS.Port, WSSCertPath: in.WSS.CertificateFile,
		WSSKeyPath: in.WSS.PrivateKeyFile, WSSCAPath: in.WSS.CAFile,
		WSSAdvertiseAddress:    in.WSS.AdvertiseAddress,
		DNSDiscoveryURLs:       cloneStrings(in.DNSDiscoveryURLs),
		DNSDiscoveryNameServer: in.DNSDiscoveryNameServer,
		ReachabilityMode:       networkapi.ReachabilityMode(in.ReachabilityMode),
		AdvertiseAddresses:     cloneStrings(in.AdvertiseAddresses),
		Limits: networkapi.Limits{
			MaxMessageBytes: in.Limits.MaxMessageBytes, MaxPeerConnections: in.Limits.MaxPeerConnections,
			MaxConnectionsPerIP:     in.Limits.MaxConnectionsPerIP,
			MaxConcurrentOperations: in.Limits.MaxConcurrentOperations,
			OperationRate:           in.Limits.OperationRate, OperationBurst: in.Limits.OperationBurst,
			MaxFilterSubscribers: in.Limits.MaxFilterSubscribers,
			MaxStoreResults:      in.Limits.MaxStoreResults,
			StoreMaxMessages:     in.Limits.StoreMaxMessages,
			StoreMaxAgeSeconds:   in.Limits.StoreMaxAgeSeconds,
			StoreMaxBytes:        in.Limits.StoreMaxBytes,
		},
	}
}

func operatorPolicyConfig(doc runtimeconfig.Document) PolicyConfig {
	in := doc.Policy
	allowed := in.AllowedPolicyRefs
	if len(allowed) == 0 {
		allowed = doc.Workloads.AllowedPolicyRefs
	}
	return PolicyConfig{
		MaxWorkloads: in.MaxWorkloads, AllowedPolicyRefs: cloneStrings(allowed),
		DeniedWorkloadRequirements:      append([]workloadregistry.WorkloadRequirement(nil), in.DeniedWorkloadRequirements...),
		DisableServicePublication:       in.DisableServicePublication,
		DisableNetworkPublishedServices: in.DisableNetworkPublishedServices,
		DeniedServiceTypes:              cloneStrings(in.DeniedServiceTypes),
		DisableUntrustedRouteUse:        in.DisableUntrustedRouteUse,
		DeniedRouteSchemes:              cloneStrings(in.DeniedRouteSchemes),
		DisablePrivateChannelGrantUse:   in.DisablePrivateChannelGrantUse,
		DisableRealmAuthorityCreation:   in.DisableRealmAuthorityCreation,
		DisableRealmAuthorityRecovery:   in.DisableRealmAuthorityRecovery,
		DisableRealmChannelDelivery:     in.DisableRealmChannelDelivery,
		DisableRealmChannelRotation:     in.DisableRealmChannelRotation,
		DisableRealmChannelMembership:   in.DisableRealmChannelMembership,
		DeniedChannelGrantScopes:        cloneStrings(in.DeniedChannelGrantScopes),
		DisableLocalBlobRetention:       in.DisableLocalBlobRetention,
		DisableRelayBlobRetention:       in.DisableRelayBlobRetention,
		DisableBlobPinning:              in.DisableBlobPinning,
		DisablePeerBlobReserving:        in.DisablePeerBlobReserving,
		AllowPinRelayRetainedBlobs:      in.AllowPinRelayRetainedBlobs,
		AllowReservingRelayBlobs:        in.AllowReservingRelayBlobs,
		MaxLocalRetentionTTL:            durationOrZero(in.MaxLocalRetention),
		MaxRelayRetentionTTL:            durationOrZero(in.MaxRelayRetention),
	}
}

func durationOrZero(raw string) time.Duration {
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0
	}
	return value
}

func operatorServiceConfigs(in []runtimeconfig.ServiceConfig) []ServiceConfig {
	out := make([]ServiceConfig, 0, len(in))
	for _, item := range in {
		out = append(out, ServiceConfig{
			ID: item.ID, Type: item.Type, Owner: item.Owner, Mode: item.Mode,
			Endpoints: cloneStrings(item.Endpoints), ProbeEndpoints: cloneStrings(item.ProbeEndpoints),
		})
	}
	return out
}

func operatorWorkloadConfigs(in []runtimeconfig.WorkloadSpec) []WorkloadConfig {
	out := make([]WorkloadConfig, 0, len(in))
	for _, item := range in {
		out = append(out, WorkloadConfig{
			ID: item.ID, Kind: item.Kind, Owner: item.Owner, Config: item.Config,
			Desired: item.Desired, Requirements: append([]workloadregistry.WorkloadRequirement(nil), item.Requirements...),
			PolicyRef: item.PolicyRef, RestartPolicy: item.RestartPolicy,
			Services: operatorServiceConfigs(item.Services),
		})
	}
	return out
}

type loggingApplier struct {
	level *slog.LevelVar
}

func configureOperatorLogging(cfg runtimeconfig.LoggingConfig) *loggingApplier {
	level := &slog.LevelVar{}
	level.Set(operatorLogLevel(cfg.Level))
	options := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.Format == "text" {
		handler = slog.NewTextHandler(os.Stderr, options)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, options)
	}
	slog.SetDefault(slog.New(handler))
	return &loggingApplier{level: level}
}

func (*loggingApplier) Prepare(context.Context, runtimeconfig.Document, runtimeconfig.Document) error {
	return nil
}

func (a *loggingApplier) Apply(_ context.Context, _ runtimeconfig.Document, next runtimeconfig.Document) error {
	a.level.Set(operatorLogLevel(next.Logging.Level))
	return nil
}

func (a *loggingApplier) Rollback(_ context.Context, previous runtimeconfig.Document) error {
	a.level.Set(operatorLogLevel(previous.Logging.Level))
	return nil
}

func operatorLogLevel(value string) slog.Level {
	switch value {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func operatorPrivacyChannels(
	doc runtimeconfig.Document,
	policyConfig PolicyConfig,
	trustedPrincipals *identitytrust.Registry,
) (
	*networkprivacy.Channel,
	*networkprivacy.Channel,
	*apppolicy.Service,
	*channeldelivery.Service,
	error,
) {
	policyService := apppolicy.New(runtimePolicyConfig(policyConfig))
	if !doc.Privacy.Required && !doc.Privacy.DeliveryEnabled {
		return nil, nil, policyService, nil, nil
	}
	private, storeKey, err := operatorDeliveryInputs(doc)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	Workloads, err := identitycapability.NewService(
		doc.Privacy.ChannelGrantStore, storeKey, doc.Privacy.Subject, trustedPrincipals, policyService, time.Now,
	)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("protected privacy channel grant store is unavailable or invalid")
	}
	delivery, err := channeldelivery.New(Workloads, private, doc.Privacy.Subject, time.Now)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("configure member channel delivery: %w", err)
	}
	if !doc.Privacy.Required {
		return nil, nil, policyService, delivery, nil
	}
	replayKey, err := readProtectedKey(doc.Privacy.ReplayKeyFile, "privacy replay key file")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	discovery, err := buildOperatorPrivacyChannel(Workloads, private, replayKey, doc.Privacy.Subject,
		doc.Privacy.Discovery, identity.CapabilityRealmDiscovery)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("configure discovery privacy channel: %w", err)
	}
	data, err := buildOperatorPrivacyChannel(Workloads, private, replayKey, doc.Privacy.Subject,
		doc.Privacy.Data, identity.CapabilityDataExchange)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("configure data privacy channel: %w", err)
	}
	return discovery, data, policyService, delivery, nil
}

func operatorDeliveryInputs(doc runtimeconfig.Document) (ed25519.PrivateKey, []byte, error) {
	private, err := operatorIdentityPrivate(doc.Node.DataDir, doc.Privacy.Subject)
	if err != nil {
		return nil, nil, err
	}
	storeKey, err := readProtectedKey(doc.Privacy.ChannelGrantStoreKeyFile, "privacy channel-grant-store key file")
	if err != nil {
		return nil, nil, err
	}
	return private, storeKey, nil
}

func buildOperatorPrivacyChannel(
	Workloads *identitycapability.Service,
	private ed25519.PrivateKey,
	replayKey []byte,
	subject string,
	cfg runtimeconfig.PrivacyChannelConfig,
	scope identity.CapabilityScope,
) (*networkprivacy.Channel, error) {
	ref := identity.CapabilityRef(cfg.Reference)
	now := time.Now().UTC().Truncate(time.Second)
	for _, permission := range []identity.CapabilityPermission{
		identity.CapabilityPublish, identity.CapabilitySubscribe, identity.CapabilityStoreFetch,
	} {
		if _, err := Workloads.ResolveCapability(identity.CapabilityUse{
			Ref: ref, Subject: subject, Permission: permission, Scope: scope, At: now,
		}); err != nil {
			return nil, fmt.Errorf("required capability is unavailable: %w", err)
		}
	}
	replay, err := networkprivacy.NewDurableReplayLedger(cfg.ReplayPath, replayKey, 4096, 16384)
	if err != nil {
		return nil, fmt.Errorf("durable privacy replay ledger is unavailable or invalid")
	}
	return networkprivacy.NewChannel(networkprivacy.ChannelConfig{
		Resolver: Workloads, Authorizer: Workloads, Replay: replay,
		Reference: ref, Subject: subject, Scope: scope,
		Signer: func() ed25519.PrivateKey { return private }, Clock: time.Now,
	})
}

func readProtectedKey(path, label string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("%s is unavailable", label)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular file", label)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%s permissions must not allow group or other access", label)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s cannot be read", label)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(decoded) != 32 {
		return nil, fmt.Errorf("%s must contain one base64-encoded 32-byte key", label)
	}
	return decoded, nil
}

func operatorIdentityPrivate(dataDir, subject string) (ed25519.PrivateKey, error) {
	encoded, err := identitykeyring.NewKeyStoreInDir(dataDir).Load()
	if err != nil || encoded == "" {
		return nil, fmt.Errorf("privacy requires an existing protected node identity")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("protected node identity is invalid")
	}
	private := ed25519.PrivateKey(raw)
	principalID, err := identityprincipal.FromEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("derive operator Principal")
	}
	principal := principalID.String()
	if principal != subject {
		return nil, fmt.Errorf("privacy.subject does not match the protected node identity")
	}
	state := identity.NewStoreInDir(dataDir)
	if err := state.Load(); err != nil {
		return nil, fmt.Errorf("privacy requires readable canonical node identity state")
	}
	statePrincipal, encodedPublic := state.LoadIdentity()
	public, decodeErr := base64.StdEncoding.DecodeString(encodedPublic)
	if statePrincipal != principal || decodeErr != nil ||
		!private.Public().(ed25519.PublicKey).Equal(ed25519.PublicKey(public)) {
		return nil, fmt.Errorf("privacy requires complete matching canonical node identity state")
	}
	return private, nil
}

func operatorTrustRegistry(configured runtimeconfig.TrustConfig) (*identitytrust.Registry, error) {
	entries := make([]identitytrust.Entry, 0, len(configured.Principals))
	for _, item := range configured.Principals {
		public, err := base64.StdEncoding.DecodeString(item.PublicKey)
		if err != nil || len(public) != ed25519.PublicKeySize || base64.StdEncoding.EncodeToString(public) != item.PublicKey {
			return nil, fmt.Errorf("trusted Principal public key is invalid")
		}
		entries = append(entries, identitytrust.Entry{
			Principal: item.Principal, PublicKey: ed25519.PublicKey(public),
			Purposes: append([]identitytrust.Purpose(nil), item.Purposes...),
		})
	}
	registry, err := identitytrust.NewRegistry(entries)
	if err != nil {
		return nil, fmt.Errorf("trusted Principal registry is invalid: %w", err)
	}
	return registry, nil
}
