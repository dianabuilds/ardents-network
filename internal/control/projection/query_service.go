package projection

import (
	"context"

	diagapi "ardents/internal/diagnostics/api"
	discoveryapi "ardents/internal/discovery/api"
	transport "ardents/internal/network/api"
	nodeapi "ardents/internal/node/api"
)

type runtimeSync interface {
	SyncObservedTruthLocked()
}

type workloadSync interface {
	SyncObservedWorkloadsLocked(context.Context) error
}

type QueryService struct {
	runtime  runtimeSync
	workload workloadSync
	reader   *Reader
}

func NewQueryService(runtime runtimeSync, workload workloadSync, reader *Reader) *QueryService {
	return &QueryService{
		runtime:  runtime,
		workload: workload,
		reader:   reader,
	}
}

func (s *QueryService) SnapshotLocked() nodeapi.Snapshot {
	s.syncObservedTruthLocked()
	return s.reader.SnapshotLocked()
}

func (s *QueryService) RoutingDetailsLocked() discoveryapi.RouteSnapshot {
	return s.reader.RoutingDetailsLocked()
}

func (s *QueryService) DiagnosticsSnapshotLocked() diagapi.DiagSnapshot {
	s.syncObservedTruthLocked()
	return s.reader.DiagnosticsSnapshotLocked()
}

func (s *QueryService) RecentDiagnosticsLocked(limit int) []string {
	return s.reader.RecentDiagnosticsLocked(limit)
}

func (s *QueryService) PendingOperationsLocked() []diagapi.OperationSnapshot {
	return s.reader.PendingOperationsLocked()
}

func (s *QueryService) Capabilities() Capabilities {
	return s.reader.Capabilities()
}

func (s *QueryService) CapabilitiesSnapshotLocked() nodeapi.CapabilitiesSnapshot {
	caps := s.reader.Capabilities()
	return nodeapi.CapabilitiesSnapshot{
		Version:  caps.Version,
		Services: cloneStrings(caps.Services),
		Features: cloneBoolMap(caps.Features),
	}
}

func (s *QueryService) LastTransportCandidatesLocked() []transport.Candidate {
	return s.reader.LastTransportCandidatesLocked()
}

func (s *QueryService) syncObservedTruthLocked() {
	s.runtime.SyncObservedTruthLocked()
	_ = s.workload.SyncObservedWorkloadsLocked(context.Background())
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	if in == nil {
		return nil
	}
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
