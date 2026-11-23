//go:build linux

package selection

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClosedSetCommitGuardsRetainedGenerationExhaustion is the causal G2 proof.
// The guard must observe the RETAINED generation (the value about to be
// incremented), not the caller's next value which commit immediately overwrites.
// Before the fix a zero caller value overflowed retained+1 to zero and was caught
// only by the validator ("commit invalid"); the exhaustion guard was dead.
func TestClosedSetCommitGuardsRetainedGenerationExhaustion(t *testing.T) {
	config, _ := closedEntryFixture(t)
	owner, err := openClosedSets(config)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	owner.state.Generation = ^uint64(0)
	floorBefore, err := os.ReadFile(filepath.Join(config.Root, "watermark"))
	if err != nil {
		t.Fatal(err)
	}
	next := owner.state
	next.Generation = 0 // immediately overwritten; must not mask the retained guard
	err = owner.commitClosedSet(next)
	if err == nil || !strings.Contains(err.Error(), "closed Entry generation exhausted") {
		t.Fatalf("exhausted commit = %v; want retained-generation exhaustion refusal", err)
	}
	floorAfter, err := os.ReadFile(filepath.Join(config.Root, "watermark"))
	if err != nil || string(floorBefore) != string(floorAfter) {
		t.Fatalf("refused commit changed the durable floor: %v", err)
	}
}

func TestInteriorCommitGuardsRetainedGenerationExhaustion(t *testing.T) {
	root := filepath.Join(t.TempDir(), "interior")
	owner, err := openInteriorStoreMode(root, [32]byte{1}, 1, closedSetEither)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.close()
	owner.record.Generation = ^uint64(0)
	next := owner.record
	next.Generation = 0
	if err := owner.commit(next); err == nil || !strings.Contains(err.Error(), "interior generation exhausted") {
		t.Fatalf("exhausted commit = %v; want retained-generation exhaustion refusal", err)
	}
}

// TestGenerationReadRefusesTrailingDataUniformly is the G6 regression guard: both
// owners accept exactly one JSON object and refuse trailing data through the
// shared errors.Is(io.EOF) check (the interior previously used a bare != io.EOF).
func TestGenerationReadRefusesTrailingDataUniformly(t *testing.T) {
	root := t.TempDir()
	write := func(raw []byte) string {
		name := sha256Hex(raw)
		if err := os.WriteFile(filepath.Join(root, "state-"+name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		return name
	}
	csRaw, err := json.Marshal(closedSetState{Version: 1, NetworkID: [32]byte{1}, Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readClosedSetGeneration(root, write(csRaw)); err != nil {
		t.Fatalf("valid closed Entry generation refused: %v", err)
	}
	if _, err := readClosedSetGeneration(root, write(append(append([]byte{}, csRaw...), []byte("\n{}")...))); err == nil {
		t.Fatal("closed Entry trailing data accepted")
	}
	irRaw, err := json.Marshal(interiorRecord{Version: 1, Network: [32]byte{1}, Domain: 1, Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readInteriorRecord(root, write(irRaw)); err != nil {
		t.Fatalf("valid interior generation refused: %v", err)
	}
	if _, err := readInteriorRecord(root, write(append(append([]byte{}, irRaw...), []byte("\n{}")...))); err == nil {
		t.Fatal("interior trailing data accepted")
	}
}

// TestClosedSetCanonicalBytesUnchanged locks the durable byte grammar after the
// shared-mechanism dedup: marker, watermark floor, current pointer and the
// content-addressed generation keep their exact canonical format, and a reopen
// retains the identical bytes without rewrite drift.
func TestClosedSetCanonicalBytesUnchanged(t *testing.T) {
	config, _ := closedEntryFixture(t)
	owner, err := openClosedSets(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Members(1); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	marker, err := os.ReadFile(filepath.Join(config.Root, closedSetMarker))
	if err != nil || string(marker) != "ardents-entry-set-v1\n" {
		t.Fatalf("marker bytes = %q, %v", marker, err)
	}
	floor, exists, err := loadWatermark(config.Root)
	if err != nil || !exists || floor.generation == 0 || !stateName.MatchString(floor.name) {
		t.Fatalf("watermark floor = %+v, %v, %v", floor, exists, err)
	}
	watermark, err := os.ReadFile(filepath.Join(config.Root, "watermark"))
	if err != nil || string(watermark) != fmt.Sprintf("%d %s\n", floor.generation, floor.name) {
		t.Fatalf("watermark bytes = %q, %v", watermark, err)
	}
	current, err := os.ReadFile(filepath.Join(config.Root, "current"))
	if err != nil || string(current) != floor.name+"\n" {
		t.Fatalf("current bytes = %q, %v", current, err)
	}
	raw, err := os.ReadFile(filepath.Join(config.Root, "state-"+floor.name))
	if err != nil || sha256Hex(raw) != floor.name {
		t.Fatalf("generation bytes do not match content address: %v", err)
	}
	reopened, err := openClosedSets(config)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.name != floor.name || reopened.state.Generation != floor.generation {
		t.Fatal("reopen drifted from the retained floor")
	}
}
