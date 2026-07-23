package content

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLocalRetentionLifecycle(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	blob, err := svc.StoreBlob(Blob{MediaType: "text/plain"}, []byte("retain me"))
	require.NoErrorf(t, err, "store blob: %v", err)

	until := time.Now().UTC().Add(time.Hour)
	retained, err := svc.RetainBlob(blob.Reference.String(), until)
	require.NoErrorf(t, err, "retain blob: %v", err)
	require.Falsef(t, retained.State !=
		"retained-temporary", "state = %q, want retained-temporary", retained.State)

	pinned, err := svc.PinBlob(blob.Reference.String())
	require.NoErrorf(t, err, "pin blob: %v", err)
	require.Falsef(t, pinned.State !=
		"pinned", "state = %q, want pinned", pinned.State)

	dropped, err := svc.DropBlob(blob.Reference.String())
	require.NoErrorf(t, err, "drop blob: %v", err)
	require.Falsef(t, dropped.State !=
		"deleted", "state = %q, want deleted", dropped.State)
	{

		_, err := svc.GetBlobPayload(blob.Reference.String())
		require.Error(t, err, "expected dropped payload to be unavailable")
	}

}

func TestPruneExpiredRetention(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	blob, err := svc.StoreBlob(Blob{MediaType: "text/plain"}, []byte("expire me"))
	require.NoErrorf(t, err, "store blob: %v", err)
	{

		_, err := svc.RetainBlob(blob.Reference.String(), time.Now().UTC().Add(-time.Minute))
		require.NoErrorf(t, err, "retain blob: %v", err)
	}

	pruned, err := svc.PruneExpired(time.Now().UTC())
	require.NoErrorf(t, err, "prune expired: %v", err)
	require.Falsef(t, len(pruned) !=
		1, "pruned = %d, want 1", len(pruned))
	require.Falsef(t, pruned[0].State !=
		"expired", "state = %q, want expired", pruned[0].State)
	{

		_, err := svc.GetBlobPayload(blob.Reference.String())
		require.Error(t, err, "expected pruned payload to be unavailable")
	}

}

func TestLoadReconcilesExpiredRetentionState(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	blob, err := svc.StoreBlob(Blob{MediaType: "text/plain"}, []byte("expire on load"))
	require.NoErrorf(t, err, "store blob: %v", err)
	{

		_, err := svc.RetainBlob(blob.Reference.String(), time.Now().UTC().Add(-time.Minute))
		require.NoErrorf(t, err, "retain blob: %v", err)
	}

	restored := NewInDir(dir)
	{
		err := restored.Load()
		require.NoErrorf(t, err, "restore load: %v", err)
	}

	item, ok := restored.GetBlob(blob.Reference.String())
	require.True(t, ok, "expected restored blob")
	require.Falsef(t, item.State !=
		"expired", "state = %q, want expired", item.State)
	{

		_, err := restored.GetBlobPayload(blob.Reference.String())
		require.Error(t, err, "expected expired payload to be unavailable after load reconciliation")
	}

	inv := restored.Inventory()
	require.Falsef(t, inv.RetainedTemporary !=
		0 ||
		inv.LocalBlobs !=
			0 || inv.
		AvailableForResend !=
		0, "inventory = %#v, want no retained local truth after reconciliation", inv)
	require.Falsef(t, inv.Expired !=
		1, "expired = %d, want 1", inv.Expired)

}

func TestLoadReconcilesMissingAvailableLocalPayloadState(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	blob, err := svc.StoreBlob(Blob{MediaType: "text/plain"}, []byte("lost file"))
	require.NoErrorf(t, err, "store blob: %v", err)
	{

		err := os.Remove(svc.payloadPath(blob.Reference.String()))
		require.NoErrorf(t, err, "remove payload: %v", err)
	}

	restored := NewInDir(dir)
	{
		err := restored.Load()
		require.NoErrorf(t, err, "restore load: %v", err)
	}

	item, ok := restored.GetBlob(blob.Reference.String())
	require.True(t, ok, "expected restored blob")
	require.Falsef(t, item.State !=
		"deleted", "state = %q, want deleted", item.State)
	{

		_, err := restored.GetBlobPayload(blob.Reference.String())
		require.Error(t, err, "expected deleted payload to be unavailable after load reconciliation")
	}

	inv := restored.Inventory()
	require.Falsef(t, inv.LocalBlobs !=
		0 ||
		inv.AvailableForResend !=
			0, "inventory = %#v, want no local truth after payload loss reconciliation", inv)
	require.Falsef(t, inv.Deleted !=
		1, "deleted = %d, want 1", inv.Deleted)

}

