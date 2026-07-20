package controller

import (
	"context"

	"ardents/internal/workload/execution"
)

type localExecutorAdapter struct {
	inner execution.Executor
}

func NewLocalExecutor() Executor {
	return localExecutorAdapter{inner: execution.NewLocalExecutor()}
}

func NewDockerExecutor(nodeID string) (Executor, error) {
	return NewDockerExecutorWithConfig(execution.DockerExecutorConfig{NodeID: nodeID})
}

func NewDockerExecutorWithConfig(cfg execution.DockerExecutorConfig) (Executor, error) {
	inner, err := execution.NewDockerExecutor(cfg)
	if err != nil {
		return nil, err
	}
	return localExecutorAdapter{inner: inner}, nil
}

func (a localExecutorAdapter) Prepare(ctx context.Context, spec Spec) (PreparedWorkload, error) {
	return a.inner.Prepare(ctx, execution.Request{
		WorkloadID: spec.ID,
		Config:     spec.Config,
		PolicyRef:  spec.PolicyRef,
		Ingress:    executionIngress(spec.Services),
	})
}

func executionIngress(services []ServiceSpec) []execution.IngressRequest {
	out := make([]execution.IngressRequest, 0, len(services))
	for _, service := range services {
		out = append(out, execution.IngressRequest{Mode: service.Mode,
			Endpoints: append([]string(nil), service.Endpoints...), ProbeEndpoints: append([]string(nil), service.ProbeEndpoints...)})
	}
	return out
}

func (a localExecutorAdapter) Start(ctx context.Context, prepared PreparedWorkload) (Instance, error) {
	return a.inner.Start(ctx, prepared)
}

func (a localExecutorAdapter) Stop(ctx context.Context, instance Instance) error {
	return a.inner.Stop(ctx, instance)
}

func (a localExecutorAdapter) Inspect(ctx context.Context, workloadID string) (Instance, error) {
	return a.inner.Inspect(ctx, workloadID)
}

func (a localExecutorAdapter) Remove(ctx context.Context, instance Instance) error {
	remover, ok := a.inner.(execution.Remover)
	if !ok {
		return nil
	}
	return remover.Remove(ctx, instance)
}

func (a localExecutorAdapter) Managed(ctx context.Context) ([]Instance, error) {
	inventory, ok := a.inner.(execution.Inventory)
	if !ok {
		return nil, nil
	}
	return inventory.Managed(ctx)
}

func (a localExecutorAdapter) ReconcileAncillary(ctx context.Context, current []Instance) error {
	reconciler, ok := a.inner.(execution.AncillaryReconciler)
	if !ok {
		return nil
	}
	return reconciler.ReconcileAncillary(ctx, current)
}
