//go:build live

package network_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type finalSeedSchedule struct {
	CellOrder []string `json:"cell_order"`
	Seeds     []string `json:"seeds"`
}

func bindFinalFixtureSeed(t *testing.T, fixture blockedEntryFixture, cell, label string) {
	t.Helper()
	seed, ok := finalDerivedCellSeed(t, cell, label)
	if !ok {
		return
	}
	writeFinalFixtureSeed(t, fixture, seed)
}

func bindFinalFixturePairSeed(t *testing.T, fixture blockedEntryFixture, first, second, label string) {
	t.Helper()
	firstSeed, firstOK := finalDerivedCellSeed(t, first, label+"-first")
	secondSeed, secondOK := finalDerivedCellSeed(t, second, label+"-second")
	if !firstOK || !secondOK {
		return
	}
	combined := sha256.Sum256(append(firstSeed[:], secondSeed[:]...))
	writeFinalFixtureSeed(t, fixture, combined)
}

func writeFinalFixtureSeed(t *testing.T, fixture blockedEntryFixture, seed [32]byte) {
	t.Helper()
	writeFinalApplicationSeedPair(t, filepath.Join(fixture.root, "input", "client-app"),
		filepath.Join(fixture.root, "input", "publisher-app"), seed)
}

func writeFinalApplicationSeedPair(t *testing.T, client, publisher string, seed [32]byte) {
	t.Helper()
	peer := finalSeedDigest(seed, "publisher-application")
	writeLiveFile(t, filepath.Join(client, "own.hex"),
		[]byte(hex.EncodeToString(seed[:])))
	writeLiveFile(t, filepath.Join(client, "peer.hex"),
		[]byte(hex.EncodeToString(peer[:])))
	writeLiveFile(t, filepath.Join(publisher, "own.hex"),
		[]byte(hex.EncodeToString(peer[:])))
	writeLiveFile(t, filepath.Join(publisher, "peer.hex"),
		[]byte(hex.EncodeToString(seed[:])))
}

func finalCapacityUnitSeed(t *testing.T, cell, purpose string, ordinal int) [32]byte {
	t.Helper()
	label := fmt.Sprintf("%s-capacity-unit-%02d-stream", purpose, ordinal)
	if seed, ok := finalDerivedCellSeed(t, cell, label); ok {
		return seed
	}
	return finalSeedDigest([32]byte{0xc0}, label)
}

func bindFinalOfferSeed(t *testing.T, fixture blockedEntryFixture, cell, label string) {
	t.Helper()
	seed, ok := finalDerivedCellSeed(t, cell, label)
	if ok {
		writeLiveFile(t, filepath.Join(fixture.root, "input", "capacity-probe", "corpus-seed.bin"), seed[:])
	}
}

func bindFinalPressureSeed(t *testing.T, fixture blockedEntryFixture, cell, label string) {
	t.Helper()
	seed, ok := finalDerivedCellSeed(t, cell, label)
	if ok {
		writeLiveFile(t, filepath.Join(fixture.root, "input", "pressure", "corpus-seed.bin"), seed[:])
	}
}

func bindFinalProbeSeed(t *testing.T, fixture blockedEntryFixture, cell, label string) {
	t.Helper()
	seed, ok := finalDerivedCellSeed(t, cell, label)
	if ok {
		writeLiveFile(t, filepath.Join(fixture.root, "input", "probe", "corpus-seed.bin"), seed[:])
	}
}

func finalDerivedCellSeed(t *testing.T, cell, label string) ([32]byte, bool) {
	t.Helper()
	if encoded := os.Getenv("ARDENTS_FINAL_CELL_SEED"); encoded != "" {
		if selected := os.Getenv("ARDENTS_FINAL_CELL"); selected != cell {
			t.Fatalf("final worker seed cell=%q want=%q", selected, cell)
		}
		seed, err := hex.DecodeString(encoded)
		if err != nil || len(seed) != 32 {
			t.Fatalf("final worker cell seed is invalid: %v", err)
		}
		var fixed [32]byte
		copy(fixed[:], seed)
		return finalSeedDigest(fixed, label), true
	}
	path := os.Getenv("ARDENTS_BLOCKED_CAMPAIGN_SPEC")
	if path == "" {
		return [32]byte{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var schedule finalSeedSchedule
	if err := json.Unmarshal(raw, &schedule); err != nil || len(schedule.CellOrder) != len(schedule.Seeds) {
		t.Fatalf("final campaign seed schedule is invalid: %v", err)
	}
	for index, identity := range schedule.CellOrder {
		if identity != cell {
			continue
		}
		seed, decodeErr := hex.DecodeString(schedule.Seeds[index])
		if decodeErr != nil || len(seed) != 32 || label == "" {
			t.Fatalf("final seed for %s/%s is invalid: %v", cell, label, decodeErr)
		}
		var fixed [32]byte
		copy(fixed[:], seed)
		return finalSeedDigest(fixed, label), true
	}
	t.Fatalf("final campaign cell %q is absent from the seed schedule", cell)
	return [32]byte{}, false
}

func finalSeedDigest(seed [32]byte, label string) [32]byte {
	input := make([]byte, 0, len(seed)+len(label)+1)
	input = append(input, seed[:]...)
	input = append(input, 0)
	input = append(input, label...)
	return sha256.Sum256(input)
}

func TestFinalDerivedCellSeedBindsCellSeedAndPurpose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "final-spec.json")
	seed := hex.EncodeToString(make([]byte, 32))
	secondSeed := hex.EncodeToString(append([]byte{1}, make([]byte, 31)...))
	raw, err := json.Marshal(finalSeedSchedule{CellOrder: []string{"profile/C1/00", "profile/C1/01"},
		Seeds: []string{seed, secondSeed}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARDENTS_BLOCKED_CAMPAIGN_SPEC", path)
	workload, ok := finalDerivedCellSeed(t, "profile/C1/00", "short-workload")
	probe, probeOK := finalDerivedCellSeed(t, "profile/C1/00", "probe-corpus")
	if !ok || !probeOK || workload == probe || workload == [32]byte{} || probe == [32]byte{} {
		t.Fatalf("derived seeds are not purpose-bound: workload=%x probe=%x", workload, probe)
	}
	second, secondOK := finalDerivedCellSeed(t, "profile/C1/01", "short-workload")
	if !secondOK || second == workload {
		t.Fatalf("distinct cell seeds collapsed: first=%x second=%x", workload, second)
	}
	unit0 := finalCapacityUnitSeed(t, "profile/C1/00", "pressure", 0)
	unit1 := finalCapacityUnitSeed(t, "profile/C1/00", "pressure", 1)
	if unit0 == unit1 || unit0 == workload || unit1 == workload {
		t.Fatalf("capacity-unit streams are not independently derived: %x %x", unit0, unit1)
	}
}

func TestFinalWorkerSeedIsBoundWithoutSecretSpecPath(t *testing.T) {
	t.Setenv("ARDENTS_FINAL_CELL", "profile/C1/00")
	t.Setenv("ARDENTS_FINAL_CELL_SEED", strings.Repeat("ab", 32))
	first, ok := finalDerivedCellSeed(t, "profile/C1/00", "first")
	second, secondOK := finalDerivedCellSeed(t, "profile/C1/00", "second")
	if !ok || !secondOK || first == second {
		t.Fatalf("worker seeds are not purpose separated: %x %x", first, second)
	}
}
