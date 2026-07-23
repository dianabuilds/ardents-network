//go:build integration

package network_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ardents/internal/discovery"
	discoveryrecord "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
	transport "ardents/internal/network"
	"ardents/internal/publication"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestTransportStoreFetchesPublishedDiscoveryRecord(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "network-foundation",
		ScenarioID:  "NFI-003",
		Suite:       "integration",
		Tags:        []string{"integration", "network", "store"},
		Speed:       "default",
		Environment: "local",
	})

	ctx := t.Context()
	var remote transport.Service
	var local transport.Service
	var published discovery.Record
	now := time.Now().UTC().Truncate(time.Second)
	fixture := testkit.NewDiscoveryPrivacyFixture(t, now)
	senderChannel, receiverChannel := fixture.Sender, fixture.Receiver

	scenario.Precondition("start remote transport with published discovery entry", func(t *testing.T) {
		remote = testkit.StartTransport(t)
		published = signedNodeDiscoveryRecord(t, 1, now, nil)
		require.NoError(t, publication.PublishPrivateDiscoveryEntries(ctx, []discovery.Entry{{
			Record: published,
			Source: "local",
			SeenAt: time.Now().UTC(),
		}}, senderChannel, testkit.PrivateCarrier(remote)))
	})

	scenario.Step("start bootstrapped local transport", func(t *testing.T) {
		local = testkit.StartBootstrappedTransport(t, remote)
		testkit.WaitForRelayReadiness(t, local)
	})

	scenario.Assert("store fetch returns the published discovery record", func(t *testing.T) {
		testkit.WaitForCondition(t, 5*time.Second, "store fetch returns the published discovery record", func() (bool, string) {
			result, err := discovery.FetchPrivateRecords(ctx, remote.Endpoints(), receiverChannel, testkit.PrivateCarrier(local))
			if err != nil {
				return false, err.Error()
			}
			if len(result.Entries) != 1 {
				return false, "unexpected record count"
			}
			if result.Entries[0].Record.Subject() != published.Subject() {
				return false, result.Entries[0].Record.Subject()
			}
			return true, ""
		})
	})
}

func TestTransportStoreKeepsLatestWithdrawalEntry(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "network-foundation",
		ScenarioID:  "NFI-003",
		Suite:       "integration",
		Tags:        []string{"integration", "network", "store"},
		Speed:       "default",
		Environment: "local",
	})

	ctx := t.Context()
	var remote transport.Service
	var local transport.Service
	now := time.Now().UTC().Truncate(time.Second)
	fixture := testkit.NewDiscoveryPrivacyFixture(t, now)
	senderChannel, receiverChannel := fixture.Sender, fixture.Receiver

	scenario.Precondition("start remote transport with published and withdrawn service entries", func(t *testing.T) {
		remote = testkit.StartTransport(t)
		published := signedServiceDiscoveryRecord(t, 2, "svc.remote.echo", "echo", []string{"tcp://remote"}, now)
		withdrawn := signedServiceDiscoveryRecord(t, 2, "svc.remote.echo", "echo", nil, now.Add(time.Second))
		require.NoError(t, publication.PublishPrivateDiscoveryEntries(ctx, []discovery.Entry{
			{
				Record: published,
				Source: "local",
				SeenAt: now,
			},
			{
				Record: withdrawn,
				Source: "local",
				SeenAt: now.Add(time.Second),
			},
		}, senderChannel, testkit.PrivateCarrier(remote)))
	})

	scenario.Step("start bootstrapped local transport", func(t *testing.T) {
		local = testkit.StartBootstrappedTransport(t, remote)
		testkit.WaitForRelayReadiness(t, local)
	})

	scenario.Assert("store fetch returns the latest withdrawal entry", func(t *testing.T) {
		testkit.WaitForCondition(t, 5*time.Second, "store fetch keeps latest withdrawal entry", func() (bool, string) {
			result, err := discovery.FetchPrivateRecords(ctx, remote.Endpoints(), receiverChannel, testkit.PrivateCarrier(local))
			if err != nil {
				return false, err.Error()
			}
			if len(result.Entries) != 1 {
				return false, "unexpected record count"
			}
			if len(result.Entries[0].Record.EndpointList()) != 0 {
				return false, "withdrawal entry still exposes endpoints"
			}
			return true, ""
		})
	})
}

