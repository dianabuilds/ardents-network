package updatetransaction

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

const (
	oracleVectorPath    = "../../docs/development/testdata/s7.2/c0-happy-path-v1.json"
	oraclePreviousPath  = "../../docs/development/testdata/s7.2/previous-payload-v1.txt"
	oracleCandidatePath = "../release/testdata/r049-public-vector-v1/artifact.bin"
	oracleMarker        = "ardents-update-transaction-v1\n"
)

type v0OracleVector struct {
	Schema         string            `json:"schema"`
	ReleaseOutcome string            `json:"release_outcome"`
	Candidate      v0OracleCandidate `json:"candidate"`
	Initial        v0OracleInitial   `json:"initial"`
	Request        v0OracleRequest   `json:"request"`
	Expected       v0OracleExpected  `json:"expected"`
}

type v0OracleCandidate struct {
	ReleaseIdentity string `json:"release_identity"`
	ReleaseVersion  int64  `json:"release_version"`
	Path            string `json:"path"`
	Length          uint64 `json:"length"`
	SHA256          string `json:"sha256"`
}

type v0OracleInitial struct {
	TransactionGeneration uint64                `json:"transaction_generation"`
	ActivePayload         v0OracleActivePayload `json:"active_payload"`
}

type v0OracleActivePayload struct {
	Identity            string                      `json:"identity"`
	Path                string                      `json:"path"`
	Length              uint64                      `json:"length"`
	SHA256              string                      `json:"sha256"`
	Manifest            v0OracleManifest            `json:"manifest"`
	StoredAuthorization v0OracleStoredAuthorization `json:"stored_authorization"`
}

type v0OracleManifest struct {
	Generation                 uint64         `json:"generation"`
	TargetPath                 string         `json:"target_path"`
	Platform                   string         `json:"platform"`
	Architecture               string         `json:"architecture"`
	Environment                string         `json:"environment"`
	Network                    string         `json:"network"`
	ReleaseIdentity            string         `json:"release_identity"`
	ReleaseVersion             uint64         `json:"release_version"`
	SourceRevision             string         `json:"source_revision"`
	BuildInputCommitment       string         `json:"build_input_commitment"`
	BuildIdentity              string         `json:"build_identity"`
	DependencyIdentity         string         `json:"dependency_identity"`
	SBOMIdentity               string         `json:"sbom_identity"`
	AttestationPolicy          string         `json:"attestation_policy"`
	Qualification              string         `json:"qualification"`
	BuildState                 string         `json:"build_state"`
	ProtocolPhase              string         `json:"protocol_phase"`
	BuildSafety                string         `json:"build_safety"`
	Protocol                   string         `json:"protocol"`
	ReferenceTime              string         `json:"reference_time"`
	BuildSafetyNoNewWorkAfter  string         `json:"build_safety_no_new_work_after"`
	BuildSafetyTerminateAfter  string         `json:"build_safety_terminate_after"`
	ProtocolTransitionDeadline *string        `json:"protocol_transition_deadline"`
	SchemaPlan                 string         `json:"schema_plan"`
	SafeNotice                 string         `json:"safe_notice"`
	CustodyNotice              string         `json:"custody_notice"`
	ReleaseFloors              v0OracleFloors `json:"release_floors"`
}

type v0OracleStoredAuthorization struct {
	Classification   string `json:"classification"`
	Platform         string `json:"platform"`
	Architecture     string `json:"architecture"`
	Environment      string `json:"environment"`
	Network          string `json:"network"`
	SchemaCompatible bool   `json:"schema_compatible"`
	Revoked          bool   `json:"revoked"`
	AboveLocalFloors bool   `json:"above_local_floors"`
}

type v0OracleFloors struct {
	RootVersion      int64  `json:"root_version"`
	RootSHA256       string `json:"root_sha256"`
	TimestampVersion int64  `json:"timestamp_version"`
	TimestampSHA256  string `json:"timestamp_sha256"`
	SnapshotVersion  int64  `json:"snapshot_version"`
	SnapshotSHA256   string `json:"snapshot_sha256"`
	TargetsVersion   int64  `json:"targets_version"`
	TargetsSHA256    string `json:"targets_sha256"`
}

