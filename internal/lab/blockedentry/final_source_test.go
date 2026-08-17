package blockedentry

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMaterializeCommittedSourceExcludesWorktreeSubstitution(t *testing.T) {
	workspace := t.TempDir()
	runGit(t, workspace, "init")
	runGit(t, workspace, "config", "user.email", "stage5@example.invalid")
	runGit(t, workspace, "config", "user.name", "Stage 5 Test")
	tracked := filepath.Join(workspace, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("committed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, workspace, "add", "tracked.txt")
	runGit(t, workspace, "-c", "commit.gpgsign=false", "commit", "-m", "fixture")
	if err := os.WriteFile(filepath.Join(workspace, "untracked.txt"), []byte("substitution\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	commit, digest, source, temporary, err := materializeCommittedSource(workspace)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(temporary) })
	if !hexDigest(commit, 20) || !hexDigest(digest, 32) {
		t.Fatalf("commit=%q digest=%q", commit, digest)
	}
	if _, err := os.Stat(filepath.Join(source, "untracked.txt")); !os.IsNotExist(err) {
		t.Fatalf("untracked substitution entered frozen source: %v", err)
	}
	if err := os.WriteFile(tracked, []byte("changed after freeze\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	frozen, err := os.ReadFile(filepath.Join(source, "tracked.txt"))
	if err != nil || strings.TrimSpace(string(frozen)) != "committed" {
		t.Fatalf("frozen source changed with worktree: %q %v", frozen, err)
	}
}

func TestCopyBoundedArchiveStopsAtLimit(t *testing.T) {
	var output bytes.Buffer
	written, overflow, err := copyBoundedArchive(&output, strings.NewReader("123456"), 5)
	if err != nil || !overflow || written != 6 || output.String() != "123456" {
		t.Fatalf("bounded copy=(%d,%v,%v,%q)", written, overflow, err, output.String())
	}
}

func runGit(t *testing.T, workspace string, arguments ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", append([]string{"-C", workspace}, arguments...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(arguments, " "), err, output)
	}
}
