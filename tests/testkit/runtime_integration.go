//go:build integration

package testkit

import (
	"context"

	runtimeprocess "ardents/internal/daemon"
	"ardents/internal/workload/execution"
	workloadcontroller "ardents/internal/workload/execution"
)

func ReplaceWorkloadForIntegrationTest(h *RuntimeHarness, svc *workloadcontroller.Service) {
	node := ensureNode(h)
	runtimeprocess.ReplaceWorkloadForIntegrationTest(node, svc)
	owners, ok := runtimeprocess.OwnersFor(node)
	if !ok {
		panic("replaced workload must remain attached to its node")
	}
	h.Workload = owners.Workloads
	h.Hosting = owners.Hosting
	h.Diagnostics = owners.Diagnostics
}

func SetBlobExchangeStarterForIntegrationTest(h *RuntimeHarness, fn func(context.Context) error) {
	runtimeprocess.SetBlobExchangeStarterForIntegrationTest(ensureNode(h), fn)
}

func TransportStateForIntegrationTest(h *RuntimeHarness) string {
	return runtimeprocess.TransportStateForIntegrationTest(ensureNode(h))
}

func NetworkSideEffectsClearedForIntegrationTest(h *RuntimeHarness) bool {
	return runtimeprocess.NetworkSideEffectsClearedForIntegrationTest(ensureNode(h))
}

func StopTransportForIntegrationTest(h *RuntimeHarness, ctx context.Context) error {
	return runtimeprocess.StopTransportForIntegrationTest(ensureNode(h), ctx)
}

func WorkloadStatusForIntegrationTest(h *RuntimeHarness, id string) (execution.Status, bool) {
	return runtimeprocess.WorkloadStatusForIntegrationTest(ensureNode(h), id)
}

func ensureNode(h *RuntimeHarness) *runtimeprocess.Node {
	if h == nil || h.Node == nil {
		panic("runtime harness must expose node runtime")
	}
	return h.Node
}
