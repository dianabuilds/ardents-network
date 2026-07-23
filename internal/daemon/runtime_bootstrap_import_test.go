package daemon

import (
	"ardents/internal/discovery"
	discoveryrecord "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImportBootstrapEntriesSkipsLocalPrincipalAndSyncsTrust(t *testing.T) {
	local := bootstrapNodeRecord(t, 1)
	remote := bootstrapNodeRecord(t, 2)
	entries := []discovery.Entry{
		{Record: local},
		{Record: remote},
	}
	var called []string
	synced := false

	hadErrors := ImportBootstrapEntries(
		local.NodeID(),
		entries,
		func(record discovery.Record) (bool, error) {
			called = append(called, record.RecordID())
			return true, nil
		},
		func(recordID, detail string) {},
		func() { synced = true },
	)

	require.False(t, hadErrors)
	require.Equal(t, []string{remote.RecordID()}, called)
	require.True(t, synced)
}

func TestImportBootstrapEntriesDegradesOnImportError(t *testing.T) {
	remote := bootstrapNodeRecord(t, 2)
	entries := []discovery.Entry{
		{Record: remote},
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
	require.Equal(t, remote.RecordID()+":bad signature", degraded)
}

func bootstrapNodeRecord(t *testing.T, seedByte byte) discovery.Record {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = seedByte
	}
	key := ed25519.NewKeyFromSeed(seed)
	principal, err := identityprincipal.FromEd25519PublicKey(key.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	return discovery.Record{Version: discoveryrecord.Version, Node: &discoveryrecord.NodeFacts{Principal: principal, PublicKey: base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey))}}
}
