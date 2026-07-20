package transport

import (
	"context"
	"sync"
	"time"

	networkreadiness "ardents/internal/network/readiness"

	libp2pevent "github.com/libp2p/go-libp2p/core/event"
	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
	"golang.org/x/time/rate"
)

const BindAddressEnv = "ARDENTS_TRANSPORT_BIND_ADDRESS"

type Config struct {
	NodeProfile            networkreadiness.NodeProfile
	StorePath              string
	PrivateKeyPath         string
	BindAddress            string
	ListenPort             int
	Profile                networkreadiness.Profile
	WSSPort                int
	WSSCertPath            string
	WSSKeyPath             string
	WSSCAPath              string
	WSSAdvertiseAddress    string
	DNSDiscoveryURLs       []string
	DNSDiscoveryNameServer string
	ReachabilityMode       networkreadiness.ReachabilityMode
	AdvertiseAddresses     []string
	Limits                 Limits
}

type endpointObservation struct {
	usable bool
	reason string
}

type Service struct {
	mu                   sync.Mutex
	state                string
	reason               string
	id                   string
	activeProfile        networkreadiness.Profile
	activeMode           networkreadiness.Mode
	endpoints            []string
	bootstrap            networkreadiness.BootstrapStatus
	bootstrapNodes       []string
	discoveredNodes      []string
	dnsDiscoveryError    string
	lastDNSRefresh       time.Time
	dnsDiscovery         dnsPeerDiscovery
	reachability         networkreadiness.ReachabilitySnapshot
	reachabilityEvents   libp2pevent.Subscription
	reachabilityObs      func(networkreadiness.ReachabilitySnapshot)
	bootstrapObs         func(networkreadiness.BootstrapDialReport)
	observed             map[string]endpointObservation
	controller           *networkreadiness.ModeController
	switchReason         networkreadiness.SwitchReason
	switchAuto           bool
	recoveryState        networkreadiness.RecoveryState
	modeRestartPending   bool
	lastBootstrapAttempt time.Time
	runtimeCancel        context.CancelFunc
	runtimeDone          chan struct{}
	node                 *wakuNode.WakuNode
	operationSlots       chan struct{}
	operationRate        *rate.Limiter
	providerPenalties    map[string]providerPenalty
	abuse                AbuseSnapshot
	cfg                  Config
}

func New(cfg ...Config) *Service {
	svc := &Service{
		state:     "new",
		bootstrap: networkreadiness.BootstrapStatus{State: "idle", Reason: "transport not started"},
		observed:  map[string]endpointObservation{},
	}
	if len(cfg) > 0 {
		svc.cfg = cfg[0]
	}
	svc.cfg.DNSDiscoveryURLs = cloneStrings(svc.cfg.DNSDiscoveryURLs)
	svc.cfg.AdvertiseAddresses = cloneStrings(svc.cfg.AdvertiseAddresses)
	svc.cfg.ReachabilityMode = networkreadiness.NormalizeReachabilityMode(svc.cfg.ReachabilityMode)
	svc.reachability = initialReachability(svc.cfg.ReachabilityMode)
	svc.dnsDiscovery = wakuDNSPeerDiscovery{}
	svc.cfg.Profile = networkreadiness.NormalizeProfile(svc.cfg.Profile)
	svc.cfg.NodeProfile = networkreadiness.NormalizeNodeProfile(svc.cfg.NodeProfile)
	svc.activeProfile = svc.cfg.Profile
	svc.activeMode = networkreadiness.ModeSteady
	svc.controller = networkreadiness.NewModeController(networkreadiness.DefaultSelectionPolicy())
	svc.initializeLimits()
	return svc
}

func (s *Service) State() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.serviceStateLocked().State
}

func (s *Service) Reason() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.serviceStateLocked().Reason
}

func (s *Service) Endpoints() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneStrings(s.publishableEndpointsLocked())
}

func (s *Service) PeerCount() int {
	s.mu.Lock()
	node := s.node
	s.mu.Unlock()
	if node == nil {
		return 0
	}
	return node.PeerCount()
}

func (s *Service) RelayPeerCount(pubsubTopic string) int {
	s.mu.Lock()
	node := s.node
	constrained := s.cfg.NodeProfile == networkreadiness.NodeProfileConstrainedClient
	s.mu.Unlock()
	if node == nil || constrained {
		return 0
	}
	if pubsubTopic == "" {
		pubsubTopic = networkreadiness.DefaultPubsubTopic()
	}
	return len(node.Relay().PubSub().ListPeers(pubsubTopic))
}

func (s *Service) SetBootstrapNodes(nodes []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bootstrapNodes = cloneStrings(nodes)
}

func (s *Service) SetBootstrapObserver(fn func(networkreadiness.BootstrapDialReport)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bootstrapObs = fn
}

func (s *Service) BootstrapStatus() networkreadiness.BootstrapStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentBootstrapStatusViewLocked()
}
