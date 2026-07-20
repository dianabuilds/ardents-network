package publication

import (
	"context"
	"crypto/ed25519"
	"testing"

	discovery "ardents/internal/discovery"
	hostingapi "ardents/internal/hosting/api"
	hostingregistry "ardents/internal/hosting/registry"
	transport "ardents/internal/network/api"
	nodelifecycle "ardents/internal/node/lifecycle"
	apppolicy "ardents/internal/policy"
	workloadcontroller "ardents/internal/workload/controller"

	"github.com/stretchr/testify/require"
)

func TestLocalPresenceSnapshotLockedRequiresPublicationWhenReady(t *testing.T) {
	mgr := testPublicationManager(t)
	require.NoError(t, mgr.life.Move(nodelifecycle.Starting))
	require.NoError(t, mgr.life.Move(nodelifecycle.Initializing))
	require.NoError(t, mgr.life.Move(nodelifecycle.Ready))

	snapshot := mgr.LocalPresenceSnapshotLocked()

	require.Equal(t, "unpublished", snapshot.State)
	require.True(t, snapshot.OperatorActionRequired)
}

func TestServicePublicationStatusLockedReflectsWithdrawnLocalService(t *testing.T) {
	mgr := testPublicationManager(t)
	id := mgr.ident.NodeSummary()
	private := mgr.privateKey()
	require.NoError(t, PublishLocalService(mgr.disco, id, private, LocalServiceSpec{
		ID:   "svc.echo",
		Type: "echo",
	}))

	snapshot := mgr.ServicePublicationStatusLocked("svc.echo")

	require.Equal(t, hostingapi.PublicationStatusSnapshot{
		State:                  "withdrawn",
		Reason:                 "service publication has no active endpoints",
		PublishedAt:            snapshot.PublishedAt,
		ExpiresAt:              snapshot.ExpiresAt,
		OperatorActionRequired: true,
	}, snapshot)
}

func TestServicePublicationStatusLockedIgnoresRemoteDiscoveryKnowledge(t *testing.T) {
	mgr := testPublicationManager(t)
	helper := discovery.NewInDir(t.TempDir())
	id := mgr.ident.NodeSummary()
	private := mgr.privateKey()
	require.NoError(t, PublishLocalService(helper, id, private, LocalServiceSpec{
		ID:        "svc.remote-only",
		Type:      "echo",
		Endpoints: []string{"tcp://remote:9000"},
	}))
	record := helper.Entries()[0].Record
	_, err := mgr.disco.Import(record, "remote")
	require.NoError(t, err)

	snapshot := mgr.ServicePublicationStatusLocked("svc.remote-only")

	require.Equal(t, hostingapi.PublicationStatusSnapshot{}, snapshot)
}

func TestHostedServiceSnapshotLockedUsesPolicyFilteredPublicationTruth(t *testing.T) {
	dir := t.TempDir()
	identSvc, private := publicationIdentity(t, dir)
	workloadSvc := workloadcontroller.NewWithExecutorInDir(dir, &publicationRunningExecutor{})
	require.NoError(t, workloadSvc.Register(workloadcontroller.Spec{
		ID:      "work.admin",
		Kind:    "service",
		Owner:   "node",
		Config:  "test",
		Desired: workloadcontroller.DesiredRunning,
		Services: []workloadcontroller.ServiceSpec{{
			ID:        "svc.admin",
			Type:      "admin",
			Owner:     "work.admin",
			Mode:      "NetworkPublished",
			Endpoints: []string{"tcp://admin:9000"},
		}},
	}))
	require.NoError(t, workloadSvc.Reconcile(context.Background()))
	mgr := NewManager(
		"publication-test",
		nil,
		nodelifecycle.NewMachine(),
		discovery.NewInDir(dir),
		apppolicy.New(apppolicy.Config{DeniedServiceTypes: []string{"admin"}}),
		hostingregistry.New(nil),
		workloadSvc,
		transport.New(),
		identSvc,
		nil,
		func() ed25519.PrivateKey { return private },
		func(string, map[string]any) {},
	)

	snapshot, err := mgr.HostedServiceSnapshotLocked("svc.admin")

	require.NoError(t, err)
	require.False(t, snapshot.Published)
	require.Equal(t, "warming", snapshot.State)
	require.Equal(t, "warming_up", snapshot.Reason)
	require.False(t, snapshot.Ready)
	require.False(t, snapshot.ExposureEligible)
	require.Equal(t, "unpublished", snapshot.Publication.State)
	require.Equal(t, "policy_publication_denied: service type is denied by policy", snapshot.Publication.Reason)
	require.True(t, snapshot.OperatorActionRequired)
}

func TestHostedServiceSnapshotLockedKeepsInventoryWithoutRuntimeBacking(t *testing.T) {
	dir := t.TempDir()
	identSvc, private := publicationIdentity(t, dir)
	workloadSvc := workloadcontroller.NewWithExecutorInDir(dir, &publicationRunningExecutor{})
	require.NoError(t, workloadSvc.Register(workloadcontroller.Spec{
		ID:      "work.echo",
		Kind:    "service",
		Owner:   "node",
		Config:  "test",
		Desired: workloadcontroller.DesiredStopped,
		Services: []workloadcontroller.ServiceSpec{{
			ID:        "svc.echo",
			Type:      "echo",
			Owner:     "work.echo",
			Mode:      "NetworkPublished",
			Endpoints: []string{"tcp://echo:9000"},
		}},
	}))
	require.NoError(t, workloadSvc.Reconcile(context.Background()))
	mgr := NewManager(
		"publication-test",
		nil,
		nodelifecycle.NewMachine(),
		discovery.NewInDir(dir),
		nil,
		hostingregistry.New(nil),
		workloadSvc,
		transport.New(),
		identSvc,
		nil,
		func() ed25519.PrivateKey { return private },
		func(string, map[string]any) {},
	)

	snapshot, err := mgr.HostedServiceSnapshotLocked("svc.echo")

	require.NoError(t, err)
	require.Equal(t, "svc.echo", snapshot.ServiceID)
	require.Equal(t, "inactive", snapshot.State)
	require.False(t, snapshot.Published)
	require.False(t, snapshot.Ready)
	require.Equal(t, "runtime_inactive", snapshot.Reason)
	require.Equal(t, "unpublished", snapshot.Publication.State)
	require.Equal(t, "workload not running", snapshot.Publication.Reason)
}

func testPublicationManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	identSvc, private := publicationIdentity(t, dir)
	return NewManager(
		"publication-test",
		nil,
		nodelifecycle.NewMachine(),
		discovery.NewInDir(dir),
		nil,
		nil,
		nil,
		nil,
		identSvc,
		nil,
		func() ed25519.PrivateKey { return private },
		func(string, map[string]any) {},
	)
}
