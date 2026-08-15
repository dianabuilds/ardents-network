package recoverysmoke

import (
	"path/filepath"
	"testing"
	"time"
)

func TestParseConfigFreezesRecoveryBounds(t *testing.T) {
	value, err := parseConfig([]string{"--fixture", "fixture", "--evidence", "evidence"})
	if err != nil || value.Bytes != 4<<20 || value.ChunkDelay != "20ms" {
		t.Fatalf("config=%+v err=%v", value, err)
	}
}

func TestRunRejectsUnsafeRootsBeforeCreatingFixture(t *testing.T) {
	root := t.TempDir()
	result := run(config{FixtureRoot: filepath.Join(root, "fixture"), EvidenceRoot: filepath.Join(root, "fixture", "evidence"),
		ComposeFile: filepath.Join(root, "compose.yaml"), SourceRoot: root, Duration: 10 * time.Minute})
	if result.Verdict != "invalid" {
		t.Fatalf("unsafe roots verdict=%+v", result)
	}
}

func TestExecuteRejectsUnexpectedPositionals(t *testing.T) {
	if _, err := Execute([]string{"unexpected"}); err == nil {
		t.Fatal("unexpected positional argument was accepted")
	}
}

func TestParseConfigSelectsOnlyAnImplementedRecoverySlice(t *testing.T) {
	value, err := parseConfig([]string{"--slice", "s4.2", "--s4.1-evidence", "s41.json",
		"--stage3-evidence", "s3.json"})
	if err != nil || value.Slice != "s4.2" {
		t.Fatalf("slice config=%+v err=%v", value, err)
	}
	if _, err := parseConfig([]string{"--slice", "s4.4"}); err == nil {
		t.Fatal("unauthorized recovery slice was accepted")
	}
	if _, err := parseConfig([]string{"--slice", "s4.2"}); err == nil {
		t.Fatal("S4.2 accepted missing prerequisite receipts")
	}
}
