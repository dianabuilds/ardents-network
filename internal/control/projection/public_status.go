package projection

import (
	"context"
	"time"

	controlstatus "ardents/internal/control/status"
	"ardents/internal/diagnostics"
	networkprivacy "ardents/internal/network/privacy"
	nodeapi "ardents/internal/node/api"
)

func (s *QueryService) NodeRuntimeSnapshotLocked() nodeapi.NodeRuntimeSnapshot {
	s.syncObservedTruthLocked()
	return controlstatus.NodeRuntimeSnapshot(s.reader.SnapshotLocked(), diagnostics.HealthSnapshot(s.reader.diag.Health()))
}

func (s *QueryService) NetworkStatusSnapshotLocked() nodeapi.NetworkStatusSnapshot {
	s.runtime.SyncObservedTruthLocked()
	profile := s.reader.trans.ProfileSnapshot()
	return controlstatus.NetworkStatusSnapshot(
		s.reader.nodeProfile,
		s.reader.trans.State(),
		s.reader.trans.Reason(),
		s.reader.boot.Result().Joined,
		profile,
		s.reader.trans.ReachabilitySnapshot(),
		s.reader.trans.AbuseSnapshot(),
		s.reader.life.Snapshot().TransitionedAt,
		networkprivacy.Snapshot(s.reader.privacy, s.reader.dataPrivacy),
	)
}

func (s *QueryService) DiscoveryStatusSnapshotLocked(now time.Time) nodeapi.DiscoveryStatusSnapshot {
	s.runtime.SyncObservedTruthLocked()
	return controlstatus.DiscoveryStatusSnapshot(
		s.reader.disco.State(),
		s.reader.disco.Reason(),
		s.reader.disco.Entries(),
		now,
		s.reader.trust.Evaluate,
	)
}

func (s *QueryService) PeerSnapshotsLocked() []nodeapi.PeerSnapshot {
	s.runtime.SyncObservedTruthLocked()
	return controlstatus.PeerSnapshots(
		s.reader.disco.Entries(),
		s.reader.ident.NodeSummary().Principal,
		s.reader.trans.BuildCandidates,
		s.reader.trust.Evaluate,
	)
}

func (s *QueryService) SyncDiagnosticsLocked() error {
	s.runtime.SyncObservedTruthLocked()
	return s.workload.SyncObservedWorkloadsLocked(context.Background())
}
