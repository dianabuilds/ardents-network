package waku

import (
	"ardents/internal/network"
	"context"
	"time"

	libp2pevent "github.com/libp2p/go-libp2p/core/event"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
)

var timeNowUTC = func() time.Time {
	return time.Now().UTC()
}

func (s *Service) reconcileRuntimeLocked(now time.Time) {
	status := s.currentBootstrapStatusViewLocked()
	s.bootstrap = status
	raw := baseRuntimeAssessment(s.readinessStateLocked(status))
	if s.controller == nil {
		s.controller = network.NewModeController(network.DefaultSelectionPolicy())
	}
	decision := s.controller.Evaluate(now, s.activeMode, raw.Health())
	if decision.Changed {
		s.activeMode = decision.Mode
		s.modeRestartPending = s.cfg.NodeProfile != network.NodeProfileConstrainedClient && s.node != nil
		s.switchReason = decision.Reason
		s.switchAuto = decision.Automatic
		s.recoveryState = decision.RecoveryState
		return
	}
	if s.switchReason == "" || raw.Health() == network.HealthStateFailed || raw.Health() == network.HealthStateStopped || raw.Health() == network.HealthStateStarting {
		s.switchReason = raw.SwitchReason()
		s.switchAuto = raw.SwitchAutomatic()
	}
	if decision.RecoveryState != "" {
		s.recoveryState = decision.RecoveryState
	} else {
		s.recoveryState = raw.RecoveryState()
	}
}

func (s *Service) startRuntimeLoopLocked() {
	if s.runtimeCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.runtimeCancel = cancel
	s.runtimeDone = done
	go s.runRuntimeLoop(ctx, done)
}

func (s *Service) runRuntimeLoop(ctx context.Context, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		s.mu.Lock()
		reachabilityEvents := s.reachabilityEventChannelLocked()
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case raw, ok := <-reachabilityEvents:
			if !ok {
				s.handleReachabilityEventsClosed(reachabilityEvents)
				continue
			}
			s.handleReachabilityEvent(raw)
		case <-ticker.C:
			if !s.handleRuntimeTick(ctx) {
				return
			}
		}
	}
}

func (s *Service) handleReachabilityEventsClosed(closed <-chan any) {
	s.mu.Lock()
	if s.reachabilityEvents == nil || s.reachabilityEvents.Out() != closed {
		s.mu.Unlock()
		return
	}
	s.reachabilityEvents = nil
	s.applyReachabilityEventLocked(libp2pnetwork.ReachabilityUnknown, timeNowUTC())
	s.reachability.Reason = "public ingress observation stream closed"
	s.reconcileRuntimeLocked(timeNowUTC())
	observer := s.reachabilityObs
	s.mu.Unlock()
	if observer != nil {
		go observer()
	}
}

func (s *Service) reachabilityEventChannelLocked() <-chan any {
	if s.reachabilityEvents == nil {
		return nil
	}
	return s.reachabilityEvents.Out()
}

func (s *Service) handleReachabilityEvent(raw any) {
	event, ok := raw.(libp2pevent.EvtLocalReachabilityChanged)
	if !ok {
		return
	}
	s.mu.Lock()
	s.applyReachabilityEventLocked(event.Reachability, timeNowUTC())
	s.reconcileRuntimeLocked(timeNowUTC())
	observer := s.reachabilityObs
	s.mu.Unlock()
	if observer != nil {
		go observer()
	}
}

func (s *Service) handleRuntimeTick(ctx context.Context) bool {
	now := timeNowUTC()
	s.mu.Lock()
	if s.node == nil {
		s.mu.Unlock()
		return false
	}
	reachabilityChanged := s.expirePrivateLANProbeLocked(now)
	s.reconcileRuntimeLocked(now)
	restartMode := s.modeRestartPending
	refreshDNS := s.shouldRefreshDNSLocked(now)
	retryBootstrap := s.shouldRetryBootstrapLocked(now)
	observer := s.reachabilityObs
	s.mu.Unlock()
	if reachabilityChanged && observer != nil {
		go observer()
	}
	if restartMode {
		return s.restartForActiveMode(ctx)
	}
	if refreshDNS {
		s.refreshDNSPeersObserved(ctx)
		retryBootstrap = true
	}
	if retryBootstrap {
		s.applyBootstrapRetry(s.dialBootstrapPeers(ctx))
	}
	return true
}

func (s *Service) applyBootstrapRetry(status network.BootstrapStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.node != nil {
		s.bootstrap = status
		s.reconcileRuntimeLocked(timeNowUTC())
	}
}

func (s *Service) ProfileSnapshot() network.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ProfileSnapshot(s.readinessStateLocked(s.currentBootstrapStatusViewLocked()))
}

func (s *Service) HealthSignals() network.HealthSignals {
	s.mu.Lock()
	defer s.mu.Unlock()
	return HealthSnapshot(s.readinessStateLocked(s.currentBootstrapStatusViewLocked()))
}

func (s *Service) serviceStateLocked() ServicePartSnapshot {
	return BuildServicePartSnapshot(s.readinessStateLocked(s.currentBootstrapStatusViewLocked()))
}

func (s *Service) currentBootstrapStatusViewLocked() network.BootstrapStatus {
	sources := s.effectiveBootstrapNodesLocked()
	status := s.currentParticipationStatusLocked(sources)
	if s.dnsDiscoveryError != "" && status.State != "ready" {
		return network.BootstrapStatus{State: "degraded", Reason: s.dnsDiscoveryError}
	}
	return status
}

func (s *Service) currentParticipationStatusLocked(sources []string) network.BootstrapStatus {
	if s.cfg.NodeProfile != network.NodeProfileConstrainedClient {
		return CurrentBootstrapStatus(s.node, sources, s.bootstrap)
	}
	if len(sources) == 0 {
		return network.BootstrapStatus{State: "idle", Reason: "no network bootstrap sources"}
	}
	providers := InspectLightProviders(s.node)
	if providers.Ready() {
		return network.BootstrapStatus{Joined: true, State: "ready"}
	}
	reason := s.bootstrap.Reason
	if reason == "" || s.bootstrap.State == "ready" {
		reason = "bootstrap peers do not provide required Filter, Lightpush, and Store protocols"
	}
	return network.BootstrapStatus{State: "degraded", Reason: reason}
}

func (s *Service) effectiveBootstrapNodesLocked() []string {
	return mergeUniqueStrings(s.bootstrapNodes, s.discoveredNodes)
}

func (s *Service) readinessStateLocked(status network.BootstrapStatus) ServiceState {
	state := ServiceState{
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
	storePressure := network.AbuseSnapshot{State: "ready"}
	s.populateStorePressureLocked(&storePressure)
	state.StorePressureState = storePressure.State
	state.StorePressureReason = storePressure.Reason
	for _, endpoint := range s.observed {
		if endpoint.usable {
			state.ObservedUsable++
		}
	}
	if s.node != nil {
		state.NodePeerCount = s.node.PeerCount()
		if s.cfg.NodeProfile != network.NodeProfileConstrainedClient {
			state.NodeRelayCount = len(s.node.Relay().PubSub().ListPeers(network.DefaultPubsubTopic))
		} else {
			providers := InspectLightProviders(s.node)
			state.FilterPeerCount = providers.FilterPeers
			state.LightpushPeerCount = providers.LightpushPeers
			state.StorePeerCount = providers.StorePeers
		}
	}
	return state
}
