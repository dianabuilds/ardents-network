package nativecircuit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/preflight"
)

func TestDockerProfilesReuseFrozenImages(t *testing.T) {
	applicationImage := os.Getenv("CARRIER_LAB_APPLICATION_IMAGE")
	toolImage := os.Getenv("CARRIER_LAB_TOOL_IMAGE")
	if applicationImage == "" || toolImage == "" {
		t.Skip("set immutable Carrier Lab image IDs to run the Docker integration test")
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	temporaryRoot := t.TempDir()
	for _, profile := range []string{workloadDirect, workloadC3, workloadC5} {
		t.Run(profile, func(t *testing.T) {
			runID := "integration-" + profile + "-" + time.Now().UTC().Format("20060102T150405.000000000")
			sessionRoot := filepath.Join(temporaryRoot, "ardents-carrier-lab-preflight-session."+runID)
			if err := os.Mkdir(sessionRoot, 0o700); err != nil {
				t.Fatal(err)
			}
			layout, err := preflight.NewRunLayout(sessionRoot, repositoryRoot, temporaryRoot, runID)
			if err != nil {
				t.Fatal(err)
			}
			workload := nativeWorkload{
				SchemaVersion: nativeWorkloadSchema, Profile: profile, Kind: "setup",
				Seed: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
			}
			data, err := json.Marshal(workload)
			if err != nil {
				t.Fatal(err)
			}
			workloadPath := filepath.Join(sessionRoot, "workload.json")
			if err := os.WriteFile(workloadPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := RunWorkload(context.Background(), layout, applicationImage, toolImage, workloadPath); err != nil {
				t.Fatal(err)
			}
		})
	}
}
