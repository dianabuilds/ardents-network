package daemon

import (
	"ardents/internal/discovery"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImportBootstrapEntriesSkipsLocalPrincipalAndSyncsTrust(t *testing.T) {
	entries := []discovery.Entry{
		{Record: discovery.Record{ID: "local", Node: "node-local"}},
		{Record: discovery.Record{ID: "remote", Node: "node-remote"}},
	}
	var called []string
	synced := false

	hadErrors := ImportBootstrapEntries(
		"node-local",
		entries,
		func(record discovery.Record) (bool, error) {
			called = append(called, record.ID)
			return true, nil
		},
		func(recordID, detail string) {},
		func() { synced = true },
	)

	require.False(t, hadErrors)
	require.Equal(t, []string{"remote"}, called)
	require.True(t, synced)
}

func TestImportBootstrapEntriesDegradesOnImportError(t *testing.T) {
	entries := []discovery.Entry{
		{Record: discovery.Record{ID: "remote", Node: "node-remote"}},
	}
	degraded := ""

	hadErrors := ImportBootstrapEntries(
		"node-local",
		entries,
		func(record discovery.Record) (bool, error) {
			return false, errors.New("bad signature")
		},
		func(recordID, detail string) { degraded = recordID + ":" + detail },
		func() {},
	)

	require.True(t, hadErrors)
	require.Equal(t, "remote:bad signature", degraded)
}
