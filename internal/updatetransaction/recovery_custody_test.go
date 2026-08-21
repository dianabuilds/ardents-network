package updatetransaction

import (
	"context"
	"testing"
)

// TestRecoverR10UsesRestoredPredecessorCustody proves that R10 reports the
// custody notice of the predecessor that it restores, not of the successor
// that was selected when recovery started.
func TestRecoverR10UsesRestoredPredecessorCustody(t *testing.T) {
	const successorCustody = "candidate custody must not survive restored predecessor selection"

	root, predecessor := recoveryOracleBootstrap(t)
	artifact, manifest := recoveryOracleStageWithCustody(t, root, 1, successorCustody)
	recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, byte(stateDraining))
	recoveryOraclePublish(t, root, 1)
	recoveryOracleSuccessorCurrent(t, root, 1, artifact, manifest,
		recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleBootstrapManifestDigest(t, root),
		recoveryOracleCandidateLength(), recoveryOraclePreviousLength)

	result, err := Recover(context.Background(), root)
	if err != nil {
		t.Fatalf("Recover R10: %v", err)
	}
	if result.CustodyNotice != recoveryOracleCustodyNotice {
		t.Fatalf("Recover R10 custody=%q, want restored predecessor custody %q", result.CustodyNotice, recoveryOracleCustodyNotice)
	}
}
