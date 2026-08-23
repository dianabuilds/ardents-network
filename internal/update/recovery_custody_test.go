package update

import (
	"context"
	"testing"
)

func TestRecoverR10RestoresPredecessorSelection(t *testing.T) {
	root, predecessor := recoveryOracleBootstrap(t)
	artifact, manifest := recoveryOracleStage(t, root, 1)
	recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, byte(stateDraining))
	recoveryOraclePublish(t, root, 1)
	recoveryOracleSuccessorCurrent(t, root, 1, artifact, manifest,
		recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleBootstrapManifestDigest(t, root),
		recoveryOracleCandidateLength(), recoveryOraclePreviousLength)

	result, err := Recover(context.Background(), root)
	if err != nil {
		t.Fatalf("Recover R10: %v", err)
	}
	if result.Outcome != "recovered" || result.State != "draining" || result.CurrentDigest != recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex) {
		t.Fatalf("Recover R10 = %+v", result)
	}
}