type v0OracleRequest struct {
	TransactionGeneration uint64 `json:"transaction_generation"`
	ActiveWork            uint64 `json:"active_work"`
	SchemaPlan            string `json:"schema_plan"`
}

type v0OracleExpected struct {
	CommandResult      v0OracleResult `json:"command_result"`
	ReleaseFloors      v0OracleFloors `json:"release_floors"`
	AuthorityMutations uint64         `json:"authority_mutations"`
	StopNewWorkCalls   uint64         `json:"stop_new_work_calls"`
	DrainCalls         uint64         `json:"drain_calls"`
	SelfTestCalls      uint64         `json:"self_test_calls"`
}

type v0OracleResult struct {
	Schema                string `json:"schema"`
	Outcome               string `json:"outcome"`
	State                 string `json:"state"`
	TransactionGeneration uint64 `json:"transaction_generation"`
	CurrentSHA256         string `json:"current_sha256"`
	RollbackSHA256        string `json:"rollback_sha256"`
	StagingPresent        bool   `json:"staging_present"`
	SafeNotice            string `json:"safe_notice"`
	CustodyNotice         string `json:"custody_notice"`
}

type oracleCurrentTuple struct {
	Generation uint64
	Length     uint64
	Artifact   [32]byte
	Manifest   [32]byte
}

type oraclePassSelfTest struct{}

func (oraclePassSelfTest) Check(context.Context, CandidateIdentity) error { return nil }

type oracleDeadlineContext struct{ expired bool }

func (*oracleDeadlineContext) Deadline() (time.Time, bool) { return time.Time{}, true }
func (*oracleDeadlineContext) Done() <-chan struct{}       { return nil }
func (ctx *oracleDeadlineContext) Err() error {
	if ctx.expired {
		return context.DeadlineExceeded
	}
	return nil
}
func (*oracleDeadlineContext) Value(any) any { return nil }

func TestBoundedCleanupStopsAfterDeadlineOverrun(t *testing.T) {
	ctx := &oracleDeadlineContext{}
	calls := 0
	err := boundedCleanup(ctx, func(string) error {
		calls++
		ctx.expired = true
		return nil
	}, "first", "must-not-run")
	if !errors.Is(err, context.DeadlineExceeded) || calls != 1 {
		t.Fatalf("bounded cleanup = %v, calls=%d", err, calls)
	}
}

