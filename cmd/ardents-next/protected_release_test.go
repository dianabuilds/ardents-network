package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProtectedReleaseRejectsInvalidArgumentsWithoutEffects(t *testing.T) {
	var output bytes.Buffer
	if code := runRelease(t.Context(), []string{"authenticate-protected-initial"}, &output); code != 2 {
		t.Fatal("incomplete authentication was not refused", code, output.String())
	}
	output.Reset()
	history := filepath.Join(t.TempDir(), "history")
	args := []string{"authenticate-protected-initial", "/absent", strings.Repeat("0", 64), history, "2030-01-02T03:04:05Z"}
	if code := runRelease(t.Context(), args, &output); code != 2 {
		t.Fatal("detached initial authentication was not refused", code, output.String())
	}
	if _, err := os.Lstat(history); !os.IsNotExist(err) {
		t.Fatal("detached initial refusal created history", err)
	}
}

func TestDetachedReleaseRoutesAreRetiredWithoutEffects(t *testing.T) {
	root := t.TempDir()
	history := filepath.Join(root, "history")
	for _, operation := range []string{"authenticate-source-candidate", "authenticate-forwarding-candidate", "authenticate-join-candidate", "authenticate-descriptor-candidate", "authenticate-introduction-candidate", "authenticate-protected-candidate"} {
		var output bytes.Buffer
		if code := runRelease(t.Context(), []string{operation, filepath.Join(root, "bundle"), history, "closed", "network", "2030-01-02T03:04:05Z"}, &output); code != 2 {
			t.Fatalf("retired operation %s accepted: %d %s", operation, code, output.String())
		}
		if _, err := os.Lstat(history); !os.IsNotExist(err) {
			t.Fatalf("retired operation %s changed history: %v", operation, err)
		}
	}
}
