//go:build integration

package discovery_test

import (
	"context"
	"testing"
	"time"

	runtimeinfra "ardents/internal/daemon"
	discoveryapi "ardents/internal/discovery"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestPrivateDiscoveryImportsSignedRecordFromWakuStore(t *testing.T) {
	testkit.BeginScenario(t, testkit.Spec{
		Layer: testkit.LayerIntegration, Domain: "discovery", ScenarioID: "DKI-003",
		Suite: "integration", Tags: []string{"integration", "discovery", "privacy", "store"},
		Speed: "default", Environment: "local",
	})
	now := time.Now().UTC().Truncate(time.Second)
	privacy := testkit.NewDiscoveryPrivacyFixture(t, now)
	remote := testkit.StartNode(t, runtimeinfra.Config{
		Name: "private-discovery-remote", Boot: runtimeinfra.BootConfig{Sources: []string{"local://bootstrap"}},
		Data: runtimeinfra.DataConfig{Dir: t.TempDir()}, Privacy: privacy.Sender,
	})
	remoteRecords, err := remote.ListRecords()
	require.NoError(t, err)
	require.NotEmpty(t, remoteRecords)
	remoteRecord := remoteRecords[0]
	require.NotEmpty(t, remoteRecord.Endpoints)

	localDir := t.TempDir()
	localConfig := runtimeinfra.Config{
		Name: "private-discovery-local", Boot: runtimeinfra.BootConfig{Sources: append([]string(nil), remoteRecord.Endpoints...)},
		Data: runtimeinfra.DataConfig{Dir: localDir}, Privacy: privacy.Receiver,
	}
	local := testkit.StartNode(t, runtimeinfra.Config{
		Name: localConfig.Name, Boot: localConfig.Boot, Data: localConfig.Data, Privacy: localConfig.Privacy,
	})

	testkit.WaitForCondition(t, 5*time.Second, "signed private record imported from Waku Store", func() (bool, string) {
		records, listErr := local.ListRecords()
		if listErr != nil {
			return false, listErr.Error()
		}
		for _, record := range records {
			if record.ID == remoteRecord.ID {
				return true, ""
			}
		}
		return false, "remote record is absent"
	})
	require.NoError(t, local.Stop(context.Background()))

	restarted := testkit.StartNode(t, localConfig)
	restartedRecords, err := restarted.ListRecords()
	require.NoError(t, err)
	require.Contains(t, recordIDs(restartedRecords), remoteRecord.ID)
}

func recordIDs(records []discoveryapi.CatalogRecordSnapshot) []string {
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	return ids
}
