package lifecycle

import (
	"context"

	nodereadiness "ardents/internal/node/readiness"
)

func (m *Manager) StopLocked(ctx context.Context) error {
	if m.life.State() == Stopped {
		return nil
	}

	m.moveLifecycleLocked(Stopping)
	m.publish("node.stopping", map[string]any{"id": m.cfgName, "state": Stopping})
	op := m.diag.BeginOperation(ShutdownPhaseNode, "node", m.cfgName, true, "restart node")
	m.diag.MarkRecoveringExcept(op.ID, "operation interrupted by shutdown")

	if err := m.authority.ShutdownWorkloadsLocked(ctx); err != nil {
		m.diag.FailOperation(op.ID, err.Error())
		m.FailLocked("node.shutdown.workloads_failed", "workload", "workload shutdown failed", err.Error(), "node stop left workload execution uncertain", "operator")
		return m.runtimeFailureLocked("stop")
	}
	if err := m.publication.WithdrawNetworkPublicationLocked(ctx); err != nil {
		m.diag.FailOperation(op.ID, err.Error())
		m.FailLocked("node.shutdown.publication_failed", "discovery", "discovery shutdown publication failed", err.Error(), "stopped node may remain discoverable on the network", "operator")
		return m.runtimeFailureLocked("stop")
	}

	if err := m.trans.Stop(ctx); err != nil {
		m.diag.FailOperation(op.ID, err.Error())
		m.FailLocked("node.shutdown.failed", "node", "shutdown failed", err.Error(), "node safety is uncertain", "terminal")
		return m.runtimeFailureLocked("stop")
	}
	m.diag.CompleteOperation(op.ID, "node shutdown completed")
	m.clearRuntimeHealthForStopLocked()
	m.moveLifecycleLocked(Stopped)
	m.publish("node.stopped", map[string]any{"id": m.cfgName, "state": Stopped})
	m.diag.RecordEvent("node", "stopped", m.cfgName, "node shutdown completed", "", map[string]any{"id": m.cfgName})
	return nil
}

func (m *Manager) clearRuntimeHealthForStopLocked() {
	nodereadiness.ClearRuntimeHealthForStop(m.diag, m.boot)
}
