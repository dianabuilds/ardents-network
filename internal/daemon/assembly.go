package daemon

import (
	"ardents/internal/content"
	"ardents/internal/diagnostics"
	"ardents/internal/discovery"
	"ardents/internal/hosting"
	"ardents/internal/identity"
	identityaccess "ardents/internal/identity/access"
	identitykeyring "ardents/internal/identity/keyring"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"
	networkprivacy "ardents/internal/messaging"
	"ardents/internal/network"
	noderoute "ardents/internal/network/routing"
	networkwaku "ardents/internal/network/waku"
	apppolicy "ardents/internal/policy"
	"ardents/internal/publication"
	"ardents/internal/replication"
	"ardents/internal/storage"
	"ardents/internal/transfer"
	"ardents/internal/workload"
	"ardents/internal/workload/execution"
	"ardents/internal/workload/registry"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

type ownerAssemblyConfig struct {
	NodeName      string
	NodeProfile   network.NodeProfile
	BootSources   []string
	Workloads     []registry.Spec
	Life          *diagnostics.Machine
	Diag          *diagnostics.Recorder
	State         *identity.Store
	Keys          identity.KeyStore
	Boot          *BootStatus
	Identity      identity.Service
	Trust         *discovery.TrustEvaluator
	TrustRegistry *identitytrust.Registry
	Discovery     *discovery.Service
	Transport     network.Service
	Privacy       *networkprivacy.Channel
	DataPrivacy   *networkprivacy.Channel
	Route         *noderoute.State
	Policy        apppolicy.Policy
	Data          *content.Service
	Replica       *replication.Repository
	Transfer      *transfer.Journal
	Hosting       *registry.Registry
	Workload      *execution.Service
	GetPrivate    func() ed25519.PrivateKey
	SetPrivate    func(ed25519.PrivateKey)
	Publish       func(string, map[string]any)
	Lock          sync.Locker
}

type ownerCollaborators struct {
	Publication publication.Coordinator
	workloads   *workload.Runtime
	hosting     *hosting.Service
	RemoteData  *remoteContent
	Runtime     *RuntimeManager
	Query       *QueryService
}

func assembleOwners(cfg ownerAssemblyConfig) ownerCollaborators {
	remoteData := newRemoteContent(cfg)
	publicationMgr := newPublicationManager(cfg)
	workloadRuntime := newWorkloadRuntimeOwner(cfg, publicationMgr)
	hostingService := hosting.NewService(cfg.Lock, workloadRuntime, cfg.Hosting, publicationMgr)
	query := newReader(cfg)
	runtimeMgr := newRuntimeManager(cfg, workloadRuntime, publicationMgr, &contentLifecycle{
		content: cfg.Data, replica: cfg.Replica, transfer: cfg.Transfer, remote: remoteData,
	})
	query.bindSynchronizers(runtimeMgr, workloadRuntime)
	return ownerCollaborators{
		Publication: publicationMgr,
		workloads:   workloadRuntime,
		hosting:     hostingService,
		RemoteData:  remoteData,
		Runtime:     runtimeMgr,
		Query:       query,
	}
}

func newPublicationManager(cfg ownerAssemblyConfig) publication.Coordinator {
	return publication.NewManager(
		cfg.NodeName,
		cfg.Diag,
		cfg.Life,
		cfg.Discovery,
		cfg.Policy,
		cfg.Hosting,
		cfg.Workload,
		cfg.Transport,
		messagingCarrier{cfg.Transport},
		cfg.Identity,
		cfg.Trust,
		cfg.GetPrivate,
		cfg.Publish,
		cfg.Privacy,
	)
}

func newWorkloadRuntimeOwner(cfg ownerAssemblyConfig, publicationMgr publication.Coordinator) *workload.Runtime {
	return newWorkloadRuntime(
		cfg.NodeName,
		cfg.Life,
		cfg.Diag,
		cfg.Policy,
		cfg.Workload,
		publicationMgr,
		cfg.Publish,
	)
}

func newReader(cfg ownerAssemblyConfig) *QueryService {
	return newQueryService(
		cfg.NodeName,
		cfg.NodeProfile,
		cfg.Boot,
		cfg.Life,
		cfg.Diag,
		cfg.Identity,
		cfg.Trust,
		cfg.Discovery,
		cfg.Transport,
		cfg.Privacy,
		cfg.DataPrivacy,
		cfg.Route,
		cfg.Policy,
		cfg.Data,
		cfg.Workload,
	)
}

func newRuntimeManager(cfg ownerAssemblyConfig, workloadRuntime *workload.Runtime, publicationMgr publication.Coordinator, data *contentLifecycle) *RuntimeManager {
	return newRuntimeLifecycle(
		cfg.NodeName,
		cfg.BootSources,
		cfg.Workloads,
		cfg.Life,
		cfg.Diag,
		cfg.State,
		cfg.Keys,
		cfg.Boot,
		cfg.Identity,
		cfg.Trust,
		cfg.TrustRegistry,
		cfg.Discovery,
		cfg.Transport,
		data,
		workloadRuntime,
		publicationMgr,
		cfg.GetPrivate,
		cfg.SetPrivate,
		cfg.Publish,
		cfg.Privacy,
	)
}

func runtimeCoreConfig(cfg Config) coreConfig {
	return coreConfig{
		Name:             cfg.Name,
		DataDir:          cfg.Data.Dir,
		Boot:             BootConfig{Sources: cloneStrings(cfg.Boot.Sources)},
		Trust:            cfg.Trust.Registry,
		Transport:        runtimeTransportConfig(cfg.Transport),
		Policy:           runtimePolicyConfig(cfg.Policy),
		PolicyService:    cfg.PolicyService,
		Data:             runtimeDataConfig(cfg.Data),
		Services:         runtimeServiceConfigs(cfg.Service),
		WorkloadExecutor: cfg.WorkloadExecutor,
	}
}

func runtimeTransportConfig(cfg TransportConfig) coreTransportConfig {
	return coreTransportConfig{
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
		DeniedWorkloadRequirements:      append([]registry.WorkloadRequirement(nil), cfg.DeniedWorkloadRequirements...),
		DisableServicePublication:       cfg.DisableServicePublication,
		DisableNetworkPublishedServices: cfg.DisableNetworkPublishedServices,
		DeniedServiceTypes:              cloneStrings(cfg.DeniedServiceTypes),
		DisableUntrustedRouteUse:        cfg.DisableUntrustedRouteUse,
		DeniedRouteSchemes:              cloneStrings(cfg.DeniedRouteSchemes),
		DisablePrivateChannelGrantUse:   cfg.DisablePrivateChannelGrantUse,
		DeniedChannelGrantScopes:        cloneStrings(cfg.DeniedChannelGrantScopes),
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

func runtimeDataConfig(cfg DataConfig) content.Config {
	return content.Config{
		DefaultLocalRetentionTTL: cfg.DefaultLocalRetentionTTL,
		DefaultRelayRetentionTTL: cfg.DefaultRelayRetentionTTL,
		MaxRelayRetentionBytes:   cfg.MaxRelayRetentionBytes,
		MaxReplicaRetentionBytes: cfg.MaxReplicaRetentionBytes,
		MaxLocalStorageBytes:     cfg.MaxLocalStorageBytes,
		DefaultDesiredReplicas:   cfg.DefaultDesiredReplicas,
		DefaultMinimumReplicas:   cfg.DefaultMinimumReplicas,
	}
}

func runtimeWorkloadSpecs(items []WorkloadConfig) []registry.Spec {
	configs := make([]registry.Spec, 0, len(items))
	for _, item := range items {
		configs = append(configs, registry.Spec{
			ID:           item.ID,
			Kind:         item.Kind,
			Owner:        item.Owner,
			Config:       item.Config,
			Desired:      item.Desired,
			Requirements: append([]registry.WorkloadRequirement(nil), item.Requirements...),
			PolicyRef:    item.PolicyRef, RestartPolicy: item.RestartPolicy,
			Services: runtimeWorkloadServiceSpecs(item.Services),
		})
	}
	return configs
}

func runtimeServiceConfigs(items []ServiceConfig) []registry.ServiceSpec {
	configs := make([]registry.ServiceSpec, 0, len(items))
	for _, item := range items {
		configs = append(configs, registry.ServiceSpec{
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

func runtimeWorkloadServiceSpecs(items []ServiceConfig) []registry.ServiceSpec {
	configs := make([]registry.ServiceSpec, 0, len(items))
	for _, item := range items {
		configs = append(configs, registry.ServiceSpec{
			ID:             item.ID,
			Type:           item.Type,
			Mode:           item.Mode,
			Endpoints:      cloneStrings(item.Endpoints),
			ProbeEndpoints: cloneStrings(item.ProbeEndpoints),
		})
	}
	return configs
}

type coreConfig struct {
	Name             string
	DataDir          string
	Boot             BootConfig
	Trust            *identitytrust.Registry
	Transport        coreTransportConfig
	Policy           apppolicy.Config
	PolicyService    *apppolicy.Service
	Data             content.Config
	Services         []registry.ServiceSpec
	WorkloadExecutor execution.Executor
}

type coreTransportConfig struct {
	NodeProfile            network.NodeProfile
	StorePath              string
	PrivateKeyPath         string
	BindAddress            string
	ListenPort             int
	Profile                network.Profile
	WSSPort                int
	WSSCertPath            string
	WSSKeyPath             string
	WSSCAPath              string
	WSSAdvertiseAddress    string
	DNSDiscoveryURLs       []string
	DNSDiscoveryNameServer string
	ReachabilityMode       network.ReachabilityMode
	AdvertiseAddresses     []string
	Limits                 network.Limits
}

type coreServices struct {
	Life      *diagnostics.Machine
	Diag      *diagnostics.Recorder
	State     *identity.Store
	Keys      identity.KeyStore
	Boot      *BootStatus
	Identity  identity.Service
	Trust     *discovery.TrustEvaluator
	Discovery *discovery.Service
	Transport network.Service
	Route     *noderoute.State
	Policy    *apppolicy.Service
	Data      *content.Service
	Replica   *replication.Repository
	Transfer  *transfer.Journal
	Hosting   *registry.Registry
	Workload  *execution.Service
}

func normalizeNode(name, dir string) (string, string) {
	nodeName := defaultNodeName(name)
	return nodeName, defaultDataDir(nodeName, dir)
}

func buildCore(cfg coreConfig) coreServices {
	executor := cfg.WorkloadExecutor
	if executor == nil {
		executor = execution.NewDisabledExecutor()
	}
	contentStore := content.NewInDirWithConfig(cfg.DataDir, cfg.Data)
	trustService := discovery.NewTrustEvaluator(cfg.Trust)
	return coreServices{
		Life:      diagnostics.NewMachine(),
		Diag:      diagnostics.NewInDir(cfg.DataDir),
		State:     identity.NewStoreInDir(cfg.DataDir),
		Keys:      identitykeyring.NewKeyStoreInDir(cfg.DataDir),
		Boot:      newBootStatus(cfg.Boot.Sources),
		Identity:  identity.NewService(),
		Trust:     trustService,
		Discovery: discovery.NewInDirWithTrust(cfg.DataDir, trustService),
		Transport: networkwaku.New(network.Config{
			NodeProfile:            cfg.Transport.NodeProfile,
			StorePath:              cfg.Transport.StorePath,
			PrivateKeyPath:         defaultTransportKeyPath(cfg.DataDir, cfg.Transport.PrivateKeyPath),
			BindAddress:            cfg.Transport.BindAddress,
			ListenPort:             cfg.Transport.ListenPort,
			Profile:                cfg.Transport.Profile,
			WSSPort:                cfg.Transport.WSSPort,
			WSSCertPath:            cfg.Transport.WSSCertPath,
			WSSKeyPath:             cfg.Transport.WSSKeyPath,
			WSSCAPath:              cfg.Transport.WSSCAPath,
			WSSAdvertiseAddress:    cfg.Transport.WSSAdvertiseAddress,
			DNSDiscoveryURLs:       append([]string(nil), cfg.Transport.DNSDiscoveryURLs...),
			DNSDiscoveryNameServer: cfg.Transport.DNSDiscoveryNameServer,
			ReachabilityMode:       cfg.Transport.ReachabilityMode,
			AdvertiseAddresses:     append([]string(nil), cfg.Transport.AdvertiseAddresses...),
			Limits:                 cfg.Transport.Limits,
		}),
		Route:  noderoute.NewState(),
		Policy: corePolicyService(cfg),
		Data:   contentStore,
		Replica: replication.NewRepository(replication.RepositoryConfig{
			Path: storage.PathInDir(cfg.DataDir), Content: contentStore,
			MaxRetentionBytes:      replicaRetentionLimit(cfg.Data),
			DefaultDesiredReplicas: cfg.Data.DefaultDesiredReplicas,
			DefaultMinimumReplicas: cfg.Data.DefaultMinimumReplicas,
		}),
		Transfer: transfer.NewJournal(storage.PathInDir(cfg.DataDir)),
		Hosting:  registry.New(cfg.Services),
		Workload: execution.NewWithExecutorInDir(cfg.DataDir, executor),
	}
}

func replicaRetentionLimit(config content.Config) int64 {
	if config.MaxReplicaRetentionBytes > 0 {
		return config.MaxReplicaRetentionBytes
	}
	return config.MaxRelayRetentionBytes
}

func corePolicyService(cfg coreConfig) *apppolicy.Service {
	if cfg.PolicyService != nil {
		return cfg.PolicyService
	}
	return apppolicy.New(cfg.Policy)
}

func defaultTransportKeyPath(dataDir, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, "waku_node_key.json")
}

func NewNode(cfg Config) *Node {
	cfg = normalizedConfig(cfg)
	n := newNodeCore(cfg)
	n.configureLocalServicesLocked()
	n.initOwnerCollaboratorsLocked()
	n.initOperatorConfig()
	n.runtimeMgr.SyncObservedTruthLocked()
	return n
}

func (n *Node) initOwnerCollaboratorsLocked() {
	collaborators := assembleOwners(runtimeAssemblyConfig(n))
	n.publicationMgr = collaborators.Publication
	n.workloadRuntime = collaborators.workloads
	n.hosting = collaborators.hosting
	n.remoteData = collaborators.RemoteData
	n.runtimeMgr = collaborators.Runtime
	n.queryService = collaborators.Query
	n.discoveryCommands = discovery.NewCommands(n.disco, discovery.CommandConfig{
		Guard:     n.requireProcessMutableLocked,
		Emit:      n.publishLocked,
		OnChanged: n.runtimeMgr.SyncDiscoveryTrustDiagnosticsLocked,
	})
	n.discoveryResolver = discovery.NewResolver(discovery.ResolverConfig{
		Store: n.disco, Trust: n.trust, Network: n.trans, Routes: n.route, Policy: n.policy,
		BeforeServiceResolve: n.workloadRuntime.SyncObserved,
		PolicyDenied:         n.emitPolicyDeniedLocked,
	})
	n.trans.SetReachabilityObserver(n.onReachabilityChanged)
}

func runtimeAssemblyConfig(n *Node) ownerAssemblyConfig {
	return ownerAssemblyConfig{
		NodeName:      n.cfg.Name,
		NodeProfile:   n.cfg.NodeProfile,
		BootSources:   cloneStrings(n.cfg.Boot.Sources),
		Workloads:     runtimeWorkloadSpecs(n.cfg.Workload),
		Life:          n.life,
		Diag:          n.diag,
		State:         n.state,
		Keys:          n.keys,
		Boot:          n.boot,
		Identity:      n.ident,
		Trust:         n.trust,
		TrustRegistry: n.cfg.Trust.Registry,
		Discovery:     n.disco,
		Transport:     n.trans,
		Privacy:       n.privacy,
		DataPrivacy:   n.dataPrivacy,
		Route:         n.route,
		Policy:        n.policy,
		Data:          n.data,
		Replica:       n.replica,
		Transfer:      n.transfers,
		Hosting:       n.srv,
		Workload:      n.workload,
		GetPrivate:    func() ed25519.PrivateKey { return n.private },
		SetPrivate:    func(key ed25519.PrivateKey) { n.private = key },
		Publish:       n.publishLocked,
		Lock:          &n.mu,
	}
}

func normalizedConfig(cfg Config) Config {
	cfg.Name = defaultNodeName(cfg.Name)
	cfg.Data.Dir = defaultDataDir(cfg.Name, cfg.Data.Dir)
	defaultProfile := cfg.NodeProfile == ""
	cfg.NodeProfile = network.NormalizeNodeProfile(cfg.NodeProfile)
	cfg.Transport.NodeProfile = cfg.NodeProfile
	if defaultProfile && cfg.Transport.BindAddress == "" {
		cfg.Transport.BindAddress = "127.0.0.1"
	}
	if cfg.Transport.ReachabilityMode == "" {
		if cfg.NodeProfile == network.NodeProfileLocalDevelopment {
			cfg.Transport.ReachabilityMode = network.ReachabilityLocalOnly
		} else if cfg.NodeProfile == network.NodeProfileConstrainedClient {
			cfg.Transport.ReachabilityMode = network.ReachabilityOutboundOnly
		} else {
			cfg.Transport.ReachabilityMode = network.ReachabilityPrivateLAN
		}
	}
	return cfg
}

func newNodeCore(cfg Config) *Node {
	core := buildCore(runtimeCoreConfig(cfg))
	return &Node{
		cfg:            cfg,
		life:           core.Life,
		diag:           core.Diag,
		state:          core.State,
		keys:           core.Keys,
		boot:           core.Boot,
		ident:          core.Identity,
		trust:          core.Trust,
		disco:          core.Discovery,
		trans:          core.Transport,
		privacy:        cfg.Privacy,
		dataPrivacy:    cfg.DataPrivacy,
		route:          core.Route,
		policy:         core.Policy,
		policyLive:     core.Policy,
		data:           core.Data,
		replica:        core.Replica,
		transfers:      core.Transfer,
		srv:            core.Hosting,
		workload:       core.Workload,
		backgroundStop: make(chan struct{}),
		subs:           map[chan Event]struct{}{},
	}
}

func (n *Node) configureLocalServicesLocked() {
	configureLocalServices(
		n.policy,
		n.workload,
		n.data,
		n.trans,
		n.cfg.Boot.Sources,
		n.handleBootstrapDialLocked,
	)
	n.dataCommands = content.NewCommands(n.data, content.CommandConfig{
		Guard: n.requireDataMutableLocked,
		AuthorizePin: func(blob content.Blob) error {
			return n.policy.AllowBlobPin(content.BlobPolicyView{State: blob.State, Retention: blob.Retention, Encrypted: blob.Encrypted})
		},
		Emit: n.publishLocked,
	})
}

type Owners struct {
	Node              *Node
	IdentityAccess    storage.Database
	PrincipalAccess   *identityaccess.Service
	Content           *content.Service
	ContentCommands   *content.Commands
	DiscoveryCommands *discovery.Commands
	Transfers         *transfer.Journal
	Workloads         *workload.Runtime
	Hosting           *hosting.Service
	Diagnostics       *diagnostics.Query
	Events            *diagnostics.Recorder
}

func NewOwners(cfg Config) Owners {
	return ownersFor(NewNode(cfg))
}

func OwnersFor(node *Node) (Owners, bool) {
	if node == nil {
		return Owners{}, false
	}
	return ownersFor(node), true
}

func ownersFor(node *Node) Owners {
	query := diagnostics.NewQuery(node.diag, node.SyncDiagnostics, func(id string) (*diagnostics.ServiceStatus, bool) {
		status, err := node.hosting.GetHostedService(id)
		if err != nil {
			return nil, false
		}
		return &diagnostics.ServiceStatus{Published: status.Published, Reason: status.Reason}, true
	})
	return Owners{Node: node, Content: node.data, ContentCommands: node.dataCommands, DiscoveryCommands: node.discoveryCommands, Transfers: node.transfers, Workloads: node.workloadRuntime, Hosting: node.hosting, Diagnostics: query, Events: node.diag}
}

type querySurface interface {
	SnapshotLocked() SystemSnapshot
	RoutingDetailsLocked() discovery.RouteSnapshot
	DiagnosticsSnapshotLocked() diagnostics.DiagSnapshot
	PendingOperationsLocked() []diagnostics.OperationSnapshot
	NodeFeaturesSnapshotLocked() NodeFeaturesSnapshot
	NodeRuntimeSnapshotLocked() RuntimeSnapshot
	NetworkStatusSnapshotLocked() network.StatusSnapshot
	DiscoveryStatusSnapshotLocked(time time.Time) discovery.StatusSnapshot
	PeerSnapshotsLocked() []discovery.PeerSnapshot
	SyncDiagnosticsLocked() error
}

type Node struct {
	mu                 sync.Mutex
	stopMu             sync.Mutex
	cfg                Config
	life               *diagnostics.Machine
	diag               *diagnostics.Recorder
	state              *identity.Store
	keys               identity.KeyStore
	boot               *BootStatus
	ident              identity.Service
	trust              *discovery.TrustEvaluator
	disco              *discovery.Service
	discoveryCommands  *discovery.Commands
	discoveryResolver  *discovery.Resolver
	trans              network.Service
	privacy            *networkprivacy.Channel
	dataPrivacy        *networkprivacy.Channel
	route              *noderoute.State
	policy             apppolicy.Policy
	policyLive         *apppolicy.Service
	data               *content.Service
	dataCommands       *content.Commands
	replica            *replication.Repository
	transfers          *transfer.Journal
	remoteData         *remoteContent
	srv                *registry.Registry
	workload           *execution.Service
	private            ed25519.PrivateKey
	network            context.Context
	cancel             context.CancelFunc
	refreshStop        context.CancelFunc
	refreshLoops       []<-chan struct{}
	backgroundMu       sync.Mutex
	backgroundWriters  sync.WaitGroup
	backgroundStopping bool
	backgroundStop     chan struct{}
	seq                int64
	subs               map[chan Event]struct{}

	startBlobExchange func(context.Context) error

	publicationMgr  publication.Coordinator
	workloadRuntime *workload.Runtime
	hosting         *hosting.Service
	queryService    querySurface
	runtimeMgr      *RuntimeManager
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	return append([]string(nil), in...)
}

func replaceTrustWithLocalPrincipal(trust *discovery.TrustEvaluator, configured *identitytrust.Registry, rawPrincipal, encodedPublic string) error {
	principalID, err := identityprincipal.Parse(rawPrincipal)
	if err != nil {
		return fmt.Errorf("local Principal is invalid")
	}
	public, err := base64.StdEncoding.DecodeString(encodedPublic)
	if err != nil || len(public) != ed25519.PublicKeySize || base64.StdEncoding.EncodeToString(public) != encodedPublic {
		return fmt.Errorf("local Principal public key is invalid")
	}
	derived, err := identityprincipal.FromEd25519PublicKey(ed25519.PublicKey(public))
	if err != nil || !derived.Equal(principalID) {
		return fmt.Errorf("local Principal public key does not match")
	}
	var entries []identitytrust.Entry
	if configured != nil {
		entries = configured.Snapshot().Entries
	}
	found := false
	for index := range entries {
		if entries[index].Principal != rawPrincipal {
			continue
		}
		found = true
		if !slices.Contains(entries[index].Purposes, identitytrust.PurposeDiscoveryPublish) {
			entries[index].Purposes = append(entries[index].Purposes, identitytrust.PurposeDiscoveryPublish)
		}
	}
	if !found {
		entries = append(entries, identitytrust.Entry{
			Principal: rawPrincipal, PublicKey: ed25519.PublicKey(public),
			Purposes: []identitytrust.Purpose{identitytrust.PurposeDiscoveryPublish},
		})
	}
	replacement, err := identitytrust.NewRegistry(entries)
	if err != nil {
		return fmt.Errorf("local Principal trust is invalid: %w", err)
	}
	trust.ReplaceRegistry(replacement)
	return nil
}

func configureLocalServices(
	policy apppolicy.Policy,
	workload *execution.Service,
	data *content.Service,
	trans network.Service,
	bootSources []string,
	bootstrapObserver func(network.BootstrapDialReport),
) {
	if workload != nil && policy != nil {
		workload.SetAdmission(func(spec registry.Spec, items []execution.Status) error {
			return policy.AdmitWorkload(spec, items)
		})
	}
	if data != nil && policy != nil {
		data.SetRetentionAuthorizer(func(blob content.BlobPolicyView, relay bool, expiresAt time.Time) error {
			return policy.AllowBlobRetention(blob, relay, expiresAt, time.Now().UTC())
		})
	}
	if trans != nil {
		trans.SetBootstrapNodes(bootSources)
		trans.SetBootstrapObserver(bootstrapObserver)
	}
}

type NodeFeaturesSnapshot struct {
	Version  string
	Services []string
	Features map[string]bool
}

type Event struct {
	Seq   int64
	Time  time.Time
	Topic string
	Data  map[string]any
}