func TestV0JournalHasExactIndependentChain(t *testing.T) {
	root := filepath.Join(t.TempDir(), "update")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	vector := oracleBootstrapV0(t, root)
	currentSum := oracleFileSum(filepath.Join(root, "current"))
	currentBefore := oracleReadExact(t, filepath.Join(root, "current"), 105,
		hex.EncodeToString(currentSum[:]))
	previousManifest, err := os.ReadFile(filepath.Join(root, "generations", "0", "manifest.bin"))
	if err != nil {
		t.Fatal(err)
	}
	previousArtifact := oracleReadExact(t, oraclePreviousPath,
		vector.Initial.ActivePayload.Length, vector.Initial.ActivePayload.SHA256)
	candidate := oracleReadExact(t, oracleCandidatePath, vector.Candidate.Length, vector.Candidate.SHA256)
	decision := oracleAcceptedDecision(t, vector)
	work := &oracleWorkControl{}
	_, err = Apply(context.Background(), Request{UpdateRoot: root,
		Generation: vector.Request.TransactionGeneration, ActiveWork: 0, SchemaPlan: "no-op-v1",
		decision: decision, Artifact: candidate, Work: work, SelfTest: oraclePassSelfTest{}})
	if err != nil {
		t.Fatal(err)
	}
	candidateManifest, err := os.ReadFile(filepath.Join(root, "generations", "1", "manifest.bin"))
	if err != nil {
		t.Fatal(err)
	}
	expectedManifest := v0OracleManifest{
		Generation: 1, TargetPath: decision.Path, Platform: decision.Platform,
		Architecture: decision.Architecture, Environment: decision.Environment, Network: decision.Network,
		ReleaseIdentity: decision.ReleaseIdentity, ReleaseVersion: uint64(decision.ReleaseVersion),
		SourceRevision: decision.SourceRevision, BuildInputCommitment: decision.BuildInputCommitment,
		BuildIdentity: decision.BuildIdentity, DependencyIdentity: decision.DependencyIdentity,
		SBOMIdentity: decision.SBOMIdentity, AttestationPolicy: decision.AttestationPolicy,
		Qualification: decision.Qualification, BuildState: decision.BuildState,
		ProtocolPhase: decision.ProtocolPhase, BuildSafety: string(decision.BuildSafety),
		Protocol: string(decision.Protocol), ReferenceTime: decision.ReferenceTime.Format(time.RFC3339),
		BuildSafetyNoNewWorkAfter:  decision.BuildSafetyNoNewWorkAfter.Format(time.RFC3339),
		BuildSafetyTerminateAfter:  decision.BuildSafetyTerminateAfter.Format(time.RFC3339),
		ProtocolTransitionDeadline: nil, SchemaPlan: "no-op-v1", SafeNotice: "update committed",
		CustodyNotice: decision.CustodyNotice, ReleaseFloors: vector.Expected.ReleaseFloors,
	}
	authorization := v0OracleStoredAuthorization{Classification: "release-accepted",
		Platform: decision.Platform, Architecture: decision.Architecture, Environment: decision.Environment,
		Network: decision.Network, SchemaCompatible: true, AboveLocalFloors: true}
	if !bytes.Equal(candidateManifest, oracleManifest(t, expectedManifest, authorization, candidate)) {
		t.Fatal("candidate manifest differs from the independent V0 oracle")
	}
	previousManifestDigest := sha256.Sum256(previousManifest)
	predecessorBody := append([]byte(nil), oracleSum(currentBefore)[:]...)
	predecessorBody = oracleAppendUint64(predecessorBody, 0)
	predecessorBody = oracleAppendUint64(predecessorBody, uint64(len(previousArtifact)))
	predecessorBody = oracleAppendDigest(t, predecessorBody, oracleSum(previousArtifact)[:])
	predecessorBody = oracleAppendDigest(t, predecessorBody, previousManifestDigest[:])
	predecessorBody = append(predecessorBody, 0)
	predecessorBody = oracleAppendDigest(t, predecessorBody, oracleSum(previousArtifact)[:])
	predecessorBody = oracleAppendDigest(t, predecessorBody, previousManifestDigest[:])
	predecessor := sha256.Sum256(oracleEnvelope(t, 3, predecessorBody))
	artifactDigest := oracleSum(candidate)
	manifestDigest := sha256.Sum256(candidateManifest)
	adapters := []byte{0, 0, 0, 0, 1, 1, 0, 1, 0}
	var elapsed uint64
	for state := byte(1); state <= 9; state++ {
		name := []string{"", "01-release-accepted.entry", "02-artifact-verified.entry",
			"03-staged.entry", "04-rollback-reserved.entry", "05-stop-new-work.entry",
			"06-draining.entry", "07-activated.entry", "08-self-testing.entry",
			"09-committed.entry"}[state]
		entryPath := filepath.Join(root, "transactions", "1", "journal", name)
		entrySum := oracleFileSum(entryPath)
		raw := oracleReadExact(t, entryPath, 139, hex.EncodeToString(entrySum[:]))
		if !bytes.Equal(raw[:8], []byte("ARDUPD01")) || raw[8] != 4 || raw[9] != 1 ||
			binary.BigEndian.Uint32(raw[12:16]) != 123 {
			t.Fatalf("state %d has invalid journal envelope", state)
		}
		body := raw[16:]
		deadline := decision.BuildSafetyTerminateAfter.Unix()
		if state == 5 {
			deadline = decision.BuildSafetyNoNewWorkAfter.Unix()
		}
		gotElapsed := binary.BigEndian.Uint64(body[107:115])
		if body[0] != state || binary.BigEndian.Uint64(body[1:9]) != 1 ||
			!bytes.Equal(body[9:41], predecessor[:]) || !bytes.Equal(body[41:73], artifactDigest[:]) ||
			!bytes.Equal(body[73:105], manifestDigest[:]) || body[105] != adapters[state-1] ||
			body[106] != state || gotElapsed < elapsed ||
			int64(binary.BigEndian.Uint64(body[115:123])) != deadline {
			t.Fatalf("state %d journal body violates frozen oracle", state)
		}
		predecessor, elapsed = sha256.Sum256(raw), gotElapsed
	}
}

