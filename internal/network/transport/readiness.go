package transport

import (
	networkmessaging "ardents/internal/network/messaging"
	networkreadiness "ardents/internal/network/readiness"
)

func (s *Service) ProfileSnapshot() networkreadiness.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return networkreadiness.ProfileSnapshot(s.readinessStateLocked(s.currentBootstrapStatusViewLocked()))
}

func (s *Service) HealthSignals() networkreadiness.HealthSignals {
	s.mu.Lock()
	defer s.mu.Unlock()
	return networkreadiness.HealthSnapshot(s.readinessStateLocked(s.currentBootstrapStatusViewLocked()))
}

func (s *Service) serviceStateLocked() networkreadiness.ServicePartSnapshot {
	return networkreadiness.BuildServicePartSnapshot(s.readinessStateLocked(s.currentBootstrapStatusViewLocked()))
}

func (s *Service) currentBootstrapStatusViewLocked() networkreadiness.BootstrapStatus {
	sources := s.effectiveBootstrapNodesLocked()
	status := s.currentParticipationStatusLocked(sources)
	if s.dnsDiscoveryError != "" && status.State != "ready" {
		return networkreadiness.BootstrapStatus{State: "degraded", Reason: s.dnsDiscoveryError}
	}
	return status
}

func (s *Service) currentParticipationStatusLocked(sources []string) networkreadiness.BootstrapStatus {
	if s.cfg.NodeProfile != networkreadiness.NodeProfileConstrainedClient {
		return networkreadiness.CurrentBootstrapStatus(s.node, sources, s.bootstrap)
	}
	if len(sources) == 0 {
		return networkreadiness.BootstrapStatus{State: "idle", Reason: "no network bootstrap sources"}
	}
	providers := networkmessaging.InspectLightProviders(s.node)
	if providers.Ready() {
		return networkreadiness.BootstrapStatus{Joined: true, State: "ready"}
	}
	reason := s.bootstrap.Reason
	if reason == "" || s.bootstrap.State == "ready" {
		reason = "bootstrap peers do not provide required Filter, Lightpush, and Store protocols"
	}
	return networkreadiness.BootstrapStatus{State: "degraded", Reason: reason}
}

func (s *Service) effectiveBootstrapNodesLocked() []string {
	return mergeUniqueStrings(s.bootstrapNodes, s.discoveredNodes)
}

func (s *Service) readinessStateLocked(status networkreadiness.BootstrapStatus) networkreadiness.ServiceState {
	state := networkreadiness.ServiceState{
		NodeProfile:     s.cfg.NodeProfile,
		State:           s.state,
		Reason:          s.reason,
		BootstrapNodes:  s.effectiveBootstrapNodesLocked(),
		Endpoints:       cloneStrings(s.publishableEndpointsLocked()),
		Bootstrap:       status,
		ActiveProfile:   s.activeProfile,
		ActiveMode:      s.activeMode,
		SwitchReason:    s.switchReason,
		SwitchAutomatic: s.switchAuto,
		RecoveryState:   s.recoveryState,
		Reachability:    s.reachability,
	}
	for _, endpoint := range s.observed {
		if endpoint.usable {
			state.ObservedUsable++
		}
	}
	if s.node != nil {
		state.NodePeerCount = s.node.PeerCount()
		if s.cfg.NodeProfile != networkreadiness.NodeProfileConstrainedClient {
			state.NodeRelayCount = len(s.node.Relay().PubSub().ListPeers(networkreadiness.DefaultPubsubTopic()))
		} else {
			providers := networkmessaging.InspectLightProviders(s.node)
			state.FilterPeerCount = providers.FilterPeers
			state.LightpushPeerCount = providers.LightpushPeers
			state.StorePeerCount = providers.StorePeers
		}
	}
	return state
}
