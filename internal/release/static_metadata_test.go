//go:build linux

package release

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestFixedMetadataProjectionPreservesReferencedOnlyEnvelope(t *testing.T) {
	h := freshSignedHistory(t)
	files, err := StaticMetadataFiles(h.input.RootBytes, h.input.Files[metadataBaseURL+"timestamp.json"], h.input.Files[metadataBaseURL+"1.snapshot.json"], h.input.Files[metadataBaseURL+"1.targets.json"])
	if err != nil || len(files) != 3 || files[metadataBaseURL+"snapshot.json"] != nil || files[metadataBaseURL+"targets.json"] != nil {
		t.Fatal("consistent names gained unreferenced aliases", err)
	}
	h.input.Files = files
	v, err := Open(filepath.Join(t.TempDir(), "history"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := v.Close(); err != nil {
			t.Error(err)
		}
	}()
	d := v.Evaluate(t.Context(), h.input)
	if _, ok := d.Authorization(); !ok {
		t.Fatalf("genuine referenced bytes %s %s %v", d.Outcome, d.Notice, d.Err())
	}
	files[metadataBaseURL+"1.targets.json"] = bytes.Replace(files[metadataBaseURL+"1.targets.json"], []byte("h3-test"), []byte("forgery"), 1)
	d = v.Evaluate(t.Context(), h.input)
	if _, ok := d.Authorization(); ok {
		t.Fatal("projection became signature authority")
	}
}
