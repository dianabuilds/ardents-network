package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
)

func TestSourceReleaseReceiptPreservesPublicResult(t *testing.T) {
	var out bytes.Buffer
	// This exercises only the detached report format, not Release authentication.
	err := writeSourceReleaseReceipt(context.Background(), &out, releaseartifact.Authorization{}, release.Decision{Outcome: release.OutcomeReleaseAccepted}, release.Decision{Outcome: release.OutcomeNoUpdate})
	if err != nil {
		t.Fatal(err)
	}
	const emptyDigest = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	want := `{"outcome":"authenticated-source-artifact","program_outcome":"release-accepted","generation_outcome":"no-update","program_sha256":"` + emptyDigest + `","descriptor_sha256":"` + emptyDigest + `","template_sha256":"` + emptyDigest + `"}` + "\n"
	if out.String() != want {
		t.Fatalf("unexpected Source release receipt: %s", out.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out.Reset()
	if writeSourceReleaseReceipt(ctx, &out, releaseartifact.Authorization{}, release.Decision{}, release.Decision{}) == nil || out.Len() != 0 {
		t.Fatal("canceled receipt was published")
	}
}

func TestSourceReleaseRefusesMissingInventoryBeforeHistory(t *testing.T) {
	for _, initial := range []bool{true, false} {
		t.Run(map[bool]string{true: "initial", false: "candidate"}[initial], func(t *testing.T) {
			root := t.TempDir()
			bundle := filepath.Join(root, "absent-bundle")
			history := filepath.Join(root, "absent-history")
			args := []string{"authenticate-candidate", bundle, history, "closed", "network", "2030-01-02T03:04:05Z"}
			if initial {
				args = []string{"authenticate-initial", bundle, strings.Repeat("0", 64), history, "closed", "network", "2030-01-02T03:04:05Z"}
			}
			request, invalid := parseSourceRelease(args)
			if invalid != "" {
				t.Fatal("valid declarations refused", invalid)
			}
			if request.initial != initial || request.bundle != bundle || request.history != history || request.environment != "closed" || request.network != "network" || request.reference.Format(time.RFC3339) != "2030-01-02T03:04:05Z" {
				t.Fatal("Source release fields changed", request)
			}
			if initial && request.pin != strings.Repeat("0", 64) || !initial && request.pin != "" {
				t.Fatal("initial pin crossed the candidate boundary")
			}
			a, v, refused, err := openSourceRelease(t.Context(), request)
			if err == nil || refused != "infrastructure-inventory-refused" || v != nil || a.HasInventory() {
				t.Fatal("missing inventory was accepted", refused, err)
			}
			if _, err := os.Lstat(history); !os.IsNotExist(err) {
				t.Fatal("refusal created Release history", err)
			}
		})
	}
}

func TestSourceReleaseInputRefusesOverlappingAndIncompletePaths(t *testing.T) {
	root := t.TempDir()
	args := []string{"authenticate-candidate", filepath.Join(root, "bundle"), filepath.Join(root, "bundle", "history"), "closed", "network", "2030-01-02T03:04:05Z"}
	if _, outcome := parseSourceRelease(args); outcome != "infrastructure-paths-refused" {
		t.Fatal("overlapping history accepted", outcome)
	}
	for _, invalid := range [][]string{nil, args[:len(args)-1], {"unknown", "a", "b", "c", "d", "2030-01-02T03:04:05Z"}} {
		if _, outcome := parseSourceRelease(invalid); outcome != "invalid-input" {
			t.Fatal("invalid source arguments accepted", outcome)
		}
	}
}
