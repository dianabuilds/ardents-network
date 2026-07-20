package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	statepkg "ardents/internal/data/state"

	"github.com/stretchr/testify/require"
)

func TestPublishObjectAndPersist(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	blob, err := svc.StoreBlob(Blob{MediaType: "text/plain"}, []byte("hello"))
	require.NoErrorf(t, err, "store blob: %v", err)

	object, err := svc.PublishObject(Object{
		Type:  "chat.message",
		Owner: "principal.local",
		Body: map[string]any{
			"text": "hello",
		},
		BlobRefs: []Ref{{
			Kind: "blob",
			ID:   blob.ID,
		}},
	})
	require.NoErrorf(t, err, "publish object: %v", err)
	require.False(t, object.ID ==
		"", "expected generated object id")

	restored := NewInDir(dir)
	{
		err := restored.Load()
		require.NoErrorf(t, err, "restore load: %v", err)
	}

	items := restored.ListObjects()
	require.Falsef(t, len(items) !=
		1, "objects = %d, want 1", len(items))
	require.Falsef(t, items[0].Body["text"] !=
		"hello", "text = %v, want hello", items[0].Body["text"])
	require.Falsef(t, len(items[0].
		BlobRefs,
	) !=
		1 ||
		items[0].BlobRefs[0].ID !=
			blob.ID, "blob refs = %v, want %q", items[0].BlobRefs, blob.ID)

}

func TestPublishObjectRejectsMissingBlobRef(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}
	{

		_, err := svc.PublishObject(Object{
			Type:  "chat.message",
			Owner: "principal.local",
			BlobRefs: []Ref{{
				Kind: "blob",
				ID:   "missing",
			}},
		})
		require.Error(t, err, "expected missing blob ref error")
	}

}

func TestPublishBlobAndPersist(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	blob, err := svc.StoreBlob(Blob{
		MediaType: "application/octet-stream",
		Encrypted: true,
	}, []byte("hello"))
	require.NoErrorf(t, err, "publish blob: %v", err)
	require.False(t, blob.ID == "", "expected generated blob id")
	require.False(t, blob.CID == "", "expected generated blob cid")
	require.Falsef(t, blob.State !=
		"available-local", "state = %q, want available-local", blob.State)
	require.True(t, blob.Encrypted, "expected encrypted blob metadata")

	restored := NewInDir(dir)
	{
		err := restored.Load()
		require.NoErrorf(t, err, "restore load: %v", err)
	}

	items := restored.ListBlobs()
	require.Falsef(t, len(items) !=
		1, "blobs = %d, want 1", len(items))
	require.Falsef(t, items[0].MediaType !=
		"application/octet-stream", "media type = %q, want application/octet-stream", items[0].MediaType)
	require.False(t, items[0].CID ==
		"", "expected restored blob cid")

	payload, err := restored.GetBlobPayload(blob.ID)
	require.NoErrorf(t, err, "get payload: %v", err)
	require.Falsef(t, string(payload) != "hello", "payload = %q, want hello", string(payload))

}

func TestStoreBlobRejectsMismatchedContentIdentity(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}
	{

		_, err := svc.StoreBlob(Blob{
			ID:        "blob.logical",
			MediaType: "application/octet-stream",
		}, []byte("hello"))
		require.Error(t, err, "expected blob id mismatch")
	}
	{

		blobs := svc.ListBlobs()
		require.Falsef(t, len(blobs) !=
			0, "blobs = %d, want 0 after rejected store", len(blobs))
	}

}

func TestPublishAnnouncedBlobWithoutLocalPayload(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	blob, err := svc.PublishBlob(Blob{
		ID:        "blob.remote.demo",
		MediaType: "application/octet-stream",
		Hash:      "sha256:demo",
		State:     "announced",
		Retention: "temporary",
	})
	require.NoErrorf(t, err, "publish announced blob: %v", err)
	require.Falsef(t, blob.State !=
		"announced", "state = %q, want announced", blob.State)
	{

		_, err := svc.GetBlobPayload(blob.ID)
		require.Error(t, err, "expected missing local payload for announced blob")
	}

}

