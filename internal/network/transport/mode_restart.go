package transport

import (
	"context"
	"fmt"

	networkmessaging "ardents/internal/network/messaging"
	networkparticipation "ardents/internal/network/participation"

	wakuNode "github.com/waku-org/go-waku/waku/v2/node"
)

func (s *Service) restartForActiveMode(ctx context.Context) bool {
	_, stopOld, ok := s.detachForModeRestart()
	if !ok {
		return true
	}
	stopOld()
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

func (s *Service) detachForModeRestart() (*wakuNode.WakuNode, func(), bool) {
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
	stop := func() {
		if oldEvents != nil {
			_ = oldEvents.Close()
		}
		oldNode.Stop()
	}
	return oldNode, stop, true
}

func (s *Service) startModeNode(ctx context.Context) (*wakuNode.WakuNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, err := s.prepareNodeLocked()
	if err == nil {
		err = networkparticipation.StartWakuNode(ctx, node, networkmessaging.DefaultPubsubTopic, s.relayEnabledLocked())
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

	_ = s.refreshDNSPeers(ctx)
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
