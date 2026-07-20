//go:build integration

package testing

import (
	"context"

	runtimeprocess "ardents/internal/runtime/process"
	workloadcontroller "ardents/internal/workload/controller"
	"ardents/internal/workload/observedstate"
)

func ReplaceWorkload(rt runtimeprocess.NodeRuntime, svc *workloadcontroller.Service) {
	runtimeprocess.ReplaceWorkloadForIntegrationTest(unwrap(rt), svc)
}

func SetBlobExchangeStarter(rt runtimeprocess.NodeRuntime, fn func(context.Context) error) {
	runtimeprocess.SetBlobExchangeStarterForIntegrationTest(unwrap(rt), fn)
}

func TransportState(rt runtimeprocess.NodeRuntime) string {
	return runtimeprocess.TransportStateForIntegrationTest(unwrap(rt))
}

func NetworkSideEffectsCleared(rt runtimeprocess.NodeRuntime) bool {
	return runtimeprocess.NetworkSideEffectsClearedForIntegrationTest(unwrap(rt))
}

func StopTransport(rt runtimeprocess.NodeRuntime, ctx context.Context) error {
	return runtimeprocess.StopTransportForIntegrationTest(unwrap(rt), ctx)
}

func WorkloadStatus(rt runtimeprocess.NodeRuntime, id string) (observedstate.Status, bool) {
	return runtimeprocess.WorkloadStatusForIntegrationTest(unwrap(rt), id)
}

type nodeProvider interface {
	NodeForTesting() any
}

func unwrap(rt runtimeprocess.NodeRuntime) *runtimeprocess.Node {
	provider, ok := rt.(nodeProvider)
	if !ok {
		panic("runtime does not expose testing node handle")
	}
	n, ok := provider.NodeForTesting().(*runtimeprocess.Node)
	if !ok || n == nil {
		panic("runtime testing node handle is not *process.Node")
	}
	return n
}
