package updatetransaction

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// frozen V0 digests used as independent expected values
const (
	recoveryOraclePreviousDigestHex  = "8bdad9bde29bb6ee2a9d1d7005ec8ba2461b2bad3627372ee8458693c1fc08af"
	recoveryOracleCandidateDigestHex = "a52b68413e0cd723547790c7ac161ece935d6459377442644b18031c3dc27d0a"
	recoveryOracleCustodyNotice      = "H3 threshold identities and both rebuild records are project-controlled; no independent custody or builder claim is made"
)

// TestRecoverInterruptionMatrix is the literal independent R00-R14 oracle.
// Each row constructs the exact physical checkpoint from the V0 fixture and
// saved reference facts, then asserts the frozen Result, digests, and
// normalized tree state. The expected Results, digests, and notices are
// independently encoded; no production validator, classifier, planner,
// encoder, hash helper, inspector, or Result constructor is used to derive
// expected values. In the current implementation every row except R14
// returns release-invalid because Recover was terminal-only and recognized
// no interruption states. R14 is the frozen terminal control and must
// remain green.
func TestRecoverInterruptionMatrix(t *testing.T) {
	for _, row := range recoveryOracleRows() {
		row := row
		t.Run(row.id, func(t *testing.T) {
			root, predecessor := recoveryOracleBootstrap(t)
			artifact, manifest := row.setup(t, root, predecessor)
			result, err := Recover(context.Background(), root)
			row.assert(t, result, err, root, artifact, manifest)
			// Post-Recover inventory assertion; runs after Result assertion.
			recoveryOracleAssertInventory(t, root, recoveryOracleInventoryFor(t, row.id, row.lastJournalState))
		})
	}
}

// recoveryOracleCandidateLength is the frozen V0 candidate payload length.
func recoveryOracleCandidateLength() uint64 {
	return uint64(4096)
}

// recoveryOracleChainOnly returns a setup that writes a chain of journal
// entries without staging any candidate bytes. The chain binds the
// deterministic candidate manifest prepared in memory before entry 01.
func recoveryOracleChainOnly(lastState byte) func(t *testing.T, root string, predecessor [32]byte) ([32]byte, [32]byte) {
	return func(t *testing.T, root string, predecessor [32]byte) ([32]byte, [32]byte) {
		_, _, artifactDigest, manifestDigest := recoveryOracleCandidateManifest(t)
		recoveryOracleWriteChain(t, root, 1, predecessor, artifactDigest, manifestDigest, lastState)
		return artifactDigest, manifestDigest
	}
}

// recoveryOracleBootstrap builds the V0 root and returns the predecessor
// inspection envelope SHA-256 used as entry 01's predecessor commitment.
func recoveryOracleBootstrap(t *testing.T) (string, [32]byte) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	oracleBootstrapV0(t, root)
	vector := oracleLoadV0(t)
	previous := oracleReadExact(t, oraclePreviousPath,
		vector.Initial.ActivePayload.Length, vector.Initial.ActivePayload.SHA256)
	manifest, err := os.ReadFile(filepath.Join(root, "generations", "0", "manifest.bin"))
	if err != nil {
		t.Fatal(err)
	}
	currentDigest := oracleFileSum(filepath.Join(root, "current"))
	prevSum := sha256.Sum256(previous)
	manifestSum := sha256.Sum256(manifest)
	body := append([]byte(nil), currentDigest[:]...)
	body = oracleAppendUint64(body, 0)
	body = oracleAppendUint64(body, uint64(len(previous)))
	body = oracleAppendDigest(t, body, prevSum[:])
	body = oracleAppendDigest(t, body, manifestSum[:])
	body = append(body, 0)
	body = oracleAppendDigest(t, body, prevSum[:])
	body = oracleAppendDigest(t, body, manifestSum[:])
	return root, sha256.Sum256(oracleEnvelope(t, 3, body))
}

