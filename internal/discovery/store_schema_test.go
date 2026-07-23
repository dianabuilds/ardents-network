package discovery

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	discoveryrecord "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"
	"ardents/internal/storage"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
)

func TestLoadRejectsPreReleaseUnversionedDiscoverySnapshot(t *testing.T) {
	dir := t.TempDir()
	db, err := bbolt.Open(storage.PathInDir(dir), 0o600, nil)
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucket([]byte("discovery"))
		if err != nil {
			return err
		}
		return bucket.Put([]byte("records"), []byte(`{"records":[],"state":"ready"}`))
	}))
	require.NoError(t, db.Close())

	require.Error(t, NewInDir(dir).Load())
}

func TestSnapshotV2RoundTripsNodeAndServiceRecords(t *testing.T) {
	dir := t.TempDir()
	node, key := schemaNodeRecord(t, 1, time.Now().UTC())
	service := schemaServiceRecord(t, key, node.Node.Principal, time.Now().UTC())
	store := NewInDir(dir)
	_, err := store.Import(node, discoveryrecord.Local)
	require.NoError(t, err)
	_, err = store.Import(service, discoveryrecord.Local)
	require.NoError(t, err)

	reloaded := NewInDir(dir)
	require.NoError(t, reloaded.Load())
	require.Equal(t, []Record{node, service}, reloaded.Records())
}

func TestSnapshotV2RetainsCanonicalBootstrapProvenance(t *testing.T) {
	dir := t.TempDir()
	node, _ := schemaNodeRecord(t, 1, time.Now().UTC())
	require.NoError(t, SaveSnapshot(PathInDir(dir), Snapshot{
		SchemaVersion: 2,
		Records:       []Entry{{Record: node, Source: discoveryrecord.Bootstrap, SeenAt: time.Now().UTC(), Evidence: schemaEvidence(t, node)}},
		State:         "ready",
	}))
	store := NewInDir(dir)
	require.NoError(t, store.Load())
	require.Equal(t, discoveryrecord.Bootstrap, store.Entries()[0].Source)
}

func TestLoadRetainsExpiredSignedRecordButDoesNotRouteIt(t *testing.T) {
	dir := t.TempDir()
	issued := time.Now().UTC().Add(-2 * time.Hour)
	node, key := schemaNodeRecord(t, 1, issued)
	service := schemaServiceRecord(t, key, node.Node.Principal, issued)
	require.NoError(t, SaveSnapshot(PathInDir(dir), Snapshot{
		SchemaVersion: 2,
		Records:       []Entry{{Record: service, Source: discoveryrecord.Local, SeenAt: issued, Evidence: schemaEvidence(t, service)}},
		State:         "ready",
	}))

	store := NewInDir(dir)
	require.NoError(t, store.Load())
	require.Len(t, store.Records(), 1)
	require.Empty(t, store.FindService("echo"))
}

func TestLoadRejectsStrictSnapshotViolations(t *testing.T) {
	tests := map[string]string{
		"missing records": `{"schema_version":2,"state":"ready"}`,
		"null records":    `{"schema_version":2,"records":null,"state":"ready"}`,
		"unknown field":   `{"schema_version":2,"records":[],"state":"ready","surprise":true}`,
		"duplicate field": `{"schema_version":2,"schema_version":2,"records":[],"state":"ready"}`,
		"trailing value":  `{"schema_version":2,"records":[],"state":"ready"} true`,
		"old version":     `{"schema_version":1,"records":[],"state":"ready"}`,
		"unknown version": `{"schema_version":3,"records":[],"state":"ready"}`,
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			schemaWriteRawSnapshot(t, dir, payload)
			require.Error(t, NewInDir(dir).Load())
		})
	}
}

