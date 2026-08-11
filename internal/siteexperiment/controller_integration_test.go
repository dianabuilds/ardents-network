package siteexperiment

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/experimentrun"
)

func TestReferenceTopologyCarriesOneAuthenticatedWorkload(t *testing.T) {
	applicationImage := os.Getenv("GATEC_TEST_APPLICATION_IMAGE")
	toolImage := os.Getenv("GATEC_TEST_TOOL_IMAGE")
	referenceImage := os.Getenv("GATEC_TEST_REFERENCE_IMAGE")
	if applicationImage == "" || toolImage == "" || referenceImage == "" {
		t.Skip("set the three GATEC_TEST_*_IMAGE variables to run the Docker integration test")
	}
	images, err := bindExperimentImages(applicationImage, toolImage, referenceImage)
	if err != nil {
		t.Fatal(err)
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	runID := "gatec-integration-" + time.Now().UTC().Format("20060102t150405")
	root := filepath.Join(os.TempDir(), experimentrun.SessionPrefix+runID)
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	identity, err := experimentrun.New(root, repositoryRoot, os.TempDir(), runID)
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := newAuthorityFixture(runID, "gatec-network", time.Now(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, _, runDirectory, evidence, err := identity.OwnedPaths(false, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{runDirectory, evidence} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var stages []string
	if err := runRouteAttempt(ctx, identity, fixture, nil, 1, images, evidence, func(stage string) error {
		stages = append(stages, stage)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	wantStages := []string{"reference-preparing", "reference-compose-started", "reference-sockets-ready", "reference-published", "reference-isolation-verified", "native-route-running", "retaining-evidence"}
	if !slices.Equal(stages, wantStages) {
		t.Fatalf("progress stages=%v, want %v", stages, wantStages)
	}
	failureFixture, err := newAuthorityFixture(runID, "gatec-network", time.Now(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	wantProgressErr := errors.New("progress write failed after Compose start")
	failureContext, failureCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer failureCancel()
	err = runRouteAttempt(failureContext, identity, failureFixture, nil, 2, images, evidence, func(stage string) error {
		if stage == "reference-compose-started" {
			return wantProgressErr
		}
		return nil
	})
	if !errors.Is(err, wantProgressErr) || !errors.Is(err, errMatrixOperational) || !attemptCleanupProven(evidence, 2) {
		t.Fatalf("post-Compose progress failure did not clean up: %v", err)
	}
}

func TestReferenceTopologyRejectsSupersededPublicationDuringMigration(t *testing.T) {
	applicationImage := os.Getenv("GATEC_TEST_APPLICATION_IMAGE")
	toolImage := os.Getenv("GATEC_TEST_TOOL_IMAGE")
	referenceImage := os.Getenv("GATEC_TEST_REFERENCE_IMAGE")
	if applicationImage == "" || toolImage == "" || referenceImage == "" {
		t.Skip("set the three GATEC_TEST_*_IMAGE variables to run the Docker integration test")
	}
	images, err := bindExperimentImages(applicationImage, toolImage, referenceImage)
	if err != nil {
		t.Fatal(err)
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	runID := "gatec-migration-" + time.Now().UTC().Format("20060102t150405")
	session := filepath.Join(os.TempDir(), experimentrun.SessionPrefix+runID)
	if err := os.MkdirAll(session, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(session) })
	identity, err := experimentrun.New(session, repositoryRoot, os.TempDir(), runID)
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := newAuthorityFixture(runID, "gatec-network", time.Now(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, _, runDirectory, evidence, err := identity.OwnedPaths(false, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{runDirectory, evidence} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	target, oldCredential := fixture.target, fixture.credential
	if err := runRouteAttempt(ctx, identity, fixture, nil, 1, images, evidence, nil); err != nil {
		t.Fatal(err)
	}
	if !attemptCleanupProven(evidence, 1) {
		t.Fatal("generation one did not stop cleanly")
	}
	if err := fixture.migrate(time.Now(), rand.Reader); err != nil {
		t.Fatal(err)
	}
	if fixture.target != target || fixture.instanceGeneration != 2 {
		t.Fatal("migration changed Target or skipped generation")
	}
	if err := runRouteAttempt(ctx, identity, fixture, &oldCredential, 2, images, evidence, nil); err != nil {
		t.Fatal(err)
	}
	if !supersededPublicationRejected(evidence, 2) {
		t.Fatal("generation two did not reject the old publication handle")
	}
}
