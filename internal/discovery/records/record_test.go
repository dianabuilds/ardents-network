package records_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	discoveryrecord "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
	"github.com/stretchr/testify/require"
)

var recordNow = time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)

func TestValidateAcceptsVersionedNodeAndServiceRecords(t *testing.T) {
	node, key := signedNodeRecord(t)
	require.NoError(t, discoveryrecord.ValidateAt(node, recordNow))

	service := signedServiceRecord(t, key, node.Node.Principal)
	require.NoError(t, discoveryrecord.ValidateAt(service, recordNow))
	require.Equal(t, "svc.echo", service.RecordID())
	require.Equal(t, "work.echo", service.WorkloadID())
}

func TestValidateRejectsMalformedOrAmbiguousFacts(t *testing.T) {
	node, key := signedNodeRecord(t)
	service := signedServiceRecord(t, key, node.Node.Principal)
	for name, mutate := range map[string]func(*discoveryrecord.Record){
		"unknown version": func(record *discoveryrecord.Record) { record.Version++ },
		"both bodies":     func(record *discoveryrecord.Record) { record.Service = service.Service },
		"no body":         func(record *discoveryrecord.Record) { record.Node = nil },
		"duplicate endpoint": func(record *discoveryrecord.Record) {
			record.Node.Endpoints = []string{"tcp://node:9000", "tcp://node:9000"}
		},
		"invalid interval": func(record *discoveryrecord.Record) { record.ExpiresAt = record.IssuedAt },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := node.Clone()
			mutate(&candidate)
			signRecord(t, &candidate, key)
			require.Error(t, discoveryrecord.ValidateAt(candidate, recordNow))
		})
	}
}

func TestValidateRejectsTamperingAndWrongNodeBinding(t *testing.T) {
	node, key := signedNodeRecord(t)
	node.Node.Endpoints = append(node.Node.Endpoints, "tcp://tampered:9000")
	require.Error(t, discoveryrecord.ValidateAt(node, recordNow))

	service := signedServiceRecord(t, key, node.Node.Principal)
	other := ed25519.NewKeyFromSeed(bytesOf(9))
	otherPrincipal, err := identityprincipal.FromEd25519PublicKey(other.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	service.Service.NodePrincipal = otherPrincipal
	signRecord(t, &service, key)
	require.Error(t, discoveryrecord.ValidateAt(service, recordNow))
}

func TestValidateUsesExactExpiryBoundary(t *testing.T) {
	record, _ := signedNodeRecord(t)
	require.NoError(t, discoveryrecord.ValidateAt(record, record.ExpiresAt.Add(-time.Nanosecond)))
	require.Error(t, discoveryrecord.ValidateAt(record, record.ExpiresAt))
	require.NoError(t, discoveryrecord.ValidateRetained(record))
}

func TestValidateRejectsRecordBeforeItsIssueBoundary(t *testing.T) {
	record, _ := signedNodeRecord(t)
	require.Error(t, discoveryrecord.ValidateAt(record, record.IssuedAt.Add(-time.Nanosecond)))
	require.NoError(t, discoveryrecord.ValidateAt(record, record.IssuedAt))
	require.NoError(t, discoveryrecord.ValidateRetained(record))
}

func signedNodeRecord(t *testing.T) (discoveryrecord.Record, ed25519.PrivateKey) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytesOf(1))
	principal, err := identityprincipal.FromEd25519PublicKey(key.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	record := discoveryrecord.Record{Version: discoveryrecord.Version, Node: &discoveryrecord.NodeFacts{
		Principal: principal, PublicKey: base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)), Endpoints: []string{"tcp://node:9000"},
	}, IssuedAt: recordNow, ExpiresAt: recordNow.Add(time.Hour)}
	signRecord(t, &record, key)
	return record, key
}

func signedServiceRecord(t *testing.T, key ed25519.PrivateKey, principal identityprincipal.ID) discoveryrecord.Record {
	t.Helper()
	record := discoveryrecord.Record{Version: discoveryrecord.Version, Service: &discoveryrecord.ServiceFacts{
		ID: "svc.echo", Type: "echo", NodePrincipal: principal, Workload: "work.echo", Mode: "NetworkPublished",
		PublicKey: base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)), Endpoints: []string{"tcp://node:9001"},
	}, IssuedAt: recordNow, ExpiresAt: recordNow.Add(time.Hour)}
	signRecord(t, &record, key)
	return record
}

func signRecord(t *testing.T, record *discoveryrecord.Record, key ed25519.PrivateKey) {
	t.Helper()
	payload, err := discoveryrecord.Canonical(*record)
	require.NoError(t, err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))
}

func bytesOf(value byte) []byte {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = value
	}
	return seed
}
