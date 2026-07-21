package daemon

import (
	"ardents/internal/diagnostics"
	"ardents/internal/discovery"
	identityprincipal "ardents/internal/identity/principal"
	db "ardents/internal/storage"
	workloadapi "ardents/internal/workload"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRequireAuthoritativeStateMutableRejectsFailedNode(t *testing.T) {
	ctl := &workloadHealth{life: diagnostics.NewMachine()}
	for _, state := range []string{diagnostics.Starting, diagnostics.Initializing, diagnostics.Failed} {
		err := ctl.life.Move(state)
		require.NoErrorf(t, err, "move to %s failed: %v", state, err)
	}

	err := ctl.requireMutable("data publish")
	require.Error(t, err, "expected failed-node rejection")
}

func TestRecordWorkloadRefreshFailureProjectsDiagnosticsTruth(t *testing.T) {
	life := diagnostics.NewMachine()
	require.NoErrorf(t, life.Move(diagnostics.Starting), "move starting")
	require.NoErrorf(t, life.Move(diagnostics.Initializing), "move initializing")
	require.NoErrorf(t, life.Move(diagnostics.Ready), "move ready")

	ctl := &workloadHealth{
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
	require.Equal(t, diagnostics.Degraded, life.State())
}

func TestWorkloadSpecFromAPIClonesSlices(t *testing.T) {
	spec := workloadapi.SpecSnapshot{
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

	out := workloadapi.SpecFromSnapshot(spec)
	out.Capabilities[0] = "mutated"
	out.Services[0].Endpoints[0] = "mutated"

	require.Equal(t, "exec", spec.Capabilities[0])
	require.Equal(t, "tcp://127.0.0.1:9000", spec.Services[0].Endpoints[0])
}

func TestSyncDiscoveryTrustDiagnosticsKeepsNodeReadyForUntrustedCatalogEntry(t *testing.T) {
	life := diagnostics.NewMachine()
	require.NoError(t, life.Move(diagnostics.Starting))
	require.NoError(t, life.Move(diagnostics.Initializing))
	require.NoError(t, life.Move(diagnostics.Ready))

	record := signedTrustRecord(t)
	disco := discovery.New("")
	_, err := disco.Import(record, "bootstrap")
	require.NoError(t, err)

	ctl := &RuntimeManager{
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
	require.Equal(t, diagnostics.Ready, life.State())
}

func TestSyncDiscoveryTrustDiagnosticsPrioritizesInvalidCatalogEntryOverUntrusted(t *testing.T) {
	life := diagnostics.NewMachine()
	require.NoError(t, life.Move(diagnostics.Starting))
	require.NoError(t, life.Move(diagnostics.Initializing))
	require.NoError(t, life.Move(diagnostics.Ready))

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

	ctl := &RuntimeManager{
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
	require.Equal(t, diagnostics.Degraded, life.State())
}

func signedTrustRecord(t *testing.T) discovery.Record {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	publicKey := base64.StdEncoding.EncodeToString(public)
	principal, err := identityprincipal.FromPublicKey(publicKey)
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
