package process

import (
	"context"

	transport "ardents/internal/network/api"
)

func (n *Node) onReachabilityChanged(transport.ReachabilitySnapshot) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.cancel == nil {
		return
	}
	n.runtimeMgr.RefreshDiscoveryPublicationLocked(context.Background())
}
