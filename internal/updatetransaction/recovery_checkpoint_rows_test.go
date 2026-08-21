package updatetransaction

import (
	"os"
	"path/filepath"
	"testing"
)

// recoveryOracleRow is one R00-R14 fixture description.
type recoveryOracleRow struct {
	id               string
	setup            func(t *testing.T, root string, predecessor [32]byte) (artifact, manifest [32]byte)
	assert           func(t *testing.T, result Result, err error, root string, artifact, manifest [32]byte)
	lastJournalState byte
}

// recoveryOracleRows returns the ordered R00-R14 table.
func recoveryOracleRows() []recoveryOracleRow {
	previousStage := func(t *testing.T, root string, predecessor [32]byte) ([32]byte, [32]byte) {
		artifact, manifest := recoveryOracleStage(t, root, 1)
		return artifact, manifest
	}
	previousChain := func(lastState byte) func(t *testing.T, root string, predecessor [32]byte) ([32]byte, [32]byte) {
		return func(t *testing.T, root string, predecessor [32]byte) ([32]byte, [32]byte) {
			artifact, manifest := recoveryOracleStage(t, root, 1)
			recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, lastState)
			return artifact, manifest
		}
	}
	return []recoveryOracleRow{{
		id: "R00",
		setup: func(t *testing.T, root string, _ [32]byte) ([32]byte, [32]byte) {
			if err := os.MkdirAll(filepath.Join(root, "transactions", "1", "journal"), 0o700); err != nil {
				t.Fatal(err)
			}
			return recoveryOracleNoDigest(), recoveryOracleNoDigest()
		},
		assert: func(t *testing.T, result Result, err error, root string, _, _ [32]byte) {
			recoveryOracleAssertRecovered(t, result, err, "idle", 0, recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleZero, false, false)
			recoveryOracleAssertR00CleanupTree(t, root)
		},
		lastJournalState: 0,
	}, {
		id:    "R01",
		setup: recoveryOracleChainOnly(1),
		assert: func(t *testing.T, result Result, err error, root string, _, _ [32]byte) {
			recoveryOracleAssertRecovered(t, result, err, "release-accepted", 1, recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleZero, false, false)
			recoveryOracleAssertJournalPreserved(t, root, 1, 1)
			recoveryOracleAssertStagingAbsent(t, root, 1)
		},
		lastJournalState: 1,
	}, {
		id:    "R02",
		setup: recoveryOracleChainOnly(2),
		assert: func(t *testing.T, result Result, err error, root string, _, _ [32]byte) {
			recoveryOracleAssertRecovered(t, result, err, "artifact-verified", 1, recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleZero, false, false)
			recoveryOracleAssertJournalPreserved(t, root, 1, 2)
			recoveryOracleAssertStagingAbsent(t, root, 1)
		},
		lastJournalState: 2,
	}, {
		id: "R03",
		setup: func(t *testing.T, root string, predecessor [32]byte) ([32]byte, [32]byte) {
			artifact, manifest := recoveryOracleStage(t, root, 1)
			recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 2)
			return artifact, manifest
		},
		assert: func(t *testing.T, result Result, err error, root string, _, _ [32]byte) {
			recoveryOracleAssertRecovered(t, result, err, "artifact-verified", 1, recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleZero, false, false)
			recoveryOracleAssertJournalPreserved(t, root, 1, 2)
			recoveryOracleAssertStagingAbsent(t, root, 1)
		},
		lastJournalState: 2,
	}, {
		id:    "R04",
		setup: previousChain(3),
		assert: func(t *testing.T, result Result, err error, root string, _, _ [32]byte) {
			recoveryOracleAssertRecovered(t, result, err, "staged", 1, recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleZero, true, false)
			recoveryOracleAssertJournalPreserved(t, root, 1, 3)
			recoveryOracleAssertStagingPresent(t, root, 1)
		},
		lastJournalState: 3,
	}, {
		id:    "R05",
		setup: previousChain(4),
		assert: func(t *testing.T, result Result, err error, root string, _, _ [32]byte) {
			recoveryOracleAssertRecovered(t, result, err, "rollback-reserved", 1, recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleZero, true, false)
			recoveryOracleAssertJournalPreserved(t, root, 1, 4)
			recoveryOracleAssertStagingPresent(t, root, 1)
		},
		lastJournalState: 4,
	}, {
		id:    "R06",
		setup: previousChain(5),
		assert: func(t *testing.T, result Result, err error, root string, _, _ [32]byte) {
			recoveryOracleAssertRecovered(t, result, err, "stop-new-work", 1, recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleZero, true, false)
			recoveryOracleAssertJournalPreserved(t, root, 1, 5)
			recoveryOracleAssertStagingPresent(t, root, 1)
		},
		lastJournalState: 5,
	}, {
		id:    "R07",
		setup: previousChain(6),
		assert: func(t *testing.T, result Result, err error, root string, _, _ [32]byte) {
			recoveryOracleAssertRecovered(t, result, err, "draining", 1, recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleZero, true, false)
			recoveryOracleAssertJournalPreserved(t, root, 1, 6)
			recoveryOracleAssertStagingPresent(t, root, 1)
		},
		lastJournalState: 6,
	}, {
		id: "R08",
		setup: func(t *testing.T, root string, predecessor [32]byte) ([32]byte, [32]byte) {
			artifact, manifest := previousStage(t, root, predecessor)
			recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 6)
			recoveryOraclePublish(t, root, 1)
			return artifact, manifest
		},
		assert: func(t *testing.T, result Result, err error, root string, _, _ [32]byte) {
			recoveryOracleAssertRecovered(t, result, err, "draining", 1, recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleZero, true, false)
			recoveryOracleAssertJournalPreserved(t, root, 1, 6)
			recoveryOracleAssertStagingPresent(t, root, 1)
			recoveryOracleAssertGenerationsAbsent(t, root, 1)
		},
		lastJournalState: 6,
	}, {
		id: "R09",
		setup: func(t *testing.T, root string, predecessor [32]byte) ([32]byte, [32]byte) {
			artifact, manifest := previousStage(t, root, predecessor)
			recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 6)
			recoveryOraclePublish(t, root, 1)
			recoveryOracleWriteCurrentTemp(t, root, ".current.abcdef0123456789.tmp", artifact, manifest,
				recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleBootstrapManifestDigest(t, root),
				recoveryOracleCandidateLength(), recoveryOraclePreviousLength)
			return artifact, manifest
		},
		assert: func(t *testing.T, result Result, err error, root string, _, _ [32]byte) {
			recoveryOracleAssertRecovered(t, result, err, "draining", 1, recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleZero, true, false)
			recoveryOracleAssertJournalPreserved(t, root, 1, 6)
			recoveryOracleAssertStagingPresent(t, root, 1)
			recoveryOracleAssertGenerationsAbsent(t, root, 1)
			recoveryOracleAssertCurrentTempAbsent(t, root, ".current.abcdef0123456789.tmp")
		},
	}, {
		id: "R10",
		setup: func(t *testing.T, root string, predecessor [32]byte) ([32]byte, [32]byte) {
			artifact, manifest := previousStage(t, root, predecessor)
			recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 6)
			recoveryOraclePublish(t, root, 1)
			previousArtifact := recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex)
			previousManifest := recoveryOracleBootstrapManifestDigest(t, root)
			length := recoveryOracleCandidateLength()
			recoveryOracleSuccessorCurrent(t, root, 1, artifact, manifest, previousArtifact, previousManifest, length, recoveryOraclePreviousLength)
			return artifact, manifest
		},
		assert: func(t *testing.T, result Result, err error, root string, _, _ [32]byte) {
			recoveryOracleAssertRecovered(t, result, err, "draining", 1, recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleZero, true, false)
			recoveryOracleAssertJournalPreserved(t, root, 1, 6)
			recoveryOracleAssertStagingPresent(t, root, 1)
			recoveryOracleAssertGenerationsAbsent(t, root, 1)
			recoveryOracleAssertPredecessorCurrent(t, root)
		},
		lastJournalState: 6,
	}, {
		id: "R11",
		setup: func(t *testing.T, root string, predecessor [32]byte) ([32]byte, [32]byte) {
			artifact, manifest := previousStage(t, root, predecessor)
			recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 6)
			recoveryOraclePublish(t, root, 1)
			previousArtifact := recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex)
			previousManifest := recoveryOracleBootstrapManifestDigest(t, root)
			length := recoveryOracleCandidateLength()
			recoveryOracleSuccessorCurrent(t, root, 1, artifact, manifest, previousArtifact, previousManifest, length, recoveryOraclePreviousLength)
			return artifact, manifest
		},
		assert: func(t *testing.T, result Result, err error, root string, _, _ [32]byte) {
			recoveryOracleAssertRecovered(t, result, err, "draining", 1, recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), recoveryOracleZero, true, false)
			recoveryOracleAssertJournalPreserved(t, root, 1, 6)
			recoveryOracleAssertStagingPresent(t, root, 1)
			recoveryOracleAssertGenerationsAbsent(t, root, 1)
			recoveryOracleAssertPredecessorCurrent(t, root)
		},
		lastJournalState: 6,
	}, {
		id: "R12",
		setup: func(t *testing.T, root string, predecessor [32]byte) ([32]byte, [32]byte) {
			artifact, manifest := previousStage(t, root, predecessor)
			recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 7)
			recoveryOraclePublish(t, root, 1)
			previousArtifact := recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex)
			previousManifest := recoveryOracleBootstrapManifestDigest(t, root)
			length := recoveryOracleCandidateLength()
			recoveryOracleSuccessorCurrent(t, root, 1, artifact, manifest, previousArtifact, previousManifest, length, recoveryOraclePreviousLength)
			return artifact, manifest
		},
		assert: func(t *testing.T, result Result, err error, root string, _, _ [32]byte) {
			recoveryOracleAssertRecovered(t, result, err, "activated", 1, recoveryOracleDecodeHex(recoveryOracleCandidateDigestHex), recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), false, true)
			recoveryOracleAssertJournalPreserved(t, root, 1, 7)
			recoveryOracleAssertStagingAbsent(t, root, 1)
		},
		lastJournalState: 7,
	}, {
		id: "R13",
		setup: func(t *testing.T, root string, predecessor [32]byte) ([32]byte, [32]byte) {
			artifact, manifest := previousStage(t, root, predecessor)
			recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 8)
			recoveryOraclePublish(t, root, 1)
			previousArtifact := recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex)
			previousManifest := recoveryOracleBootstrapManifestDigest(t, root)
			length := recoveryOracleCandidateLength()
			recoveryOracleSuccessorCurrent(t, root, 1, artifact, manifest, previousArtifact, previousManifest, length, recoveryOraclePreviousLength)
			return artifact, manifest
		},
		assert: func(t *testing.T, result Result, err error, root string, _, _ [32]byte) {
			recoveryOracleAssertRecovered(t, result, err, "self-testing", 1, recoveryOracleDecodeHex(recoveryOracleCandidateDigestHex), recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), false, true)
			recoveryOracleAssertJournalPreserved(t, root, 1, 8)
			recoveryOracleAssertStagingAbsent(t, root, 1)
		},
		lastJournalState: 8,
	}, {
		id: "R14",
		setup: func(t *testing.T, root string, predecessor [32]byte) ([32]byte, [32]byte) {
			artifact, manifest := previousStage(t, root, predecessor)
			recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 9)
			recoveryOraclePublish(t, root, 1)
			previousArtifact := recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex)
			previousManifest := recoveryOracleBootstrapManifestDigest(t, root)
			length := recoveryOracleCandidateLength()
			recoveryOracleSuccessorCurrent(t, root, 1, artifact, manifest, previousArtifact, previousManifest, length, recoveryOraclePreviousLength)
			return artifact, manifest
		},
		assert: func(t *testing.T, result Result, err error, root string, _, _ [32]byte) {
			recoveryOracleAssertCommitted(t, result, err, recoveryOracleDecodeHex(recoveryOracleCandidateDigestHex), recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex), 1)
			recoveryOracleAssertJournalPreserved(t, root, 1, 9)
			recoveryOracleAssertStagingAbsent(t, root, 1)
		},
		lastJournalState: 9,
	}}
}

// recoveryOraclePreviousLength is the frozen V0 predecessor payload length.
const recoveryOraclePreviousLength = 32
