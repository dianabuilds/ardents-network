package update

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// recoveryOracleInvEntry is one expected generation or staging entry with its
// artifact and manifest SHA-256.
type recoveryOracleInvEntry struct {
	generation uint64
	artifact   [32]byte
	manifest   [32]byte
}

// recoveryOracleInvJournalEntry is one expected journal entry with its state
// code and exact canonical raw bytes.
type recoveryOracleInvJournalEntry struct {
	state byte
	bytes []byte
}

// recoveryOracleInventory is the exact expected post-Recover tree shape.
// Root children, generations, staging, and journal prefix are all listed
// with their exact bytes; the assertion fails on any unknown root child,
// on any missing expected entry, or on any current temp file.
type recoveryOracleInventory struct {
	rootDirs    []string
	rootFiles   []string
	currentRaw  []byte
	generations []recoveryOracleInvEntry
	staging     []recoveryOracleInvEntry
	journal     []recoveryOracleInvJournalEntry
}

// recoveryOracleAssertInventory asserts the post-Recover tree matches the
// literal expected inventory exactly. The expected bytes come only from
// test-only encoders (oracleEnvelope, oracleAppendUint64, oracleCurrent,
// oracleManifest, recoveryOracleCandidateManifest); no production
// validator, inspector, planner, classifier, encoder, or Result
// constructor is invoked. The assertion distinguishes INVENTORY: failures
// (post-Recover shape mismatch) from FIXTURE: setup failures and from
// RECOVER: Result assertion failures elsewhere.
func recoveryOracleAssertInventory(t *testing.T, root string, expected recoveryOracleInventory) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("INVENTORY: read root: %v", err)
	}
	gotDirs := map[string]bool{}
	gotFiles := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			gotDirs[entry.Name()] = true
		} else {
			gotFiles[entry.Name()] = true
		}
	}
	for _, name := range expected.rootDirs {
		if !gotDirs[name] {
			t.Fatalf("INVENTORY: missing root directory %q (rootDirs=%v, rootFiles=%v)", name, inventoryKeys(gotDirs), inventoryKeys(gotFiles))
		}
		delete(gotDirs, name)
	}
	for _, name := range expected.rootFiles {
		if !gotFiles[name] {
			t.Fatalf("INVENTORY: missing root file %q (rootDirs=%v, rootFiles=%v)", name, inventoryKeys(gotDirs), inventoryKeys(gotFiles))
		}
		delete(gotFiles, name)
	}
	for extra := range gotDirs {
		if inventoryIsCurrentTempName(extra) {
			t.Fatalf("INVENTORY: unexpected current temp %q under root", extra)
		}
		t.Fatalf("INVENTORY: unexpected root directory %q", extra)
	}
	for extra := range gotFiles {
		if inventoryIsCurrentTempName(extra) {
			t.Fatalf("INVENTORY: unexpected current temp %q under root", extra)
		}
		t.Fatalf("INVENTORY: unexpected root file %q", extra)
	}
	if len(expected.currentRaw) > 0 {
		gotCurrent, err := os.ReadFile(filepath.Join(root, "current"))
		if err != nil {
			t.Fatalf("INVENTORY: read current: %v", err)
		}
		if !inventoryBytesEqual(gotCurrent, expected.currentRaw) {
			t.Fatalf("INVENTORY: current bytes mismatch: got=%x want=%x", gotCurrent, expected.currentRaw)
		}
	}
	for _, gen := range expected.generations {
		dir := filepath.Join(root, "generations", strconv.FormatUint(gen.generation, 10))
		inventoryAssertGenDir(t, dir, gen)
	}
	for _, st := range expected.staging {
		dir := filepath.Join(root, "staging", strconv.FormatUint(st.generation, 10))
		inventoryAssertGenDir(t, dir, st)
	}
	for _, entry := range expected.journal {
		path := filepath.Join(root, "transactions", "1", "journal", recoveryOracleName(entry.state))
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("INVENTORY: read journal entry %d: %v", entry.state, err)
		}
		if !inventoryBytesEqual(got, entry.bytes) {
			t.Fatalf("INVENTORY: journal entry %d bytes mismatch: got=%x want=%x", entry.state, got, entry.bytes)
		}
	}
}

