package testkit

import (
	"context"
	"testing"
	"time"

	dataapi "ardents/internal/data/api"
	diagapi "ardents/internal/diagnostics/api"
	discoveryapi "ardents/internal/discovery/api"
	hostingapi "ardents/internal/hosting/api"
	nodeapi "ardents/internal/node/api"
	runtimeinfra "ardents/internal/runtime/process"
	runtimeprocess "ardents/internal/runtime/process"
	workloadapi "ardents/internal/workload/api"

	"github.com/stretchr/testify/require"
)

type RuntimeHarness struct {
	Runtime     runtimeprocess.NodeRuntime
	Node        runtimeprocess.NodeRuntime
	NodeAPI     nodeapi.RuntimeService
	Discovery   discoveryapi.Service
	Diagnostics diagapi.Service
	Workload    workloadapi.Service
	Hosting     hostingapi.Service
	Data        dataapi.Service
	Source      runtimeprocess.NodeRuntime
}

func NewRuntime(t *testing.T, cfg runtimeinfra.Config) *RuntimeHarness {
	t.Helper()

	ConfigureLoopbackTransport(t)
	if cfg.Privacy == nil {
		cfg.Privacy = NewDiscoveryPrivacyFixture(t, time.Now().UTC().Truncate(time.Second)).Receiver
	}
	if cfg.DataPrivacy == nil {
		cfg.DataPrivacy = NewDataPrivacyFixture(t, time.Now().UTC().Truncate(time.Second)).Receiver
	}
	n := runtimeprocess.NewNodeRuntime(cfg)
	return &RuntimeHarness{
		Runtime:     n,
		Node:        n,
		NodeAPI:     n,
		Discovery:   n,
		Diagnostics: n,
		Workload:    n,
		Hosting:     n,
		Data:        n,
		Source:      n,
	}
}

func StartNode(t *testing.T, cfg runtimeinfra.Config) runtimeprocess.NodeRuntime {
	t.Helper()

	harness := NewRuntime(t, cfg)
	require.NoError(t, harness.Runtime.Start(context.Background()))
	t.Cleanup(func() {
		_ = harness.Runtime.Stop(context.Background())
	})
	return harness.Node
}

func StartRuntime(t *testing.T, cfg runtimeinfra.Config) *RuntimeHarness {
	t.Helper()

	harness := NewRuntime(t, cfg)
	require.NoError(t, harness.Runtime.Start(context.Background()))
	t.Cleanup(func() {
		_ = harness.Runtime.Stop(context.Background())
	})
	return harness
}
