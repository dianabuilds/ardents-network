package controller

import (
	db "ardents/internal/persistence"
	workloadregistry "ardents/internal/workload/registry"
)

func (s *Service) saveLocked() error {
	if s.path == "" {
		return nil
	}
	return db.SaveJSON(s.path, "workload", "snapshot", persistentState{Items: s.items})
}

func normalizeStatus(item Status) Status {
	return workloadregistry.NormalizeStatus(item)
}

func serviceStatuses(spec Spec, published bool, reason string) []PublishedServiceStatus {
	return workloadregistry.ServiceStatuses(spec, published, reason)
}

func cloneStatus(item Status) Status {
	return workloadregistry.CloneStatus(item)
}

func snapshotStatusesLocked(items map[string]Status) []Status {
	return workloadregistry.SnapshotStatuses(items)
}

func CloneStatus(item Status) Status {
	return cloneStatus(item)
}
