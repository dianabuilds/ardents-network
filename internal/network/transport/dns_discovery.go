package transport

import (
	"context"
	"fmt"
	"strings"
	"time"

	networkreadiness "ardents/internal/network/readiness"

	gethdns "github.com/ethereum/go-ethereum/p2p/dnsdisc"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/waku-org/go-waku/waku/v2/dnsdisc"
)

const (
	dnsRefreshInterval = 5 * time.Minute
	dnsFailureRetry    = 10 * time.Second
	dnsRefreshTimeout  = 10 * time.Second
	maxDiscoveredPeers = 128
)

type dnsPeerDiscovery interface {
	Retrieve(context.Context, []string, string, networkreadiness.Profile) ([]string, error)
}

type wakuDNSPeerDiscovery struct {
	resolver gethdns.Resolver
}

func (w wakuDNSPeerDiscovery) Retrieve(
	ctx context.Context, urls []string, nameserver string, profile networkreadiness.Profile,
) ([]string, error) {
	var options []dnsdisc.DNSDiscoveryOption
	if w.resolver != nil {
		options = append(options, dnsdisc.WithResolver(w.resolver))
	} else if strings.TrimSpace(nameserver) != "" {
		options = append(options, dnsdisc.WithNameserver(strings.TrimSpace(nameserver)))
	}
	peers := make([]string, 0)
	seen := map[string]struct{}{}
	var lastErr error
	for _, url := range urls {
		nodes, err := dnsdisc.RetrieveNodes(ctx, strings.TrimSpace(url), options...)
		if err != nil {
			lastErr = err
			continue
		}
		peers = appendDiscoveredNodes(peers, seen, nodes, profile)
		if len(peers) == maxDiscoveredPeers {
			return peers, nil
		}
	}
	if len(peers) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return peers, nil
}

func appendDiscoveredNodes(
	peers []string, seen map[string]struct{}, nodes []dnsdisc.DiscoveredNode, profile networkreadiness.Profile,
) []string {
	for _, node := range nodes {
		addresses, err := peer.AddrInfoToP2pAddrs(&node.PeerInfo)
		if err != nil {
			continue
		}
		for _, address := range addresses {
			candidate := address.String()
			if !allowedDiscoveredAddress(candidate, profile) {
				continue
			}
			if _, exists := seen[candidate]; exists {
				continue
			}
			seen[candidate] = struct{}{}
			peers = append(peers, candidate)
			if len(peers) == maxDiscoveredPeers {
				return peers
			}
		}
	}
	return peers
}

func allowedDiscoveredAddress(address string, profile networkreadiness.Profile) bool {
	if !strings.Contains(address, "/tcp/") || strings.Contains(address, "/udp/") {
		return false
	}
	if networkreadiness.NormalizeProfile(profile) == networkreadiness.ProfileTCPOnly {
		return !strings.Contains(address, "/ws") && !strings.Contains(address, "/wss")
	}
	if strings.Contains(address, "/ws") && !isSecureWebsocketEndpoint(address) {
		return false
	}
	return true
}

func (s *Service) refreshDNSPeers(parent context.Context) error {
	s.mu.Lock()
	urls := cloneStrings(s.cfg.DNSDiscoveryURLs)
	nameserver := s.cfg.DNSDiscoveryNameServer
	resolver := s.dnsDiscovery
	profile := s.activeProfile
	s.lastDNSRefresh = timeNowUTC()
	s.mu.Unlock()
	if len(urls) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(parent, dnsRefreshTimeout)
	defer cancel()
	peers, err := resolver.Retrieve(ctx, urls, nameserver, profile)
	if err == nil && len(peers) == 0 {
		err = fmt.Errorf("DNS discovery returned no usable peers")
	}
	s.mu.Lock()
	var removed []string
	if err != nil {
		removed = s.replaceDiscoveredNodesLocked(nil)
		s.dnsDiscoveryError = "bootstrap source discovery failed"
	} else {
		removed = s.replaceDiscoveredNodesLocked(peers)
		s.dnsDiscoveryError = ""
	}
	node := s.node
	s.mu.Unlock()
	if node != nil {
		closeRemovedDNSPeers(node, removed)
	}
	return err
}

func (s *Service) replaceDiscoveredNodesLocked(peers []string) []string {
	retained := mergeUniqueStrings(s.bootstrapNodes, peers)
	retainedSet := make(map[string]struct{}, len(retained))
	for _, peer := range retained {
		retainedSet[peer] = struct{}{}
	}
	removed := make([]string, 0)
	for _, oldPeer := range s.discoveredNodes {
		if _, keep := retainedSet[oldPeer]; !keep {
			delete(s.observed, oldPeer)
			removed = append(removed, oldPeer)
		}
	}
	s.discoveredNodes = cloneStrings(peers)
	return removed
}

func closeRemovedDNSPeers(node interface{ ClosePeerByAddress(string) error }, peers []string) {
	if node == nil {
		return
	}
	for _, address := range peers {
		_ = node.ClosePeerByAddress(address)
	}
}

func (s *Service) shouldRefreshDNSLocked(now time.Time) bool {
	if s.node == nil || len(s.cfg.DNSDiscoveryURLs) == 0 {
		return false
	}
	interval := dnsRefreshInterval
	if s.dnsDiscoveryError != "" {
		interval = dnsFailureRetry
	}
	return s.lastDNSRefresh.IsZero() || now.Sub(s.lastDNSRefresh) >= interval
}
