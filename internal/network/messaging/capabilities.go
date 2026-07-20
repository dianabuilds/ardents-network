package messaging

import (
	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
	"github.com/waku-org/go-waku/waku/v2/protocol/filter"
	legacyStore "github.com/waku-org/go-waku/waku/v2/protocol/legacy_store"
	"github.com/waku-org/go-waku/waku/v2/protocol/lightpush"
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
		if protocol, _ := node.Host().Peerstore().FirstSupportedProtocol(id, filter.FilterSubscribeID_v20beta1); protocol != "" {
			status.FilterPeers++
		}
		if protocol, _ := node.Host().Peerstore().FirstSupportedProtocol(id, lightpush.LightPushID_v20beta1); protocol != "" {
			status.LightpushPeers++
		}
		if protocol, _ := node.Host().Peerstore().FirstSupportedProtocol(id, legacyStore.StoreID_v20beta4); protocol != "" {
			status.StorePeers++
		}
	}
	return status
}