// inventoryAssertGenDir asserts a generations/<n> or staging/<n> directory
// exists with exactly the canonical artifact and manifest.bin files at
// the expected SHA-256 digests.
func inventoryAssertGenDir(t *testing.T, dir string, expected recoveryOracleInvEntry) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("INVENTORY: read generation/staging directory %s: %v", dir, err)
	}
	if len(entries) != 2 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("INVENTORY: directory %s has %d entries, want 2 (artifact, manifest.bin): %v", dir, len(entries), names)
	}
	artifactPath := filepath.Join(dir, "artifact")
	artifactBytes, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("INVENTORY: read %s: %v", artifactPath, err)
	}
	gotArtifact := sha256.Sum256(artifactBytes)
	if gotArtifact != expected.artifact {
		t.Fatalf("INVENTORY: artifact digest mismatch in %s: got=%x want=%x", dir, gotArtifact, expected.artifact)
	}
	manifestPath := filepath.Join(dir, "manifest.bin")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("INVENTORY: read %s: %v", manifestPath, err)
	}
	gotManifest := sha256.Sum256(manifestBytes)
	if gotManifest != expected.manifest {
		t.Fatalf("INVENTORY: manifest digest mismatch in %s: got=%x want=%x", dir, gotManifest, expected.manifest)
	}
}

// recoveryOracleInventoryFor returns the expected post-Recover tree
// inventory for the named R row. The journal prefix and digest bindings
// come from the saved V0 facts plus the in-memory candidate manifest;
// staging/generations sets reflect the documented normalization for each
// row. R12-R14 expose the successor current and the candidate generation;
// R00-R11 expose the canonical predecessor current and only the
// generations and staging rows required by their state.
func recoveryOracleInventoryFor(t *testing.T, rowID string, lastState byte) recoveryOracleInventory {
	t.Helper()
	previous := oracleReadExact(t, oraclePreviousPath, recoveryOraclePreviousLength, recoveryOraclePreviousDigestHex)
	previousDigest := sha256.Sum256(previous)
	previousManifest := inventoryBootstrapManifestDigest(t)
	predecessorRaw := inventoryBootstrapPredecessorCurrent(t)
	_, _, candidateDigest, candidateManifestDigest := recoveryOracleCandidateManifest(t)
	successorRaw := inventoryBootstrapSuccessorCurrent(t)
	rootDirs := []string{"generations", "staging", "transactions"}
	rootFiles := []string{lockFileName, rootMarkerName, "current"}
	previousEntry := recoveryOracleInvEntry{
		generation: 0, artifact: previousDigest, manifest: previousManifest,
	}
	candidateEntry := recoveryOracleInvEntry{
		generation: 1, artifact: candidateDigest, manifest: candidateManifestDigest,
	}
	switch rowID {
	case "R00":
		return recoveryOracleInventory{
			rootDirs: rootDirs, rootFiles: rootFiles,
			currentRaw:  predecessorRaw,
			generations: []recoveryOracleInvEntry{previousEntry},
			staging:     nil,
			journal:     nil,
		}
	case "R01":
		return recoveryOracleInventory{
			rootDirs:    rootDirs,
			rootFiles:   rootFiles,
			currentRaw:  predecessorRaw,
			generations: []recoveryOracleInvEntry{previousEntry},
			journal:     inventoryExpectedJournal(t, 1, lastState, inventoryPredecessorEnvelope(t), candidateDigest, candidateManifestDigest),
		}
	case "R02", "R03":
		return recoveryOracleInventory{
			rootDirs:    rootDirs,
			rootFiles:   rootFiles,
			currentRaw:  predecessorRaw,
			generations: []recoveryOracleInvEntry{previousEntry},
			journal:     inventoryExpectedJournal(t, 1, lastState, inventoryPredecessorEnvelope(t), candidateDigest, candidateManifestDigest),
		}
	case "R04", "R05", "R06", "R07":
		return recoveryOracleInventory{
			rootDirs:    rootDirs,
			rootFiles:   rootFiles,
			currentRaw:  predecessorRaw,
			generations: []recoveryOracleInvEntry{previousEntry},
			staging:     []recoveryOracleInvEntry{candidateEntry},
			journal:     inventoryExpectedJournal(t, 1, lastState, inventoryPredecessorEnvelope(t), candidateDigest, candidateManifestDigest),
		}
	case "R08", "R09", "R10", "R11":
		return recoveryOracleInventory{
			rootDirs:    rootDirs,
			rootFiles:   rootFiles,
			currentRaw:  predecessorRaw,
			generations: []recoveryOracleInvEntry{previousEntry},
			staging:     []recoveryOracleInvEntry{candidateEntry},
			journal:     inventoryExpectedJournal(t, 1, 6, inventoryPredecessorEnvelope(t), candidateDigest, candidateManifestDigest),
		}
	case "R12", "R13":
		return recoveryOracleInventory{
			rootDirs:    rootDirs,
			rootFiles:   rootFiles,
			currentRaw:  successorRaw,
			generations: []recoveryOracleInvEntry{previousEntry, candidateEntry},
			journal:     inventoryExpectedJournal(t, 1, lastState, inventoryPredecessorEnvelope(t), candidateDigest, candidateManifestDigest),
		}
	case "R14":
		return recoveryOracleInventory{
			rootDirs:    rootDirs,
			rootFiles:   rootFiles,
			currentRaw:  successorRaw,
			generations: []recoveryOracleInvEntry{previousEntry, candidateEntry},
			journal:     inventoryExpectedJournal(t, 1, 9, inventoryPredecessorEnvelope(t), candidateDigest, candidateManifestDigest),
		}
	default:
		t.Fatalf("INVENTORY: unknown row %q", rowID)
		return recoveryOracleInventory{}
	}
}

