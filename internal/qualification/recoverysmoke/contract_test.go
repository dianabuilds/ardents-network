package recoverysmoke

import (
	"path/filepath"
	"testing"
	"time"
)

func TestParseConfigFreezesRecoveryBounds(t *testing.T) {
	value, err := parseConfig([]string{"--fixture", "fixture", "--evidence", "evidence"})
	if err != nil || value.Bytes != 4<<20 || value.ChunkDelay != "8ms" {
		t.Fatalf("config=%+v err=%v", value, err)
	}
}

func TestRecoveryDeadlineScopesFailureDetectionToRendezvous(t *testing.T) {
	for _, role := range []string{"client", "initiator", "introduction", "responder", "publisher"} {
		if got := recoveryRouteDeadline(role); got != "15s" {
			t.Fatalf("role %s deadline=%s, want 15s", role, got)
		}
	}
	if got := recoveryRouteDeadline("rendezvous"); got != "4s" {
		t.Fatalf("rendezvous deadline=%s, want 4s", got)
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
