package waku

import (
	"ardents/internal/network"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p"
	libp2pevent "github.com/libp2p/go-libp2p/core/event"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
)

const maxAdvertiseAddresses = 1

func initialReachability(mode network.ReachabilityMode) network.ReachabilitySnapshot {
	mode = network.NormalizeReachabilityMode(mode)
	state, reason := "unknown", "ingress reachability has not been observed"
	switch mode {
	case network.ReachabilityLocalOnly:
		state, reason = "local", "listener is restricted to the local host"
	case network.ReachabilityOutboundOnly:
		state, reason = "outbound_only", "inbound endpoint publication is disabled"
	}
	return network.ReachabilitySnapshot{Mode: mode, State: state, Reason: reason}
}

func (s *Service) ReachabilitySnapshot() network.ReachabilitySnapshot {
	s.mu.Lock()
	changed := s.expirePrivateLANProbeLocked(timeNowUTC())
	snapshot := s.reachability
	observer := s.reachabilityObs
	s.mu.Unlock()
	if changed && observer != nil {
		go observer()
	}
	return snapshot
}

func (s *Service) SetReachabilityObserver(observer func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reachabilityObs = observer
}

func validateReachabilityConfig(cfg network.Config) error {
	nodeProfile := network.NormalizeNodeProfile(cfg.NodeProfile)
	mode := network.ReachabilityModeForProfile(cfg.ReachabilityMode, nodeProfile)
	if !network.ValidReachabilityMode(mode) {
		return fmt.Errorf("unsupported reachability mode %q", mode)
	}
	switch nodeProfile {
	case network.NodeProfileLocalDevelopment:
		if mode != network.ReachabilityLocalOnly {
			return fmt.Errorf("node profile %q requires reachability mode %q", nodeProfile, network.ReachabilityLocalOnly)
		}
	case network.NodeProfileConstrainedClient:
		if mode != network.ReachabilityOutboundOnly {
			return fmt.Errorf("node profile %q requires reachability mode %q", nodeProfile, network.ReachabilityOutboundOnly)
		}
	case network.NodeProfileServiceNode:
		if mode == network.ReachabilityLocalOnly {
			return fmt.Errorf("node profile %q does not allow reachability mode %q", nodeProfile, mode)
		}
	default:
		return fmt.Errorf("unsupported node profile %q", nodeProfile)
	}
	if len(cfg.AdvertiseAddresses) > maxAdvertiseAddresses {
		return fmt.Errorf("reachability accepts at most %d advertised addresses", maxAdvertiseAddresses)
	}
	if mode == network.ReachabilityPrivateLAN && len(cfg.AdvertiseAddresses) != 1 {
		return fmt.Errorf("reachability mode %q requires exactly one private advertised address", mode)
	}
	if mode != network.ReachabilityPublicDirect && mode != network.ReachabilityPrivateLAN &&
		len(cfg.AdvertiseAddresses) > 0 {
		return fmt.Errorf("reachability mode %q does not accept public advertised addresses", mode)
	}
	if mode == network.ReachabilityPublicDirect && len(cfg.AdvertiseAddresses) == 0 {
		return fmt.Errorf("reachability mode %q requires at least one public advertised address", mode)
	}
	if mode == network.ReachabilityPublicDirect && len(cfg.AdvertiseAddresses) > 1 {
		return fmt.Errorf("reachability mode %q requires exactly one public advertised address", mode)
	}
	seen := make(map[string]struct{}, len(cfg.AdvertiseAddresses))
	for _, raw := range cfg.AdvertiseAddresses {
		address := strings.TrimSpace(raw)
		parsed, err := ma.NewMultiaddr(address)
		if err != nil || !strings.Contains(address, "/tcp/") || strings.Contains(address, "/p2p/") ||
			strings.Contains(address, "/p2p-circuit") {
			return fmt.Errorf("advertised address must be a TCP multiaddr without a peer ID")
		}
		switch mode {
		case network.ReachabilityPrivateLAN:
			if !privateLiteralMultiaddrHost(parsed) {
				return fmt.Errorf("private advertised address must use a private literal IP")
			}
		case network.ReachabilityPublicDirect:
			if !publicMultiaddrHost(parsed) {
				return fmt.Errorf("public advertised address must use a public IP or DNS name")
			}
		}
		if !allowedDiscoveredAddress(address, network.NormalizeProfile(cfg.Profile)) {
			return fmt.Errorf("advertised address is incompatible with the active transport profile")
		}
		if isSecureWebsocketEndpoint(address) && !matchesWSSAdvertisement(parsed, cfg) {
			return fmt.Errorf("public secure websocket address must match the configured certificate host and port")
		}
		if _, exists := seen[address]; exists {
			return fmt.Errorf("public advertised addresses must be unique")
		}
		seen[address] = struct{}{}
	}
	return nil
}

