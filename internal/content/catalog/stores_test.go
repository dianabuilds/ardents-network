package catalog

import "testing"

func TestStoreSnapshotsDoNotAliasMutableState(t *testing.T) {
	blobs := NewBlobStore()
	blobs.Put(Blob{ID: "blob-1", State: "stored"})
	delete(blobs.Snapshot(), "blob-1")
	if _, ok := blobs.Get("blob-1"); !ok {
		t.Fatal("snapshot deletion mutated blob store")
	}

	objects := NewObjectStore()
	objects.Put(Object{ID: "object-1", Body: map[string]any{"nested": map[string]any{"value": "original"}}})
	objects.Snapshot()["object-1"].Body["nested"].(map[string]any)["value"] = "changed"
	storedObject, _ := objects.Get("object-1")
	if storedObject.Body["nested"].(map[string]any)["value"] != "original" {
		t.Fatal("snapshot mutation changed object store")
	}

	manifests := NewManifestStore()
	manifests.Put(Manifest{ID: "manifest-1", Refs: []Ref{{ID: "ref-1"}}})
	snapshot := manifests.Snapshot()["manifest-1"]
	snapshot.Refs[0].ID = "changed"
	storedManifest, _ := manifests.Get("manifest-1")
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