func TestPrivateStoreRejectsSenderRevokedAfterPublication(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "network-foundation", ScenarioID: "NPI-003",
		Suite: "integration", Tags: []string{"integration", "network", "store", "privacy", "revocation"},
		Speed: "default", Environment: "local",
	})
	ctx := t.Context()
	now := time.Now().UTC().Truncate(time.Second)
	privacy := testkit.NewDiscoveryPrivacyFixture(t, now)
	var remote transport.Service
	var local transport.Service

	scenario.Precondition("publish a private discovery record before sender revocation", func(t *testing.T) {
		remote = testkit.StartTransport(t)
		require.NoError(t, publication.PublishPrivateDiscoveryEntries(ctx, []discovery.Entry{{
			Record: signedNodeDiscoveryRecord(t, 3, now, nil),
			Source: "local", SeenAt: now,
		}}, privacy.Sender, testkit.PrivateCarrier(remote)))
		privacy.RevokeSender(t, now)
	})

	scenario.Step("fetch encrypted history after revocation", func(t *testing.T) {
		local = testkit.StartBootstrappedTransport(t, remote)
		testkit.WaitForRelayReadiness(t, local)
	})

	scenario.Assert("receive-time authorization rejects the retained envelope", func(t *testing.T) {
		testkit.WaitForCondition(t, 5*time.Second, "revoked retained envelope is rejected", func() (bool, string) {
			result, err := discovery.FetchPrivateRecords(ctx, remote.Endpoints(), privacy.Receiver, testkit.PrivateCarrier(local))
			if err != nil {
				return false, err.Error()
			}
			if result.Rejected == 0 {
				return false, "store envelope not visible yet"
			}
			return len(result.Entries) == 0, result.Reason
		})
	})
}

func TestTransportPersistentStoreCreatesBackingFile(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "network-foundation",
		ScenarioID:  "NFI-003",
		Suite:       "integration",
		Tags:        []string{"integration", "network", "store"},
		Speed:       "default",
		Environment: "local",
	})

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	storePath := filepath.Join(t.TempDir(), "waku-store.db")
	testkit.ConfigureLoopbackTransport(t)
	svc := testkit.NewTransport(transport.Config{StorePath: storePath})

	scenario.Precondition("start transport with persistent store path", func(t *testing.T) {
		require.NoError(t, svc.Start(ctx))
		t.Cleanup(func() {
			_ = svc.Stop(context.Background())
		})
	})

	scenario.Assert("persistent store creates a backing file", func(t *testing.T) {
		info, err := os.Stat(storePath)
		require.NoError(t, err)
		require.False(t, info.IsDir())
	})
}

func signedNodeDiscoveryRecord(t *testing.T, seed byte, issued time.Time, endpoints []string) discovery.Record {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(private.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	record := discovery.Record{
		Version: discoveryrecord.Version,
		Node: &discoveryrecord.NodeFacts{
			Principal: principal,
			PublicKey: base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey)),
			Endpoints: append([]string(nil), endpoints...),
		},
		IssuedAt: issued, ExpiresAt: issued.Add(time.Hour),
	}
	return signDiscoveryRecord(t, record, private)
}

func signedServiceDiscoveryRecord(t *testing.T, seed byte, id, serviceType string, endpoints []string, issued time.Time) discovery.Record {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(private.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	record := discovery.Record{
		Version: discoveryrecord.Version,
		Service: &discoveryrecord.ServiceFacts{
			ID: discoveryrecord.ServiceID(id), Type: serviceType, NodePrincipal: principal,
			Workload: "work.remote.echo", Mode: "NetworkPublished",
			PublicKey: base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey)),
			Endpoints: append([]string(nil), endpoints...),
		},
		IssuedAt: issued, ExpiresAt: issued.Add(time.Hour),
	}
	return signDiscoveryRecord(t, record, private)
}

func signDiscoveryRecord(t *testing.T, record discovery.Record, private ed25519.PrivateKey) discovery.Record {
	t.Helper()
	payload, err := discovery.Canonical(record)
	require.NoError(t, err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
	return record
}
