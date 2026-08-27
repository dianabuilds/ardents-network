package replacement

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/release"
)

func TestCommitBindsAcceptedProgramAndVerifyRejectsSubstitution(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the Linux replacement profile")
	}
	if err := requireLinux(); err != nil {
		t.Skip(err)
	}
	root := t.TempDir()
	program := filepath.Join(root, "ardents")
	artifact := []byte("accepted successor program bytes")
	if err := os.WriteFile(program, artifact, 0o700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(artifact)
	decision := release.Decision{Outcome: release.OutcomeReleaseAccepted, BuildSafety: release.OutcomeReleaseAccepted,
		Protocol: release.OutcomeReleaseAccepted, Path: "ardents/linux-amd64/ardents", Length: int64(len(artifact)),
		Digest: digest[:], Platform: "linux-amd64", Architecture: "amd64", Environment: "h4-alpha", Network: "ardents-alpha",
		ReleaseIdentity: "h4-1b-test", ReleaseVersion: 2, ReferenceTime: time.Unix(1, 0).UTC()}
	stateRoot := filepath.Join(root, "state", "replacement")
	record, err := Prepare(context.Background(), Request{StateRoot: stateRoot, Artifact: artifact, decision: decision})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if record.Digest != digest || record.ReleaseVersion != 2 {
		t.Fatalf("Prepare() record = %+v", record)
	}
	running, err := VerifyPreparedRunning(stateRoot, program)
	if err != nil || running.State != StatePrepared || running.Record.Digest != digest {
		t.Fatalf("VerifyPreparedRunning() = %+v, %v", running, err)
	}
	if _, err := CommitPrepared(stateRoot, program); err != nil {
		t.Fatalf("CommitPrepared() error = %v", err)
	}
	running, err = VerifyRunning(stateRoot, program)
	if err != nil || running.State != StateCurrent || running.Record.Digest != digest {
		t.Fatalf("VerifyRunning() = %+v, %v", running, err)
	}
	if err := os.WriteFile(program, []byte("substituted program bytes"), 0o700); err != nil {
		t.Fatal(err)
	}
	running, err = VerifyRunning(stateRoot, program)
	if err != nil || running.State != StateMismatch {
		t.Fatalf("VerifyRunning() after substitution = %+v, %v", running, err)
	}
}

func TestVerifyRunningTreatsAbsentRootAsFirstEnrollmentOnly(t *testing.T) {
	if err := requireLinux(); err != nil {
		t.Skip(err)
	}
	program := filepath.Join(t.TempDir(), "ardents")
	if err := os.WriteFile(program, []byte("first program"), 0o700); err != nil {
		t.Fatal(err)
	}
	running, err := VerifyRunning(filepath.Join(t.TempDir(), "absent"), program)
	if err != nil || running.State != StateUnbound {
		t.Fatalf("VerifyRunning() = %+v, %v", running, err)
	}
}
