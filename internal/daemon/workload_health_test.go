package daemon

import (
	"ardents/internal/diagnostics"
	"ardents/internal/discovery"
	discoveryrecord "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
	db "ardents/internal/storage"
	workloadapi "ardents/internal/workload"
	workloadregistry "ardents/internal/workload/registry"
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
		Requirements: []workloadregistry.WorkloadRequirement{"exec"},
		Services: []workloadapi.PublishedServiceSnapshot{{
			ID:        "svc-1",
			Type:      "http",
			Owner:     "node",
			Mode:      "hosted",
			Endpoints: []string{"tcp://127.0.0.1:9000"},
		}},
	}

	out, err := workloadapi.SpecFromSnapshot(spec)
	require.NoError(t, err)
	out.Requirements[0] = "mutated"
	out.Services[0].Endpoints[0] = "mutated"

	require.Equal(t, workloadregistry.WorkloadRequirement("exec"), spec.Requirements[0])
	require.Equal(t, "tcp://127.0.0.1:9000", spec.Services[0].Endpoints[0])
}

func TestSyncDiscoveryTrustDiagnosticsKeepsNodeReadyForUntrustedCatalogEntry(t *testing.T) {
	life := diagnostics.NewMachine()
	require.NoError(t, life.Move(diagnostics.Starting))
	require.NoError(t, life.Move(diagnostics.Initializing))
	require.NoError(t, life.Move(diagnostics.Ready))

	record := signedTrustRecord(t)
	disco := discovery.New("")
	_, err := disco.Import(record, discoveryrecord.Imported)
	require.NoError(t, err)

	ctl := &RuntimeManager{
		cfgName: "node-1",
		life:    life,
		diag:    diagnostics.New(""),
		disco:   disco,
		trust:   discovery.NewTrustEvaluator(nil),
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

func TestDiscoveryLoadRejectsInvalidCatalogEntryBeforeDiagnostics(t *testing.T) {
	valid := signedTrustRecord(t)
	evidence, err := discovery.NewTrustEvaluator(nil).VerifyRetained(valid)
	require.NoError(t, err)
	invalid := valid.Clone()
	invalid.Signature = "not-base64"

	dir := t.TempDir()
	path := filepath.Join(dir, "ardents.db")
	err = db.SaveJSON(path, "discovery", "records", map[string]any{
		"schema_version": 2,
		"records": []discovery.Entry{
			{Record: invalid, Source: "cache", SeenAt: time.Now().UTC(), Evidence: evidence},
		},
		"state": "ready",
	})
	require.NoError(t, err)

	disco := discovery.New(path)
	require.Error(t, disco.Load())
}

func signedTrustRecord(t *testing.T) discovery.Record {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	publicKey := base64.StdEncoding.EncodeToString(public)
	principal, err := identityprincipal.FromEd25519PublicKey(public)
	require.NoError(t, err)

	record := discovery.Record{
		Version:   discoveryrecord.Version,
		Node:      &discoveryrecord.NodeFacts{Principal: principal, PublicKey: publicKey, Endpoints: []string{"tcp://remote:9000"}},
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	payload, err := discovery.Canonical(record)
	require.NoError(t, err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
	return record
}
