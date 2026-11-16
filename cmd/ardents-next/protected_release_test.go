package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProtectedReleaseRefusesMissingCandidateWithoutCreatingHistory(t *testing.T) {
	parent := t.TempDir()
	candidate := filepath.Join(parent, "missing-bundle")
	history := filepath.Join(parent, "missing-release-history")
	for _, test := range []struct {
		name string
		run  func(context.Context, []string, *bytes.Buffer) int
		want string
	}{
		{"legacy", func(ctx context.Context, args []string, out *bytes.Buffer) int {
			return runInstallation(ctx, append([]string{"authenticate-candidate"}, args...), out)
		}, "installation-refused"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			code := test.run(t.Context(), []string{candidate, history, "2030-01-02T03:04:05Z"}, &output)
			var result struct {
				Outcome string `json:"outcome"`
			}
			if code != 1 || json.Unmarshal(output.Bytes(), &result) != nil || result.Outcome != test.want {
				t.Fatalf("wrong refusal: %d %s", code, output.String())
			}
			if _, err := os.Lstat(history); !os.IsNotExist(err) {
				t.Fatal("refusal created Release history", err)
			}
		})
	}
}

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

func TestProtectedReleaseCancellationPrecedesHistory(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	parent := t.TempDir()
	history := filepath.Join(parent, "history")
	var output bytes.Buffer
	code := runInstallation(ctx, []string{"authenticate-candidate", filepath.Join(parent, "bundle"), history, "2030-01-02T03:04:05Z"}, &output)
	var result struct {
		Outcome string `json:"outcome"`
	}
	if code != 130 || json.Unmarshal(output.Bytes(), &result) != nil || result.Outcome != "installation-canceled" {
		t.Fatalf("cancellation was not retained: %d %s", code, output.String())
	}
	if _, err := os.Lstat(history); !os.IsNotExist(err) {
		t.Fatal("canceled authentication created Release history", err)
	}
}
