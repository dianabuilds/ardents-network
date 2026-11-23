package durablefile

import (
	"os"
	"path/filepath"
	"testing"
)

func testReadBounded(path string, maximum int64) ([]byte, error) {
	return ReadBounded(path, maximum, "test bound refusal")
}

func TestWatermarkRoundTrip(t *testing.T) {
	root := t.TempDir()
	name := SHA256Hex([]byte("generation"))
	if err := ReplaceWatermark(root, 7, name, "invalid value"); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(filepath.Join(root, "watermark")); err != nil || string(raw) != "7 "+name+"\n" {
		t.Fatalf("watermark bytes = %q, %v", raw, err)
	}
	generation, loaded, exists, err := LoadWatermark(root, testReadBounded, "invalid watermark")
	if err != nil || !exists || generation != 7 || loaded != name {
		t.Fatalf("load = %d, %s, %v, %v", generation, loaded, exists, err)
	}
}

func TestLoadWatermarkAbsent(t *testing.T) {
	_, _, exists, err := LoadWatermark(t.TempDir(), testReadBounded, "invalid watermark")
	if err != nil || exists {
		t.Fatalf("absent watermark = %v, %v; want exists=false, nil", exists, err)
	}
}

func TestLoadWatermarkRefusesMalformed(t *testing.T) {
	root := t.TempDir()
	name := SHA256Hex([]byte("generation"))
	for _, malformed := range []string{"7 " + name, "0 " + name + "\n", "seven " + name + "\n", "7 " + name + "\n\n", "7  " + name + "\n"} {
		if err := os.WriteFile(filepath.Join(root, "watermark"), []byte(malformed), 0o600); err != nil {
			t.Fatal(err)
		}
		_, _, _, err := LoadWatermark(root, testReadBounded, "invalid watermark")
		if err == nil || err.Error() != "invalid watermark" {
			t.Fatalf("malformed %q accepted: %v", malformed, err)
		}
	}
}

func TestReplaceWatermarkRefusesInvalidValue(t *testing.T) {
	root := t.TempDir()
	name := SHA256Hex([]byte("generation"))
	if err := ReplaceWatermark(root, 0, name, "invalid value"); err == nil {
		t.Fatal("zero generation accepted")
	}
	if err := ReplaceWatermark(root, 1, "not-a-digest", "invalid value"); err == nil {
		t.Fatal("non-canonical name accepted")
	}
}
