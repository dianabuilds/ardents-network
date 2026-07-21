package publication

import (
	"ardents/internal/diagnostics"
	"ardents/internal/discovery"
	"ardents/internal/identity"
	identityapi "ardents/internal/identity"
	identitykeyring "ardents/internal/identity/keyring"
	transport "ardents/internal/network"
	apppolicy "ardents/internal/policy"
	workloadcontroller "ardents/internal/workload/execution"
	hostingregistry "ardents/internal/workload/registry"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRollbackWorkloadMutationLockedStopsOnCanceledContext(t *testing.T) {
	dir := t.TempDir()
	identSvc, private := publicationIdentity(t, dir)
	disco := discovery.NewInDir(dir)
	workloadSvc := workloadcontroller.NewWithExecutorInDir(dir, publicationCancelAwareExecutor{})
	{
		err := workloadSvc.Register(hostingregistry.Spec{
			ID:      "work.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  "test",
			Desired: hostingregistry.DesiredRunning,
		})
		require.NoErrorf(t, err, "register workload: %v", err)
	}
	{

		err := workloadSvc.Reconcile(context.Background())
		require.NoErrorf(t, err, "reconcile workload: %v", err)
	}
	{

		err := PublishLocalNode(disco, identSvc.NodeSummary(), private, []string{"tcp://node:9000"})
		require.NoErrorf(t, err, "publish node: %v", err)
	}

	mgr := NewManager(
		"publication-test",
		diagnostics.New(""),
		diagnostics.NewMachine(),
		disco,
		apppolicy.New(apppolicy.Config{}),
		hostingregistry.New(nil),
		workloadSvc,
		publicationReachableNetwork(),
		nil,
		identSvc,
		nil,
		func() ed25519.PrivateKey { return nil },
		func(string, map[string]any) {},
	)
	mgr.privateKey = func() ed25519.PrivateKey { return private }

	snapshot := mgr.CaptureWorkloadPublicationSnapshotLocked()
	snapshot.Workloads[0].Spec.Desired = hostingregistry.DesiredStopped
	snapshot.Workloads[0].Observed = workloadcontroller.ObservedStopped
	compensated := false
	mgr.publishDiscoveryEntries = func(_ context.Context, entries []discovery.Entry) error {
		compensated = len(entries) > 0
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cause := &transport.DiscoveryPublishError{Published: 1, Err: errors.New("transport publish failed")}
	{
		err := mgr.RollbackWorkloadMutationLocked(ctx, "workload start", cause, snapshot)
		require.NoErrorf(t, err, "rollback error = %v, want nil", err)
	}
	require.True(t, compensated, "expected rollback to complete network compensation after caller cancellation")

}

func TestRefreshNetworkPublicationLockedPublishesNodeAndServices(t *testing.T) {
	dir := t.TempDir()
	identSvc, private := publicationIdentity(t, dir)
	disco := discovery.NewInDir(dir)
	servedGeneration := int64(42)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Ardents-Generation", generationHeader(servedGeneration))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	probeURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	advertisedURL := "http://10.0.0.2:" + probeURL.Port()
	executor := &publicationRunningExecutor{generation: 42}
	workloadSvc := workloadcontroller.NewWithExecutorInDir(dir, executor)
	{
		err = workloadSvc.Register(hostingregistry.Spec{
			ID:      "work.echo",
			Kind:    "service",
			Owner:   "node",
			Config:  "test",
			Desired: hostingregistry.DesiredRunning,
			Services: []hostingregistry.ServiceSpec{{
				ID:             "svc.echo",
				Type:           "echo",
				Mode:           "NetworkPublished",
				Endpoints:      []string{advertisedURL},
				ProbeEndpoints: []string{server.URL},
			}},
		})
		require.NoErrorf(t, err, "register workload: %v", err)
	}
	{

		err := workloadSvc.Reconcile(context.Background())
		require.NoErrorf(t, err, "reconcile workload: %v", err)
	}

	network := &publicationNetwork{snapshot: transport.ReachabilitySnapshot{
		Mode: transport.ReachabilityPrivateLAN, State: "lan", Reachable: true,
	}}
	mgr := NewManager(
		"publication-test",
		diagnostics.New(""),
		diagnostics.NewMachine(),
		disco,
		apppolicy.New(apppolicy.Config{}),
		hostingregistry.New(nil),
		workloadSvc,
		network,
		nil,
		identSvc,
		discovery.NewTrustEvaluator(),
		func() ed25519.PrivateKey { return private },
		func(string, map[string]any) {},
	)
	published := 0
	mgr.publishDiscoveryEntries = func(_ context.Context, entries []discovery.Entry) error {
		published = len(entries)
		return nil
	}
	for range 2 {
		err := mgr.RefreshNetworkPublicationLocked(context.Background())
		require.NoErrorf(t, err, "refresh network publication: %v", err)
	}

	entries := disco.Entries()
	require.Truef(t, containsRecord(entries,

		identSvc.
			NodeSummary().Principal+":node",
	), "entries = %v, want published node record", recordIDs(entries))
	require.Truef(t, containsRecord(entries,

		"svc.echo",
	), "entries = %v, want published service record", recordIDs(entries))
	require.Falsef(t, published != len(entries), "published entries = %d, want %d", published, len(entries))

	executor.generation = 43
	require.NoError(t, mgr.RefreshNetworkPublicationLocked(context.Background()))
	require.Empty(t, serviceRecordEndpoints(disco.Entries(), "svc.echo"), "new generation must withdraw before readiness")

	servedGeneration = 43
	for range 2 {
		require.NoError(t, mgr.RefreshNetworkPublicationLocked(context.Background()))
	}
	require.Equal(t, []string{advertisedURL}, serviceRecordEndpoints(disco.Entries(), "svc.echo"))

	executor.generation = 42
	servedGeneration = 42
	for range 2 {
		require.NoError(t, mgr.RefreshNetworkPublicationLocked(context.Background()))
	}
	require.Empty(t, serviceRecordEndpoints(disco.Entries(), "svc.echo"), "stale generation must never republish")

	executor.generation = 43
	servedGeneration = 43
	for range 2 {
		require.NoError(t, mgr.RefreshNetworkPublicationLocked(context.Background()))
	}
	require.Equal(t, []string{advertisedURL}, serviceRecordEndpoints(disco.Entries(), "svc.echo"))

	servedGeneration = 999
	for range 2 {
		require.NoError(t, mgr.RefreshNetworkPublicationLocked(context.Background()))
	}
	require.Empty(t, serviceRecordEndpoints(disco.Entries(), "svc.echo"), "probe ownership failure must withdraw")

	servedGeneration = 43
	for range 2 {
		require.NoError(t, mgr.RefreshNetworkPublicationLocked(context.Background()))
	}
	require.Equal(t, []string{advertisedURL}, serviceRecordEndpoints(disco.Entries(), "svc.echo"))

	network.snapshot = transport.ReachabilitySnapshot{Mode: transport.ReachabilityOutboundOnly, State: "outbound_only"}
	require.NoError(t, mgr.RefreshNetworkPublicationLocked(context.Background()))
	require.Empty(t, serviceRecordEndpoints(disco.Entries(), "svc.echo"), "network capability loss must withdraw")

	network.snapshot = transport.ReachabilitySnapshot{Mode: transport.ReachabilityPrivateLAN, State: "lan", Reachable: true}
	require.NoError(t, mgr.RefreshNetworkPublicationLocked(context.Background()))
	require.Equal(t, []string{advertisedURL}, serviceRecordEndpoints(disco.Entries(), "svc.echo"))

}

func TestWithdrawNetworkPublicationLockedPublishesWithdrawnNodeRecord(t *testing.T) {
	dir := t.TempDir()
	identSvc, private := publicationIdentity(t, dir)
	disco := discovery.NewInDir(dir)
	mgr := NewManager(
		"publication-test",
		diagnostics.New(""),
		diagnostics.NewMachine(),
		disco,
		apppolicy.New(apppolicy.Config{}),
		hostingregistry.New(nil),
		workloadcontroller.NewWithExecutorInDir(dir, &publicationRunningExecutor{}),
		publicationReachableNetwork(),
		nil,
		identSvc,
		discovery.NewTrustEvaluator(),
		func() ed25519.PrivateKey { return private },
		func(string, map[string]any) {},
	)
	requirePublishedNodeRecord(t, disco, identSvc.NodeSummary(), private)

	var published []discovery.Entry
	mgr.publishDiscoveryEntries = func(_ context.Context, entries []discovery.Entry) error {
		published = cloneEntries(entries)
		return nil
	}
	{

		err := mgr.WithdrawNetworkPublicationLocked(context.Background())
		require.NoErrorf(t, err, "withdraw network publication: %v", err)
	}
	require.Falsef(t, len(published) != 1, "published entries = %d, want 1", len(published))
	require.Falsef(t, published[0].Record.
		Kind !=
		"node", "published kind = %q, want node", published[0].Record.
		Kind)
	require.Falsef(t, len(published[0].Record.
		Endpoints,
	) != 0, "published endpoints = %v, want empty", published[0].Record.
		Endpoints)

}

func publicationIdentity(t *testing.T, dir string) (identityapi.Service, []byte) {
	t.Helper()
	store := identity.NewStore(filepath.Join(dir, "ardents.db"))
	keys := identitykeyring.NewKeyStoreInDir(dir)
	identSvc := identityapi.NewService()
	_, private, err := identSvc.EnsureNode(store, keys)
	require.NoErrorf(t, err, "ensure identity: %v", err)

	return identSvc, private
}

type publicationRunningExecutor struct {
	generation int64
}

func (e *publicationRunningExecutor) Prepare(context.Context, workloadcontroller.Request) (workloadcontroller.PreparedWorkload, error) {
	generation := e.generation
	if generation == 0 {
		generation = time.Now().UTC().UnixNano()
		e.generation = generation
	}
	return workloadcontroller.PreparedWorkload{WorkloadID: "work.echo", Generation: generation, PreparedAt: time.Now().UTC()}, nil
}

func (e *publicationRunningExecutor) Start(_ context.Context, prepared workloadcontroller.PreparedWorkload) (workloadcontroller.Instance, error) {
	return workloadcontroller.Instance{WorkloadID: "work.echo", Generation: prepared.Generation, Running: true, StartedAt: time.Now().UTC()}, nil
}

func (*publicationRunningExecutor) Stop(context.Context, workloadcontroller.Instance) error {
	return nil
}

func (e *publicationRunningExecutor) Inspect(context.Context, string) (workloadcontroller.Instance, error) {
	return workloadcontroller.Instance{WorkloadID: "work.echo", Generation: e.generation, Running: true, StartedAt: time.Now().UTC()}, nil
}

type publicationCancelAwareExecutor struct{}

func (publicationCancelAwareExecutor) Prepare(context.Context, workloadcontroller.Request) (workloadcontroller.PreparedWorkload, error) {
	return workloadcontroller.PreparedWorkload{WorkloadID: "work.echo", Generation: time.Now().UTC().UnixNano(), PreparedAt: time.Now().UTC()}, nil
}

func (publicationCancelAwareExecutor) Start(context.Context, workloadcontroller.PreparedWorkload) (workloadcontroller.Instance, error) {
	return workloadcontroller.Instance{WorkloadID: "work.echo", Running: true, StartedAt: time.Now().UTC()}, nil
}

func (publicationCancelAwareExecutor) Stop(ctx context.Context, _ workloadcontroller.Instance) error {
	return ctx.Err()
}

func (publicationCancelAwareExecutor) Inspect(context.Context, string) (workloadcontroller.Instance, error) {
	return workloadcontroller.Instance{WorkloadID: "work.echo", Running: true, StartedAt: time.Now().UTC()}, nil
}

type publicationNetwork struct {
	transport.Service
	snapshot transport.ReachabilitySnapshot
}

func (n *publicationNetwork) Endpoints() []string {
	return []string{"/ip4/127.0.0.1/tcp/60000/p2p/publication-test"}
}

func publicationReachableNetwork() *publicationNetwork {
	return &publicationNetwork{snapshot: transport.ReachabilitySnapshot{
		Mode: transport.ReachabilityPrivateLAN, State: "lan", Reachable: true,
	}}
}

func (n *publicationNetwork) ReachabilitySnapshot() transport.ReachabilitySnapshot {
	return n.snapshot
}

func serviceRecordEndpoints(entries []discovery.Entry, id string) []string {
	for _, entry := range entries {
		if entry.Source == "local" && entry.Record.ID == id {
			return append([]string(nil), entry.Record.Endpoints...)
		}
	}
	return nil
}

func generationHeader(generation int64) string {
	return fmt.Sprintf("%d", generation)
}

func recordIDs(entries []discovery.Entry) []string {
	out := make([]string, 0, len(entries))
	for _, item := range entries {
		out = append(out, item.Record.ID)
	}
	return out
}

func cloneEntries(entries []discovery.Entry) []discovery.Entry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]discovery.Entry, 0, len(entries))
	for _, item := range entries {
		entry := item
		entry.Record.Endpoints = append([]string(nil), item.Record.Endpoints...)
		out = append(out, entry)
	}
	return out
}

func containsRecord(entries []discovery.Entry, id string) bool {
	for _, item := range entries {
		if item.Record.ID == id {
			return true
		}
	}
	return false
}

func TestCloneStringsCopiesPublicationEndpoints(t *testing.T) {
	in := []string{"tcp://node:9000"}
	out := cloneStrings(in)
	out[0] = "mutated"
	require.Equal(t, "tcp://node:9000", in[0])
}

func requirePublishedNodeRecord(t *testing.T, disco *discovery.Service, ident identityapi.Summary, private ed25519.PrivateKey) {
	t.Helper()
	{
		err := PublishLocalNode(disco, ident, private, []string{"tcp://node:9000"})
		require.NoErrorf(t, err, "publish node: %v", err)
	}

}
