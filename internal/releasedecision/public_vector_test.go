package releasedecision

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestR049FrozenPublicVector(t *testing.T) {
	t.Parallel()
	directory := filepath.Join("testdata", "r049-public-vector-v1")
	var expected struct {
		Schema         string `json:"schema"`
		RefTime        string `json:"ref_time"`
		TargetPath     string `json:"target_path"`
		ArtifactSHA256 string `json:"artifact_sha256"`
		Outcome        string `json:"outcome"`
		BuildSafety    string `json:"build_safety"`
		Protocol       string `json:"protocol"`
		RootVersion    int64  `json:"root_version"`
	}
	decodeVectorJSON(t, filepath.Join(directory, "expected.json"), &expected)
	if expected.Schema != "ardents-r049-public-vector-v1" {
		t.Fatalf("unexpected vector schema %q", expected.Schema)
	}
	refTime, err := time.Parse(time.RFC3339, expected.RefTime)
	if err != nil {
		t.Fatal(err)
	}
	artifact := readVectorFile(t, filepath.Join(directory, "artifact.bin"))
	decision := Evaluate(context.Background(), Inputs{
		RootBytes: readVectorFile(t, filepath.Join(directory, "root.json")),
		Files: map[string][]byte{
			metadataBaseURL + "timestamp.json":  readVectorFile(t, filepath.Join(directory, "timestamp.json")),
			metadataBaseURL + "1.snapshot.json": readVectorFile(t, filepath.Join(directory, "1.snapshot.json")),
			metadataBaseURL + "1.targets.json":  readVectorFile(t, filepath.Join(directory, "1.targets.json")),
		},
		TargetPath: expected.TargetPath,
		Artifact:   artifact,
		Local:      defaultLocalEnvironment(refTime),
	}, newMemoryStoreForTest())
	wantDigest, err := hex.DecodeString(expected.ArtifactSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if string(decision.Outcome) != expected.Outcome || string(decision.BuildSafety) != expected.BuildSafety ||
		string(decision.Protocol) != expected.Protocol || decision.RootVersion != expected.RootVersion ||
		!bytes.Equal(decision.Digest, wantDigest) {
		t.Fatalf("public vector result mismatch: %+v", decision)
	}
}

func readVectorFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func decodeVectorJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := json.Unmarshal(readVectorFile(t, path), value); err != nil {
		t.Fatal(err)
	}
}
