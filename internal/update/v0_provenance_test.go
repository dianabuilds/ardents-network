package update_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"
)

const (
	v0VectorPath              = "../../docs/development/testdata/s7.2/c0-happy-path-v1.json"
	v0BootstrapManifestSHA256 = "54d1f66e06df8e09fd734cccb6cc61b9f4880646ecef9dab8926bf65e5bfea96"
	v0CustodyNotice           = "H3 threshold identities and both rebuild records are project-controlled; no independent custody or builder claim is made"
)

// TestV0ProvenanceVector is a C4-only verifier. It imports no maintained
// Update code and reconstructs the retired V0 manifest directly from the
// immutable source vector. It must never become a reader for V2 runtime state.
func TestV0ProvenanceVector(t *testing.T) {
	data, err := os.ReadFile(v0VectorPath)
	if err != nil {
		t.Fatal(err)
	}
	var vector v0Vector
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	if vector.Schema != "ardents-s72-update-vector-v1" || vector.Initial.TransactionGeneration != 0 ||
		vector.Expected.CommandResult.Schema != "ardents-update-result-v1" ||
		vector.Initial.ActivePayload.Manifest.CustodyNotice != v0CustodyNotice ||
		vector.Expected.CommandResult.CustodyNotice != v0CustodyNotice {
		t.Fatalf("V0 vector is not the frozen custody-evidence representation: %+v", vector)
	}
	artifact, err := os.ReadFile("../../docs/development/testdata/s7.2/previous-payload-v1.txt")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := encodeV0Manifest(vector.Initial.ActivePayload.Manifest, vector.Initial.ActivePayload.StoredAuthorization, artifact)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(manifest)
	if hex.EncodeToString(digest[:]) != v0BootstrapManifestSHA256 {
		t.Fatalf("V0 bootstrap manifest SHA-256 = %x, want %s", digest, v0BootstrapManifestSHA256)
	}
}

type v0Vector struct {
	Schema   string     `json:"schema"`
	Initial  v0Initial  `json:"initial"`
	Expected v0Expected `json:"expected"`
}

type v0Initial struct {
	TransactionGeneration uint64    `json:"transaction_generation"`
	ActivePayload         v0Payload `json:"active_payload"`
}

type v0Payload struct {
	Manifest            v0Manifest      `json:"manifest"`
	StoredAuthorization v0Authorization `json:"stored_authorization"`
}

type v0Expected struct {
	CommandResult v0Result `json:"command_result"`
}

type v0Result struct {
	Schema        string `json:"schema"`
	CustodyNotice string `json:"custody_notice"`
}

type v0Manifest struct {
	Generation                 uint64   `json:"generation"`
	TargetPath                 string   `json:"target_path"`
	Platform                   string   `json:"platform"`
	Architecture               string   `json:"architecture"`
	Environment                string   `json:"environment"`
	Network                    string   `json:"network"`
	ReleaseIdentity            string   `json:"release_identity"`
	ReleaseVersion             uint64   `json:"release_version"`
	SourceRevision             string   `json:"source_revision"`
	BuildInputCommitment       string   `json:"build_input_commitment"`
	BuildIdentity              string   `json:"build_identity"`
	DependencyIdentity         string   `json:"dependency_identity"`
	SBOMIdentity               string   `json:"sbom_identity"`
	AttestationPolicy          string   `json:"attestation_policy"`
	Qualification              string   `json:"qualification"`
	BuildState                 string   `json:"build_state"`
	ProtocolPhase              string   `json:"protocol_phase"`
	BuildSafety                string   `json:"build_safety"`
	Protocol                   string   `json:"protocol"`
	ReferenceTime              string   `json:"reference_time"`
	BuildSafetyNoNewWorkAfter  string   `json:"build_safety_no_new_work_after"`
	BuildSafetyTerminateAfter  string   `json:"build_safety_terminate_after"`
	ProtocolTransitionDeadline *string  `json:"protocol_transition_deadline"`
	SchemaPlan                 string   `json:"schema_plan"`
	SafeNotice                 string   `json:"safe_notice"`
	CustodyNotice              string   `json:"custody_notice"`
	ReleaseFloors              v0Floors `json:"release_floors"`
}