func TestLoadRejectsMissingOrTamperedVerificationEvidence(t *testing.T) {
	node, _ := schemaNodeRecord(t, 1, time.Now().UTC())
	for name, entry := range map[string]map[string]any{
		"missing evidence": {
			"record": node, "source": discoveryrecord.Local, "seen_at": time.Now().UTC(),
		},
		"tampered record": {
			"record": func() Record {
				tampered := node.Clone()
				tampered.Node.Endpoints = append(tampered.Node.Endpoints, "tcp://tampered:9000")
				return tampered
			}(),
			"source": discoveryrecord.Local, "seen_at": time.Now().UTC(), "evidence": schemaEvidence(t, node),
		},
		"tampered trust result": {
			"record": node, "source": discoveryrecord.Local, "seen_at": time.Now().UTC(),
			"evidence": func() discoveryrecord.VerificationEvidence {
				evidence := schemaEvidence(t, node)
				evidence.Trusted = !evidence.Trusted
				return evidence
			}(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, storage.SaveJSON(PathInDir(dir), "discovery", "records", map[string]any{
				"schema_version": 2, "records": []map[string]any{entry}, "state": "ready",
			}))
			require.Error(t, NewInDir(dir).Load())
		})
	}
}

func TestLoadRefreshesAndPersistsEvidenceAfterTrustRotation(t *testing.T) {
	dir := t.TempDir()
	node, _ := schemaNodeRecord(t, 1, time.Now().UTC())
	trusted := schemaTrustRegistry(t, node)
	store := NewInDirWithTrust(dir, NewTrustEvaluator(trusted))
	_, err := store.Import(node, discoveryrecord.Local)
	require.NoError(t, err)
	before := store.Entries()[0].Evidence
	require.True(t, before.Trusted)

	empty, err := identitytrust.NewRegistry(nil)
	require.NoError(t, err)
	reloaded := NewInDirWithTrust(dir, NewTrustEvaluator(empty))
	require.NoError(t, reloaded.Load())
	after := reloaded.Entries()[0].Evidence
	require.False(t, after.Trusted)
	require.NotEqual(t, before.TrustGeneration, after.TrustGeneration)

	var persisted Snapshot
	found, err := LoadSnapshot(PathInDir(dir), &persisted)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, after, persisted.Records[0].Evidence)
}

func TestLoadRollsBackEvidenceRefreshWhenPersistenceFails(t *testing.T) {
	dir := t.TempDir()
	node, _ := schemaNodeRecord(t, 1, time.Now().UTC())
	trusted := schemaTrustRegistry(t, node)
	store := NewInDirWithTrust(dir, NewTrustEvaluator(trusted))
	_, err := store.Import(node, discoveryrecord.Local)
	require.NoError(t, err)
	before := store.Entries()[0].Evidence

	empty, err := identitytrust.NewRegistry(nil)
	require.NoError(t, err)
	reloaded := NewInDirWithTrust(dir, NewTrustEvaluator(empty))
	reloaded.persist = func(string, any) error { return errors.New("injected persistence failure") }
	require.ErrorContains(t, reloaded.Load(), "persist refreshed discovery evidence")
	require.Empty(t, reloaded.Entries())
	require.Equal(t, "new", reloaded.State())

	var persisted Snapshot
	found, err := LoadSnapshot(PathInDir(dir), &persisted)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, before, persisted.Records[0].Evidence)
}

func TestRestoreRejectsDuplicateRecordWithoutMutation(t *testing.T) {
	node, _ := schemaNodeRecord(t, 1, time.Now().UTC())
	entry := Entry{Record: node, Source: discoveryrecord.Local, SeenAt: time.Now().UTC()}
	store := New("")
	require.NoError(t, store.Restore([]Entry{entry}, "ready", ""))
	want := store.Entries()

	err := store.Restore([]Entry{entry, entry}, "ready", "")
	require.Error(t, err)
	got, state, reason := store.Snapshot()
	require.Equal(t, want, got)
	require.Equal(t, "ready", state)
	require.Empty(t, reason)
}