// recoveryOracleBootstrapManifestDigest returns the V0 predecessor manifest SHA-256.
func recoveryOracleBootstrapManifestDigest(t *testing.T, root string) [32]byte {
	t.Helper()
	manifest, err := os.ReadFile(filepath.Join(root, "generations", "0", "manifest.bin"))
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(manifest)
}

// recoveryOracleAssertJournalPreserved asserts journal entries 1..lastState exist.
func recoveryOracleAssertJournalPreserved(t *testing.T, root string, generation uint64, lastState byte) {
	t.Helper()
	for state := byte(1); state <= lastState; state++ {
		path := filepath.Join(root, "transactions", strconv.FormatUint(generation, 10), "journal",
			recoveryOracleName(state))
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("expected journal %s to be preserved: %v", path, err)
		}
	}
}

// recoveryOracleAssertStagingAbsent asserts staging/<gen> contains no entries.
func recoveryOracleAssertStagingAbsent(t *testing.T, root string, generation uint64) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "staging", strconv.FormatUint(generation, 10)))
	if err == nil && len(entries) != 0 {
		t.Fatalf("staging/%d still has %d entries", generation, len(entries))
	}
}

// recoveryOracleAssertR00CleanupTree asserts the R00 cleanup allowlist:
// transactions/1/journal and transactions/1 are removed, and top-level
// transactions/ remains as an empty admitted child of the root.
func recoveryOracleAssertR00CleanupTree(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root, "transactions", "1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("R00 must remove transactions/1: lstat=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "transactions", "1", "journal")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("R00 must remove transactions/1/journal: lstat=%v", err)
	}
	txDir, err := os.Lstat(filepath.Join(root, "transactions"))
	if err != nil {
		t.Fatalf("R00 must keep transactions/ as an empty directory: lstat=%v", err)
	}
	if !txDir.IsDir() {
		t.Fatalf("R00 transactions/ must remain a directory: mode=%v", txDir.Mode())
	}
	entries, err := os.ReadDir(filepath.Join(root, "transactions"))
	if err != nil {
		t.Fatalf("R00 transactions/ must be readable: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("R00 transactions/ must be empty, has entries: %v", names)
	}
}

// recoveryOracleAssertStagingPresent asserts staging/<gen> has both artifact and manifest.
func recoveryOracleAssertStagingPresent(t *testing.T, root string, generation uint64) {
	t.Helper()
	directory := filepath.Join(root, "staging", strconv.FormatUint(generation, 10))
	for _, name := range []string{"artifact", "manifest.bin"} {
		if _, err := os.Lstat(filepath.Join(directory, name)); err != nil {
			t.Fatalf("expected %s to be present: %v", name, err)
		}
	}
}

// recoveryOracleAssertGenerationsAbsent asserts generations/<gen> does not exist.
func recoveryOracleAssertGenerationsAbsent(t *testing.T, root string, generation uint64) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root, "generations", strconv.FormatUint(generation, 10))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("generations/%d must be absent: %v", generation, err)
	}
}

// recoveryOracleAssertPredecessorCurrent asserts current selects generation 0 with no rollback.
func recoveryOracleAssertPredecessorCurrent(t *testing.T, root string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	if raw[8] != 2 {
		t.Fatalf("current is not a current record: kind=%d", raw[8])
	}
	body := raw[16:]
	if body[0] != 0 || body[1+8+8] != 0 {
		t.Fatalf("predecessor current must select generation 0 with no rollback: %x", body)
	}
}

// recoveryOracleAssertCurrentTempAbsent asserts a temporary current file is absent.
func recoveryOracleAssertCurrentTempAbsent(t *testing.T, root, name string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("current temp %s must be removed: %v", name, err)
	}
}

// recoveryOracleZero is a constant all-zero SHA-256 used for empty digests.
var recoveryOracleZero = [32]byte{}

// recoveryOracleNoDigest returns the all-zero digest for fixture-only unused values.
func recoveryOracleNoDigest() [32]byte {
	return recoveryOracleZero
}

