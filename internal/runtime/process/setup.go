package process

import (
	"crypto/ed25519"

	appdata "ardents/internal/data"
	hostingservice "ardents/internal/hosting/service"
	transport "ardents/internal/network/api"
	nodeapi "ardents/internal/node/api"
	noderecovery "ardents/internal/node/recovery"
	apppolicy "ardents/internal/policy"
	runtimeassembly "ardents/internal/runtime/assembly"
	runtimeorchestration "ardents/internal/runtime/orchestration"
	domainworkload "ardents/internal/workload/workload"
)

type Store = noderecovery.Store

func NewNode(cfg Config) *Node {
	cfg = normalizedConfig(cfg)
	n := newNodeCore(cfg)
	n.applyTrustAnchorsLocked()
	n.configureLocalServicesLocked()
	n.initOwnerCollaboratorsLocked()
	n.initOperatorConfig()
	n.runtimeMgr.SyncObservedTruthLocked()
	return n
}

func (n *Node) initOwnerCollaboratorsLocked() {
	collaborators := runtimeassembly.New(runtimeAssemblyConfig(n))
	n.publicationMgr = collaborators.Publication
	n.authorityCtl = collaborators.Authority
	n.runtimeMgr = collaborators.Runtime
	n.queryService = collaborators.Query
	n.commandService = collaborators.Command
	n.trans.SetReachabilityObserver(n.onReachabilityChanged)
}

func runtimeAssemblyConfig(n *Node) runtimeassembly.Config {
	return runtimeassembly.Config{
		NodeName:    n.cfg.Name,
		NodeProfile: n.cfg.NodeProfile,
		BootSources: cloneStrings(n.cfg.Boot.Sources),
		Workloads:   runtimeWorkloadSpecs(n.cfg.Workload),
		Life:        n.life,
		Diag:        n.diag,
		State:       n.state,
		Keys:        n.keys,
		Boot:        n.boot,
		Identity:    n.ident,
		Trust:       n.trust,
		Discovery:   n.disco,
		Transport:   n.trans,
		Privacy:     n.privacy,
		DataPrivacy: n.dataPrivacy,
		Route:       n.route,
		Policy:      n.policy,
		Data:        n.data,
		Hosting:     n.srv,
		Workload:    n.workload,
		GetPrivate:  func() ed25519.PrivateKey { return n.private },
		SetPrivate:  func(key ed25519.PrivateKey) { n.private = key },
		Publish:     n.publishLocked,
	}
}

func normalizedConfig(cfg Config) Config {
	cfg.Name = defaultNodeName(cfg.Name)
	cfg.Data.Dir = defaultDataDir(cfg.Name, cfg.Data.Dir)
	defaultProfile := cfg.NodeProfile == ""
	cfg.NodeProfile = transport.NormalizeNodeProfile(cfg.NodeProfile)
	cfg.Transport.NodeProfile = cfg.NodeProfile
	if defaultProfile && cfg.Transport.BindAddress == "" {
		cfg.Transport.BindAddress = "127.0.0.1"
	}
	if cfg.Transport.ReachabilityMode == "" {
		if cfg.NodeProfile == transport.NodeProfileLocalDevelopment {
			cfg.Transport.ReachabilityMode = transport.ReachabilityLocalOnly
		} else if cfg.NodeProfile == transport.NodeProfileConstrainedClient {
			cfg.Transport.ReachabilityMode = transport.ReachabilityOutboundOnly
		} else {
			cfg.Transport.ReachabilityMode = transport.ReachabilityPrivateLAN
		}
	}
	return cfg
}

func newNodeCore(cfg Config) *Node {
	core := runtimeorchestration.NewCore(runtimeCoreConfig(cfg))
	return &Node{
		cfg:         cfg,
		life:        core.Life,
		diag:        core.Diag,
		state:       core.State,
		keys:        core.Keys,
		boot:        core.Boot,
		ident:       core.Identity,
		trust:       core.Trust,
		disco:       core.Discovery,
		trans:       core.Transport,
		privacy:     cfg.Privacy,
		dataPrivacy: cfg.DataPrivacy,
		route:       core.Route,
		policy:      core.Policy,
		policyLive:  core.Policy,
		data:        core.Data,
		srv:         core.Hosting,
		workload:    core.Workload,
		subs:        map[chan nodeapi.Event]struct{}{},
	}
}

func runtimeCoreConfig(cfg Config) runtimeorchestration.CoreConfig {
	return runtimeorchestration.CoreConfig{
		Name:             cfg.Name,
		DataDir:          cfg.Data.Dir,
		Boot:             runtimeBootConfig(cfg.Boot),
		Transport:        runtimeTransportConfig(cfg.Transport),
		Policy:           runtimePolicyConfig(cfg.Policy),
		PolicyService:    cfg.PolicyService,
		Data:             runtimeDataConfig(cfg.Data),
		Services:         runtimeServiceConfigs(cfg.Service),
		WorkloadExecutor: cfg.WorkloadExecutor,
	}
}

func runtimeBootConfig(cfg BootConfig) noderecovery.BootConfig {
	return noderecovery.BootConfig{
		Sources: cloneStrings(cfg.Sources),
		Fail:    cfg.Fail,
	}
}

