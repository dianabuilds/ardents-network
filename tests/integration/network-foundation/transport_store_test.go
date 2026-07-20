//go:build integration

package networkfoundation_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	discovery "ardents/internal/discovery"
	transport "ardents/internal/network/api"
	publication "ardents/internal/publication"
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
	now := time.Now().UTC().Truncate(time.Second)
	fixture := newRelayPrivacyFixture(t, now)
	senderChannel, receiverChannel := fixture.channels(t, now)

	scenario.Precondition("start remote transport with published discovery entry", func(t *testing.T) {
		remote = testkit.StartTransport(t)
		require.NoError(t, publication.PublishPrivateDiscoveryEntries(ctx, []discovery.Entry{{
			Record: discovery.Record{
				ID:      "remote:node",
				Kind:    "node",
				Subject: "remote",
				Node:    "remote",
			},
			Source: "local",
			SeenAt: time.Now().UTC(),
		}}, senderChannel, remote))
	})

	scenario.Step("start bootstrapped local transport", func(t *testing.T) {
		local = testkit.StartBootstrappedTransport(t, remote)
		testkit.WaitForRelayReadiness(t, local)
	})

	scenario.Assert("store fetch returns the published discovery record", func(t *testing.T) {
		testkit.WaitForCondition(t, 5*time.Second, "store fetch returns the published discovery record", func() (bool, string) {
			result, err := discovery.FetchPrivateRecords(ctx, remote.Endpoints(), receiverChannel, local)
			if err != nil {
				return false, err.Error()
			}
			if len(result.Entries) != 1 {
				return false, "unexpected record count"
			}
			if result.Entries[0].Record.Subject != "remote" {
				return false, result.Entries[0].Record.Subject
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
	fixture := newRelayPrivacyFixture(t, now)
	senderChannel, receiverChannel := fixture.channels(t, now)

	scenario.Precondition("start remote transport with published and withdrawn service entries", func(t *testing.T) {
		remote = testkit.StartTransport(t)
		require.NoError(t, publication.PublishPrivateDiscoveryEntries(ctx, []discovery.Entry{
			{
				Record: discovery.Record{
					ID:        "svc.remote.echo",
					Kind:      "service",
					Subject:   "svc.remote.echo",
					Node:      "remote",
					Service:   "echo",
					Endpoints: []string{"tcp://remote"},
					IssuedAt:  now,
				},
				Source: "local",
				SeenAt: now,
			},
			{
				Record: discovery.Record{
					ID:       "svc.remote.echo",
					Kind:     "service",
					Subject:  "svc.remote.echo",
					Node:     "remote",
					Service:  "echo",
					IssuedAt: now.Add(time.Second),
				},
				Source: "local",
				SeenAt: now.Add(time.Second),
			},
		}, senderChannel, remote))
	})

	scenario.Step("start bootstrapped local transport", func(t *testing.T) {
		local = testkit.StartBootstrappedTransport(t, remote)
		testkit.WaitForRelayReadiness(t, local)
	})

	scenario.Assert("store fetch returns the latest withdrawal entry", func(t *testing.T) {
		testkit.WaitForCondition(t, 5*time.Second, "store fetch keeps latest withdrawal entry", func() (bool, string) {
			result, err := discovery.FetchPrivateRecords(ctx, remote.Endpoints(), receiverChannel, local)
			if err != nil {
				return false, err.Error()
			}
			if len(result.Entries) != 1 {
				return false, "unexpected record count"
			}
			if len(result.Entries[0].Record.Endpoints) != 0 {
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
			Record: discovery.Record{ID: "revoked:node", Kind: "node", Subject: "revoked", Node: "revoked", IssuedAt: now},
			Source: "local", SeenAt: now,
		}}, privacy.Sender, remote))
		privacy.RevokeSender(t, now)
	})

	scenario.Step("fetch encrypted history after revocation", func(t *testing.T) {
		local = testkit.StartBootstrappedTransport(t, remote)
		testkit.WaitForRelayReadiness(t, local)
	})

	scenario.Assert("receive-time authorization rejects the retained envelope", func(t *testing.T) {
		testkit.WaitForCondition(t, 5*time.Second, "revoked retained envelope is rejected", func() (bool, string) {
			result, err := discovery.FetchPrivateRecords(ctx, remote.Endpoints(), privacy.Receiver, local)
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
	svc := transport.New(transport.Config{StorePath: storePath})

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
