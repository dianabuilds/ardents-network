package provision

import (
	"bytes"
	"crypto/ed25519"
	"path/filepath"
	"testing"
	"time"

	runtimeconfig "ardents/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRenewApplicationCredentialUpdatesActiveDocument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "operator.json")
	public := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)).Public().(ed25519.PublicKey)
	document := operatorDocument(options{
		nodeName: "node-a", transportPort: 61000,
		runtimeDataDir: dir, runtimeSecretDir: dir,
	}, NodeProvision{
		Subject: "p_subject", Issuer: "p_issuer", IssuerPublic: public,
		DiscoveryRef: "discovery-ref", DataRef: "data-ref",
		ApplicationExpiresAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, writeOperatorDocument(dir, document))

	var output bytes.Buffer
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, renewApplicationCredential([]string{"--config", path}, &output, func() time.Time { return now }))

	renewed, err := runtimeconfig.Load(path)
	require.NoError(t, err)
	require.Equal(t, now.Add(applicationCredentialLifetime).Format(time.RFC3339), renewed.ApplicationInterface.CredentialExpiresAt)
	require.Contains(t, output.String(), "application-credential=renewed")
}