func runtimeTransportConfig(cfg TransportConfig) runtimeorchestration.TransportConfig {
	return runtimeorchestration.TransportConfig{
		NodeProfile:            cfg.NodeProfile,
		StorePath:              cfg.StorePath,
		PrivateKeyPath:         cfg.PrivateKeyPath,
		BindAddress:            cfg.BindAddress,
		ListenPort:             cfg.ListenPort,
		Profile:                cfg.Profile,
		WSSPort:                cfg.WSSPort,
		WSSCertPath:            cfg.WSSCertPath,
		WSSKeyPath:             cfg.WSSKeyPath,
		WSSCAPath:              cfg.WSSCAPath,
		WSSAdvertiseAddress:    cfg.WSSAdvertiseAddress,
		DNSDiscoveryURLs:       append([]string(nil), cfg.DNSDiscoveryURLs...),
		DNSDiscoveryNameServer: cfg.DNSDiscoveryNameServer,
		ReachabilityMode:       cfg.ReachabilityMode,
		AdvertiseAddresses:     append([]string(nil), cfg.AdvertiseAddresses...),
		Limits:                 cfg.Limits,
	}
}

func runtimePolicyConfig(cfg PolicyConfig) apppolicy.Config {
	return apppolicy.Config{
		MaxWorkloads:                    cfg.MaxWorkloads,
		AllowedPolicyRefs:               cloneStrings(cfg.AllowedPolicyRefs),
		DeniedCapabilities:              cloneStrings(cfg.DeniedCapabilities),
		DisableServicePublication:       cfg.DisableServicePublication,
		DisableNetworkPublishedServices: cfg.DisableNetworkPublishedServices,
		DeniedServiceTypes:              cloneStrings(cfg.DeniedServiceTypes),
		DisableUntrustedRouteUse:        cfg.DisableUntrustedRouteUse,
		DeniedRouteSchemes:              cloneStrings(cfg.DeniedRouteSchemes),
		DisablePrivateCapabilityUse:     cfg.DisablePrivateCapabilityUse,
		DeniedCapabilityScopes:          cloneStrings(cfg.DeniedCapabilityScopes),
		DisableLocalBlobRetention:       cfg.DisableLocalBlobRetention,
		DisableRelayBlobRetention:       cfg.DisableRelayBlobRetention,
		DisableBlobPinning:              cfg.DisableBlobPinning,
		DisablePeerBlobReserving:        cfg.DisablePeerBlobReserving,
		AllowPinRelayRetainedBlobs:      cfg.AllowPinRelayRetainedBlobs,
		AllowReservingRelayBlobs:        cfg.AllowReservingRelayBlobs,
		MaxLocalRetentionTTL:            cfg.MaxLocalRetentionTTL,
		MaxRelayRetentionTTL:            cfg.MaxRelayRetentionTTL,
	}
}

func runtimeDataConfig(cfg DataConfig) appdata.Config {
	return appdata.Config{
		DefaultLocalRetentionTTL: cfg.DefaultLocalRetentionTTL,
		DefaultRelayRetentionTTL: cfg.DefaultRelayRetentionTTL,
		MaxRelayRetentionBytes:   cfg.MaxRelayRetentionBytes,
		MaxReplicaRetentionBytes: cfg.MaxReplicaRetentionBytes,
		MaxLocalStorageBytes:     cfg.MaxLocalStorageBytes,
		DefaultDesiredReplicas:   cfg.DefaultDesiredReplicas,
		DefaultMinimumReplicas:   cfg.DefaultMinimumReplicas,
	}
}

func runtimeWorkloadSpecs(items []WorkloadConfig) []domainworkload.Spec {
	configs := make([]domainworkload.Spec, 0, len(items))
	for _, item := range items {
		configs = append(configs, domainworkload.Spec{
			ID:           item.ID,
			Kind:         item.Kind,
			Owner:        item.Owner,
			Config:       item.Config,
			Desired:      item.Desired,
			Capabilities: cloneStrings(item.Capabilities),
			PolicyRef:    item.PolicyRef, RestartPolicy: item.RestartPolicy,
			Services: runtimeWorkloadServiceSpecs(item.Services),
		})
	}
	return configs
}

func runtimeServiceConfigs(items []ServiceConfig) []hostingservice.Spec {
	configs := make([]hostingservice.Spec, 0, len(items))
	for _, item := range items {
		configs = append(configs, hostingservice.Spec{
			ID:             item.ID,
			Type:           item.Type,
			Owner:          item.Owner,
			Mode:           item.Mode,
			Endpoints:      cloneStrings(item.Endpoints),
			ProbeEndpoints: cloneStrings(item.ProbeEndpoints),
		})
	}
	return configs
}

func runtimeWorkloadServiceSpecs(items []ServiceConfig) []domainworkload.ServiceSpec {
	configs := make([]domainworkload.ServiceSpec, 0, len(items))
	for _, item := range items {
		configs = append(configs, domainworkload.ServiceSpec{
			ID:             item.ID,
			Type:           item.Type,
			Mode:           item.Mode,
			Endpoints:      cloneStrings(item.Endpoints),
			ProbeEndpoints: cloneStrings(item.ProbeEndpoints),
		})
	}
	return configs
}

func (n *Node) applyTrustAnchorsLocked() {
	runtimeorchestration.ApplyTrustAnchors(n.trust, n.cfg.Trust.Anchors)
}

func (n *Node) configureLocalServicesLocked() {
	runtimeorchestration.ConfigureLocalServices(
		n.policy,
		n.workload,
		n.data,
		n.trans,
		n.cfg.Boot.Sources,
		n.handleBootstrapDialLocked,
	)
}
