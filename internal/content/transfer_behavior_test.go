package content

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
			Reference: published.Reference,
			MediaType: published.MediaType,
			Hash:      published.Hash,
			Encrypted: published.Encrypted,
		})
		require.NoErrorf(t, err, "announce remote blob: %v", err)
	}

	fetched, err := target.FetchBlob(published.Reference.String(), source)
	require.NoErrorf(t, err, "fetch blob: %v", err)
	require.Falsef(t, fetched.State !=
		"available-local", "state = %q, want available-local", fetched.State)

	payload, err := target.GetBlobPayload(published.Reference.String())
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

	fetched, err := target.FetchBlob(published.Reference.String(), source)
	require.NoErrorf(t, err, "fetch blob: %v", err)
	require.True(t, fetched.Encrypted, "expected encrypted fetched blob")

	raw, err := target.GetBlobPayload(published.Reference.String())
	require.NoErrorf(t, err, "target raw payload: %v", err)
	require.False(t, string(raw) ==
		"encrypted source", "expected fetched payload to remain ciphertext")

	plaintext, err := target.DecryptBlobPayload(published.Reference.String(), key)
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

		_, err := peerOne.FetchBlob(blob.Reference.String(), source)
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

		_, err := peerTwo.FetchBlob(blob.Reference.String(), peerOne)
		require.NoErrorf(t, err, "peer two fetch from peer one: %v", err)
	}

	payload, err := peerTwo.GetBlobPayload(blob.Reference.String())
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

	retained, err := svc.RetainBlob(local.Reference.String(), time.Time{})
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
		Reference: first.Reference,
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

	observed, err := svc.ObserveBlobSource(blob.Reference.String(), BlobSourceRecord{
		NodeID:    "p_remote",
		Trust:     SourceTrust{State: "ready", Outcome: "usable", Valid: true, Trusted: true, Usable: true},
		Usable:    true,
		Transport: "remote",
		Reason:    "trusted remote source answered blob fetch",
	})
	require.NoError(t, err)
	require.True(t, blob.Reference.Equal(observed.ContentReference))

	sources := svc.ListBlobSources(blob.Reference.String())
	require.Len(t, sources, 2)

	restored := NewInDir(dir)
	restored.SetLocalNodeID("p_local")
	require.NoError(t, restored.Load())

	persisted := restored.ListBlobSources(blob.Reference.String())
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
		Reference: testContentReference(t, "blob-stale-source"),
		MediaType: "application/octet-stream",
		State:     "available-remote",
	})
	require.NoError(t, err)

	_, err = svc.ObserveBlobSource(blob.Reference.String(), BlobSourceRecord{
		NodeID:     "p_remote",
		Trust:      SourceTrust{State: "ready", Outcome: "usable", Valid: true, Trusted: true, Usable: true},
		Usable:     true,
		Transport:  "remote",
		LastSeenAt: time.Now().UTC().Add(-24 * time.Hour),
		Reason:     "trusted remote source answered blob fetch",
	})
	require.NoError(t, err)

	sources := svc.ListBlobSources(blob.Reference.String())
	require.Len(t, sources, 1)
	require.Equal(t, "p_remote", sources[0].NodeID)
	require.False(t, sources[0].Usable)
	require.False(t, sources[0].Trust.Usable)
	require.Contains(t, sources[0].Reason, "stale")
	require.Contains(t, sources[0].Reason, sources[0].LastSeenAt.UTC().Format(time.RFC3339))
}

func TestContentServiceExposesCohesiveLocalDomainFlow(t *testing.T) {
	dir := t.TempDir()

	svc := NewInDir(dir)
	require.NoError(t, svc.Load())

	blob, err := svc.StoreBlob(Blob{MediaType: "text/plain"}, []byte("facade payload"))
	require.NoError(t, err)

	object, err := svc.PublishObject(Object{
		Type:  "chat.message",
		Owner: contentTestOwner(0x32),
		BlobRefs: []Ref{{
			Kind: "blob",
			ID:   blob.Reference.String(),
		}},
	})
	require.NoError(t, err)

	manifest, err := svc.PublishManifest(Manifest{
		Owner: contentTestOwner(0x32),
		Refs: []Ref{{
			Kind: "blob",
			ID:   blob.Reference.String(),
		}},
	})
	require.NoError(t, err)

	retained, err := svc.RetainBlob(blob.Reference.String(), time.Now().UTC().Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, "retained-temporary", retained.State)

	source, err := svc.ObserveBlobSource(blob.Reference.String(), BlobSourceRecord{
		NodeID:    "p_remote",
		Trust:     SourceTrust{State: "ready", Outcome: "usable", Valid: true, Trusted: true, Usable: true},
		Usable:    true,
		Transport: "remote",
		Reason:    "trusted remote source answered blob fetch",
	})
	require.NoError(t, err)
	require.True(t, blob.Reference.Equal(source.ContentReference))

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
	require.Len(t, svc.ListBlobSources(blob.Reference.String()), 1)
}
