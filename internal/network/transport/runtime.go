package transport

import (
	"context"
	"time"

	networkreadiness "ardents/internal/network/readiness"

	libp2pevent "github.com/libp2p/go-libp2p/core/event"
)

var timeNowUTC = func() time.Time {
	return time.Now().UTC()
}

func (s *Service) reconcileRuntimeLocked(now time.Time) {
	status := s.currentBootstrapStatusViewLocked()
	s.bootstrap = status
	raw := networkreadiness.BaseRuntimeAssessment(s.readinessStateLocked(status))
	if s.controller == nil {
		s.controller = networkreadiness.NewModeController(networkreadiness.DefaultSelectionPolicy())
	}
	decision := s.controller.Evaluate(now, s.activeMode, raw.Health())
	if decision.Changed {
		s.activeMode = decision.Mode
		s.modeRestartPending = s.cfg.NodeProfile != networkreadiness.NodeProfileConstrainedClient && s.node != nil
		s.switchReason = decision.Reason
		s.switchAuto = decision.Automatic
		s.recoveryState = decision.RecoveryState
		return
	}
	if s.switchReason == "" || raw.Health() == networkreadiness.HealthStateFailed || raw.Health() == networkreadiness.HealthStateStopped || raw.Health() == networkreadiness.HealthStateStarting {
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
				reachabilityEvents = nil
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
	snapshot, observer := s.reachability, s.reachabilityObs
	s.mu.Unlock()
	if observer != nil {
		go observer(snapshot)
	}
}

func (s *Service) handleRuntimeTick(ctx context.Context) bool {
	now := timeNowUTC()
	s.mu.Lock()
	if s.node == nil {
		s.mu.Unlock()
		return false
	}
	s.reconcileRuntimeLocked(now)
	restartMode := s.modeRestartPending
	refreshDNS := s.shouldRefreshDNSLocked(now)
	retryBootstrap := s.shouldRetryBootstrapLocked(now)
	s.mu.Unlock()
	if restartMode {
		return s.restartForActiveMode(ctx)
	}
	if refreshDNS {
		_ = s.refreshDNSPeers(ctx)
		retryBootstrap = true
	}
	if retryBootstrap {
		s.applyBootstrapRetry(s.dialBootstrapPeers(ctx))
	}
	return true
}

func (s *Service) applyBootstrapRetry(status networkreadiness.BootstrapStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.node != nil {
		s.bootstrap = status
		s.reconcileRuntimeLocked(timeNowUTC())
	}
}