func TestPublishAnnouncedBlobRejectsMismatchedCIDAndID(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}
	{

		_, err := svc.PublishBlob(Blob{
			ID:        "blob.remote.demo",
			CID:       "bafkreigh2akiscaildc2",
			MediaType: "application/octet-stream",
			State:     "announced",
		})
		require.Error(t, err, "expected publish rejection for mismatched blob id and cid")
	}

}

func TestPublishBlobRejectsMetadataOnlyLocalAvailabilityState(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	for _, state := range []string{"available-local", "retained-temporary", "pinned"} {
		{
			_, err := svc.PublishBlob(Blob{
				ID:        "blob-" + state,
				MediaType: "application/octet-stream",
				State:     state,
			})
			require.Errorf(t, err, "expected publish rejection for state %q", state)
		}

	}
	{

		blobs := svc.ListBlobs()
		require.Falsef(t, len(blobs) !=
			0, "blobs = %d, want 0 after rejected publish", len(blobs))
	}

	inv := svc.Inventory()
	require.Falsef(t, inv.Blobs !=
		0 ||
		inv.LocalBlobs !=
			0 || inv.AvailableForResend !=
		0, "inventory = %#v, want zero local truth", inv)

}

func TestAnnounceRemoteBlobRewritesLocalOnlyStatesToAvailableRemote(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	for _, state := range []string{"available-local", "retained-temporary", "pinned"} {
		blob, err := svc.AnnounceRemoteBlob(Blob{
			ID:        "blob-" + state,
			MediaType: "application/octet-stream",
			State:     state,
		})
		require.NoErrorf(t, err, "announce remote blob for state %q: %v", state, err)
		require.Falsef(t, blob.State !=
			"available-remote", "state = %q, want available-remote for input %q", blob.State, state)

	}

	items := svc.ListBlobs()
	require.Falsef(t, len(items) !=
		3, "blobs = %d, want 3", len(items))

	for _, item := range items {
		require.Falsef(t, item.State !=
			"available-remote", "stored state = %q, want available-remote", item.State)

	}
}

func TestAnnounceRemoteBlobDefaultsToAvailableRemote(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	blob, err := svc.AnnounceRemoteBlob(Blob{
		ID:        "blob-remote",
		MediaType: "application/octet-stream",
	})
	require.NoErrorf(t, err, "announce remote blob: %v", err)
	require.Falsef(t, blob.State !=
		"available-remote", "state = %q, want available-remote", blob.State)

	item, ok := svc.GetBlob(blob.ID)
	require.True(t, ok, "expected announced remote blob")
	require.Falsef(t, item.State !=
		"available-remote", "stored state = %q, want available-remote", item.State)

}

func TestPublishManifestAndPersist(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	blob, err := svc.StoreBlob(Blob{
		MediaType: "text/plain",
		Encrypted: true,
	}, []byte("hello manifest"))
	require.NoErrorf(t, err, "store blob: %v", err)

	manifest, err := svc.PublishManifest(Manifest{
		Kind:      "message-attachment",
		Owner:     "principal.local",
		Access:    "participants",
		Retention: "temporary",
		Encrypted: true,
		Refs: []Ref{{
			Kind: "blob",
			ID:   blob.ID,
		}},
		Metadata: map[string]any{
			"chat": "room.demo",
		},
	})
	require.NoErrorf(t, err, "publish manifest: %v", err)
	require.False(t, manifest.ID ==
		"", "expected generated manifest id")

	restored := NewInDir(dir)
	{
		err := restored.Load()
		require.NoErrorf(t, err, "restore load: %v", err)
	}

	items := restored.ListManifests()
	require.Falsef(t, len(items) !=
		1, "manifests = %d, want 1", len(items))
	require.Falsef(t, items[0].Refs[0].ID !=
		blob.ID, "ref id = %q, want %q", items[0].Refs[0].ID, blob.ID)
	require.Falsef(t, items[0].Metadata["chat"] !=
		"room.demo", "chat = %v, want room.demo", items[0].Metadata["chat"])

}

