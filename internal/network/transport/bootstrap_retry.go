package transport

import (
	"time"

	networkreadiness "ardents/internal/network/readiness"
)

const bootstrapRetryInterval = 2 * time.Second
const desiredRelayPeers = 3

func (s *Service) shouldRetryBootstrapLocked(now time.Time) bool {
	if s.node == nil {
		return false
	}
	signals := networkreadiness.HealthSnapshot(s.readinessStateLocked(s.currentBootstrapStatusViewLocked()))
	if signals.BootstrapSourceCount == 0 {
		return false
	}
	if s.cfg.NodeProfile == networkreadiness.NodeProfileConstrainedClient {
		return signals.BootstrapStatus.State != "ready" &&
			(s.lastBootstrapAttempt.IsZero() || now.Sub(s.lastBootstrapAttempt) >= bootstrapRetryInterval)
	}
	target := min(desiredRelayPeers, signals.BootstrapSourceCount)
	if signals.RelayPeerCount >= target {
		return false
	}
	return s.lastBootstrapAttempt.IsZero() || now.Sub(s.lastBootstrapAttempt) >= bootstrapRetryInterval
}
