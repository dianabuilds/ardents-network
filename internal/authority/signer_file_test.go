package authority

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"testing"

	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestFileSignerLoadsProtectedExternalKeyAndProvesContinuity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "realm-authority-signer.json")
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x91}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(private.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]any{
		"version": ContractVersion, "principal": principal.String(),
		"private_key": base64.RawURLEncoding.EncodeToString(private),
	})
	require.NoError(t, err)
	require.NoError(t, storage.AtomicWritePrivateFile(path, raw))

	signer, err := NewFileSigner(path)
	require.NoError(t, err)
	gotPrincipal, err := signer.Principal(context.Background())
	require.NoError(t, err)
	require.Equal(t, principal.String(), gotPrincipal)
	public, err := signer.PublicKey(context.Background())
	require.NoError(t, err)
	require.True(t, private.Public().(ed25519.PublicKey).Equal(public))
	signature, err := signer.Sign(context.Background(), []byte("checkpoint digest"))
	require.NoError(t, err)
	require.True(t, ed25519.Verify(public, []byte("checkpoint digest"), signature))
}

func TestFileSignerFailsClosedOnMissingMalformedOrMismatchedMaterial(t *testing.T) {
	dir := t.TempDir()
	missing, err := NewFileSigner(filepath.Join(dir, "missing.json"))
	require.NoError(t, err)
	_, err = missing.Principal(context.Background())
	require.ErrorIs(t, err, ErrUnavailable)

	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x92}, ed25519.SeedSize))
	path := filepath.Join(dir, "signer.json")
	for name, record := range map[string]map[string]any{
		"unsupported": {"version": 2, "principal": "p1_invalid", "private_key": base64.RawURLEncoding.EncodeToString(private)},
		"unknown":     {"version": 1, "principal": "p1_invalid", "private_key": base64.RawURLEncoding.EncodeToString(private), "unknown": true},
		"mismatch":    {"version": 1, "principal": newTestSigner(t, 0x93).principal, "private_key": base64.RawURLEncoding.EncodeToString(private)},
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(record)
			require.NoError(t, err)
			require.NoError(t, storage.AtomicWritePrivateFile(path, raw))
			signer, err := NewFileSigner(path)
			require.NoError(t, err)
			_, err = signer.Principal(context.Background())
			require.Error(t, err)
		})
	}
}
