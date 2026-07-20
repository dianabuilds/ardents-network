package orchestration

import (
	"path/filepath"

	appdata "ardents/internal/data"
	"ardents/internal/diagnostics"
	discovery "ardents/internal/discovery"
	hostingregistry "ardents/internal/hosting/registry"
	hostingservice "ardents/internal/hosting/service"
	identityapi "ardents/internal/identity/api"
	identitycontinuity "ardents/internal/identity/continuity"
	identitylifecycle "ardents/internal/identity/lifecycle"
	networkapi "ardents/internal/network/api"
	noderoute "ardents/internal/network/route"
	nodelifecycle "ardents/internal/node/lifecycle"
	noderecovery "ardents/internal/node/recovery"
	apppolicy "ardents/internal/policy"
	workloadcontroller "ardents/internal/workload/controller"
)

type CoreConfig struct {
	Name             string
	DataDir          string
	Boot             noderecovery.BootConfig
	Transport        TransportConfig
	Policy           apppolicy.Config
	PolicyService    *apppolicy.Service
	Data             appdata.Config
	Services         []hostingservice.Spec
	WorkloadExecutor workloadcontroller.Executor
}

type TransportConfig struct {
	NodeProfile            networkapi.NodeProfile
	StorePath              string
	PrivateKeyPath         string
	BindAddress            string
	ListenPort             int
	Profile                networkapi.TransportProfile
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

type CoreServices struct {
	Life      *nodelifecycle.Machine
	Diag      *diagnostics.Recorder
	State     *noderecovery.Store
	Keys      identityapi.KeyStore
	Boot      *noderecovery.BootStatus
	Identity  identityapi.Service
	Trust     *discovery.TrustEvaluator
	Discovery *discovery.Service
	Transport networkapi.Service
	Route     *noderoute.State
	Policy    *apppolicy.Service
	Data      *appdata.Service
	Hosting   *hostingregistry.Registry
	Workload  *workloadcontroller.Service
}

func NormalizeNode(name, dir string) (string, string) {
	nodeName := defaultNodeName(name)
	return nodeName, defaultDataDir(nodeName, dir)
}

func NewCore(cfg CoreConfig) CoreServices {
	executor := cfg.WorkloadExecutor
	if executor == nil {
		executor = workloadcontroller.NewLocalExecutor()
	}
	return CoreServices{
		Life:      nodelifecycle.NewMachine(),
		Diag:      diagnostics.NewInDir(cfg.DataDir),
		State:     noderecovery.NewStoreInDir(cfg.DataDir),
		Keys:      identitycontinuity.NewKeyStoreInDir(cfg.DataDir),
		Boot:      noderecovery.NewBootStatus(noderecovery.BootConfig{Sources: cfg.Boot.Sources, Fail: cfg.Boot.Fail}),
		Identity:  identitylifecycle.New(),
		Trust:     discovery.NewTrustEvaluator(),
		Discovery: discovery.NewInDir(cfg.DataDir),
		Transport: networkapi.New(networkapi.Config{
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
		Route:    noderoute.NewState(),
		Policy:   corePolicyService(cfg),
		Data:     appdata.NewInDirWithConfig(cfg.DataDir, cfg.Data),
		Hosting:  hostingregistry.New(cfg.Services),
		Workload: workloadcontroller.NewWithExecutorInDir(cfg.DataDir, executor),
	}
}

func corePolicyService(cfg CoreConfig) *apppolicy.Service {
	if cfg.PolicyService != nil {
		return cfg.PolicyService
	}
	return apppolicy.New(cfg.Policy)
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

func defaultTransportKeyPath(dataDir, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, "waku_node_key.json")
}
