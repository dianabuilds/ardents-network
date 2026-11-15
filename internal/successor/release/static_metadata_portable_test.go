package release

import (
	"bytes"
	"testing"
)

func TestStaticMetadataProjectionIsDetachedAndRejectsInvalidReferences(t *testing.T) {
	root := []byte(`{"signed":{"consistent_snapshot":true}}`)
	timestamp := []byte(`{"signed":{"meta":{"snapshot.json":{"version":2}}}}`)
	snapshot := []byte(`{"signed":{"meta":{"targets.json":{"version":3}}}}`)
	targets := []byte(`{"signed":{}}`)
	files, err := StaticMetadataFiles(root, timestamp, snapshot, targets)
	if err != nil || len(files) != 3 || !bytes.Equal(files["https://release.invalid/metadata/2.snapshot.json"], snapshot) || !bytes.Equal(files["https://release.invalid/metadata/3.targets.json"], targets) {
		t.Fatalf("consistent metadata: %v", err)
	}
	timestamp[0] = 'x'
	if files["https://release.invalid/metadata/timestamp.json"][0] != '{' {
		t.Fatal("projection aliases caller bytes")
	}
	for _, raw := range [][]byte{nil, []byte(`{"signed":{"meta":{}}}`), []byte(`{"signed":{},"signed":{}}`)} {
		if _, err := StaticMetadataFiles(root, raw, snapshot, targets); err == nil {
			t.Fatal("invalid timestamp admitted")
		}
	}
}
