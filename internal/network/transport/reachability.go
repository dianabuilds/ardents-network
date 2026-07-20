package transport

import (
	"fmt"
	"net"
	"strings"
	"time"

	networkreadiness "ardents/internal/network/readiness"

	"github.com/libp2p/go-libp2p"
	libp2pevent "github.com/libp2p/go-libp2p/core/event"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
)

const maxAdvertiseAddresses = 4

func initialReachability(mode networkreadiness.ReachabilityMode) networkreadiness.ReachabilitySnapshot {
	mode = networkreadiness.NormalizeReachabilityMode(mode)
	state, reason := "unknown", "ingress reachability has not been observed"
	switch mode {
	case networkreadiness.ReachabilityLocalOnly:
		state, reason = "local", "listener is restricted to the local host"
	case networkreadiness.ReachabilityOutboundOnly:
		state, reason = "outbound_only", "inbound endpoint publication is disabled"
	}
	return networkreadiness.ReachabilitySnapshot{Mode: mode, State: state, Reason: reason}
}

func (s *Service) ReachabilitySnapshot() networkreadiness.ReachabilitySnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reachability
}

func (s *Service) SetReachabilityObserver(observer func(networkreadiness.ReachabilitySnapshot)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reachabilityObs = observer
}

func validateReachabilityConfig(cfg Config) error {
	mode := networkreadiness.NormalizeReachabilityMode(cfg.ReachabilityMode)
	if !networkreadiness.ValidReachabilityMode(mode) {
		return fmt.Errorf("unsupported reachability mode %q", mode)
	}
	if len(cfg.AdvertiseAddresses) > maxAdvertiseAddresses {
		return fmt.Errorf("reachability accepts at most %d advertised addresses", maxAdvertiseAddresses)
	}
	if mode != networkreadiness.ReachabilityPublicDirect && len(cfg.AdvertiseAddresses) > 0 {
		return fmt.Errorf("reachability mode %q does not accept public advertised addresses", mode)
	}
	if mode == networkreadiness.ReachabilityPublicDirect && len(cfg.AdvertiseAddresses) == 0 {
		return fmt.Errorf("reachability mode %q requires at least one public advertised address", mode)
	}
	seen := make(map[string]struct{}, len(cfg.AdvertiseAddresses))
	for _, raw := range cfg.AdvertiseAddresses {
		address := strings.TrimSpace(raw)
		parsed, err := ma.NewMultiaddr(address)
		if err != nil || !strings.Contains(address, "/tcp/") || strings.Contains(address, "/p2p/") ||
			strings.Contains(address, "/p2p-circuit") {
			return fmt.Errorf("public advertised address must be a TCP multiaddr without a peer ID")
		}
		if !publicMultiaddrHost(parsed) {
			return fmt.Errorf("public advertised address must use a public IP or DNS name")
		}
		if !allowedDiscoveredAddress(address, networkreadiness.NormalizeProfile(cfg.Profile)) {
			return fmt.Errorf("public advertised address is incompatible with the active transport profile")
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

func matchesWSSAdvertisement(address ma.Multiaddr, cfg Config) bool {
	host := multiaddrHost(address)
	port, err := address.ValueForProtocol(ma.P_TCP)
	return err == nil && strings.EqualFold(host, strings.TrimSpace(cfg.WSSAdvertiseAddress)) &&
		port == fmt.Sprintf("%d", cfg.WSSPort)
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
	case networkreadiness.ReachabilityLocalOnly:
		s.setReachabilityLocked("local", "local listener is active", true, timeNowUTC())
	case networkreadiness.ReachabilityPrivateLAN:
		s.setReachabilityLocked("lan", "LAN listener is active; public ingress is not claimed", true, timeNowUTC())
	}
}

func (s *Service) publishableEndpointsLocked() []string {
	if s.reachability.Mode == networkreadiness.ReachabilityPublicDirect && !s.reachability.Reachable {
		return nil
	}
	return s.endpoints
}

func reachabilityLibP2POptions(cfg Config) ([]libp2p.Option, error) {
	mode := networkreadiness.NormalizeReachabilityMode(cfg.ReachabilityMode)
	if mode != networkreadiness.ReachabilityPublicDirect {
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
	if s.reachability.Mode != networkreadiness.ReachabilityPublicDirect {
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
	if s.reachability.Mode != networkreadiness.ReachabilityPublicDirect {
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

func publishedEndpoints(node *wakuNode.WakuNode, cfg Config) []string {
	mode := networkreadiness.NormalizeReachabilityMode(cfg.ReachabilityMode)
	if mode == networkreadiness.ReachabilityOutboundOnly {
		return nil
	}
	if mode == networkreadiness.ReachabilityPublicDirect {
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