func TestPublishManifestRejectsMissingBlobRef(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}
	{

		_, err := svc.PublishManifest(Manifest{
			Owner: "principal.local",
			Refs: []Ref{{
				Kind: "blob",
				ID:   "missing",
			}},
		})
		require.Error(t, err, "expected missing blob ref error")
	}

}

func TestBlobPartStateDegradesWhenLocalPayloadIsMissing(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	blob, err := svc.StoreBlob(Blob{MediaType: "text/plain"}, []byte("missing payload"))
	require.NoErrorf(t, err, "store blob: %v", err)

	payloadPath := filepath.Join(dir, "blobs", strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(blob.ID)+".blob")
	{
		err := os.Remove(payloadPath)
		require.NoErrorf(t, err, "remove payload: %v", err)
	}

	state, reason := svc.BlobPartState()
	require.Falsef(t, state != "degraded", "state = %q, want degraded", state)
	require.Falsef(t, !strings.Contains(reason,
		blob.
			ID) || !strings.
		Contains(reason,
			"without local payload",
		), "reason = %q, want blob-specific missing-payload explanation", reason)

}

func TestObjectPartStateDegradesOnBrokenBlobRefsAfterLoad(t *testing.T) {
	dir := t.TempDir()
	persisted := statepkg.Snapshot{
		Objects: map[string]Object{
			"obj-broken": {
				ID:    "obj-broken",
				Type:  "chat.message",
				Owner: "principal.local",
				BlobRefs: []Ref{{
					Kind: "blob",
					ID:   "blob-missing",
				}},
			},
		},
		Blobs: map[string]Blob{},
		Manifests: map[string]Manifest{
			"manifest-broken": {
				ID:    "manifest-broken",
				Kind:  "blob-set",
				Owner: "principal.local",
				Refs: []Ref{{
					Kind: "blob",
					ID:   "blob-missing-2",
				}},
			},
		},
	}
	{
		err := statepkg.SaveSnapshot(statepkg.PathInDir(dir), persisted)
		require.NoErrorf(t, err, "save snapshot: %v", err)
	}

	svc := NewInDir(dir)
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	state, reason := svc.ObjectPartState()
	require.Falsef(t, state != "degraded", "state = %q, want degraded", state)
	require.Falsef(t, !strings.Contains(reason,
		"missing blob",
	) || !strings.Contains(reason,
		"broken",
	), "reason = %q, want broken-reference explanation", reason)
	require.Falsef(t, svc.State() !=
		"ready", "service state = %q, want ready", svc.State())

}

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
	retained, err := svc.RetainBlob(blob.ID, until)
	require.NoErrorf(t, err, "retain blob: %v", err)
	require.Falsef(t, retained.State !=
		"retained-temporary", "state = %q, want retained-temporary", retained.State)

	pinned, err := svc.PinBlob(blob.ID)
	require.NoErrorf(t, err, "pin blob: %v", err)
	require.Falsef(t, pinned.State !=
		"pinned", "state = %q, want pinned", pinned.State)

	dropped, err := svc.DropBlob(blob.ID)
	require.NoErrorf(t, err, "drop blob: %v", err)
	require.Falsef(t, dropped.State !=
		"deleted", "state = %q, want deleted", dropped.State)
	{

		_, err := svc.GetBlobPayload(blob.ID)
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

		_, err := svc.RetainBlob(blob.ID, time.Now().UTC().Add(-time.Minute))
		require.NoErrorf(t, err, "retain blob: %v", err)
	}

	pruned, err := svc.PruneExpired(time.Now().UTC())
	require.NoErrorf(t, err, "prune expired: %v", err)
	require.Falsef(t, len(pruned) !=
		1, "pruned = %d, want 1", len(pruned))
	require.Falsef(t, pruned[0].State !=
		"expired", "state = %q, want expired", pruned[0].State)
	{

		_, err := svc.GetBlobPayload(blob.ID)
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

		_, err := svc.RetainBlob(blob.ID, time.Now().UTC().Add(-time.Minute))
		require.NoErrorf(t, err, "retain blob: %v", err)
	}

	restored := NewInDir(dir)
	{
		err := restored.Load()
		require.NoErrorf(t, err, "restore load: %v", err)
	}

	item, ok := restored.GetBlob(blob.ID)
	require.True(t, ok, "expected restored blob")
	require.Falsef(t, item.State !=
		"expired", "state = %q, want expired", item.State)
	{

		_, err := restored.GetBlobPayload(blob.ID)
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

		err := os.Remove(svc.payloadPath(blob.ID))
		require.NoErrorf(t, err, "remove payload: %v", err)
	}

	restored := NewInDir(dir)
	{
		err := restored.Load()
		require.NoErrorf(t, err, "restore load: %v", err)
	}

	item, ok := restored.GetBlob(blob.ID)
	require.True(t, ok, "expected restored blob")
	require.Falsef(t, item.State !=
		"deleted", "state = %q, want deleted", item.State)
	{

		_, err := restored.GetBlobPayload(blob.ID)
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

		_, err := svc.PinBlob(blob.ID)
		require.NoErrorf(t, err, "pin blob: %v", err)
	}
	{

		err := os.Remove(svc.payloadPath(blob.ID))
		require.NoErrorf(t, err, "remove payload: %v", err)
	}

	restored := NewInDir(dir)
	{
		err := restored.Load()
		require.NoErrorf(t, err, "restore load: %v", err)
	}

	item, ok := restored.GetBlob(blob.ID)
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

	payload, err := svc.GetBlobPayload(retained.ID)
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
			ID:        "blob.logical",
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

	raw, err := svc.GetBlobPayload(blob.ID)
	require.NoErrorf(t, err, "get raw payload: %v", err)
	require.False(t, string(raw) ==
		"secret payload", "expected ciphertext on disk, got plaintext")
	{

		_, err := svc.DecryptBlobPayload(blob.ID, []byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"))
		require.Error(t, err, "expected decryption failure with wrong key")
	}

	plaintext, err := svc.DecryptBlobPayload(blob.ID, key)
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

	plaintext, err := svc.DecryptBlobPayload(blob.ID, key)
	require.NoError(t, err)
	require.Equal(t, []byte("secret payload"), plaintext)
	_, err = svc.DecryptBlobPayload(blob.ID, []byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"))
	require.Error(t, err)
}

func TestFetchRemoteBlobToLocalCopy(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	source := NewInDir(sourceDir)
	{
		err := source.Load()
		require.NoErrorf(t, err, "source load: %v", err)
	}

	published, err := source.StoreBlob(Blob{
		MediaType: "application/octet-stream",
	}, []byte("from source"))
	require.NoErrorf(t, err, "source store blob: %v", err)

	target := NewInDir(targetDir)
	{
		err := target.Load()
		require.NoErrorf(t, err, "target load: %v", err)
	}
	{

		_, err := target.AnnounceRemoteBlob(Blob{
			ID:        published.ID,
			CID:       published.CID,
			MediaType: published.MediaType,
			Hash:      published.Hash,
			Encrypted: published.Encrypted,
		})
		require.NoErrorf(t, err, "announce remote blob: %v", err)
	}

	fetched, err := target.FetchBlob(published.ID, source)
	require.NoErrorf(t, err, "fetch blob: %v", err)
	require.Falsef(t, fetched.State !=
		"available-local", "state = %q, want available-local", fetched.State)

	payload, err := target.GetBlobPayload(published.ID)
	require.NoErrorf(t, err, "target payload: %v", err)
	require.Falsef(t, string(payload) != "from source", "payload = %q, want from source", string(payload))

}

func TestFetchEncryptedRemoteBlobPreservesCiphertext(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	key := []byte("0123456789abcdef0123456789abcdef")

	source := NewInDir(sourceDir)
	{
		err := source.Load()
		require.NoErrorf(t, err, "source load: %v", err)
	}

	published, err := source.StoreEncryptedBlob(Blob{
		MediaType: "application/octet-stream",
	}, []byte("encrypted source"), key, "")
	require.NoErrorf(t, err, "source store encrypted blob: %v", err)

	target := NewInDir(targetDir)
	{
		err := target.Load()
		require.NoErrorf(t, err, "target load: %v", err)
	}
	{

		_, err := target.AnnounceRemoteBlob(published)
		require.NoErrorf(t, err, "announce remote blob: %v", err)
	}

	fetched, err := target.FetchBlob(published.ID, source)
	require.NoErrorf(t, err, "fetch blob: %v", err)
	require.True(t, fetched.Encrypted, "expected encrypted fetched blob")

	raw, err := target.GetBlobPayload(published.ID)
	require.NoErrorf(t, err, "target raw payload: %v", err)
	require.False(t, string(raw) ==
		"encrypted source", "expected fetched payload to remain ciphertext")

	plaintext, err := target.DecryptBlobPayload(published.ID, key)
	require.NoErrorf(t, err, "decrypt fetched blob: %v", err)
	require.Falsef(t, string(plaintext) != "encrypted source", "plaintext = %q, want encrypted source", string(plaintext))

}

func TestPeerAssistedReServingAfterFetch(t *testing.T) {
	source := NewInDir(t.TempDir())
	{
		err := source.Load()
		require.NoErrorf(t, err, "source load: %v", err)
	}

	blob, err := source.StoreBlob(Blob{
		MediaType: "application/octet-stream",
	}, []byte("shared by peers"))
	require.NoErrorf(t, err, "source store blob: %v", err)

	peerOne := NewInDir(t.TempDir())
	{
		err := peerOne.Load()
		require.NoErrorf(t, err, "peer one load: %v", err)
	}
	{

		_, err := peerOne.AnnounceRemoteBlob(blob)
		require.NoErrorf(t, err, "peer one announce: %v", err)
	}
	{

		_, err := peerOne.FetchBlob(blob.ID, source)
		require.NoErrorf(t, err, "peer one fetch: %v", err)
	}

	peerTwo := NewInDir(t.TempDir())
	{
		err := peerTwo.Load()
		require.NoErrorf(t, err, "peer two load: %v", err)
	}
	{

		_, err := peerTwo.AnnounceRemoteBlob(blob)
		require.NoErrorf(t, err, "peer two announce: %v", err)
	}
	{

		_, err := peerTwo.FetchBlob(blob.ID, peerOne)
		require.NoErrorf(t, err, "peer two fetch from peer one: %v", err)
	}

	payload, err := peerTwo.GetBlobPayload(blob.ID)
	require.NoErrorf(t, err, "peer two payload: %v", err)
	require.Falsef(t, string(payload) != "shared by peers", "payload = %q, want shared by peers", string(payload))

}

func TestRetentionUsesConfiguredDefaultTTLs(t *testing.T) {
	svc := NewInDirWithConfig(t.TempDir(), Config{
		DefaultLocalRetentionTTL: time.Hour,
		DefaultRelayRetentionTTL: 2 * time.Hour,
	})
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	local, err := svc.StoreBlob(Blob{MediaType: "text/plain"}, []byte("local default ttl"))
	require.NoErrorf(t, err, "store local blob: %v", err)

	retained, err := svc.RetainBlob(local.ID, time.Time{})
	require.NoErrorf(t, err, "retain local blob: %v", err)
	require.False(t, retained.ExpiresAt.
		IsZero(), "expected default local retention expiry")

	relay, err := svc.RetainRelayBlob(Blob{
		MediaType: "application/octet-stream",
		Encrypted: true,
	}, []byte("ciphertext"), time.Time{})
	require.NoErrorf(t, err, "retain relay blob: %v", err)
	require.False(t, relay.ExpiresAt.
		IsZero(), "expected default relay retention expiry")

}

func TestRelayRetentionHonorsConfiguredByteLimit(t *testing.T) {
	svc := NewInDirWithConfig(t.TempDir(), Config{
		DefaultRelayRetentionTTL: time.Hour,
		MaxRelayRetentionBytes:   10,
	})
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}
	{

		_, err := svc.RetainRelayBlob(Blob{
			MediaType: "application/octet-stream",
			Encrypted: true,
		}, []byte("12345"), time.Time{})
		require.NoErrorf(t, err, "retain first relay blob: %v", err)
	}
	{

		_, err := svc.RetainRelayBlob(Blob{
			MediaType: "application/octet-stream",
			Encrypted: true,
		}, []byte("123456"), time.Time{})
		require.Error(t, err, "expected relay byte limit error")
	}

}