// inventoryExpectedJournal returns the canonical bytes for each journal
// entry through `lastState`, using the in-memory candidate manifest digest
// and the documented V0 reference times. The chain commits the
// predecessor-inspection envelope SHA as entry 01's predecessor and chains
// each subsequent entry via SHA-256 of the previous entry bytes.
func inventoryExpectedJournal(t *testing.T, generation uint64, lastState byte,
	predecessorEnvelopeDigest, candidateArtifact, candidateManifest [32]byte) []recoveryOracleInvJournalEntry {
	t.Helper()
	entries := make([]recoveryOracleInvJournalEntry, 0, int(lastState))
	predecessor := predecessorEnvelopeDigest
	var elapsed uint64
	for state := byte(1); state <= lastState; state++ {
		body := inventoryJournalBody(state, generation, predecessor, candidateArtifact, candidateManifest, elapsed)
		raw := oracleEnvelope(t, 4, body)
		entries = append(entries, recoveryOracleInvJournalEntry{state: state, bytes: raw})
		predecessor = sha256.Sum256(raw)
		elapsed += 1000
	}
	return entries
}

// inventoryJournalBody builds one canonical journal body using the frozen
// V0 deadline facts. It mirrors recoveryOracleBody in the fixture file but
// lives here so the inventory assertion needs no shared mutable helper
// beyond oracleEnvelope and oracleAppendUint64.
func inventoryJournalBody(state byte, generation uint64, predecessor, artifact, manifest [32]byte, elapsed uint64) []byte {
	var adapterResult byte
	switch state {
	case 5, 6, 8:
		adapterResult = 1
	default:
		adapterResult = 0
	}
	deadline := recoveryOracleTerminate.Unix()
	if state == 5 {
		deadline = recoveryOracleNoNewWork.Unix()
	}
	body := []byte{state}
	body = oracleAppendUint64(body, generation)
	body = append(body, predecessor[:]...)
	body = append(body, artifact[:]...)
	body = append(body, manifest[:]...)
	body = append(body, adapterResult, state)
	body = oracleAppendUint64(body, elapsed)
	body = oracleAppendUint64(body, uint64(deadline))
	return body
}

