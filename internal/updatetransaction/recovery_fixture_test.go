package updatetransaction

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// recoveryOracleNoNewWork is the V0 build-no-new-work time.
var recoveryOracleNoNewWork = time.Date(2030, 2, 1, 3, 4, 5, 0, time.UTC)

// recoveryOracleTerminate is the V0 build-terminate time.
var recoveryOracleTerminate = time.Date(2030, 7, 1, 3, 4, 5, 0, time.UTC)

// recoveryOracleName returns the canonical journal file name for a state code.
func recoveryOracleName(state byte) string {
	names := [...]string{"", "01-release-accepted.entry", "02-artifact-verified.entry",
		"03-staged.entry", "04-rollback-reserved.entry", "05-stop-new-work.entry",
		"06-draining.entry", "07-activated.entry", "08-self-testing.entry",
		"09-committed.entry"}
	if int(state) >= len(names) || state == 0 {
		return ""
	}
	return names[state]
}

// recoveryOracleBody builds one canonical journal body for the given state.
func recoveryOracleBody(state byte, generation uint64, predecessor, artifact, manifest [32]byte, elapsed uint64) []byte {
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

// recoveryOracleWriteChain writes a contiguous journal chain from state 1
// through lastState into transactions/<generation>/journal/. The first entry's
// predecessor commitment is firstPredecessor; subsequent entries chain via
// SHA-256 of the previous entry's exact canonical bytes.
func recoveryOracleWriteChain(t *testing.T, root string, generation uint64,
	firstPredecessor, artifact, manifest [32]byte, lastState byte) {
	t.Helper()
	journal := filepath.Join(root, "transactions", strconv.FormatUint(generation, 10), "journal")
	if err := os.MkdirAll(journal, 0o700); err != nil {
		t.Fatal(err)
	}
	predecessor := firstPredecessor
	var elapsed uint64
	for state := byte(1); state <= lastState; state++ {
		body := recoveryOracleBody(state, generation, predecessor, artifact, manifest, elapsed)
		raw := oracleEnvelope(t, 4, body)
		if err := os.WriteFile(filepath.Join(journal, recoveryOracleName(state)), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		predecessor = sha256.Sum256(raw)
		elapsed += 1000
	}
}

// recoveryOracleCandidateManifest builds the V0 candidate manifest in memory
// without leaving any staging files behind. It returns the artifact bytes,
// manifest bytes, and SHA-256 digests that every transaction entry must
// bind. R01 and R02 use this directly; R03 stages the bytes returned here.
func recoveryOracleCandidateManifest(t *testing.T) (artifact []byte, manifest []byte, artifactDigest, manifestDigest [32]byte) {
	return recoveryOracleCandidateManifestWithCustody(t, recoveryOracleCustodyNotice)
}

// recoveryOracleCandidateManifestWithCustody independently builds a valid
// candidate manifest with the requested notice. It lets recovery rows prove
// that their public custody notice follows the selected manifest rather than
// merely the manifest selected when recovery began.
func recoveryOracleCandidateManifestWithCustody(t *testing.T, custodyNotice string) (artifact []byte, manifest []byte, artifactDigest, manifestDigest [32]byte) {
	t.Helper()
	vector := oracleLoadV0(t)
	artifact = oracleReadExact(t, oracleCandidatePath,
		vector.Candidate.Length, vector.Candidate.SHA256)
	manifest = oracleManifest(t, v0OracleManifest{
		Generation: 1, TargetPath: vector.Candidate.Path,
		Platform: "windows-amd64", Architecture: "amd64",
		Environment: "h3-test", Network: "ardents-h3-test-1",
		ReleaseIdentity: vector.Candidate.ReleaseIdentity,
		ReleaseVersion:  uint64(vector.Candidate.ReleaseVersion),
		SourceRevision:  "rev-0001", BuildInputCommitment: "inputs-0001",
		BuildIdentity: "build-0001", DependencyIdentity: "deps-0001",
		SBOMIdentity: "sbom-0001", AttestationPolicy: "two-builder",
		Qualification: "qualified", BuildState: "current", ProtocolPhase: "required",
		BuildSafety: "release-accepted", Protocol: "release-accepted",
		ReferenceTime:              "2030-01-02T03:04:05Z",
		BuildSafetyNoNewWorkAfter:  "2030-02-01T03:04:05Z",
		BuildSafetyTerminateAfter:  "2030-07-01T03:04:05Z",
		ProtocolTransitionDeadline: nil, SchemaPlan: "no-op-v1",
		SafeNotice:    "update committed",
		CustodyNotice: custodyNotice,
		ReleaseFloors: vector.Expected.ReleaseFloors,
	}, v0OracleStoredAuthorization{
		Classification: "release-accepted",
		Platform:       "windows-amd64", Architecture: "amd64",
		Environment: "h3-test", Network: "ardents-h3-test-1",
		SchemaCompatible: true, AboveLocalFloors: true,
	}, artifact)
	artifactDigest = sha256.Sum256(artifact)
	manifestDigest = sha256.Sum256(manifest)
	return artifact, manifest, artifactDigest, manifestDigest
}

// recoveryOracleStage writes the V0 candidate bytes returned by
// recoveryOracleCandidateManifest into staging/<generation>/. R03..R14 call
// this after preparing the in-memory candidate facts.
func recoveryOracleStage(t *testing.T, root string, generation uint64) (artifactDigest, manifestDigest [32]byte) {
	return recoveryOracleStageWithCustody(t, root, generation, recoveryOracleCustodyNotice)
}

// recoveryOracleStageWithCustody writes a valid candidate payload whose
// manifest has a distinct custody notice for a public Recover assertion.
func recoveryOracleStageWithCustody(t *testing.T, root string, generation uint64, custodyNotice string) (artifactDigest, manifestDigest [32]byte) {
	t.Helper()
	artifact, manifest, artifactDigest, manifestDigest := recoveryOracleCandidateManifestWithCustody(t, custodyNotice)
	directory := filepath.Join(root, "staging", strconv.FormatUint(generation, 10))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "artifact"), artifact, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "manifest.bin"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	return artifactDigest, manifestDigest
}

// recoveryOraclePublish moves staging/<generation> into generations/<generation>
// to simulate the post-publication physical checkpoint.
func recoveryOraclePublish(t *testing.T, root string, generation uint64) {
	t.Helper()
	staging := filepath.Join(root, "staging", strconv.FormatUint(generation, 10))
	generations := filepath.Join(root, "generations", strconv.FormatUint(generation, 10))
	if err := os.Rename(staging, generations); err != nil {
		t.Fatal(err)
	}
}

// recoveryOracleSuccessorCurrent writes a successor current with the predecessor
// as rollback, simulating the post-atomic-replacement physical state.
func recoveryOracleSuccessorCurrent(t *testing.T, root string, generation uint64,
	artifact, manifest, rollbackArtifact, rollbackManifest [32]byte,
	length, rollbackLength uint64) {
	t.Helper()
	successor := oracleCurrentTuple{
		Generation: generation, Length: length,
		Artifact: artifact, Manifest: manifest,
	}
	rollback := oracleCurrentTuple{
		Generation: 0, Length: rollbackLength,
		Artifact: rollbackArtifact, Manifest: rollbackManifest,
	}
	raw := oracleCurrent(t, generation, successor, &rollback)
	if err := os.WriteFile(filepath.Join(root, "current"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

// recoveryOraclePredecessorEnvelope computes the SHA-256 of the canonical
// predecessor-inspection envelope for the V0 bootstrap. The current has no
// rollback; currentRecordDigest must equal SHA-256 of the predecessor current
// bytes; artifact and manifest are the observed bytes.
func recoveryOraclePredecessorEnvelope(t *testing.T, currentRecordDigest,
	artifact, manifest, observationArtifact, observationManifest [32]byte) [32]byte {
	t.Helper()
	body := append([]byte(nil), currentRecordDigest[:]...)
	body = oracleAppendUint64(body, 0)
	body = oracleAppendUint64(body, uint64(len(observationArtifact)))
	body = oracleAppendDigest(t, body, artifact[:])
	body = oracleAppendDigest(t, body, manifest[:])
	body = append(body, 0)
	body = oracleAppendDigest(t, body, observationArtifact[:])
	body = oracleAppendDigest(t, body, observationManifest[:])
	return sha256.Sum256(oracleEnvelope(t, 3, body))
}

// recoveryOraclePredecessorEnvelopeLen is the SHA-256 length used by the
// predecessor envelope builder for the bootstrap generation.
const recoveryOraclePredecessorEnvelopeLen = 32

// recoveryOracleWriteCurrentTemp writes a successor current record to a
// temporary path under the owned root, simulating the post-atomic-replacement
// state before durability acknowledgement.
func recoveryOracleWriteCurrentTemp(t *testing.T, root, name string,
	artifact, manifest, rollbackArtifact, rollbackManifest [32]byte,
	length, rollbackLength uint64) {
	t.Helper()
	successor := oracleCurrentTuple{Generation: 1, Length: length, Artifact: artifact, Manifest: manifest}
	rollback := oracleCurrentTuple{Generation: 0, Length: rollbackLength, Artifact: rollbackArtifact, Manifest: rollbackManifest}
	raw := oracleCurrent(t, 1, successor, &rollback)
	if err := os.WriteFile(filepath.Join(root, name), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

// recoveryOracleSuccessorCurrentNoRollback writes a successor current with
// no rollback field. Used by the invalid corpus to assert that a missing
// rollback before activation is transaction-invalid.
func recoveryOracleSuccessorCurrentNoRollback(t *testing.T, root string,
	artifact, manifest [32]byte, length uint64) {
	t.Helper()
	successor := oracleCurrentTuple{
		Generation: 1, Length: length,
		Artifact: artifact, Manifest: manifest,
	}
	raw := oracleCurrent(t, 1, successor, nil)
	if err := os.WriteFile(filepath.Join(root, "current"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

// recoveryOracleCanonicalPredecessorCurrent rewrites the current record to
// the canonical predecessor (selected=0, no rollback). Used by the
// predecessor-current corpus fixtures that start from the complete base.
func recoveryOracleCanonicalPredecessorCurrent(t *testing.T, root string,
	artifact, manifest [32]byte, length uint64) {
	t.Helper()
	predecessor := oracleCurrentTuple{
		Generation: 0, Length: length,
		Artifact: artifact, Manifest: manifest,
	}
	raw := oracleCurrent(t, 0, predecessor, nil)
	if err := os.WriteFile(filepath.Join(root, "current"), raw, 0o600); err != nil {
		t.Fatalf("FIXTURE: write canonical predecessor current: %v", err)
	}
}

// recoveryOracleRemoveJournalEntries deletes every journal entry whose
// state code is in the half-open range [firstState, lastState].
func recoveryOracleRemoveJournalEntries(t *testing.T, root string, firstState, lastState byte) {
	t.Helper()
	for state := firstState; state <= lastState; state++ {
		name := recoveryOracleName(state)
		if name == "" {
			continue
		}
		path := filepath.Join(root, "transactions", "1", "journal", name)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("FIXTURE: remove journal entry %d: %v", state, err)
		}
	}
}

// recoveryOracleAssertInvalid asserts the recovered Result is
// transaction-invalid with generation zero, zero digests, false staging,
// and no custody notice. It distinguishes RECOVER: assertion failures
// from FIXTURE: setup failures produced by the corruption or lock
// mutation builders.

// recoveryOracleMutateJournal flips or sets one byte in the journal
// entry for `state` at file offset `offset`. When `value` is non-zero
// the byte is set to the literal value; otherwise the byte is XOR'd
// with 0xff. Used by every corruption and field mutation builder.
func recoveryOracleMutateJournal(t *testing.T, root string, state int, offset int, _, value byte) {
	t.Helper()
	path := filepath.Join(root, "transactions", "1", "journal", recoveryOracleName(byte(state)))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("FIXTURE: read journal entry %d: %v", state, err)
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
		t.Fatalf("FIXTURE: write journal entry %d: %v", state, err)
	}
}

// recoveryOracleCopyTree copies the rooted tree recursively to dst.
// Used by the corruption and lock test setup to snapshot a fresh
// workRoot for each mutation row.
func recoveryOracleCopyTree(t *testing.T, src, dst string) {
	t.Helper()
	if err := filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	}); err != nil {
		t.Fatalf("FIXTURE: copy rooted tree: %v", err)
	}
}
func recoveryOracleAssertInvalid(t *testing.T, result Result, err error) {
	t.Helper()
	if result.Outcome != "transaction-invalid" || result.State != "transaction-invalid" {
		t.Fatalf("RECOVER: outcome=%s state=%s, want transaction-invalid/transaction-invalid (err=%v)", result.Outcome, result.State, err)
	}
	if result.Generation != 0 {
		t.Fatalf("RECOVER: generation=%d, want 0", result.Generation)
	}
	if result.CurrentDigest != recoveryOracleZero || result.RollbackDigest != recoveryOracleZero {
		t.Fatalf("RECOVER: digests not zero: current=%x rollback=%x", result.CurrentDigest, result.RollbackDigest)
	}
	if result.StagingPresent {
		t.Fatal("RECOVER: staging must be false for invalid results")
	}
	if result.CustodyNotice != "" {
		t.Fatalf("RECOVER: custody=%q, want empty", result.CustodyNotice)
	}
	if result.SafeNotice != "update transaction invalid" {
		t.Fatalf("RECOVER: safe notice=%q, want update transaction invalid", result.SafeNotice)
	}
	if err == nil {
		t.Fatal("RECOVER: error must be non-nil for invalid results")
	}
}

// recoveryOracleRemoveSelectedPredecessor deletes the predecessor
// generation so the next inspection must report transaction-invalid.
func recoveryOracleRemoveSelectedPredecessor(t *testing.T, root string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(root, "generations", "0")); err != nil {
		t.Fatal(err)
	}
}

// recoveryOracleRemoveSelectedCandidate deletes the candidate generation so
// the next inspection must report transaction-invalid.
func recoveryOracleRemoveSelectedCandidate(t *testing.T, root string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(root, "generations", "1")); err != nil {
		t.Fatal(err)
	}
}

// recoveryOracleReplaceWithAlias swaps the selected payload for a hard-link
// or symlink to a foreign file, exercising the accepted evidence shape rules.
func recoveryOracleReplaceWithAlias(t *testing.T, root, kind string) {
	t.Helper()
	aliasSource := filepath.Join(filepath.Dir(root), "alias-source-"+kind)
	if err := os.WriteFile(aliasSource, []byte("alias"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "generations", "0", "artifact")
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	switch kind {
	case "hardlink":
		if err := os.Link(aliasSource, target); err != nil {
			t.Fatal(err)
		}
	case "symlink":
		if err := os.Symlink(aliasSource, target); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown alias kind %q", kind)
	}
}

// recoveryOracleReplaceWithJunction creates a directory junction on the
// selected payload path, exercising the accepted reparse-point rule.
func recoveryOracleReplaceWithJunction(t *testing.T, root string) {
	t.Helper()
	target := filepath.Join(root, "generations", "0", "artifact")
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	alias := filepath.Join(filepath.Dir(root), "junction-target")
	if err := os.Mkdir(alias, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(alias, target); err != nil {
		t.Fatal(err)
	}
}
