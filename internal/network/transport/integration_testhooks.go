//go:build integration

package transport

import (
	"context"
	"fmt"
	"time"

	networkreadiness "ardents/internal/network/readiness"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
)

func (s *Service) SetReachabilityForIntegration(state string) error {
	var value libp2pnetwork.Reachability
	switch state {
	case "public":
		value = libp2pnetwork.ReachabilityPublic
	case "private":
		value = libp2pnetwork.ReachabilityPrivate
	case "unknown":
		value = libp2pnetwork.ReachabilityUnknown
	default:
		return fmt.Errorf("unsupported integration reachability state")
	}
	s.mu.Lock()
	s.applyReachabilityEventLocked(value, timeNowUTC())
	snapshot := s.reachability
	observer := s.reachabilityObs
	s.mu.Unlock()
	if observer != nil {
		go observer(snapshot)
	}
	return nil
}

func (s *Service) SetModeForIntegration(ctx context.Context, mode networkreadiness.Mode) error {
	s.mu.Lock()
	if s.node == nil {
		s.mu.Unlock()
		return fmt.Errorf("transport is not started")
	}
	if mode == networkreadiness.ModeRestrictedDefense {
		controller := networkreadiness.NewModeController(networkreadiness.SelectionPolicy{
			DegradedThreshold: 1, RecoveryThreshold: 1, Cooldown: time.Hour,
		})
		decision := controller.Evaluate(timeNowUTC(), networkreadiness.ModeSteady, networkreadiness.HealthStateDegraded)
		s.controller = controller
		s.activeMode = decision.Mode
	} else if mode == networkreadiness.ModeSteady {
		s.controller = networkreadiness.NewModeController(networkreadiness.DefaultSelectionPolicy())
		s.activeMode = mode
	} else {
		s.mu.Unlock()
		return fmt.Errorf("unsupported integration mode")
	}
	s.modeRestartPending = true
	s.mu.Unlock()
	if !s.restartForActiveMode(ctx) {
		return fmt.Errorf("network mode restart failed: %s", s.Reason())
	}
	return nil
}
