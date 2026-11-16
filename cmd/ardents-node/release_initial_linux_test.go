package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
)

func TestInitialInfrastructureReleaseRefusesBeforeHistoryCreation(t *testing.T) {
	root := t.TempDir()
	bundle, history := filepath.Join(root, "missing-bundle"), filepath.Join(root, "history")
	args := []string{"authenticate-initial", bundle, strings.Repeat("0", 64), history, "closed", "network"}
	var out bytes.Buffer
	err := runInitialInfrastructureRelease(t.Context(), args, &out)
	if err == nil || out.Len() != 0 {
		t.Fatalf("unverified initial release accepted: %v %s", err, out.String())
	}
	if runtime.GOARCH != "amd64" && !errors.Is(err, releaseartifact.ErrInput) {
		t.Fatalf("incompatible architecture did not refuse: %v", err)
	}
	if _, err := os.Lstat(history); !os.IsNotExist(err) {
		t.Fatal("refusal created Release history", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := runInitialInfrastructureRelease(ctx, args, &out); err == nil || out.Len() != 0 {
		t.Fatal("canceled initial release accepted")
	}
}

func TestInfrastructureReleasePathsRefuseOverlapAndSymlink(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "bundle")
	if err := os.Mkdir(bundle, 0700); err != nil {
		t.Fatal(err)
	}
	if independentReleasePaths(bundle, filepath.Join(bundle, "history")) {
		t.Fatal("overlapping history accepted")
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(bundle, alias); err != nil {
		t.Fatal(err)
	}
	if independentReleasePaths(alias, filepath.Join(root, "history")) {
		t.Fatal("symlinked bundle accepted")
	}
	if !independentReleasePaths(bundle, filepath.Join(root, "history")) {
		t.Fatal("independent paths refused")
	}
}
