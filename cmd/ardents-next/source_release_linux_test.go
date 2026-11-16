package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