// recoveryOracleAssertRecovered asserts the recovered Result for nonterminal rows.
// The success oracle requires err == nil; the Result fields, digests, and
// notices are independently encoded.
func recoveryOracleAssertRecovered(t *testing.T, result Result, err error, state string,
	generation uint64, currentDigest, rollbackDigest [32]byte, staging bool, custodyFromSuccessor bool) {
	t.Helper()
	if err != nil {
		t.Fatalf("RECOVER: recovered Result must have err == nil: err=%v", err)
	}
	if result.Outcome != "recovered" || result.State != state {
		t.Fatalf("RECOVER: outcome=%s state=%s, want recovered/%s", result.Outcome, result.State, state)
	}
	if result.Generation != generation {
		t.Fatalf("Recover generation=%d, want %d", result.Generation, generation)
	}
	if result.CurrentDigest != currentDigest {
		t.Fatalf("Recover current digest=%x, want %x", result.CurrentDigest, currentDigest)
	}
	if result.RollbackDigest != rollbackDigest {
		t.Fatalf("Recover rollback digest=%x, want %x", result.RollbackDigest, rollbackDigest)
	}
	if result.StagingPresent != staging {
		t.Fatalf("Recover staging=%v, want %v", result.StagingPresent, staging)
	}
	if result.SafeNotice != "update interrupted" {
		t.Fatalf("Recover safe notice=%q, want update interrupted", result.SafeNotice)
	}
	if result.CustodyNotice != recoveryOracleCustodyNotice {
		t.Fatalf("Recover custody=%q, want %q", result.CustodyNotice, recoveryOracleCustodyNotice)
	}
	_ = custodyFromSuccessor
}

// recoveryOracleAssertCommitted asserts the R14 committed Result.
// The success oracle requires err == nil.
func recoveryOracleAssertCommitted(t *testing.T, result Result, err error,
	currentDigest, rollbackDigest [32]byte, generation uint64) {
	t.Helper()
	if err != nil {
		t.Fatalf("RECOVER: committed Result must have err == nil: err=%v", err)
	}
	if result.Outcome != "committed" || result.State != "committed" {
		t.Fatalf("RECOVER: outcome=%s state=%s, want committed/committed", result.Outcome, result.State)
	}
	if result.Generation != generation {
		t.Fatalf("Recover R14 generation=%d, want %d", result.Generation, generation)
	}
	if result.CurrentDigest != currentDigest {
		t.Fatalf("Recover R14 current digest=%x, want %x", result.CurrentDigest, currentDigest)
	}
	if result.RollbackDigest != rollbackDigest {
		t.Fatalf("Recover R14 rollback digest=%x, want %x", result.RollbackDigest, rollbackDigest)
	}
	if result.StagingPresent {
		t.Fatal("Recover R14 staging must be absent")
	}
	if result.SafeNotice != "update committed" {
		t.Fatalf("Recover R14 safe notice=%q, want update committed", result.SafeNotice)
	}
	if result.CustodyNotice != recoveryOracleCustodyNotice {
		t.Fatalf("Recover R14 custody=%q, want %q", result.CustodyNotice, recoveryOracleCustodyNotice)
	}
}

// recoveryOracleDecodeHex decodes a 64-char hex SHA-256 into a fixed array.
func recoveryOracleDecodeHex(value string) [32]byte {
	var digest [32]byte
	for index := 0; index < 32; index++ {
		high := decodeHexByte(value[index*2])
		low := decodeHexByte(value[index*2+1])
		digest[index] = high<<4 | low
	}
	return digest
}

// decodeHexByte decodes one hex character to its byte value.
func decodeHexByte(character byte) byte {
	switch {
	case character >= '0' && character <= '9':
		return character - '0'
	case character >= 'a' && character <= 'f':
		return character - 'a' + 10
	default:
		return 0
	}
}
