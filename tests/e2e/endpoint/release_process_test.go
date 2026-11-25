package endpoint_test

import (
	"encoding/json"
	"errors"
	releasefixture "github.com/dianabuilds/ardents-network/tests/releasefixture"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseSyntaxRefusesBeforeHistoryEffects(t *testing.T) {
	binary := buildArdentsNext(t)
	root := filepath.Join(t.TempDir(), "absent")
	for _, args := range [][]string{
		{"release", "verify-initial"},
		{"release", "verify-initial", "missing", "pin", root, "invalid-time"},
		{"release", "unknown", "missing", "pin", root, "2030-01-02T03:04:05Z"},
	} {
		command := exec.CommandContext(t.Context(), binary, args...)
		output, err := command.Output()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 || string(output) != "{\"outcome\":\"invalid-input\"}\n" {
			t.Fatalf("%v: output=%q, error=%v", args, output, err)
		}
		if _, err := os.Lstat(root); !os.IsNotExist(err) {
			t.Fatal("invalid syntax opened history", err)
		}
	}
}

func currentReleasePlatform(protected bool) string {
	if protected {
		return "linux-amd64"
	}
	return runtime.GOOS + "-" + runtime.GOARCH
}

func signedReleaseProfile(t *testing.T, platform string, protected bool) (string, string, string) {
	t.Helper()
	return releasefixture.SignedRelease(t, buildArdentsNext(t), platform, protected, false, 1, nil, nil, 1, 1)
}

func TestReleaseCompiledForeignPlatformRefusesBeforeHistory(t *testing.T) {
	foreign := "windows-amd64"
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		// The host must be able to execute the fixture before it can refuse
		// the enrolled platform. Change the declared CPU but keep .exe.
		foreign = "windows-arm64"
	}
	directory, pin, program := signedReleaseProfile(t, foreign, false)
	history := filepath.Join(t.TempDir(), "absent-history")
	cmd := exec.CommandContext(t.Context(), program, "release", "verify-initial", directory, pin, history, "2030-01-02T03:04:05Z")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "release-incompatible") {
		t.Fatalf("signed foreign platform was not refused: %v: %s", err, out)
	}
	if _, err := os.Lstat(history); !os.IsNotExist(err) {
		t.Fatalf("foreign platform opened history: %v", err)
	}
}

func TestReleaseCompiledInitialVerificationAndRetainedRetry(t *testing.T) {
	t.Run("general", func(t *testing.T) { verifyCompiledReleaseProfile(t, false) })
	// Actual protected executable invocation is selected only where the
	// current linux-amd64 artifact contract applies. Its portable projection
	// is independently exercised on every native test platform above.
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		t.Run("protected", func(t *testing.T) { verifyCompiledReleaseProfile(t, true) })
	}
}

func verifyCompiledReleaseProfile(t *testing.T, protected bool) {
	t.Helper()
	directory, pin, program := signedReleaseProfile(t, currentReleasePlatform(protected), protected)
	history := filepath.Join(t.TempDir(), "release-history")
	for _, wanted := range []string{"release-accepted", "no-update"} {
		cmd := exec.CommandContext(t.Context(), program, "release", "verify-initial", directory, pin, history, "2030-01-02T03:04:05Z")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("compiled consumer refused %v: %s", err, out)
		}
		var result struct {
			Outcome     string `json:"outcome"`
			RootVersion int64  `json:"root_version"`
		}
		if err = json.Unmarshal(out, &result); err != nil || result.Outcome != wanted || result.RootVersion != 1 {
			t.Fatalf("actual consumer result: %s", out)
		}
	}
	cmd := exec.CommandContext(t.Context(), program, "release", "verify-initial", directory, strings.Repeat("0", 64), history, "2030-01-02T03:04:05Z")
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "enrollment-refused") {
		t.Fatalf("wrong first pin accepted: %s", out)
	}
}
