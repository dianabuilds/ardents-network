package process

import (
	"path/filepath"
	"time"

	transport "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
	apppolicy "ardents/internal/policy"
	runtimeconfig "ardents/internal/runtime/config"
	workloadcontroller "ardents/internal/workload/controller"
)

type Config struct {
	Name                     string
	NodeProfile              transport.NodeProfile
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
	Fail    bool
}

type TrustConfig struct {
	Anchors []string
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
	NodeProfile            transport.NodeProfile
	StorePath              string
	PrivateKeyPath         string
	BindAddress            string
	ListenPort             int
	Profile                transport.TransportProfile
	WSSPort                int
	WSSCertPath            string
	WSSKeyPath             string
	WSSCAPath              string
	WSSAdvertiseAddress    string
	DNSDiscoveryURLs       []string
	DNSDiscoveryNameServer string
	ReachabilityMode       transport.ReachabilityMode
	AdvertiseAddresses     []string
	Limits                 transport.Limits
}

type PolicyConfig struct {
	MaxWorkloads                    int
	AllowedPolicyRefs               []string
	DeniedCapabilities              []string
	DisableServicePublication       bool
	DisableNetworkPublishedServices bool
	DeniedServiceTypes              []string
	DisableUntrustedRouteUse        bool
	DeniedRouteSchemes              []string
	DisablePrivateCapabilityUse     bool
	DeniedCapabilityScopes          []string
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
	Capabilities  []string
	PolicyRef     string
	RestartPolicy string
	Services      []ServiceConfig
}

type NodeDataConfig = DataConfig
type NodeTransportConfig = TransportConfig
type NodePolicyConfig = PolicyConfig
type NodeServiceConfig = ServiceConfig
type NodeWorkloadConfig = WorkloadConfig

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