func oracleLoadV0(t *testing.T) v0OracleVector {
	t.Helper()
	raw, err := os.ReadFile(oracleVectorPath)
	if err != nil {
		t.Fatal(err)
	}
	var vector v0OracleVector
	if err := json.Unmarshal(raw, &vector); err != nil {
		t.Fatal(err)
	}
	if vector.Schema != "ardents-s72-update-vector-v1" ||
		vector.ReleaseOutcome != "release-accepted" ||
		vector.Initial.TransactionGeneration != 0 ||
		vector.Request.TransactionGeneration != 1 ||
		vector.Request.SchemaPlan != "no-op-v1" {
		t.Fatalf("unexpected V0 control fields: %+v", vector)
	}
	if vector.Initial.ActivePayload.Path !=
		"docs/development/testdata/s7.2/previous-payload-v1.txt" ||
		vector.Initial.ActivePayload.Length != 32 ||
		vector.Initial.ActivePayload.SHA256 !=
			"8bdad9bde29bb6ee2a9d1d7005ec8ba2461b2bad3627372ee8458693c1fc08af" {
		t.Fatalf("unexpected V0 predecessor: %+v", vector.Initial.ActivePayload)
	}
	return vector
}

func oracleReadExact(t *testing.T, path string, length uint64, digest string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if uint64(len(data)) != length || hex.EncodeToString(oracleSum(data)[:]) != digest {
		t.Fatalf("fixture %s does not match frozen identity", path)
	}
	return data
}

func oracleSum(data []byte) *[32]byte {
	sum := sha256.Sum256(data)
	return &sum
}

func oracleAppendString(t *testing.T, body []byte, value string, maximum int) []byte {
	t.Helper()
	if !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 ||
		len(value) > maximum || len(value) > int(^uint16(0)) {
		t.Fatalf("invalid oracle string %q", value)
	}
	var length [2]byte
	binary.BigEndian.PutUint16(length[:], uint16(len(value)))
	return append(append(body, length[:]...), value...)
}

func oracleAppendUint64(body []byte, value uint64) []byte {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	return append(body, encoded[:]...)
}

func oracleAppendInt64(body []byte, value int64) []byte {
	return oracleAppendUint64(body, uint64(value))
}

func oracleAppendDigest(t *testing.T, body []byte, value []byte) []byte {
	t.Helper()
	if len(value) != sha256.Size {
		t.Fatalf("digest length = %d, want %d", len(value), sha256.Size)
	}
	return append(body, value...)
}

func oracleAppendFloor(t *testing.T, body []byte, version int64, digest string) []byte {
	t.Helper()
	body = oracleAppendInt64(body, version)
	return oracleAppendDigest(t, body, oracleDecodeDigest(t, digest)[:])
}

func oracleDecodeDigest(t *testing.T, value string) *[32]byte {
	t.Helper()
	raw, err := hex.DecodeString(value)
	if err != nil || len(raw) != sha256.Size {
		t.Fatalf("invalid digest %q", value)
	}
	var digest [32]byte
	copy(digest[:], raw)
	return &digest
}

func oracleAppendTime(t *testing.T, body []byte, value string) []byte {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return oracleAppendInt64(body, parsed.UTC().Unix())
}

func oracleEnvelope(t *testing.T, kind byte, body []byte) []byte {
	t.Helper()
	if kind < 1 || kind > 4 || uint64(len(body)) > uint64(^uint32(0)) {
		t.Fatalf("invalid envelope kind=%d length=%d", kind, len(body))
	}
	envelope := append([]byte("ARDUPD01"), kind, 1, 0, 0)
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(body)))
	envelope = append(envelope, length[:]...)
	return append(envelope, body...)
}

