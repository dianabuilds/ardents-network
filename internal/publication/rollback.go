package publication

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ardents/internal/diagnostics"
	nodelifecycle "ardents/internal/node/lifecycle"
	publicationapi "ardents/internal/publication/api"
)

const (
	publicationRollbackReasonCode = "publication.service_sync_rolled_back"
	publicationRollbackFailedCode = "publication.service_sync_rollback_failed"
	publicationRollbackTimeout    = 5 * time.Second
)

func (m *Manager) moveLifecycleLocked(next string) {
	if err := m.life.Move(next); err != nil {
		m.diag.RecordEvent("node", "lifecycle_transition_rejected", m.cfgName, "lifecycle transition rejected", "node.lifecycle.transition_rejected", map[string]any{
			"from":  m.life.State(),
			"to":    next,
			"error": err.Error(),
		})
	}
}

func (m *Manager) CaptureWorkloadPublicationSnapshotLocked() publicationapi.Snapshot {
	entries, state, reason := m.disco.Snapshot()
	return publicationapi.Snapshot{
		Workloads:       m.workload.Snapshot(),
		Discovery:       entries,
		DiscoveryState:  state,
		DiscoveryReason: reason,
	}
}

func (m *Manager) RollbackWorkloadMutationLocked(ctx context.Context, action string, cause error, snapshot publicationapi.Snapshot) error {
	restoreCtx, cancel := rollbackContext(ctx)
	defer cancel()
	if err := m.workload.Restore(restoreCtx, snapshot.Workloads); err != nil {
		m.recordRollbackLocked(action, cause, err)
		return err
	}
	if err := m.disco.Restore(snapshot.Discovery, snapshot.DiscoveryState, snapshot.DiscoveryReason); err != nil {
		m.recordRollbackLocked(action, cause, err)
		return err
	}
	if requiresNetworkCompensation(cause) {
		compensateCtx, cancel := rollbackContext(ctx)
		defer cancel()
		if err := m.compensateNetworkLocked(compensateCtx, snapshot.Discovery); err != nil {
			m.recordRollbackLocked(action, cause, err)
			return err
		}
	}
	m.refreshWorkloadStateLocked()
	m.recordRollbackLocked(action, cause, nil)
	return nil
}

func (m *Manager) ClearRollbackLocked() {
	health := m.diag.Health()
	m.diag.ClearSubsystem(Subsystem)
	if health.PrimaryReason != nil && strings.HasPrefix(health.PrimaryReason.Code, "publication.service_sync_") {
		m.diag.ClearPrimary()
	}
}

func (m *Manager) recordRollbackLocked(action string, cause, rollbackErr error) {
	reason := &diagnostics.Reason{
		Code:                   publicationRollbackReasonCode,
		Domain:                 Subsystem,
		Summary:                "service publication sync failed",
		Detail:                 fmt.Sprintf("%s failed after runtime reconcile: %v", action, cause),
		Impact:                 "the mutation was rolled back to preserve local, discovery, and network-visible truth",
		Recovery:               "restore transport/discovery health and retry the command",
		OperatorActionRequired: true,
		Resource:               m.cfgName,
	}
	state := diagnostics.HealthDegraded
	eventType := "sync_rolled_back"
	if rollbackErr != nil {
		reason.Code = publicationRollbackFailedCode
		reason.Detail = fmt.Sprintf("%s failed and rollback did not complete: %v", action, rollbackErr)
		reason.Impact = "local runtime truth may be inconsistent with discovery publication"
		reason.Recovery = "operator intervention required before retry"
		state = diagnostics.HealthFailed
		eventType = "sync_rollback_failed"
	}
	m.diag.SetSubsystem(Subsystem, state, reason)
	if health := m.diag.Health(); health.PrimaryReason == nil || strings.HasPrefix(health.PrimaryReason.Domain, Subsystem) {
		m.diag.SetPrimary(state, reason)
	}
	if state == diagnostics.HealthFailed {
		m.moveLifecycleLocked(nodelifecycle.Failed)
	} else if m.life.State() != nodelifecycle.Failed {
		m.moveLifecycleLocked(nodelifecycle.Degraded)
	}
	m.diag.RecordEvent(Subsystem, eventType, m.cfgName, reason.Summary, reason.Code, map[string]any{
		"action": action,
		"detail": reason.Detail,
	})
}

func (m *Manager) refreshWorkloadStateLocked() {
	for _, item := range m.workload.List() {
		m.publish("workload.updated", map[string]any{
			"id":       item.Spec.ID,
			"observed": item.Observed,
			"desired":  item.Spec.Desired,
		})
	}
}

func rollbackContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil || parent.Err() != nil {
		return context.WithTimeout(context.Background(), publicationRollbackTimeout)
	}
	return context.WithTimeout(parent, publicationRollbackTimeout)
}
