package updatetransaction

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoverRejectsUnknownRootDirectory(t *testing.T) {
	root, _ := recoveryOracleBootstrap(t)
	if err := os.Mkdir(filepath.Join(root, "unknown-directory"), 0o700); err != nil {
		t.Fatal(err)
	}
	requireInvalidRecovery(t, root)
}

func TestRecoveryPredecessorOracle(t *testing.T) {
	root, _ := recoveryOracleBootstrap(t)
	facts, err := collectInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	records, err := reconstructRecoveryRecords(facts)
	if err != nil {
		t.Fatal(err)
	}
	predecessor := generationByID(facts.Generations, 0)
	if predecessor == nil {
		t.Fatal("missing bootstrap generation")
	}
	artifact := sha256.Sum256(predecessor.Artifact.Bytes)
	manifest := sha256.Sum256(predecessor.Manifest.Bytes)
	want := recoveryOraclePredecessorEnvelope(t, sha256.Sum256(facts.Current.Bytes),
		artifact, manifest, artifact, manifest)
	if records.predecessorCommitment != want {
		t.Fatalf("predecessor commitment=%x, want independent oracle %x", records.predecessorCommitment, want)
	}
	if len(records.predecessorCommitment) != recoveryOraclePredecessorEnvelopeLen {
		t.Fatalf("predecessor commitment length=%d, want %d", len(records.predecessorCommitment), recoveryOraclePredecessorEnvelopeLen)
	}
}

func TestRecoverRejectsTerminalSemantics(t *testing.T) {
	for _, row := range []struct {
		name   string
		offset int
		value  byte
	}{{"adapter", 121, 1}, {"deadline", 131, 0xff}} {
		row := row
		t.Run(row.name, func(t *testing.T) {
			root, _, _, _ := recoveryOracleCorruptBootstrap(t)
			mutateJournalField(t, root, 9, row.offset, 0, row.value)
			requireInvalidRecovery(t, root)
		})
	}
}

func TestRecoverBindsJournalToObservedCandidate(t *testing.T) {
	root, predecessor := recoveryOracleBootstrap(t)
	recoveryOracleStage(t, root, 1)
	wrongArtifact := sha256.Sum256([]byte("journal-selected-artifact"))
	wrongManifest := sha256.Sum256([]byte("journal-selected-manifest"))
	recoveryOracleWriteChain(t, root, 1, predecessor, wrongArtifact, wrongManifest, 3)
	requireInvalidRecovery(t, root)
}

func TestRecoverRejectsPublishedCandidateBeforeDraining(t *testing.T) {
	root, predecessor := recoveryOracleBootstrap(t)
	artifact, manifest := recoveryOracleStage(t, root, 1)
	recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 3)
	recoveryOraclePublish(t, root, 1)
	requireInvalidRecovery(t, root)
}

func TestRecoverRejectsMultipleCurrentTemps(t *testing.T) {
	root, predecessor := recoveryOracleBootstrap(t)
	artifact, manifest := recoveryOracleStage(t, root, 1)
	recoveryOracleWriteChain(t, root, 1, predecessor, artifact, manifest, 6)
	recoveryOraclePublish(t, root, 1)
	previousArtifact := recoveryOracleDecodeHex(recoveryOraclePreviousDigestHex)
	previousManifest := recoveryOracleBootstrapManifestDigest(t, root)
	for _, name := range []string{".current.0123456789abcdef.tmp", ".current.abcdef0123456789.tmp"} {
		recoveryOracleWriteCurrentTemp(t, root, name, artifact, manifest,
			previousArtifact, previousManifest, recoveryOracleCandidateLength(), recoveryOraclePreviousLength)
	}
	requireInvalidRecovery(t, root)
}

func TestRecoverAcceptsArtifactAboveOneMiB(t *testing.T) {
	root, predecessor := recoveryOracleBootstrap(t)
	vector := oracleLoadV0(t)
	artifact := make([]byte, 2<<20)
	for index := range artifact {
		artifact[index] = byte(index)
	}
	manifest := oracleManifest(t, v0OracleManifest{
		Generation: 1, TargetPath: vector.Candidate.Path,
		Platform: "windows-amd64", Architecture: "amd64", Environment: "h3-test", Network: "ardents-h3-test-1",
		ReleaseIdentity: vector.Candidate.ReleaseIdentity, ReleaseVersion: uint64(vector.Candidate.ReleaseVersion),
		SourceRevision: "rev-0001", BuildInputCommitment: "inputs-0001", BuildIdentity: "build-0001",
		DependencyIdentity: "deps-0001", SBOMIdentity: "sbom-0001", AttestationPolicy: "two-builder",
		Qualification: "qualified", BuildState: "current", ProtocolPhase: "required",
		BuildSafety: "release-accepted", Protocol: "release-accepted", ReferenceTime: "2030-01-02T03:04:05Z",
		BuildSafetyNoNewWorkAfter: "2030-02-01T03:04:05Z", BuildSafetyTerminateAfter: "2030-07-01T03:04:05Z",
		SchemaPlan: "no-op-v1", SafeNotice: "update committed",
		CustodyNotice: vector.Expected.CommandResult.CustodyNotice, ReleaseFloors: vector.Expected.ReleaseFloors,
	}, v0OracleStoredAuthorization{Classification: "release-accepted", Platform: "windows-amd64", Architecture: "amd64",
		Environment: "h3-test", Network: "ardents-h3-test-1", SchemaCompatible: true, AboveLocalFloors: true}, artifact)
	staging := filepath.Join(root, "staging", "1")
	if err := os.Mkdir(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "artifact"), artifact, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "manifest.bin"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	recoveryOracleWriteChain(t, root, 1, predecessor, sha256.Sum256(artifact), sha256.Sum256(manifest), 3)
	result, err := Recover(context.Background(), root)
	if err != nil || result.Outcome != outcomeRecovered || result.State != "staged" {
		t.Fatalf("Recover rejected bounded artifact: result=%+v err=%v", result, err)
	}
}

func requireInvalidRecovery(t *testing.T, root string) {
	t.Helper()
	result, err := Recover(context.Background(), root)
	if err == nil || result.Outcome != outcomeTransactionInvalid {
		t.Fatalf("Recover accepted invalid physical evidence: result=%+v err=%v", result, err)
	}
}
