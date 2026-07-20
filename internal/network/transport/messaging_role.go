package transport

import (
	networkparticipation "ardents/internal/network/participation"
	networkreadiness "ardents/internal/network/readiness"

	"github.com/waku-org/go-waku/waku/persistence"
	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
	"github.com/waku-org/go-waku/waku/v2/protocol/filter"
	"github.com/waku-org/go-waku/waku/v2/protocol/lightpush"
	"golang.org/x/time/rate"
)

func (s *Service) prepareMessageProviderLocked() (*persistence.DBStore, bool, error) {
	if s.cfg.NodeProfile == networkreadiness.NodeProfileConstrainedClient ||
		s.activeMode == networkreadiness.ModeRestrictedDefense {
		return nil, false, nil
	}
	existed, err := networkparticipation.MessageProviderExists(s.cfg.StorePath)
	if err != nil {
		return nil, false, err
	}
	provider, err := networkparticipation.NewMessageProvider(s.cfg.StorePath)
	return provider, existed, err
}

func (s *Service) messagingNodeOptions(provider *persistence.DBStore) []wakuNode.WakuNodeOption {
	common := []wakuNode.WakuNodeOption{
		wakuNode.WithMaxMsgSize(s.cfg.Limits.MaxMessageBytes),
		wakuNode.WithMaxPeerConnections(s.cfg.Limits.MaxPeerConnections),
		wakuNode.WithMaxConnectionsPerIP(s.cfg.Limits.MaxConnectionsPerIP),
		wakuNode.WithWakuStoreRateLimit(rate.Limit(s.cfg.Limits.OperationRate)),
	}
	if s.cfg.NodeProfile == networkreadiness.NodeProfileConstrainedClient {
		return append(common, wakuNode.WithWakuFilterLightNode(), wakuNode.WithLightPush())
	}
	if s.activeMode == networkreadiness.ModeRestrictedDefense {
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
	return s.cfg.NodeProfile != networkreadiness.NodeProfileConstrainedClient
}
