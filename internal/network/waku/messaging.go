package waku

import (
	"ardents/internal/network"
	"time"

	"github.com/waku-org/go-waku/waku/persistence"
	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
	"github.com/waku-org/go-waku/waku/v2/protocol/filter"
	legacyStore "github.com/waku-org/go-waku/waku/v2/protocol/legacy_store"
	"github.com/waku-org/go-waku/waku/v2/protocol/lightpush"
	"golang.org/x/time/rate"
)

type LightProviderStatus struct {
	FilterPeers    int
	LightpushPeers int
	StorePeers     int
}

func (s LightProviderStatus) Ready() bool {
	return s.FilterPeers > 0 && s.LightpushPeers > 0 && s.StorePeers > 0
}

func InspectLightProviders(node *wakuNode.WakuNode) LightProviderStatus {
	if node == nil || node.Host() == nil {
		return LightProviderStatus{}
	}
	var status LightProviderStatus
	peers := node.Host().Network().Peers()
	for _, id := range peers {
		if protocol, err := node.Host().Peerstore().FirstSupportedProtocol(id, filter.FilterSubscribeID_v20beta1); err == nil && protocol != "" {
			status.FilterPeers++
		}
		if protocol, err := node.Host().Peerstore().FirstSupportedProtocol(id, lightpush.LightPushID_v20beta1); err == nil && protocol != "" {
			status.LightpushPeers++
		}
		if protocol, err := node.Host().Peerstore().FirstSupportedProtocol(id, legacyStore.StoreID_v20beta4); err == nil && protocol != "" {
			status.StorePeers++
		}
	}
	return status
}

func (s *Service) prepareMessageProviderLocked() (*persistence.DBStore, bool, error) {
	if s.cfg.NodeProfile == network.NodeProfileConstrainedClient ||
		s.activeMode == network.ModeRestrictedDefense {
		s.messageProvider = nil
		return nil, false, nil
	}
	existed, err := MessageProviderExists(s.cfg.StorePath)
	if err != nil {
		return nil, false, err
	}
	provider, err := NewMessageProvider(s.cfg.StorePath, network.StoreRetention{
		MaxMessages: s.cfg.Limits.StoreMaxMessages,
		MaxAge:      time.Duration(s.cfg.Limits.StoreMaxAgeSeconds) * time.Second,
		MaxBytes:    s.cfg.Limits.StoreMaxBytes,
	})
	if err == nil {
		s.messageProvider = provider
	}
	return provider, existed, err
}

func (s *Service) messagingNodeOptions(provider *persistence.DBStore) []wakuNode.WakuNodeOption {
	common := []wakuNode.WakuNodeOption{
		wakuNode.WithMaxMsgSize(s.cfg.Limits.MaxMessageBytes),
		wakuNode.WithMaxPeerConnections(s.cfg.Limits.MaxPeerConnections),
		wakuNode.WithMaxConnectionsPerIP(s.cfg.Limits.MaxConnectionsPerIP),
		wakuNode.WithWakuStoreRateLimit(rate.Limit(s.cfg.Limits.OperationRate)),
	}
	if s.cfg.NodeProfile == network.NodeProfileConstrainedClient {
		return append(common, wakuNode.WithWakuFilterLightNode(), wakuNode.WithLightPush())
	}
	if s.activeMode == network.ModeRestrictedDefense {
		return append(common, wakuNode.WithWakuRelay())
	}
	return append(common,
		wakuNode.WithWakuRelay(), wakuNode.WithMessageProvider(provider), wakuNode.WithWakuStore(),
		wakuNode.WithWakuFilterFullNode(
			filter.WithMaxSubscribers(s.cfg.Limits.MaxFilterSubscribers),
			filter.WithFullNodeRateLimiter(rate.Limit(s.cfg.Limits.OperationRate), s.cfg.Limits.OperationBurst),
		),
		wakuNode.WithLightPush(
			lightpush.WithRateLimiter(rate.Limit(s.cfg.Limits.OperationRate), s.cfg.Limits.OperationBurst),
		),
	)
}

func (s *Service) relayEnabledLocked() bool {
	return s.cfg.NodeProfile != network.NodeProfileConstrainedClient
}
