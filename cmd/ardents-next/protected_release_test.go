package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
		{"release", func(ctx context.Context, args []string, out *bytes.Buffer) int {
			return runRelease(ctx, append([]string{"authenticate-protected-candidate"}, args...), out)
		}, "release-refused"},
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
}

func TestProtectedReleaseCancellationPrecedesHistory(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	parent := t.TempDir()
	history := filepath.Join(parent, "history")
	var output bytes.Buffer
	code := runRelease(ctx, []string{"authenticate-protected-candidate", filepath.Join(parent, "bundle"), history, "2030-01-02T03:04:05Z"}, &output)
	var result struct {
		Outcome string `json:"outcome"`
	}
	if code != 130 || json.Unmarshal(output.Bytes(), &result) != nil || result.Outcome != "release-canceled" {
		t.Fatalf("cancellation was not retained: %d %s", code, output.String())
	}
	if _, err := os.Lstat(history); !os.IsNotExist(err) {
		t.Fatal("canceled authentication created Release history", err)
	}
}
