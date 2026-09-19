//go:build linux

package endpoint

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
)

func TestQualificationPreflightPreparesIdempotentPrivateRoots(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	config := TextParticipantConfig{
		LocalRoleRoot: filepath.Join(root, "roles"),
		TokenRoot:     filepath.Join(root, "tokens"),
		Clock:         func() time.Time { return now },
	}
	for range 2 {
		if err := prepareStreamQualificationParticipantRoots(config); err != nil {
			t.Fatal(err)
		}
	}
	roles, err := duty.Open(duty.Config{Root: config.LocalRoleRoot, Clock: config.Clock})
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(config.TokenRoot)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("token root is not a private directory: %v / %v", info, err)
	}
	entries, err := os.ReadDir(config.TokenRoot)
	if err != nil || len(entries) != 0 {
		t.Fatalf("preflight root preparation wrote token state: %v / %v", entries, err)
	}
}