func TestRelayRetentionRefreshDoesNotDoubleCountSameBlob(t *testing.T) {
	svc := NewInDirWithConfig(t.TempDir(), Config{
		DefaultRelayRetentionTTL: time.Hour,
		MaxRelayRetentionBytes:   10,
	})
	{
		err := svc.Load()
		require.NoErrorf(t, err, "load: %v", err)
	}

	first, err := svc.RetainRelayBlob(Blob{
		MediaType: "application/octet-stream",
		Encrypted: true,
	}, []byte("12345"), time.Now().UTC().Add(time.Hour))
	require.NoErrorf(t, err, "retain first relay blob: %v", err)

	refreshed, err := svc.RetainRelayBlob(Blob{
		ID:        first.ID,
		CID:       first.CID,
		Hash:      first.Hash,
		MediaType: first.MediaType,
		Encrypted: true,
	}, []byte("12345"), time.Now().UTC().Add(2*time.Hour))
	require.NoErrorf(t, err, "refresh relay blob: %v", err)
	require.Truef(t, refreshed.ExpiresAt.
		After(first.
			ExpiresAt), "expires_at = %v, want after %v", refreshed.ExpiresAt, first.ExpiresAt)
	{

		total := svc.relayBytesLocked()
		require.Falsef(t, total != 5, "relay bytes = %d, want 5", total)
	}

}

