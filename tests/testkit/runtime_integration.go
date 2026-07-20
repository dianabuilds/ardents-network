//go:build integration

package testkit

import (
	"context"

	runtimeprocess "ardents/internal/runtime/process"
	runtimeprocesstesting "ardents/internal/runtime/process/testing"
	workloadcontroller "ardents/internal/workload/controller"
	"ardents/internal/workload/observedstate"
)

func ReplaceWorkloadForIntegrationTest(h *RuntimeHarness, svc *workloadcontroller.Service) {
	runtimeprocesstesting.ReplaceWorkload(ensureNode(h), svc)
}

func SetBlobExchangeStarterForIntegrationTest(h *RuntimeHarness, fn func(context.Context) error) {
	runtimeprocesstesting.SetBlobExchangeStarter(ensureNode(h), fn)
}

func TransportStateForIntegrationTest(h *RuntimeHarness) string {
	return runtimeprocesstesting.TransportState(ensureNode(h))
}

func NetworkSideEffectsClearedForIntegrationTest(h *RuntimeHarness) bool {
	return runtimeprocesstesting.NetworkSideEffectsCleared(ensureNode(h))
}

func StopTransportForIntegrationTest(h *RuntimeHarness, ctx context.Context) error {
	return runtimeprocesstesting.StopTransport(ensureNode(h), ctx)
}

func WorkloadStatusForIntegrationTest(h *RuntimeHarness, id string) (observedstate.Status, bool) {
	return runtimeprocesstesting.WorkloadStatus(ensureNode(h), id)
}

func ensureNode(h *RuntimeHarness) runtimeprocess.NodeRuntime {
	if h == nil || h.Node == nil {
		panic("runtime harness must expose node runtime")
	}
	return h.Node
}
