package inspection

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/release"
)

func TestVerifyReleaseUsesMaintainedReleaseDecision(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate release vector")
	}
	directory := filepath.Join(filepath.Dir(here), "..", "..", "release", "testdata", "r049-public-vector-v1")
	refTime := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
	inputs := release.Inputs{RootBytes: readFixture(t, filepath.Join(directory, "root.json")), Files: map[string][]byte{
		release.MetadataURL("timestamp.json"):  readFixture(t, filepath.Join(directory, "timestamp.json")),
		release.MetadataURL("1.snapshot.json"): readFixture(t, filepath.Join(directory, "1.snapshot.json")),
		release.MetadataURL("1.targets.json"):  readFixture(t, filepath.Join(directory, "1.targets.json")),
	}, TargetPath: "ardents/windows-amd64/application", Artifact: readFixture(t, filepath.Join(directory, "artifact.bin")),
		Local: release.LocalEnvironment{Environment: "h3-test", Network: "ardents-h3-test-1", Platform: "windows-amd64", Architecture: "amd64", RefTime: refTime}}
	seed, err := release.Open(filepath.Join(t.TempDir(), "seed"))
	if err != nil {
		t.Fatal(err)
	}
	decision := seed.Evaluate(context.Background(), inputs)
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	if decision.Outcome != release.OutcomeReleaseAccepted || len(decision.Digest) != sha256.Size {
		t.Fatalf("release vector decision = %+v", decision)
	}
	var digest [32]byte
	copy(digest[:], decision.Digest)
	body, err := EncodeReleaseEvidence(ReleaseEvidence{ArtifactDigest: digest, TargetPath: decision.Path, ReleaseIdentity: decision.ReleaseIdentity,
		BuildIdentity: decision.BuildIdentity, ProtocolPhase: decision.ProtocolPhase, BuildState: decision.BuildState})
	if err != nil {
		t.Fatal(err)
	}
	var inspected release.Decision
	accepted := false
	if outcome := verifyRelease(context.Background(), filepath.Join(t.TempDir(), "inspection"), inputs, body, &inspected, &accepted); outcome != alphacontrol.OutcomeAccepted || !accepted || inspected.Outcome != release.OutcomeReleaseAccepted {
		t.Fatalf("release inspection = %q, accepted=%v, decision=%+v", outcome, accepted, inspected)
	}
	body[len(body)-1]++
	if outcome := verifyRelease(context.Background(), filepath.Join(t.TempDir(), "altered"), inputs, body, &inspected, &accepted); outcome != alphacontrol.OutcomeInvalid {
		t.Fatalf("altered release evidence outcome = %q", outcome)
	}
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