func TestBlobSourceTruthPersistsAndIncludesLocalSource(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	svc.SetLocalNodeID("p_local")
	require.NoError(t, svc.Load())

	blob, err := svc.StoreBlob(Blob{MediaType: "text/plain"}, []byte("source-truth"))
	require.NoError(t, err)

	observed, err := svc.ObserveBlobSource(blob.ID, BlobSourceRecord{
		NodeID:    "p_remote",
		Trust:     SourceTrust{State: "ready", Outcome: "usable", Valid: true, Trusted: true, Usable: true},
		Usable:    true,
		Transport: "remote",
		Reason:    "trusted remote source answered blob fetch",
	})
	require.NoError(t, err)
	require.Equal(t, blob.ID, observed.BlobID)

	sources := svc.ListBlobSources(blob.ID)
	require.Len(t, sources, 2)

	restored := NewInDir(dir)
	restored.SetLocalNodeID("p_local")
	require.NoError(t, restored.Load())

	persisted := restored.ListBlobSources(blob.ID)
	require.Len(t, persisted, 2)
	require.Equal(t, "p_local", persisted[0].NodeID)
	require.Equal(t, "local", persisted[0].Transport)
	require.Equal(t, "p_remote", persisted[1].NodeID)
	require.Equal(t, "remote", persisted[1].Transport)
	require.False(t, persisted[1].LastSeenAt.IsZero())
}

