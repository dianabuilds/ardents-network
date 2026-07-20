package authority

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"ardents/internal/diagnostics"
	discovery "ardents/internal/discovery"
	identityapi "ardents/internal/identity/api"
	nodelifecycle "ardents/internal/node/lifecycle"
	db "ardents/internal/persistence"
	workloadapi "ardents/internal/workload/api"

	"github.com/stretchr/testify/require"
)

func TestRequireAuthoritativeStateMutableRejectsFailedNode(t *testing.T) {
	ctl := &Controller{life: nodelifecycle.NewMachine()}
	for _, state := range []string{nodelifecycle.Starting, nodelifecycle.Initializing, nodelifecycle.Failed} {
		err := ctl.life.Move(state)
		require.NoErrorf(t, err, "move to %s failed: %v", state, err)
	}

	err := ctl.requireAuthoritativeStateMutableLocked("data publish")
	require.Error(t, err, "expected failed-node rejection")
}

func TestRecordWorkloadRefreshFailureProjectsDiagnosticsTruth(t *testing.T) {
	life := nodelifecycle.NewMachine()
	require.NoErrorf(t, life.Move(nodelifecycle.Starting), "move starting")
	require.NoErrorf(t, life.Move(nodelifecycle.Initializing), "move initializing")
	require.NoErrorf(t, life.Move(nodelifecycle.Ready), "move ready")

	ctl := &Controller{
		cfgName: "node-1",
		life:    life,
		diag:    diagnostics.New(""),
		publish: func(string, map[string]any) {},
	}

	ctl.recordWorkloadRefreshFailureLocked(errors.New("publish desired services: open blocked-parent: not a directory"))

	snapshot := ctl.diag.Snapshot()
	require.Equal(t, diagnostics.HealthDegraded, snapshot.Health.State)
	require.NotNil(t, snapshot.Health.PrimaryReason, "expected primary reason after workload refresh failure")
	require.Equal(t, workloadRefreshFailedCode, snapshot.Health.PrimaryReason.Code)
	require.Equal(t, "workload", snapshot.Health.PrimaryReason.Domain)
	require.Len(t, snapshot.RecentEvents, 1)
	require.Equal(t, "refresh_failed", snapshot.RecentEvents[0].Type)
	require.Equal(t, workloadRefreshFailedCode, snapshot.RecentEvents[0].ReasonCode)
	require.Equal(t, nodelifecycle.Degraded, life.State())
}

func TestWorkloadSpecFromAPIClonesSlices(t *testing.T) {
	spec := workloadapi.WorkloadSpecSnapshot{
		ID:           "wl-1",
		Kind:         "service",
		Capabilities: []string{"exec"},
		Services: []workloadapi.PublishedServiceSnapshot{{
			ID:        "svc-1",
			Type:      "http",
			Owner:     "node",
			Mode:      "hosted",
			Endpoints: []string{"tcp://127.0.0.1:9000"},
		}},
	}

	out := workloadSpecFromAPI(spec)
	out.Capabilities[0] = "mutated"
	out.Services[0].Endpoints[0] = "mutated"

	require.Equal(t, "exec", spec.Capabilities[0])
	require.Equal(t, "tcp://127.0.0.1:9000", spec.Services[0].Endpoints[0])
}

func TestSyncDiscoveryTrustDiagnosticsKeepsNodeReadyForUntrustedCatalogEntry(t *testing.T) {
	life := nodelifecycle.NewMachine()
	require.NoError(t, life.Move(nodelifecycle.Starting))
	require.NoError(t, life.Move(nodelifecycle.Initializing))
	require.NoError(t, life.Move(nodelifecycle.Ready))

	record := signedTrustRecord(t)
	disco := discovery.New("")
	_, err := disco.Import(record, "bootstrap")
	require.NoError(t, err)

	ctl := &Controller{
		cfgName: "node-1",
		life:    life,
		diag:    diagnostics.New(""),
		disco:   disco,
		trust:   discovery.NewTrustEvaluator(),
		publish: func(string, map[string]any) {},
	}

	ctl.SyncDiscoveryTrustDiagnosticsLocked()

	snapshot := ctl.diag.Snapshot()
	require.Equal(t, diagnostics.HealthReady, snapshot.Health.State)
	require.Nil(t, snapshot.Health.PrimaryReason)
	require.Empty(t, snapshot.Health.Subsystems)
	require.Len(t, snapshot.RecentEvents, 1)
	require.Equal(t, "catalog_untrusted", snapshot.RecentEvents[0].Type)
	require.Equal(t, "trust.record.untrusted", snapshot.RecentEvents[0].ReasonCode)
	require.Equal(t, nodelifecycle.Ready, life.State())
}

func TestSyncDiscoveryTrustDiagnosticsPrioritizesInvalidCatalogEntryOverUntrusted(t *testing.T) {
	life := nodelifecycle.NewMachine()
	require.NoError(t, life.Move(nodelifecycle.Starting))
	require.NoError(t, life.Move(nodelifecycle.Initializing))
	require.NoError(t, life.Move(nodelifecycle.Ready))

	untrusted := signedTrustRecord(t)
	invalid := signedTrustRecord(t)
	invalid.Signature = "not-base64"

	dir := t.TempDir()
	path := filepath.Join(dir, "ardents.db")
	err := db.SaveJSON(path, "discovery", "records", map[string]any{
		"records": []discovery.Entry{
			{Record: untrusted, Source: "bootstrap", SeenAt: time.Now().UTC()},
			{Record: invalid, Source: "cache", SeenAt: time.Now().UTC()},
		},
		"state": "ready",
	})
	require.NoError(t, err)

	disco := discovery.New(path)
	require.NoError(t, disco.Load())

	ctl := &Controller{
		cfgName: "node-1",
		life:    life,
		diag:    diagnostics.New(""),
		disco:   disco,
		trust:   discovery.NewTrustEvaluator(),
		publish: func(string, map[string]any) {},
	}

	ctl.SyncDiscoveryTrustDiagnosticsLocked()

	snapshot := ctl.diag.Snapshot()
	require.Equal(t, diagnostics.HealthDegraded, snapshot.Health.State)
	require.NotNil(t, snapshot.Health.PrimaryReason)
	require.Equal(t, "trust.record.invalid", snapshot.Health.PrimaryReason.Code)
	require.Len(t, snapshot.Health.Subsystems, 1)
	require.Equal(t, "trust.record.invalid", snapshot.Health.Subsystems[0].Reason.Code)
	require.Equal(t, invalid.ID, snapshot.Health.Subsystems[0].Reason.Resource)
	require.Len(t, snapshot.RecentEvents, 1)
	require.Equal(t, "catalog_degraded", snapshot.RecentEvents[0].Type)
	require.Equal(t, "trust.record.invalid", snapshot.RecentEvents[0].ReasonCode)
	require.Equal(t, nodelifecycle.Degraded, life.State())
}

func signedTrustRecord(t *testing.T) discovery.Record {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	publicKey := base64.StdEncoding.EncodeToString(public)
	principal, err := identityapi.PrincipalFromPublicKey(publicKey)
	require.NoError(t, err)

	record := discovery.Record{
		ID:        principal + ":node",
		Kind:      "node",
		Subject:   principal,
		Node:      principal,
		Device:    "device-test",
		PublicKey: publicKey,
		Endpoints: []string{"tcp://remote:9000"},
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	payload, err := discovery.Canonical(record)
	require.NoError(t, err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
	return record
}