type v0Floors struct {
	RootVersion      int64  `json:"root_version"`
	RootSHA256       string `json:"root_sha256"`
	TimestampVersion int64  `json:"timestamp_version"`
	TimestampSHA256  string `json:"timestamp_sha256"`
	SnapshotVersion  int64  `json:"snapshot_version"`
	SnapshotSHA256   string `json:"snapshot_sha256"`
	TargetsVersion   int64  `json:"targets_version"`
	TargetsSHA256    string `json:"targets_sha256"`
}

type v0Authorization struct {
	Classification   string `json:"classification"`
	Platform         string `json:"platform"`
	Architecture     string `json:"architecture"`
	Environment      string `json:"environment"`
	Network          string `json:"network"`
	SchemaCompatible bool   `json:"schema_compatible"`
	Revoked          bool   `json:"revoked"`
	AboveLocalFloors bool   `json:"above_local_floors"`
}

func encodeV0Manifest(manifest v0Manifest, authorization v0Authorization, artifact []byte) ([]byte, error) {
	body := appendV0Number(nil, manifest.Generation)
	body = appendV0Text(body, manifest.TargetPath)
	body = appendV0Number(body, uint64(len(artifact)))
	digest := sha256.Sum256(artifact)
	body = append(body, digest[:]...)
	for _, value := range []string{manifest.Platform, manifest.Architecture, manifest.Environment, manifest.Network, manifest.ReleaseIdentity} {
		body = appendV0Text(body, value)
	}
	body = appendV0Number(body, manifest.ReleaseVersion)
	for _, value := range []string{manifest.SourceRevision, manifest.BuildInputCommitment, manifest.BuildIdentity, manifest.DependencyIdentity,
		manifest.SBOMIdentity, manifest.AttestationPolicy, manifest.Qualification, manifest.BuildState, manifest.ProtocolPhase, manifest.BuildSafety, manifest.Protocol} {
		body = appendV0Text(body, value)
	}
	for _, floor := range []struct {
		version int64
		digest  string
	}{
		{manifest.ReleaseFloors.RootVersion, manifest.ReleaseFloors.RootSHA256},
		{manifest.ReleaseFloors.TimestampVersion, manifest.ReleaseFloors.TimestampSHA256},
		{manifest.ReleaseFloors.SnapshotVersion, manifest.ReleaseFloors.SnapshotSHA256},
		{manifest.ReleaseFloors.TargetsVersion, manifest.ReleaseFloors.TargetsSHA256},
	} {
		decoded, err := hex.DecodeString(floor.digest)
		if err != nil || len(decoded) != sha256.Size || floor.version <= 0 {
			return nil, err
		}
		body = appendV0Number(body, uint64(floor.version))
		body = append(body, decoded...)
	}
	for _, value := range []string{manifest.ReferenceTime, manifest.BuildSafetyNoNewWorkAfter, manifest.BuildSafetyTerminateAfter} {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, err
		}
		body = appendV0Number(body, uint64(parsed.Unix()))
	}
	if manifest.ProtocolTransitionDeadline == nil {
		body = appendV0Number(body, 0)
	} else {
		parsed, err := time.Parse(time.RFC3339, *manifest.ProtocolTransitionDeadline)
		if err != nil {
			return nil, err
		}
		body = appendV0Number(body, uint64(parsed.Unix()))
	}
	body = appendV0Text(body, manifest.SchemaPlan)
	body = appendV0Text(body, manifest.SafeNotice)
	body = appendV0Text(body, manifest.CustodyNotice)
	for _, value := range []string{authorization.Classification, authorization.Platform, authorization.Architecture, authorization.Environment, authorization.Network} {
		body = appendV0Text(body, value)
	}
	for _, value := range []bool{authorization.SchemaCompatible, authorization.Revoked, authorization.AboveLocalFloors} {
		if value {
			body = append(body, 1)
		} else {
			body = append(body, 0)
		}
	}
	header := append([]byte("ARDUPD01"), 1, 1, 0, 0)
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(body)))
	return append(append(header, length[:]...), body...), nil
}

func appendV0Number(body []byte, value uint64) []byte {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	return append(body, encoded[:]...)
}

func appendV0Text(body []byte, value string) []byte {
	var encoded [2]byte
	binary.BigEndian.PutUint16(encoded[:], uint16(len(value)))
	return append(append(body, encoded[:]...), value...)
}