func TestListBlobSourcesMarksStaleRemoteSourcesUnusable(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	require.NoError(t, svc.Load())

	blob, err := svc.AnnounceRemoteBlob(Blob{
		ID:        "blob-stale-source",
		CID:       "blob-stale-source",
		MediaType: "application/octet-stream",
		State:     "available-remote",
	})
	require.NoError(t, err)

	_, err = svc.ObserveBlobSource(blob.ID, BlobSourceRecord{
		NodeID:     "p_remote",
		Trust:      SourceTrust{State: "ready", Outcome: "usable", Valid: true, Trusted: true, Usable: true},
		Usable:     true,
		Transport:  "remote",
		LastSeenAt: time.Now().UTC().Add(-24 * time.Hour),
		Reason:     "trusted remote source answered blob fetch",
	})
	require.NoError(t, err)

	sources := svc.ListBlobSources(blob.ID)
	require.Len(t, sources, 1)
	require.Equal(t, "p_remote", sources[0].NodeID)
	require.False(t, sources[0].Usable)
	require.False(t, sources[0].Trust.Usable)
	require.Contains(t, sources[0].Reason, "stale")
	require.Contains(t, sources[0].Reason, sources[0].LastSeenAt.UTC().Format(time.RFC3339))
}

func TestTransferTruthPersistsAcrossLoad(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	require.NoError(t, svc.Load())

	started, err := svc.StartTransfer(TransferRecord{
		ID:         "xfer-1",
		Kind:       "blob_fetch",
		ResourceID: "blob-1",
		Direction:  "inbound",
		State:      "pending",
		Reason:     "waiting for remote blob response",
	})
	require.NoError(t, err)
	require.Equal(t, "xfer-1", started.ID)

	completed, err := svc.CompleteTransfer(started.ID, "p_remote", 42, "blob fetched from trusted peer")
	require.NoError(t, err)
	require.Equal(t, "completed", completed.State)
	require.EqualValues(t, 42, completed.TotalBytes)
	require.NotNil(t, completed.FinishedAt)

	restored := NewInDir(dir)
	require.NoError(t, restored.Load())

	item, ok := restored.GetTransfer(started.ID)
	require.True(t, ok)
	require.Equal(t, "completed", item.State)
	require.Equal(t, "p_remote", item.Peer)
	require.EqualValues(t, 42, item.ProgressBytes)
	require.Len(t, restored.ListTransfers(), 1)
}