func matchesWSSAdvertisement(address ma.Multiaddr, cfg network.Config) bool {
	host := multiaddrHost(address)
	port, err := address.ValueForProtocol(ma.P_TCP)
	return err == nil && strings.EqualFold(host, strings.TrimSpace(cfg.WSSAdvertiseAddress)) &&
		port == fmt.Sprintf("%d", cfg.WSSPort)
}

func privateLiteralMultiaddrHost(address ma.Multiaddr) bool {
	for _, protocol := range []int{ma.P_IP4, ma.P_IP6} {
		raw, err := address.ValueForProtocol(protocol)
		if err == nil {
			ip := net.ParseIP(raw)
			return ip != nil && ip.IsPrivate() && !ip.IsUnspecified() &&
				!ip.IsLoopback()
		}
	}
	return false
}

func multiaddrHost(address ma.Multiaddr) string {
	for _, protocol := range []int{ma.P_IP4, ma.P_IP6, ma.P_DNS, ma.P_DNS4, ma.P_DNS6} {
		if raw, err := address.ValueForProtocol(protocol); err == nil {
			return raw
		}
	}
	return ""
}

func publicMultiaddrHost(address ma.Multiaddr) bool {
	for _, protocol := range []int{ma.P_IP4, ma.P_IP6} {
		raw, err := address.ValueForProtocol(protocol)
		if err == nil {
			ip := net.ParseIP(raw)
			return ip != nil && !ip.IsUnspecified() && !ip.IsLoopback() && !ip.IsPrivate()
		}
	}
	for _, protocol := range []int{ma.P_DNS, ma.P_DNS4, ma.P_DNS6} {
		if raw, err := address.ValueForProtocol(protocol); err == nil {
			return strings.TrimSpace(raw) != "" && !strings.EqualFold(raw, "localhost")
		}
	}
	return false
}

func (s *Service) setReachabilityLocked(state, reason string, reachable bool, now time.Time) {
	s.reachability.State = state
	s.reachability.Reason = reason
	s.reachability.Reachable = reachable
	s.reachability.ObservedAt = now.UTC()
}

func (s *Service) markBoundReachabilityLocked() {
	if len(s.endpoints) == 0 {
		return
	}
	switch s.reachability.Mode {
	case network.ReachabilityLocalOnly:
		s.setReachabilityLocked("local", "local listener is active", true, timeNowUTC())
	}
}

func (s *Service) publishableEndpointsLocked() []string {
	s.expirePrivateLANProbeLocked(timeNowUTC())
	if (s.reachability.Mode == network.ReachabilityPublicDirect ||
		s.reachability.Mode == network.ReachabilityPrivateLAN) &&
		len(s.cfg.AdvertiseAddresses) > 0 && !s.reachability.Reachable {
		return nil
	}
	return s.endpoints
}