func oracleManifest(t *testing.T, manifest v0OracleManifest,
	authorization v0OracleStoredAuthorization, artifact []byte) []byte {
	t.Helper()
	body := oracleAppendUint64(nil, manifest.Generation)
	body = oracleAppendString(t, body, manifest.TargetPath, 512)
	body = oracleAppendUint64(body, uint64(len(artifact)))
	body = oracleAppendDigest(t, body, oracleSum(artifact)[:])
	for _, value := range []string{manifest.Platform, manifest.Architecture,
		manifest.Environment, manifest.Network, manifest.ReleaseIdentity} {
		body = oracleAppendString(t, body, value, 256)
	}
	body = oracleAppendUint64(body, manifest.ReleaseVersion)
	for _, value := range []string{manifest.SourceRevision,
		manifest.BuildInputCommitment, manifest.BuildIdentity,
		manifest.DependencyIdentity, manifest.SBOMIdentity,
		manifest.AttestationPolicy, manifest.Qualification,
		manifest.BuildState, manifest.ProtocolPhase,
		manifest.BuildSafety, manifest.Protocol} {
		body = oracleAppendString(t, body, value, 256)
	}
	floors := manifest.ReleaseFloors
	body = oracleAppendFloor(t, body, floors.RootVersion, floors.RootSHA256)
	body = oracleAppendFloor(t, body, floors.TimestampVersion, floors.TimestampSHA256)
	body = oracleAppendFloor(t, body, floors.SnapshotVersion, floors.SnapshotSHA256)
	body = oracleAppendFloor(t, body, floors.TargetsVersion, floors.TargetsSHA256)
	body = oracleAppendTime(t, body, manifest.ReferenceTime)
	body = oracleAppendTime(t, body, manifest.BuildSafetyNoNewWorkAfter)
	body = oracleAppendTime(t, body, manifest.BuildSafetyTerminateAfter)
	if manifest.ProtocolTransitionDeadline == nil {
		body = oracleAppendInt64(body, 0)
	} else {
		body = oracleAppendTime(t, body, *manifest.ProtocolTransitionDeadline)
	}
	body = oracleAppendString(t, body, manifest.SchemaPlan, 256)
	body = oracleAppendString(t, body, manifest.SafeNotice, 512)
	body = oracleAppendString(t, body, manifest.CustodyNotice, 512)
	for _, value := range []string{authorization.Classification,
		authorization.Platform, authorization.Architecture,
		authorization.Environment, authorization.Network} {
		body = oracleAppendString(t, body, value, 256)
	}
	for _, value := range []bool{authorization.SchemaCompatible,
		authorization.Revoked, authorization.AboveLocalFloors} {
		if value {
			body = append(body, 1)
		} else {
			body = append(body, 0)
		}
	}
	return oracleEnvelope(t, 1, body)
}

func oracleCurrent(t *testing.T, selected uint64, current oracleCurrentTuple,
	rollback *oracleCurrentTuple) []byte {
	t.Helper()
	body := oracleAppendUint64(nil, selected)
	body = oracleAppendUint64(body, current.Generation)
	body = oracleAppendUint64(body, current.Length)
	body = oracleAppendDigest(t, body, current.Artifact[:])
	body = oracleAppendDigest(t, body, current.Manifest[:])
	if rollback == nil {
		body = append(body, 0)
	} else {
		body = append(body, 1)
		body = oracleAppendUint64(body, rollback.Generation)
		body = oracleAppendUint64(body, rollback.Length)
		body = oracleAppendDigest(t, body, rollback.Artifact[:])
		body = oracleAppendDigest(t, body, rollback.Manifest[:])
	}
	return oracleEnvelope(t, 2, body)
}

func oracleBootstrapV0(t *testing.T, root string) v0OracleVector {
	t.Helper()
	vector := oracleLoadV0(t)
	previous := oracleReadExact(t, oraclePreviousPath,
		vector.Initial.ActivePayload.Length, vector.Initial.ActivePayload.SHA256)
	if err := os.MkdirAll(filepath.Join(root, "generations", "0"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"staging", "transactions"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".ardents-update-transaction-lock"),
		nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ardents-update-transaction-v1"),
		[]byte(oracleMarker), 0o600); err != nil {
		t.Fatal(err)
	}
	generation := filepath.Join(root, "generations", "0")
	if err := os.WriteFile(filepath.Join(generation, "artifact"), previous, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := oracleManifest(t, vector.Initial.ActivePayload.Manifest,
		vector.Initial.ActivePayload.StoredAuthorization, previous)
	if err := os.WriteFile(filepath.Join(generation, "manifest.bin"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	current := oracleCurrent(t, 0, oracleCurrentTuple{
		Generation: 0,
		Length:     uint64(len(previous)),
		Artifact:   *oracleSum(previous),
		Manifest:   *oracleSum(manifest),
	}, nil)
	if err := os.WriteFile(filepath.Join(root, "current"), current, 0o600); err != nil {
		t.Fatal(err)
	}
	return vector
}
