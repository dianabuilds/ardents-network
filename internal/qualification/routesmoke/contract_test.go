package routesmoke_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/routesmoke"
)

func TestRunRejectsIncompleteCampaignBeforeDocker(t *testing.T) {
	result := routesmoke.Run(context.Background(), routesmoke.Config{})
	if result.Verdict != "invalid" || !strings.Contains(result.Reason, "route smoke evidence root") {
		t.Fatalf("incomplete campaign = %+v", result)
	}
}

func TestRunRejectsNestedEvidenceBeforeCreatingFixture(t *testing.T) {
	source, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(source, "route-smoke-contract-fixture-do-not-create")
	result := routesmoke.Run(context.Background(), routesmoke.Config{FixtureRoot: fixture,
		EvidenceRoot: filepath.Join(fixture, "evidence"), ComposeFile: filepath.Join(source, "compose.yaml"),
		SourceRoot: source, Duration: 10 * time.Minute})
	if result.Verdict != "invalid" {
		t.Fatalf("nested roots = %+v", result)
	}
	if _, err := os.Stat(fixture); !os.IsNotExist(err) {
		t.Fatalf("nested fixture was created: %v", err)
	}
}
