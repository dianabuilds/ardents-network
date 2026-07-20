package transport

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	networkmessaging "ardents/internal/network/messaging"
	networkparticipation "ardents/internal/network/participation"
	networkpeer "ardents/internal/network/peer"
	networkreadiness "ardents/internal/network/readiness"

	"github.com/libp2p/go-libp2p"
	libp2ptcp "github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/waku-org/go-waku/waku/persistence"
	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
	"github.com/waku-org/go-waku/waku/v2/protocol"
)

var transportSeq int64

func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if len(s.endpoints) > 0 {
		s.mu.Unlock()
		return nil
	}
	node, err := s.prepareNodeLocked()
	if err != nil {
		return s.failStartLocked(err)
	}
	if err := networkparticipation.StartWakuNode(ctx, node, networkmessaging.DefaultPubsubTopic, s.relayEnabledLocked()); err != nil {
		return s.failStartLocked(err)
	}
	s.markStartedLocked(node)
	if err := s.startReachabilityObservationLocked(node); err != nil {
		s.node = nil
		s.endpoints = nil
		startErr := s.failStartLocked(err)
		node.Stop()
		return startErr
	}
	s.markBoundReachabilityLocked()
	s.mu.Unlock()

	_ = s.refreshDNSPeers(ctx)
	status := s.dialBootstrapPeers(ctx)

	s.mu.Lock()
	s.bootstrap = status
	s.reconcileRuntimeLocked(timeNowUTC())
	s.startRuntimeLoopLocked()
	s.mu.Unlock()
	return nil
}

func (s *Service) prepareNodeLocked() (*wakuNode.WakuNode, error) {
	if err := s.validateStartupConfigLocked(); err != nil {
		return nil, err
	}
	hostAddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(bindAddress(s.cfg), strconv.Itoa(listenPort(s.cfg))))
	if err != nil {
		return nil, err
	}
	provider, storeExisted, err := s.prepareMessageProviderLocked()
	if err != nil {
		return nil, err
	}
	nodeKey, err := newTransportKeyStore(s.cfg.PrivateKeyPath).Ensure(storeExisted)
	if err != nil {
		return nil, err
	}
	return s.buildNode(hostAddr, provider, nodeKey)
}

func (s *Service) validateStartupConfigLocked() error {
	if _, err := networkreadiness.ResolveProfile(s.activeProfile); err != nil {
		return err
	}
	return ValidateConfig(s.cfg, timeNowUTC())
}

func (s *Service) Stop(_ context.Context) error {
	s.mu.Lock()
	cancel := s.runtimeCancel
	done := s.runtimeDone
	node := s.node
	reachabilityEvents := s.reachabilityEvents
	s.runtimeCancel = nil
	s.runtimeDone = nil
	s.node = nil
	s.reachabilityEvents = nil
	s.endpoints = nil
	s.state = "stopped"
	s.reason = ""
	s.bootstrap = networkreadiness.BootstrapStatus{State: "idle", Reason: "transport stopped"}
	s.observed = map[string]endpointObservation{}
	s.discoveredNodes = nil
	s.dnsDiscoveryError = ""
	s.lastDNSRefresh = time.Time{}
	s.reachability = initialReachability(s.cfg.ReachabilityMode)
	s.activeMode = networkreadiness.ModeSteady
	s.switchReason = networkreadiness.SwitchReasonStopped
	s.switchAuto = false
	s.recoveryState = ""
	s.modeRestartPending = false
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	if reachabilityEvents != nil {
		_ = reachabilityEvents.Close()
	}
	if node != nil {
		node.Stop()
	}
	return nil
}

func (s *Service) failStartLocked(err error) error {
	s.state = "failed"
	s.reason = err.Error()
	s.bootstrap = networkreadiness.BootstrapStatus{State: "degraded", Reason: err.Error()}
	s.activeMode = networkreadiness.ModeSteady
	s.switchReason = networkreadiness.SwitchReasonStartupFailed
	s.switchAuto = false
	s.recoveryState = networkreadiness.RecoveryStateBlocked
	s.mu.Unlock()
	return err
}

func (s *Service) markStartedLocked(node *wakuNode.WakuNode) {
	id := atomic.AddInt64(&transportSeq, 1)
	s.id = fmt.Sprintf("peer-%d", id)
	s.node = node
	s.endpoints = publishedEndpoints(node, s.cfg)
	s.observed = newEndpointObservations(s.endpoints, true)
	s.state = "ready"
	s.reason = ""
	s.activeMode = networkreadiness.ModeSteady
	s.modeRestartPending = false
}

func (s *Service) buildNode(hostAddr *net.TCPAddr, provider *persistence.DBStore, key *ecdsa.PrivateKey) (*wakuNode.WakuNode, error) {
	profile := networkreadiness.NormalizeProfile(s.activeProfile)
	definition, err := networkreadiness.ResolveProfile(profile)
	if err != nil {
		return nil, err
	}
	libp2pOptions, err := libP2POptionsForDefinition(definition)
	if err != nil {
		return nil, err
	}
	reachabilityOptions, err := reachabilityLibP2POptions(s.cfg)
	if err != nil {
		return nil, err
	}
	libp2pOptions = append(libp2pOptions, reachabilityOptions...)
	profileOptions, err := s.profileNodeOptions(definition, hostAddr)
	if err != nil {
		return nil, err
	}
	options := append(baseWakuNodeOptions(hostAddr, libp2pOptions, key), s.messagingNodeOptions(provider)...)
	options = append(options, profileOptions...)
	return wakuNode.New(options...)
}

