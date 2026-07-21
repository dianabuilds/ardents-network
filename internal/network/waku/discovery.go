// Package waku owns the go-waku and libp2p implementation adapter.
// It does not own product messaging, discovery, or content decisions.
package waku

import (
	"ardents/internal/network"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

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
	Retrieve(context.Context, []string, string, network.Profile) ([]string, error)
}

type wakuDNSPeerDiscovery struct {
	resolver gethdns.Resolver
}

func (w wakuDNSPeerDiscovery) Retrieve(
	ctx context.Context, urls []string, nameserver string, profile network.Profile,
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
	peers []string, seen map[string]struct{}, nodes []dnsdisc.DiscoveredNode, profile network.Profile,
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

func allowedDiscoveredAddress(address string, profile network.Profile) bool {
	if !strings.Contains(address, "/tcp/") || strings.Contains(address, "/udp/") {
		return false
	}
	if network.NormalizeProfile(profile) == network.ProfileTCPOnly {
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
		err = errors.Join(err, closeRemovedDNSPeers(node, removed))
	}
	return err
}

func (s *Service) refreshDNSPeersObserved(parent context.Context) {
	if err := s.refreshDNSPeers(parent); err != nil {
		slog.Warn("refresh Waku DNS peers", "error", err)
	}
}

func (s *Service) replaceDiscoveredNodesLocked(peers []string) []string {
	retained := mergeUniqueStrings(s.bootstrapNodes, peers)
	retainedSet := make(map[string]struct{}, len(retained))
	for _, peerAddress := range retained {
		retainedSet[peerAddress] = struct{}{}
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

func closeRemovedDNSPeers(node interface{ ClosePeerByAddress(string) error }, peers []string) error {
	if node == nil {
		return nil
	}
	var failures []error
	for _, address := range peers {
		if err := node.ClosePeerByAddress(address); err != nil {
			failures = append(failures, fmt.Errorf("close removed DNS peer: %w", err))
		}
	}
	return errors.Join(failures...)
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

const maxDNSDiscoveryURLs = 4

func validateDiscoveryConfig(cfg network.Config) error {
	urls := cfg.DNSDiscoveryURLs
	if len(urls) > maxDNSDiscoveryURLs {
		return fmt.Errorf("DNS discovery accepts at most %d signed ENR trees", maxDNSDiscoveryURLs)
	}
	seen := make(map[string]struct{}, len(urls))
	for _, rawURL := range urls {
		url := strings.TrimSpace(rawURL)
		if _, _, err := gethdns.ParseURL(url); err != nil {
			return fmt.Errorf("DNS discovery URL must be a signed enrtree URL")
		}
		if _, exists := seen[url]; exists {
			return fmt.Errorf("DNS discovery URLs must be unique")
		}
		seen[url] = struct{}{}
	}
	nameserver := strings.TrimSpace(cfg.DNSDiscoveryNameServer)
	if nameserver != "" && len(urls) == 0 {
		return fmt.Errorf("DNS discovery nameserver requires at least one signed ENR tree")
	}
	if nameserver != "" && net.ParseIP(nameserver) == nil {
		return fmt.Errorf("DNS discovery nameserver must be an IP address without a port")
	}
	return nil
}
