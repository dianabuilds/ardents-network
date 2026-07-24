package catalog

import (
	"bytes"
	"crypto/ed25519"
	"testing"

	"ardents/internal/identity/principal"

	"github.com/stretchr/testify/require"
)

func TestStoreSnapshotsDoNotAliasMutableState(t *testing.T) {
	blobs := NewBlobStore()
	reference, err := ParseContentReference("bafkreibm6jg3ux5qumhcn2b3flc3tyu6dmlb4xa7u5bf44yegnrjhc4yeq")
	if err != nil {
		t.Fatal(err)
	}
	blobs.Put(Blob{Reference: reference, State: "stored"})
	delete(blobs.Snapshot(), reference.String())
	if _, ok := blobs.Get(reference.String()); !ok {
		t.Fatal("snapshot deletion mutated blob store")
	}

	objects := NewObjectStore()
	owner := storeTestOwner(t, 0x40)
	require.NoError(t, objects.Put(Object{ID: "object-1", Owner: owner, Body: map[string]any{"nested": map[string]any{"value": "original"}}}))
	objects.Snapshot()[RecordStorageKey(owner, "object-1")].Body["nested"].(map[string]any)["value"] = "changed"
	storedObject, _ := objects.Get(owner, "object-1")
	if storedObject.Body["nested"].(map[string]any)["value"] != "original" {
		t.Fatal("snapshot mutation changed object store")
	}

	manifests := NewManifestStore()
	require.NoError(t, manifests.Put(Manifest{ID: "manifest-1", Owner: owner, Refs: []Ref{{ID: "ref-1"}}}))
	snapshot := manifests.Snapshot()[RecordStorageKey(owner, "manifest-1")]
	snapshot.Refs[0].ID = "changed"
	storedManifest, _ := manifests.Get(owner, "manifest-1")
	if storedManifest.Refs[0].ID != "ref-1" {
		t.Fatal("snapshot mutation changed manifest store")
	}

	sources := NewSourceLedger()
	sources.Replace("blob-1", []BlobSourceRecord{{NodeID: "node-1"}})
	sources.Snapshot()["blob-1"][0].NodeID = "changed"
	if sources.List("blob-1")[0].NodeID != "node-1" {
		t.Fatal("snapshot mutation changed source ledger")
	}
}

func TestManifestStoreDeleteIsOwnerQualified(t *testing.T) {
	alice := storeTestOwner(t, 0x41)
	bob := storeTestOwner(t, 0x42)
	manifests := NewManifestStore()
	require.NoError(t, manifests.Put(Manifest{ID: "shared", Owner: alice}))
	require.NoError(t, manifests.Put(Manifest{ID: "shared", Owner: bob}))

	manifests.Delete(alice, "shared")

	_, aliceExists := manifests.Get(alice, "shared")
	require.False(t, aliceExists)
	_, bobExists := manifests.Get(bob, "shared")
	require.True(t, bobExists)
}

func TestOwnerQualifiedStoresRejectEmptyIdentity(t *testing.T) {
	objects := NewObjectStore()
	require.ErrorContains(t, objects.Put(Object{ID: "object"}), "identity is invalid")
	require.ErrorContains(t, objects.Put(Object{Owner: storeTestOwner(t, 0x43)}), "identity is invalid")
	manifests := NewManifestStore()
	require.ErrorContains(t, manifests.Put(Manifest{ID: "manifest"}), "identity is invalid")
	require.ErrorContains(t, manifests.Put(Manifest{Owner: storeTestOwner(t, 0x44)}), "identity is invalid")
}

func storeTestOwner(t *testing.T, marker byte) principal.ID {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	owner, err := principal.FromEd25519PublicKey(key.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	return owner
}