func baseWakuNodeOptions(hostAddr *net.TCPAddr, libp2pOptions []libp2p.Option, key *ecdsa.PrivateKey) []wakuNode.WakuNodeOption {
	return []wakuNode.WakuNodeOption{
		wakuNode.WithLogger(silentWakuLogger()),
		wakuNode.WithHostAddress(hostAddr),
		wakuNode.WithLibP2POptions(libp2pOptions...),
		wakuNode.WithPrivateKey(key),
		wakuNode.WithClusterID(protocol.ClusterIndex),
		wakuNode.WithShards([]uint16{0}),
	}
}

func libP2POptionsForDefinition(definition networkreadiness.Definition) ([]libp2p.Option, error) {
	switch definition.StartupVariant {
	case networkreadiness.StartupVariantTCPOnly, networkreadiness.StartupVariantTCPWSS:
		return []libp2p.Option{
			libp2p.NoTransports,
			libp2p.Transport(libp2ptcp.NewTCPTransport),
		}, nil
	default:
		return nil, fmt.Errorf("transport profile %q is not implemented", definition.Profile)
	}
}

func (s *Service) profileNodeOptions(definition networkreadiness.Definition, hostAddr *net.TCPAddr) ([]wakuNode.WakuNodeOption, error) {
	switch definition.StartupVariant {
	case networkreadiness.StartupVariantTCPOnly:
		return nil, nil
	case networkreadiness.StartupVariantTCPWSS:
		certPath := strings.TrimSpace(s.cfg.WSSCertPath)
		keyPath := strings.TrimSpace(s.cfg.WSSKeyPath)
		if certPath == "" || keyPath == "" {
			return nil, fmt.Errorf("transport profile %q requires secure websocket certificate and key paths", definition.Profile)
		}
		return []wakuNode.WakuNodeOption{
			wakuNode.WithSecureWebsockets(secureWebsocketAddress(hostAddr, s.cfg), s.cfg.WSSPort, certPath, keyPath),
		}, nil
	default:
		return nil, fmt.Errorf("transport profile %q is not implemented", definition.Profile)
	}
}

func secureWebsocketAddress(hostAddr *net.TCPAddr, cfg Config) string {
	addr := hostAddr.IP.String()
	if strings.TrimSpace(addr) == "" || addr == "<nil>" {
		addr = bindAddress(cfg)
	}
	return addr
}

func advertisedEndpoints(endpoints []string, cfg Config) []string {
	if networkreadiness.NormalizeProfile(cfg.Profile) != networkreadiness.ProfileTCPWSS {
		return endpoints
	}
	hostPrefix := multiaddrHostPrefix(strings.TrimSpace(cfg.WSSAdvertiseAddress))
	portMarker := "/tcp/" + strconv.Itoa(cfg.WSSPort)
	for index, endpoint := range endpoints {
		if !isSecureWebsocketEndpoint(endpoint) {
			continue
		}
		if marker := strings.Index(endpoint, portMarker); marker >= 0 {
			endpoints[index] = hostPrefix + endpoint[marker:]
		}
	}
	return endpoints
}

func multiaddrHostPrefix(host string) string {
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() != nil {
			return "/ip4/" + host
		}
		return "/ip6/" + host
	}
	return "/dns4/" + host
}

func isSecureWebsocketEndpoint(endpoint string) bool {
	return strings.Contains(endpoint, "/wss") || strings.Contains(endpoint, "/tls/ws")
}

func bindAddress(cfg Config) string {
	return networkparticipation.BindAddress(cfg.BindAddress, BindAddressEnv)
}

func listenPort(cfg Config) int {
	return networkparticipation.ListenPort(cfg.ListenPort)
}

func (s *Service) dialBootstrapPeers(parent context.Context) networkreadiness.BootstrapStatus {
	s.mu.Lock()
	node := s.node
	peers := s.effectiveBootstrapNodesLocked()
	observer := s.bootstrapObs
	s.lastBootstrapAttempt = timeNowUTC()
	s.mu.Unlock()
	if node == nil {
		return networkreadiness.BootstrapStatus{State: "degraded", Reason: "transport node is not started"}
	}
	attempts, failures, lastErr := 0, 0, ""
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	for _, addr := range peers {
		addr, ok := networkpeer.Normalize(addr)
		if !ok {
			continue
		}
		attempts++
		err := node.DialPeer(ctx, addr)
		s.observeEndpoint(addr, err == nil, err)
		lastErr = networkreadiness.UpdateBootstrapFailures(err, failures, &failures, lastErr)
		s.reportBootstrapDial(observer, addr, err)
	}
	if s.cfg.NodeProfile == networkreadiness.NodeProfileConstrainedClient {
		providers := networkmessaging.InspectLightProviders(node)
		return networkreadiness.ClassifyLightBootstrapStatus(
			providers.FilterPeers, providers.LightpushPeers, providers.StorePeers, attempts, failures,
		)
	}
	relayPeers := networkmessaging.AwaitRelayPeerCount(ctx, node, networkmessaging.DefaultPubsubTopic)
	return networkreadiness.ClassifyBootstrapStatus(relayPeers, attempts, failures, lastErr)
}

func (s *Service) reportBootstrapDial(observer func(networkreadiness.BootstrapDialReport), addr string, err error) {
	report := networkreadiness.BootstrapDialReport{Peer: addr, Success: err == nil}
	if err != nil {
		report.Detail = err.Error()
	}
	if observer != nil {
		go observer(report)
	}
}

func (s *Service) observeEndpoint(addr string, usable bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := endpointObservation{usable: usable}
	if err != nil {
		state.reason = err.Error()
	}
	s.observed[addr] = state
}