func TestLoadReconcilesMissingPinnedPayloadState(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	blob, err := svc.StoreBlob(Blob{MediaType: "text/plain"}, []byte("pinned file"))
	require.NoErrorf(t, err, "store blob: %v", err)
	{

		_, err := svc.PinBlob(blob.Reference.String())
		require.NoErrorf(t, err, "pin blob: %v", err)
	}
	{

		err := os.Remove(svc.payloadPath(blob.Reference.String()))
		require.NoErrorf(t, err, "remove payload: %v", err)
	}

	restored := NewInDir(dir)
	{
		err := restored.Load()
		require.NoErrorf(t, err, "restore load: %v", err)
	}

	item, ok := restored.GetBlob(blob.Reference.String())
	require.True(t, ok, "expected restored blob")
	require.Falsef(t, item.State !=
		"deleted", "state = %q, want deleted", item.State)

}

func TestRetainRelayBlobRequiresEncryptedPayload(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}
	{

		_, err := svc.RetainRelayBlob(Blob{
			MediaType: "application/octet-stream",
		}, []byte("plaintext"), time.Now().UTC().Add(time.Hour))
		require.Error(t, err, "expected relay retention to reject non-encrypted blob")
	}

	retained, err := svc.RetainRelayBlob(Blob{
		MediaType: "application/octet-stream",
		Encrypted: true,
	}, []byte("ciphertext"), time.Now().UTC().Add(time.Hour))
	require.NoErrorf(t, err, "retain relay blob: %v", err)
	require.Falsef(t, retained.State !=
		"retained-temporary", "state = %q, want retained-temporary", retained.State)
	require.Falsef(t, retained.Retention !=
		"relay-temporary", "retention = %q, want relay-temporary", retained.Retention)

	payload, err := svc.GetBlobPayload(retained.Reference.String())
	require.NoErrorf(t, err, "get payload: %v", err)
	require.Falsef(t, string(payload) != "ciphertext", "payload = %q, want ciphertext", string(payload))

}

func TestRetainRelayBlobRejectsMismatchedContentIdentity(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}
	{

		_, err := svc.RetainRelayBlob(Blob{
			Reference: testContentReference(t, "different"),
			MediaType: "application/octet-stream",
			Encrypted: true,
		}, []byte("ciphertext"), time.Now().UTC().Add(time.Hour))
		require.Error(t, err, "expected relay retention to reject mismatched blob id")
	}
	{

		blobs := svc.ListBlobs()
		require.Falsef(t, len(blobs) !=
			0, "blobs = %d, want 0 after rejected relay retention", len(blobs))
	}

}

func TestStoreEncryptedBlobKeepsCiphertextAndRequiresKey(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	key := []byte("0123456789abcdef0123456789abcdef")
	blob, err := svc.StoreEncryptedBlob(Blob{
		MediaType: "application/octet-stream",
	}, []byte("secret payload"), key, "")
	require.NoErrorf(t, err, "store encrypted blob: %v", err)
	require.True(t, blob.Encrypted, "expected encrypted blob")
	require.Falsef(t, blob.Cipher !=
		blobCipherAES256GCM, "cipher = %q, want %q", blob.Cipher, blobCipherAES256GCM)

	raw, err := svc.GetBlobPayload(blob.Reference.String())
	require.NoErrorf(t, err, "get raw payload: %v", err)
	require.False(t, string(raw) ==
		"secret payload", "expected ciphertext on disk, got plaintext")
	{

		_, err := svc.DecryptBlobPayload(blob.Reference.String(), []byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"))
		require.Error(t, err, "expected decryption failure with wrong key")
	}

	plaintext, err := svc.DecryptBlobPayload(blob.Reference.String(), key)
	require.NoErrorf(t, err, "decrypt blob: %v", err)
	require.Falsef(t, string(plaintext) != "secret payload", "plaintext = %q, want secret payload", string(plaintext))

}

func TestStoreEncryptedBlobDecryptsWithExplicitOpaqueKeyID(t *testing.T) {
	svc := NewInDir(t.TempDir())
	require.NoError(t, svc.Load())
	key := []byte("0123456789abcdef0123456789abcdef")

	blob, err := svc.StoreEncryptedBlob(Blob{MediaType: "application/octet-stream"}, []byte("secret payload"), key, "key-1")
	require.NoError(t, err)
	require.Equal(t, "key-1", blob.KeyID)

	plaintext, err := svc.DecryptBlobPayload(blob.Reference.String(), key)
	require.NoError(t, err)
	require.Equal(t, []byte("secret payload"), plaintext)
	_, err = svc.DecryptBlobPayload(blob.Reference.String(), []byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"))
	require.Error(t, err)
}