// ApplyPrivateLANProbe admits or withdraws the exact translated-host endpoint
// after one bounded observation from a distinct topology slot.
func (s *Service) ApplyPrivateLANProbe(probe network.PrivateLANProbe) error {
	now := timeNowUTC()
	s.mu.Lock()
	if s.reachability.Mode != network.ReachabilityPrivateLAN {
		s.mu.Unlock()
		return fmt.Errorf("private LAN probe is incompatible with reachability mode")
	}
	if strings.TrimSpace(probe.SourceSlot) == "" ||
		strings.TrimSpace(probe.TargetSlot) == "" ||
		!validPrivateLANProbeSlot(probe.SourceSlot) ||
		!validPrivateLANProbeSlot(probe.TargetSlot) ||
		probe.SourceSlot == probe.TargetSlot {
		s.mu.Unlock()
		return fmt.Errorf("private LAN probe requires distinct source and target slots")
	}
	if len(probe.ManifestDigest) != 64 ||
		probe.ManifestDigest != s.cfg.PrivateLANManifestDigest ||
		probe.TargetSlot != s.cfg.PrivateLANTargetSlot ||
		!privateLANProbeSourceAllowed(probe.SourceSlot, s.cfg.PrivateLANSourceSlots) {
		s.mu.Unlock()
		return fmt.Errorf("private LAN probe is outside the admitted topology scope")
	}
	if len(s.cfg.AdvertiseAddresses) != 1 ||
		strings.TrimSpace(probe.Address) != strings.TrimSpace(s.cfg.AdvertiseAddresses[0]) {
		s.mu.Unlock()
		return fmt.Errorf("private LAN probe address does not match configured advertisement")
	}
	observedAt := probe.ObservedAt.UTC()
	if observedAt.IsZero() || observedAt.Before(now.Add(-network.PrivateLANProbeMaxAge)) ||
		observedAt.After(now.Add(network.PrivateLANProbeFutureSkew)) {
		s.mu.Unlock()
		return fmt.Errorf("private LAN probe observation is outside the freshness window")
	}
	if probe.Success {
		s.privateLANProbeUntil = observedAt.Add(network.PrivateLANProbeMaxAge)
		s.setReachabilityLocked(
			"lan",
			"cross-host LAN probe succeeded; public ingress is not claimed",
			true,
			observedAt,
		)
	} else {
		s.privateLANProbeUntil = time.Time{}
		s.setReachabilityLocked(
			"unknown",
			"cross-host LAN probe failed",
			false,
			observedAt,
		)
	}
	s.reconcileRuntimeLocked(now)
	observer := s.reachabilityObs
	s.mu.Unlock()
	if observer != nil {
		go observer()
	}
	return nil
}

func privateLANProbeSourceAllowed(source string, allowed []string) bool {
	if len(allowed) != 2 {
		return false
	}
	for _, candidate := range allowed {
		if candidate == source {
			return true
		}
	}
	return false
}

func validPrivateLANProbeSlot(value string) bool {
	if len(value) < 1 || len(value) > 32 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		current := value[index]
		if (current < 'a' || current > 'z') &&
			(current < '0' || current > '9') && current != '-' {
			return false
		}
	}
	return true
}

func (s *Service) expirePrivateLANProbeLocked(now time.Time) bool {
	if s.reachability.Mode != network.ReachabilityPrivateLAN ||
		s.privateLANProbeUntil.IsZero() || now.Before(s.privateLANProbeUntil) {
		return false
	}
	s.privateLANProbeUntil = time.Time{}
	s.setReachabilityLocked(
		"unknown",
		"cross-host LAN probe expired",
		false,
		now,
	)
	return true
}

func reachabilityLibP2POptions(cfg network.Config) ([]libp2p.Option, error) {
	mode := network.NormalizeReachabilityMode(cfg.ReachabilityMode)
	if mode != network.ReachabilityPublicDirect {
		return nil, nil
	}
	addresses := make([]ma.Multiaddr, 0, len(cfg.AdvertiseAddresses))
	for _, raw := range cfg.AdvertiseAddresses {
		address, err := ma.NewMultiaddr(strings.TrimSpace(raw))
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}
	return []libp2p.Option{
		libp2p.AddrsFactory(func(bound []ma.Multiaddr) []ma.Multiaddr {
			return append(append([]ma.Multiaddr(nil), bound...), addresses...)
		}),
		libp2p.EnableNATService(),
		libp2p.AutoNATServiceRateLimit(30, 3, time.Minute),
	}, nil
}

func (s *Service) startReachabilityObservationLocked(node *wakuNode.WakuNode) error {
	s.reachability = initialReachability(s.cfg.ReachabilityMode)
	s.privateLANProbeUntil = time.Time{}
	if s.reachability.Mode != network.ReachabilityPublicDirect {
		return nil
	}
	subscription, err := node.Host().EventBus().Subscribe(new(libp2pevent.EvtLocalReachabilityChanged))
	if err != nil {
		return fmt.Errorf("reachability observation unavailable")
	}
	s.reachabilityEvents = subscription
	return nil
}

