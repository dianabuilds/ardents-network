package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	model "ardents/internal/content/catalog"

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
	persisted := persistedContent{
		BlobOwnership: persistedBlobOwnership{Version: blobOwnershipVersion},
		Objects: map[string]model.Object{
			"obj-broken": {
				ID:    "obj-broken",
				Type:  "chat.message",
				Owner: "principal.local",
				BlobRefs: []model.Ref{{
					Kind: "blob",
					ID:   "blob-missing",
				}},
			},
		},
		Blobs: map[string]Blob{},
		Manifests: map[string]model.Manifest{
			"manifest-broken": {
				ID:    "manifest-broken",
				Kind:  "blob-set",
				Owner: "principal.local",
				Refs: []model.Ref{{
					Kind: "blob",
					ID:   "blob-missing-2",
				}},
			},
		},
	}
	{
		err := saveContent(contentPath(dir), persisted)
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