// inventoryBootstrapManifestDigest builds a throwaway bootstrap tree and
// returns the bootstrap manifest SHA-256. It exists so the inventory file
// can derive the canonical predecessor manifest digest without depending
// on the checkpoint test's t.TempDir layout.
func inventoryBootstrapManifestDigest(t *testing.T) [32]byte {
	t.Helper()
	root := filepath.Join(t.TempDir(), "inventory-bootstrap")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatalf("FIXTURE: create inventory bootstrap root: %v", err)
	}
	oracleBootstrapV0(t, root)
	return recoveryOracleBootstrapManifestDigest(t, root)
}

// inventoryPredecessorEnvelope computes the SHA-256 of the canonical
// predecessor-inspection envelope for the V0 bootstrap (no rollback).
// It drives a fresh bootstrap root and returns the predecessor envelope
// SHA that entry 01 of every chain binds.
func inventoryPredecessorEnvelope(t *testing.T) [32]byte {
	t.Helper()
	_, envelope := recoveryOracleBootstrap(t)
	return envelope
}

// inventoryBootstrapPredecessorCurrent builds a canonical bootstrap
// predecessor current and returns its exact bytes.
func inventoryBootstrapPredecessorCurrent(t *testing.T) []byte {
	t.Helper()
	previous := oracleReadExact(t, oraclePreviousPath, recoveryOraclePreviousLength, recoveryOraclePreviousDigestHex)
	previousDigest := sha256.Sum256(previous)
	bootstrapManifest := inventoryBootstrapManifestDigest(t)
	predecessorTuple := oracleCurrentTuple{
		Generation: 0, Length: recoveryOraclePreviousLength,
		Artifact: previousDigest, Manifest: bootstrapManifest,
	}
	return oracleCurrent(t, 0, predecessorTuple, nil)
}

// inventoryBootstrapSuccessorCurrent builds the canonical post-R12
// successor current with rollback to generation 0 and returns its bytes.
func inventoryBootstrapSuccessorCurrent(t *testing.T) []byte {
	t.Helper()
	previous := oracleReadExact(t, oraclePreviousPath, recoveryOraclePreviousLength, recoveryOraclePreviousDigestHex)
	previousDigest := sha256.Sum256(previous)
	bootstrapManifest := inventoryBootstrapManifestDigest(t)
	predecessorTuple := oracleCurrentTuple{
		Generation: 0, Length: recoveryOraclePreviousLength,
		Artifact: previousDigest, Manifest: bootstrapManifest,
	}
	_, _, candidateDigest, candidateManifestDigest := recoveryOracleCandidateManifest(t)
	candidateTuple := oracleCurrentTuple{
		Generation: 1, Length: recoveryOracleCandidateLength(),
		Artifact: candidateDigest, Manifest: candidateManifestDigest,
	}
	return oracleCurrent(t, 1, candidateTuple, &predecessorTuple)
}

// inventoryIsCurrentTempName reports whether a root child name looks like
// the post-atomic-replace temporary pattern. The post-Recover tree must
// never contain any such file.
func inventoryIsCurrentTempName(name string) bool {
	const prefix = ".current."
	const suffix = ".tmp"
	return len(name) > len(prefix)+len(suffix) && name[:len(prefix)] == prefix && name[len(name)-len(suffix):] == suffix
}

// inventoryKeys returns the keys of a string set as a slice for error
// messages.
func inventoryKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

// inventoryBytesEqual is a local equality helper so the inventory file
// does not import bytes.
func inventoryBytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}
