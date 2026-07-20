package process

import (
	"context"
	"time"

	"ardents/internal/diagnostics"
	nodelifecycle "ardents/internal/node/lifecycle"
	runtimeconfig "ardents/internal/runtime/config"
)

func (n *Node) initOperatorConfig() {
	if n.cfg.OperatorConfig == nil {
		return
	}
	_ = n.cfg.OperatorConfig.RegisterApplier(&nodeConfigApplier{node: n})
}

func (n *Node) GetEffectiveConfig() runtimeconfig.EffectiveSnapshot {
	if n.cfg.OperatorConfig == nil {
		return runtimeconfig.EffectiveSnapshot{}
	}
	return n.cfg.OperatorConfig.Snapshot()
}

func (n *Node) ReloadConfig(ctx context.Context) runtimeconfig.ReloadResult {
	if n.cfg.OperatorConfig == nil {
		return runtimeconfig.ReloadResult{
			Outcome: runtimeconfig.OutcomeRejectedInvalid,
			Reason:  "operator configuration source is unavailable",
		}
	}
	result := n.cfg.OperatorConfig.Reload(ctx)
	n.mu.Lock()
	if result.Outcome == runtimeconfig.OutcomeRollbackFailed {
		n.recordConfigRollbackFailureLocked(result.Reason)
	}
	n.publishLocked("config.reload", map[string]any{
		"outcome": string(result.Outcome), "active_generation": result.ActiveGeneration,
		"candidate_generation": result.CandidateGeneration,
	})
	n.mu.Unlock()
	return result
}

func (n *Node) recordConfigRollbackFailureLocked(detail string) {
	reason := &diagnostics.Reason{
		Code: "config.reload.rollback_failed", Domain: "configuration",
		Summary: "operator configuration rollback failed", Detail: detail,
		Impact:                 "runtime owners may have mixed effective configuration",
		Recovery:               "restart node from the last validated configuration",
		OperatorActionRequired: true, Resource: n.cfg.Name,
	}
	n.diag.SetSubsystem("configuration", diagnostics.HealthDegraded, reason)
	if n.life.State() == nodelifecycle.Ready || n.life.State() == nodelifecycle.Initializing {
		_ = n.life.Move(nodelifecycle.Degraded)
	}
	n.diag.RecordEvent("configuration", "rollback_failed", n.cfg.Name, reason.Summary, reason.Code, nil)
}

type nodeConfigApplier struct {
	node *Node
}

func (*nodeConfigApplier) Prepare(context.Context, runtimeconfig.Document, runtimeconfig.Document) error {
	return nil
}

func (a *nodeConfigApplier) Apply(_ context.Context, _ runtimeconfig.Document, next runtimeconfig.Document) error {
	a.node.applyOperatorDocument(next)
	return nil
}

func (a *nodeConfigApplier) Rollback(_ context.Context, previous runtimeconfig.Document) error {
	a.node.applyOperatorDocument(previous)
	return nil
}

func (n *Node) applyOperatorDocument(doc runtimeconfig.Document) {
	n.mu.Lock()
	defer n.mu.Unlock()
	policy := policyConfigFromOperator(doc.Policy)
	n.cfg.Policy = policy
	n.policyLive.Reconfigure(runtimePolicyConfig(policy))
	n.cfg.DiscoveryRefreshInterval = time.Duration(doc.Network.DiscoveryRefreshSeconds) * time.Second
	n.diag.SetMaxEvents(doc.Diagnostics.MaxEvents)
	n.diag.SetDetailLevel(doc.Diagnostics.DetailLevel)
	n.restartDiscoveryRefreshLocked()
}
