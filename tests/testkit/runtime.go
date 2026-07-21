package testkit

import (
	"context"
	"testing"
	"time"

	appdata "ardents/internal/content"
	runtimeinfra "ardents/internal/daemon"
	runtimeprocess "ardents/internal/daemon"
	diagapi "ardents/internal/diagnostics"
	discoveryapi "ardents/internal/discovery"
	"ardents/internal/hosting"
	"ardents/internal/transfer"
	workloadapi "ardents/internal/workload"

	"github.com/stretchr/testify/require"
)

type RuntimeHarness struct {
	Runtime     *runtimeprocess.Node
	Node        *runtimeprocess.Node
	NodeAPI     *runtimeprocess.Node
	Discovery   DiscoveryService
	Diagnostics diagapi.Service
	Workload    *workloadapi.Runtime
	Hosting     *hosting.Service
	Data        *appdata.Service
	Transfers   *transfer.Journal
	Source      *runtimeprocess.Node
}

type DiscoveryService interface {
	ListRecords() ([]discoveryapi.CatalogRecordSnapshot, error)
	ResolveRecord(string, string) (discoveryapi.ResolutionResult, error)
	ImportRecord(discoveryapi.CatalogRecordSnapshot) (discoveryapi.RecordImportResult, error)
	ResolveService(string) (discoveryapi.ServiceResult, error)
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
	n := runtimeprocess.NewNode(cfg)
	owners, ok := runtimeprocess.OwnersFor(n)
	require.True(t, ok)
	return &RuntimeHarness{
		Runtime:     n,
		Node:        n,
		NodeAPI:     n,
		Discovery:   n,
		Diagnostics: owners.Diagnostics,
		Workload:    owners.Workloads,
		Hosting:     owners.Hosting,
		Data:        owners.Content,
		Transfers:   owners.Transfers,
		Source:      n,
	}
}

func Workloads(node *runtimeprocess.Node) *workloadapi.Runtime {
	owners, ok := runtimeprocess.OwnersFor(node)
	if !ok {
		panic("workloads require a node")
	}
	return owners.Workloads
}

func Hosting(node *runtimeprocess.Node) *hosting.Service {
	owners, ok := runtimeprocess.OwnersFor(node)
	if !ok {
		panic("hosting requires a node")
	}
	return owners.Hosting
}

func StartNode(t *testing.T, cfg runtimeinfra.Config) *runtimeprocess.Node {
	t.Helper()

	harness := NewRuntime(t, cfg)
	require.NoError(t, harness.Runtime.Start(context.Background()))
	t.Cleanup(func() {
		require.NoError(t, harness.Runtime.Stop(context.Background()))
	})
	return harness.Node
}

func StartRuntime(t *testing.T, cfg runtimeinfra.Config) *RuntimeHarness {
	t.Helper()

	harness := NewRuntime(t, cfg)
	require.NoError(t, harness.Runtime.Start(context.Background()))
	t.Cleanup(func() {
		require.NoError(t, harness.Runtime.Stop(context.Background()))
	})
	return harness
}

func Diagnostics(node *runtimeprocess.Node) diagapi.Service {
	owners, ok := runtimeprocess.OwnersFor(node)
	if !ok {
		panic("diagnostics require a node")
	}
	return owners.Diagnostics
}

func Content(node *runtimeprocess.Node) *appdata.Service {
	owners, ok := runtimeprocess.OwnersFor(node)
	if !ok {
		panic("content requires a node")
	}
	return owners.Content
}

func Transfers(node *runtimeprocess.Node) *transfer.Journal {
	owners, ok := runtimeprocess.OwnersFor(node)
	if !ok {
		panic("transfer history requires a node")
	}
	return owners.Transfers
}
