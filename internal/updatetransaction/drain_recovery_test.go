package updatetransaction

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDrainRecovery(t *testing.T) {
	tests := []struct {
		name      string
		lastState byte
		wantState string
	}{
		{name: "stop", lastState: byte(stateStopNewWork), wantState: "rollback-reserved"},
		{name: "drain", lastState: byte(stateDraining), wantState: "stop-new-work"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, predecessor := recoveryOracleBootstrap(t)
			artifact, manifest := recoveryOracleStage(t, root, 1)
			recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, test.lastState)
			// The adapter-result byte is body offset 105 in the fixed v1
			// record, after the sixteen-byte envelope header.
			recoveryOracleMutateJournal(t, root, int(test.lastState), 16+105, 0, byte(adapterFailed))
			result, err := Recover(context.Background(), root)
			if err != nil || result.Outcome != "drain-expired" || result.State != test.wantState ||
				result.Generation != 1 || result.CurrentDigest != recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex) ||
				result.RollbackDigest != [32]byte{} || result.StagingPresent || result.SafeNotice != "update drain expired" ||
				result.CustodyNotice != recoveryOracleCustodyNotice {
				t.Fatalf("Recover = %+v, %v", result, err)
			}
			for _, path := range []string{filepath.Join(root, "staging", "1"), filepath.Join(root, "transactions", "1")} {
				if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("adapter failure residue %s: %v", path, statErr)
				}
			}
		})
	}
}