func (s *Service) applyReachabilityEventLocked(value libp2pnetwork.Reachability, now time.Time) {
	if s.reachability.Mode != network.ReachabilityPublicDirect {
		return
	}
	switch value {
	case libp2pnetwork.ReachabilityPublic:
		s.setReachabilityLocked("public", "public ingress was verified by peer dialback", true, now)
	case libp2pnetwork.ReachabilityPrivate:
		s.setReachabilityLocked("nat_blocked", "public ingress dialback failed", false, now)
	default:
		s.setReachabilityLocked("unknown", "public ingress has not been verified", false, now)
	}
}

func publishedEndpoints(node *wakuNode.WakuNode, cfg network.Config) []string {
	mode := network.NormalizeReachabilityMode(cfg.ReachabilityMode)
	if mode == network.ReachabilityOutboundOnly {
		return nil
	}
	if mode == network.ReachabilityPublicDirect ||
		mode == network.ReachabilityPrivateLAN && len(cfg.AdvertiseAddresses) > 0 {
		addresses := make([]ma.Multiaddr, 0, len(cfg.AdvertiseAddresses))
		for _, raw := range cfg.AdvertiseAddresses {
			address, err := ma.NewMultiaddr(strings.TrimSpace(raw))
			if err == nil {
				addresses = append(addresses, address)
			}
		}
		withPeer, err := peer.AddrInfoToP2pAddrs(&peer.AddrInfo{ID: node.Host().ID(), Addrs: addresses})
		if err == nil {
			return stringifyListenAddresses(withPeer)
		}
		return nil
	}
	return advertisedEndpoints(stringifyListenAddresses(node.ListenAddresses()), cfg)
}

func (s *Service) restartForActiveMode(ctx context.Context) bool {
	_, stopOld, ok := s.detachForModeRestart()
	if !ok {
		return true
	}
	if err := stopOld(); err != nil {
		s.recordModeRestartFailure(nil, err)
		return false
	}
	if ctx.Err() != nil {
		return false
	}
	node, err := s.startModeNode(ctx)
	if err != nil {
		s.recordModeRestartFailure(node, err)
		return false
	}
	s.completeModeRestart(ctx, node)
	return true
}

func (s *Service) detachForModeRestart() (*wakuNode.WakuNode, func() error, bool) {
	s.mu.Lock()
	if !s.modeRestartPending || s.node == nil {
		s.mu.Unlock()
		return nil, nil, false
	}
	oldNode := s.node
	oldEvents := s.reachabilityEvents
	s.modeRestartPending = false
	s.node = nil
	s.reachabilityEvents = nil
	s.endpoints = nil
	s.state = "starting"
	s.reason = "applying network participation mode"
	s.mu.Unlock()
	stop := func() error {
		if oldEvents != nil {
			if err := oldEvents.Close(); err != nil {
				return fmt.Errorf("close reachability events for mode restart: %w", err)
			}
		}
		oldNode.Stop()
		return nil
	}
	return oldNode, stop, true
}

func (s *Service) startModeNode(ctx context.Context) (*wakuNode.WakuNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, err := s.prepareNodeLocked()
	if err == nil {
		err = StartWakuNode(ctx, node, network.DefaultPubsubTopic, s.relayEnabledLocked())
	}
	if err == nil {
		err = s.startReachabilityObservationLocked(node)
	}
	return node, err
}

func (s *Service) completeModeRestart(ctx context.Context, node *wakuNode.WakuNode) {
	s.mu.Lock()
	s.markModeRestartedLocked(node)
	s.markBoundReachabilityLocked()
	s.mu.Unlock()

	s.refreshDNSPeersObserved(ctx)
	status := s.dialBootstrapPeers(ctx)
	s.mu.Lock()
	s.bootstrap = status
	s.reconcileRuntimeLocked(timeNowUTC())
	s.mu.Unlock()
}

func (s *Service) markModeRestartedLocked(node *wakuNode.WakuNode) {
	s.node = node
	s.endpoints = publishedEndpoints(s.node, s.cfg)
	s.observed = newEndpointObservations(s.endpoints, true)
	s.state = "ready"
	s.reason = ""
}

func (s *Service) recordModeRestartFailure(node *wakuNode.WakuNode, err error) {
	s.mu.Lock()
	s.state = "failed"
	s.reason = fmt.Sprintf("network participation mode restart failed: %v", err)
	s.mu.Unlock()
	if node != nil {
		node.Stop()
	}
}
