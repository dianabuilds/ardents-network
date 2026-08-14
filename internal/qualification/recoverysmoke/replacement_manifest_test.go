package recoverysmoke

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPrepareReplacementManifestFreezesCellInputs(t *testing.T) {
	root := t.TempDir()
	seed := [32]byte{7}
	roles := []string{"initiator", "rendezvous", "responder"}
	offsets := []uint32{64 * 16_381, 128 * 16_381, 192 * 16_381}
	value, err := prepareReplacementManifest(root, "client-to-publisher", "sequential-three", seed,
		roles, offsets, "12m", "2350ms")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "replacement-cell-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var retained replacementCellManifest
	if err := json.Unmarshal(raw, &retained); err != nil {
		t.Fatal(err)
	}
	if retained.Digest != value.Digest || retained.Digest != replacementManifestDigest(retained) ||
		retained.ChunkDelayNanos != int64(2350*time.Millisecond) ||
		retained.LifetimeNanos != int64(12*time.Minute) || len(retained.FailureRoles) != 3 ||
		len(retained.FaultOffsets) != 3 {
		t.Fatalf("replacement manifest is incomplete: %+v", retained)
	}
	retained.FaultOffsets[0]++
	if replacementManifestDigest(retained) == value.Digest {
		t.Fatal("changed fault schedule retained the manifest digest")
	}
}

func TestPrepareReplacementManifestRejectsInvalidDurations(t *testing.T) {
	if _, err := prepareReplacementManifest(t.TempDir(), "client-to-publisher", "isolated-initiator",
		[32]byte{1}, []string{"initiator"}, []uint32{17 * 16_381}, "invalid", "20ms"); err == nil {
		t.Fatal("invalid replacement lifetime was accepted")
	}
	if _, err := prepareReplacementManifest(t.TempDir(), "client-to-publisher", "isolated-initiator",
		[32]byte{1}, []string{"initiator"}, []uint32{17 * 16_381}, "30s", "invalid"); err == nil {
		t.Fatal("invalid replacement pacing was accepted")
	}
}
