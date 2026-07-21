package waku

import (
	"os"
	"testing"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestTransportKeyStoreEnsurePersistsStablePrivateKey(t *testing.T) {
	path := t.TempDir() + "/waku_node_key.json"
	store := newTransportKeyStore(path)

	first, err := store.Ensure(false)
	require.NoError(t, err)
	second, err := store.Ensure(false)
	require.NoError(t, err)

	require.Equal(t,
		gethcrypto.PubkeyToAddress(first.PublicKey).Hex(),
		gethcrypto.PubkeyToAddress(second.PublicKey).Hex(),
	)
}

func TestTransportKeyStoreRejectsMissingKeyForExistingStore(t *testing.T) {
	path := t.TempDir() + "/waku_node_key.json"
	store := newTransportKeyStore(path)

	_, err := store.Ensure(true)
	require.ErrorContains(t, err, "restore matching backup")
}

func TestTransportKeyStoreRejectsCorruptKey(t *testing.T) {
	path := t.TempDir() + "/waku_node_key.json"
	require.NoError(t, os.WriteFile(path, []byte(`{"private_key":"not-hex"}`), 0o600))

	_, err := newTransportKeyStore(path).Ensure(false)
	require.Error(t, err)
}
