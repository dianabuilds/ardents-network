package updatetransaction

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestRecoverRemovesOnlyDeclaredTemporaryStaging proves the S7.2-03 recovery
// extension through public Recover. The fixture writes literal canonical
// journal and payload bytes, then changes only the final directory name to
// the declared pre-publication temporary form.
func TestRecoverRemovesOnlyDeclaredTemporaryStaging(t *testing.T) {
	root, predecessor := recoveryOracleBootstrap(t)
	artifact, manifest := recoveryOracleStage(t, root, 1)
	if err := os.Rename(filepath.Join(root, "staging", "1"), filepath.Join(root, "staging", "1.tmp")); err != nil {
		t.Fatalf("FIXTURE: rename staging to temporary: %v", err)
	}
	recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, byte(stateArtifactVerified))

	result, err := Recover(context.Background(), root)
	if err != nil {
		t.Fatalf("Recover temporary staging: %v", err)
	}
	if result.Outcome != "recovered" || result.State != "artifact-verified" || result.Generation != 1 ||
		result.CurrentDigest != recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex) || result.RollbackDigest != [32]byte{} ||
		result.StagingPresent || result.SafeNotice != "update interrupted" || result.CustodyNotice != recoveryOracleCustodyNotice {
		t.Fatalf("Recover temporary staging result = %+v", result)
	}
	for _, path := range []string{filepath.Join(root, "staging", "1.tmp"), filepath.Join(root, "staging", "1")} {
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Recover temporary staging retained %s: %v", path, statErr)
		}
	}
}

// TestRecoverRemovesEmptyDeclaredTemporaryStaging covers the crash boundary
// immediately after temporary-directory creation, before either payload file
// exists. The journal remains the binding evidence; recovery removes only the
// declared incomplete tree.
func TestRecoverRemovesEmptyDeclaredTemporaryStaging(t *testing.T) {
	root, predecessor := recoveryOracleBootstrap(t)
	_, _, artifactDigest, manifestDigest := recoveryOracleCandidateManifest(t)
	if err := os.Mkdir(filepath.Join(root, "staging", "1.tmp"), 0o700); err != nil {
		t.Fatalf("FIXTURE: create temporary staging: %v", err)
	}
	recoveryOracleWriteChain(t, root, 1, predecessor, artifactDigest, manifestDigest, byte(stateArtifactVerified))

	result, err := Recover(context.Background(), root)
	if err != nil || result.Outcome != "recovered" || result.State != "artifact-verified" || result.Generation != 1 ||
		result.StagingPresent || result.SafeNotice != "update interrupted" {
		t.Fatalf("Recover empty temporary staging result=%+v err=%v", result, err)
	}
	if _, statErr := os.Lstat(filepath.Join(root, "staging", "1.tmp")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("Recover empty temporary staging retained tree: %v", statErr)
	}
}

// TestRecoverRejectsTemporaryAndFinalStaging proves that recovery does not
// choose between two candidate trees when a rename boundary is ambiguous.
func TestRecoverRejectsTemporaryAndFinalStaging(t *testing.T) {
	root, predecessor := recoveryOracleBootstrap(t)
	artifact, manifest := recoveryOracleStage(t, root, 1)
	recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, byte(stateArtifactVerified))
	if err := os.Mkdir(filepath.Join(root, "staging", "1.tmp"), 0o700); err != nil {
		t.Fatalf("FIXTURE: create competing temporary staging: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "staging", "1.tmp", "artifact"), oracleReadExact(t, oracleCandidatePath, recoveryOracleCandidateLength(), recoveryOracleCandidateDigestHex), 0o600); err != nil {
		t.Fatalf("FIXTURE: temporary artifact: %v", err)
	}
	if data, readErr := os.ReadFile(filepath.Join(root, "staging", "1", "manifest.bin")); readErr != nil {
		t.Fatalf("FIXTURE: read manifest: %v", readErr)
	} else if err := os.WriteFile(filepath.Join(root, "staging", "1.tmp", "manifest.bin"), data, 0o600); err != nil {
		t.Fatalf("FIXTURE: temporary manifest: %v", err)
	}

	result, err := Recover(context.Background(), root)
	recoveryOracleAssertInvalid(t, result, err)
	for _, path := range []string{filepath.Join(root, "staging", "1.tmp"), filepath.Join(root, "staging", "1"), filepath.Join(root, "transactions", "1")} {
		if _, statErr := os.Lstat(path); statErr != nil {
			t.Fatalf("Recover ambiguous staging mutated %s: %v", path, statErr)
		}
	}
}
