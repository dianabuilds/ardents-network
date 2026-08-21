package updatetransaction

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestRecoverRejectsJournalFieldMutations exercises every independent bounded
// journal field and confirms each independent corruption returns
// transaction-invalid with no Adapter invoked and no other mutation. The
// tree snapshot is taken AFTER the mutation so the preservation check
// compares the recovered state to the mutated state, not to the pristine
// R14 bootstrap.
func TestRecoverRejectsJournalFieldMutations(t *testing.T) {
	root, predecessor, artifact, manifest := recoveryOracleCorruptBootstrap(t)
	for _, mutation := range recoveryOracleFieldMutations() {
		mutation := mutation
		t.Run(mutation.id, func(t *testing.T) {
			workRoot := recoveryOracleCorruptClone(t, root)
			defer os.RemoveAll(workRoot)
			mutation.apply(t, workRoot, predecessor, artifact, manifest)
			snapshot := recoveryOracleTreeDigest(t, workRoot)
			result, err := Recover(context.Background(), workRoot)
			recoveryOracleAssertInvalid(t, result, err)
			if after := recoveryOracleTreeDigest(t, workRoot); !bytesEqualField(after, snapshot) {
				t.Fatalf("RECOVER: tree mutated by Recover: before=%x after=%x", snapshot, after)
			}
		})
	}
}

// recoveryOracleFieldMutation is one journal-field corruption.
type recoveryOracleFieldMutation struct {
	id    string
	apply func(t *testing.T, root string, predecessor, artifact, manifest [32]byte)
}

// recoveryOracleFieldMutations returns every bounded journal field mutation.
func recoveryOracleFieldMutations() []recoveryOracleFieldMutation {
	return []recoveryOracleFieldMutation{
		{id: "envelope-magic", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			mutateJournalField(t, root, 1, 0, 0, 'X')
		}},
		{id: "envelope-kind", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			mutateJournalField(t, root, 1, 8, 0, 1)
		}},
		{id: "envelope-version", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			mutateJournalField(t, root, 1, 9, 0, 2)
		}},
		{id: "envelope-flags", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			mutateJournalField(t, root, 1, 10, 0, 0xff)
		}},
		{id: "envelope-body-length", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			mutateJournalField(t, root, 1, 12, 0, 0xff)
		}},
		{id: "state-code", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			mutateJournalField(t, root, 2, 16, 0, 0)
		}},
		{id: "generation", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			mutateJournalField(t, root, 2, 17, 0, 0xff)
		}},
		{id: "predecessor-commitment", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			mutateJournalField(t, root, 2, 25, 0, 0xff)
		}},
		{id: "artifact-commitment", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			mutateJournalField(t, root, 2, 57, 0, 0xff)
		}},
		{id: "manifest-commitment", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			mutateJournalField(t, root, 2, 89, 0, 0xff)
		}},
		{id: "adapter-result", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			mutateJournalField(t, root, 2, 121, 0, 0xff)
		}},
		{id: "observation", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			mutateJournalField(t, root, 2, 122, 0, 0)
		}},
		{id: "monotonic-elapsed", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			mutateJournalField(t, root, 3, 123, 0, 0xff)
		}},
		{id: "effective-deadline", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			mutateJournalField(t, root, 3, 131, 0, 0xff)
		}},
		{id: "trailing-byte", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			path := filepath.Join(root, "transactions", "1", "journal", "01-release-accepted.entry")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("FIXTURE: read journal entry: %v", err)
			}
			data = append(data, 0xff)
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatalf("FIXTURE: append trailing byte: %v", err)
			}
		}},
		{id: "non-canonical-encoding", apply: func(t *testing.T, root string, _, _, _ [32]byte) {
			path := filepath.Join(root, "transactions", "1", "journal", "02-artifact-verified.entry")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("FIXTURE: read entry 02: %v", err)
			}
			data[25], data[26] = data[26], data[25]
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatalf("FIXTURE: swap bytes: %v", err)
			}
		}},
	}
}

// mutateJournalField is a local alias so this file does not depend on
// helpers outside its own scope.
// mutateJournalField is a local alias so this file does not depend on
// helpers outside its own scope.
// recoveryOracleAssertInvalid lives in recovery_fixture_test.go; both
// corruption and lock tests reuse it.
func mutateJournalField(t *testing.T, root string, state int, offset int, _, value byte) {
	t.Helper()
	path := filepath.Join(root, "transactions", "1", "journal", recoveryOracleName(byte(state)))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("FIXTURE: read journal entry: %v", err)
	}
	if offset >= len(data) {
		t.Fatalf("FIXTURE: offset %d exceeds entry length %d", offset, len(data))
	}
	if value == 0 {
		data[offset] ^= 0xff
	} else {
		data[offset] = value
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("FIXTURE: write journal entry: %v", err)
	}
}

// bytesEqualField is a local alias so this file does not import bytes.
func bytesEqualField(a, b []byte) bool {
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
