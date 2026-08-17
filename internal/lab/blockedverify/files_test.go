package blockedverify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashFileAcceptsEvidenceAboveJSONInputBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.bin")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	want := int64(maximumInput + 1)
	if err := file.Truncate(want); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	hash, size, err := hashFile(path)
	if err != nil || size != want || !isHexDigest(hash, 32) {
		t.Fatalf("large evidence hash=%q size=%d err=%v", hash, size, err)
	}
}

func TestMeasurementSnapshotBindsHashAndParseBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "measurement.jsonl")
	raw := []byte("{\"value\":1}\n")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	got, hash, size, err := snapshotMeasurement(path)
	if err != nil || string(got) != string(raw) || hash != digest(raw) || size != int64(len(raw)) {
		t.Fatalf("snapshot bytes=%q hash=%q size=%d err=%v", got, hash, size, err)
	}
}