func TestServiceCapabilityFacadesExposeCohesiveDomainFlow(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	require.NoError(t, svc.Load())

	blob, err := svc.StoreBlob(Blob{MediaType: "text/plain"}, []byte("facade payload"))
	require.NoError(t, err)

	object, err := svc.PublishObject(Object{
		Type:  "chat.message",
		Owner: "principal.local",
		BlobRefs: []Ref{{
			Kind: "blob",
			ID:   blob.ID,
		}},
	})
	require.NoError(t, err)

	manifest, err := svc.PublishManifest(Manifest{
		Owner: "principal.local",
		Refs: []Ref{{
			Kind: "blob",
			ID:   blob.ID,
		}},
	})
	require.NoError(t, err)

	retained, err := svc.RetainBlob(blob.ID, time.Now().UTC().Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, "retained-temporary", retained.State)

	source, err := svc.ObserveBlobSource(blob.ID, BlobSourceRecord{
		NodeID:    "p_remote",
		Trust:     SourceTrust{State: "ready", Outcome: "usable", Valid: true, Trusted: true, Usable: true},
		Usable:    true,
		Transport: "remote",
		Reason:    "trusted remote source answered blob fetch",
	})
	require.NoError(t, err)
	require.Equal(t, blob.ID, source.BlobID)

	transfer, err := svc.StartTransfer(TransferRecord{
		ID:         "xfer-facade",
		Kind:       "blob_fetch",
		ResourceID: blob.ID,
		Direction:  "inbound",
	})
	require.NoError(t, err)
	require.Equal(t, "xfer-facade", transfer.ID)

	inventory := svc.Inventory()
	require.Equal(t, 1, inventory.Objects)
	require.Equal(t, 1, inventory.Manifests)
	require.Equal(t, 1, inventory.Blobs)

	storedObject, ok := svc.GetObject(object.ID)
	require.True(t, ok)
	require.Equal(t, object.ID, storedObject.ID)

	storedManifest, ok := svc.GetManifest(manifest.ID)
	require.True(t, ok)
	require.Equal(t, manifest.ID, storedManifest.ID)
	require.Len(t, svc.ListBlobSources(blob.ID), 1)
	require.Len(t, svc.ListTransfers(), 1)
}