func TestImportAndRestoreRollBackMemoryOnSaveFailure(t *testing.T) {
	now := time.Now().UTC()
	first, _ := schemaNodeRecord(t, 1, now)
	second, _ := schemaNodeRecord(t, 2, now)
	initial := Entry{Record: first, Source: discoveryrecord.Local, SeenAt: now}
	store := New(t.TempDir()) // A directory cannot be opened as the bbolt database file.
	store.records, store.state = []Entry{initial}, "ready"

	_, err := store.Import(second, discoveryrecord.Local)
	require.Error(t, err)
	require.Equal(t, []Entry{initial}, store.Entries())
	require.Equal(t, "ready", store.State())

	replacement := Entry{Record: second, Source: discoveryrecord.Local, SeenAt: now}
	err = store.Restore([]Entry{replacement}, "degraded", "test")
	require.Error(t, err)
	got, state, reason := store.Snapshot()
	require.Equal(t, []Entry{initial}, got)
	require.Equal(t, "ready", state)
	require.Empty(t, reason)
}

func TestRecordsAndEntriesReturnDetachedFacts(t *testing.T) {
	node, _ := schemaNodeRecord(t, 1, time.Now().UTC())
	entry := Entry{Record: node, Source: discoveryrecord.Local, SeenAt: time.Now().UTC()}
	store := New("")
	require.NoError(t, store.Restore([]Entry{entry}, "ready", ""))

	records := store.Records()
	entries := store.Entries()
	records[0].Node.Endpoints[0] = "tcp://mutated"
	entries[0].Record.Node.Endpoints[0] = "tcp://also-mutated"
	require.Equal(t, node.Node.Endpoints, store.Records()[0].Node.Endpoints)
}

func schemaNodeRecord(t *testing.T, seed byte, issued time.Time) (Record, ed25519.PrivateKey) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(key.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	record := Record{Version: discoveryrecord.Version, Node: &discoveryrecord.NodeFacts{
		Principal: principal, PublicKey: base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)), Endpoints: []string{"tcp://node:9000"},
	}, IssuedAt: issued, ExpiresAt: issued.Add(time.Hour)}
	schemaSignRecord(t, &record, key)
	return record, key
}

func schemaServiceRecord(t *testing.T, key ed25519.PrivateKey, principal identityprincipal.ID, issued time.Time) Record {
	t.Helper()
	record := Record{Version: discoveryrecord.Version, Service: &discoveryrecord.ServiceFacts{
		ID: "svc.echo", Type: "echo", NodePrincipal: principal, Workload: "work.echo", Mode: "direct",
		PublicKey: base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)), Endpoints: []string{"tcp://node:9001"},
	}, IssuedAt: issued, ExpiresAt: issued.Add(time.Hour)}
	schemaSignRecord(t, &record, key)
	return record
}

func schemaSignRecord(t *testing.T, record *Record, key ed25519.PrivateKey) {
	t.Helper()
	payload, err := Canonical(*record)
	require.NoError(t, err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))
}

func schemaEvidence(t *testing.T, record Record) discoveryrecord.VerificationEvidence {
	t.Helper()
	evidence, err := NewTrustEvaluator(nil).VerifyRetained(record)
	require.NoError(t, err)
	return evidence
}

func schemaTrustRegistry(t *testing.T, record Record) *identitytrust.Registry {
	t.Helper()
	public, err := base64.StdEncoding.DecodeString(record.PublicKeyText())
	require.NoError(t, err)
	registry, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: record.NodeID(), PublicKey: ed25519.PublicKey(public),
		Purposes: []identitytrust.Purpose{identitytrust.PurposeDiscoveryPublish},
	}})
	require.NoError(t, err)
	return registry
}

func schemaWriteRawSnapshot(t *testing.T, dir, payload string) {
	t.Helper()
	db, err := bbolt.Open(storage.PathInDir(dir), 0o600, nil)
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucket([]byte("discovery"))
		if err != nil {
			return err
		}
		return bucket.Put([]byte("records"), []byte(payload))
	}))
	require.NoError(t, db.Close())
}
