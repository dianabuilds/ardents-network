package duty

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ADR-0107 conversion oracle: a genuine version-1 generation with Transit
// Grant spend records opens, keeps its conflict duties and generation
// continuity, and the next replacement commits version 2 without the field.

func conversionClock() (time.Time, func() time.Time) {
	now := time.Unix(1_750_000_000, 0).UTC()
	return now, func() time.Time { return now }
}

func writeSchemaGeneration(t *testing.T, root string, version uint8, generation uint64, previous string, duties []dutyRecord, spends []transitGrantSpend) string {
	t.Helper()
	raw, err := json.Marshal(legacyDurableState{Version: version, Generation: generation, Previous: previous, Duties: duties, TransitGrantSpends: spends})
	if err != nil {
		t.Fatal(err)
	}
	name := sha256Hex(raw)
	if err := os.WriteFile(filepath.Join(root, "state-"+name), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "watermark"), []byte(fmt.Sprintf("%d %s\n", generation, name)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "current"), []byte(name+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

func currentGenerationName(t *testing.T, root string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(raw))
}

func seedConvertedRoot(t *testing.T, now time.Time) (string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "local-roles")
	clock := func() time.Time { return now }
	seed, err := Open(Config{Root: root, Clock: clock, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := seed.Replace([32]byte{200}, []Duty{{Identity: [32]byte{11}, Family: [32]byte{31},
		Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour)}}); err != nil {
		_ = seed.Close()
		t.Fatal(err)
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	return root, currentGenerationName(t, root)
}

func TestLegacyVersionOneGenerationConvertsInPlace(t *testing.T) {
	t.Parallel()
	now, clock := conversionClock()
	root, previous := seedConvertedRoot(t, now)
	legacy := writeSchemaGeneration(t, root, 1, 3, previous,
		[]dutyRecord{
			{Producer: [32]byte{200}, Identity: [32]byte{11}, Family: [32]byte{31}, Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour).Unix()},
			{Producer: [32]byte{201}, Identity: [32]byte{12}, Family: [32]byte{32}, Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour).Unix()},
		},
		[]transitGrantSpend{{NodeID: [32]byte{7}, GrantID: [32]byte{9}, NotAfter: now.Add(time.Hour).Unix()}})
	store, err := Open(Config{Root: root, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if conflict, err := store.Conflict([32]byte{12}, [32]byte{32}); err != nil || !conflict {
		t.Fatalf("converted legacy duties lost their conflict fact: %v, %v", conflict, err)
	}
	if err := store.Replace([32]byte{201}, []Duty{{Identity: [32]byte{13}, Family: [32]byte{33},
		Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour)}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "state-"+currentGenerationName(t, root)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "transit_grant_spends") {
		t.Error("committed generation still carries the retired spend field")
	}
	if !strings.Contains(string(raw), "\"version\":2") {
		t.Error("committed generation is not schema version 2")
	}
	if legacy == currentGenerationName(t, root) {
		t.Error("replacement did not commit a new generation")
	}
}

func TestWatermarkRecoveryRetainsLegacyGeneration(t *testing.T) {
	t.Parallel()
	now, clock := conversionClock()
	root, previous := seedConvertedRoot(t, now)
	legacy := writeSchemaGeneration(t, root, 1, 3, previous,
		[]dutyRecord{
			{Producer: [32]byte{200}, Identity: [32]byte{11}, Family: [32]byte{31}, Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour).Unix()},
			{Producer: [32]byte{201}, Identity: [32]byte{12}, Family: [32]byte{32}, Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour).Unix()},
		},
		[]transitGrantSpend{{NodeID: [32]byte{7}, GrantID: [32]byte{9}, NotAfter: now.Add(time.Hour).Unix()}})
	// Simulate the interrupted pointer update: the watermark already commits
	// the legacy generation 3 while "current" still names generation 2.
	if err := os.WriteFile(filepath.Join(root, "current"), []byte(previous+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(Config{Root: root, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if conflict, err := store.Conflict([32]byte{12}, [32]byte{32}); err != nil || !conflict {
		t.Fatalf("watermark recovery lost the converted legacy generation: %v, %v", conflict, err)
	}
	if recovered := currentGenerationName(t, root); recovered != legacy {
		t.Errorf("recovery repointed current to %s, want the legacy generation %s", recovered, legacy)
	}
}

func TestInvalidLegacyAndUnknownVersionsRefuse(t *testing.T) {
	t.Parallel()
	now, clock := conversionClock()
	duplicateSpendRoot, previous := seedConvertedRoot(t, now)
	writeSchemaGeneration(t, duplicateSpendRoot, 1, 3, previous,
		[]dutyRecord{{Producer: [32]byte{200}, Identity: [32]byte{11}, Family: [32]byte{31}, Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour).Unix()}},
		[]transitGrantSpend{
			{NodeID: [32]byte{7}, GrantID: [32]byte{9}, NotAfter: now.Add(time.Hour).Unix()},
			{NodeID: [32]byte{8}, GrantID: [32]byte{9}, NotAfter: now.Add(time.Hour).Unix()},
		})
	if store, err := Open(Config{Root: duplicateSpendRoot, Clock: clock}); err == nil {
		_ = store.Close()
		t.Error("a legacy generation with a duplicated GrantID still opened")
	}
	unknownRoot, unknownPrevious := seedConvertedRoot(t, now)
	writeSchemaGeneration(t, unknownRoot, 3, 3, unknownPrevious,
		[]dutyRecord{{Producer: [32]byte{200}, Identity: [32]byte{11}, Family: [32]byte{31}, Class: "direct-source", State: "exposed", NotAfter: now.Add(time.Hour).Unix()}},
		[]transitGrantSpend{})
	if store, err := Open(Config{Root: unknownRoot, Clock: clock}); err == nil {
		_ = store.Close()
		t.Error("an unknown schema version still opened")
	}
}
