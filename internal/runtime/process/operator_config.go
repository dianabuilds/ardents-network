package process

import (
	"context"
	"time"

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
	n.publishLocked("config.reload", map[string]any{
		"outcome": string(result.Outcome), "active_generation": result.ActiveGeneration,
		"candidate_generation": result.CandidateGeneration,
	})
	n.mu.Unlock()
	return result
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
