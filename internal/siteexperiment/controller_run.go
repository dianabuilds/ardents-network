package siteexperiment

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/experimentidentity"
	"github.com/dianabuilds/ardents-network/internal/experimentrun"
)

type runManifest struct {
	SchemaVersion        string   `json:"schema_version"`
	RunID                string   `json:"run_id"`
	SourceSHA256         string   `json:"source_sha256"`
	Images               []string `json:"image_ids"`
	OHTTPModule          string   `json:"ohttp_module"`
	DependencyClosureSHA string   `json:"dependency_closure_sha256"`
	R013ReceiptSHA       string   `json:"r013_receipt_sha256"`
	Topology             string   `json:"topology"`
	Schedule             string   `json:"schedule"`
}

// Run owns the complete fixed Gate C scenario and writes its terminal bounded
// evidence. Images are immutable inputs and are never built by this function.
func Run(ctx context.Context, identity experimentrun.Layout, applicationImage, toolImage, referenceImage, r013Receipt string) (evidenceDirectory string, runErr error) {
	if ctx == nil {
		return "", errors.New("gate C requires a context")
	}
	images, err := bindExperimentImages(applicationImage, toolImage, referenceImage)
	if err != nil {
		return "", err
	}
	runID, repositoryRoot, runDirectory, evidenceDirectory, err := identity.OwnedPaths(false, false)
	if err != nil {
		return "", err
	}
	prepared := false
	defer func() {
		if !prepared {
			runErr = errors.Join(runErr, cleanupGateCPreparation(identity))
		}
	}()
	for _, directory := range []string{runDirectory, evidenceDirectory} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return "", err
		}
	}
	if _, _, _, _, err := identity.OwnedPaths(true, true); err != nil {
		return "", err
	}
	prepared = true
	cleaned := false
	defer func() {
		if !cleaned {
			runErr = errors.Join(runErr, cleanupGateCRuntime(identity, runDirectory))
		}
	}()
	receiptSHA, err := validateR013Receipt(r013Receipt)
	if err != nil {
		return evidenceDirectory, err
	}
	sourceSHA, err := experimentidentity.SourceSHA256(repositoryRoot)
	if err != nil {
		return evidenceDirectory, err
	}
	if err := verifyReferenceImage(ctx, images.reference, sourceSHA); err != nil {
		return evidenceDirectory, err
	}
	dependencySHA, err := fileSHA256(filepath.Join(repositoryRoot, "go.sum"))
	if err != nil {
		return evidenceDirectory, err
	}
	sequence, allAttemptsClean := 0, true
	runAttempt := func(operation context.Context, fixture *authorityFixture, superseded *instanceCredential) error {
		sequence++
		err := runRouteAttempt(operation, identity, fixture, superseded, sequence, images, evidenceDirectory)
		allAttemptsClean = allAttemptsClean && attemptCleanupProven(evidenceDirectory, sequence)
		return err
	}
	runner := matrixRunner{
		positive: func(operation context.Context, _ int, _ uint64) error {
			fixture, err := newAuthorityFixture(runID, "gatec-network", time.Now(), rand.Reader)
			if err != nil {
				return err
			}
			return runAttempt(operation, fixture, nil)
		},
		failure: func(operation context.Context, name string) error {
			return runContractFailure(operation, name, evidenceDirectory)
		},
		migrate: func(operation context.Context, episode int) (migrationResult, error) {
			fixture, err := newAuthorityFixture(runID, "gatec-network", time.Now(), rand.Reader)
			if err != nil {
				return migrationResult{Episode: episode}, err
			}
			originalName, originalTarget := "site.reference", fixture.target
			oldNonce := make([]byte, 32)
			if _, err := rand.Read(oldNonce); err != nil {
				return migrationResult{Episode: episode}, err
			}
			oldDescriptor, err := fixture.signedDescriptor(oldNonce, time.Now().Add(10*time.Second))
			if err != nil {
				return migrationResult{Episode: episode}, err
			}
			if err := runAttempt(operation, fixture, nil); err != nil {
				return migrationResult{Episode: episode}, err
			}
			generationOneStopped := attemptCleanupProven(evidenceDirectory, sequence)
			oldCredential := fixture.credential
			if err := fixture.migrate(time.Now(), rand.Reader); err != nil {
				return migrationResult{Episode: episode}, err
			}
			oldRejected := fixture.target == originalTarget && originalName == "site.reference"
			if _, verifyErr := verifyDescriptor(oldDescriptor, fixture.servicePublic, fixture.runID, fixture.networkID, oldNonce, fixture.target, fixture.instanceGeneration, time.Now()); verifyErr == nil {
				oldRejected = false
			}
			err = runAttempt(operation, fixture, &oldCredential)
			oldRejected = oldRejected && supersededPublicationRejected(evidenceDirectory, sequence)
			return migrationResult{Episode: episode, GenerationOneStopped: generationOneStopped, GenerationTwoPassed: err == nil, OldInstanceRejected: oldRejected}, err
		},
	}
	if err := validateMatrixRunner(runner); err != nil {
		return evidenceDirectory, err
	}
	matrix := runFixedMatrix(ctx, runner)
	measurements, measurementErr := summarizeAttempts(evidenceDirectory)
	if (measurementErr != nil || measurements.Attempts != 30 || measurements.MaximumQueueBytes > 256*1024) && matrix.Verdict == "advance" {
		matrix = failMatrix(matrix, "resource observations are incomplete", false)
	}
	manifest := runManifest{
		SchemaVersion: "gatec-manifest/v1", RunID: runID, SourceSHA256: sourceSHA,
		Images: images.identities(), OHTTPModule: "github.com/openpcc/ohttp@v0.0.80",
		DependencyClosureSHA: dependencySHA, R013ReceiptSHA: receiptSHA,
		Topology: "reference-site/compose.yaml+carrier-lab/compose.yaml", Schedule: "20 positives; 17 failures; 5 migrations",
	}
	cleanupErr := cleanupGateCRuntime(identity, runDirectory)
	if !allAttemptsClean {
		cleanupErr = errors.Join(cleanupErr, errors.New("one or more Gate C attempts lack complete Docker cleanup evidence"))
	}
	cleaned = cleanupErr == nil
	bundleErr := writeGateCBundle(evidenceDirectory, manifest, matrix, measurements, cleaned, time.Now())
	return evidenceDirectory, errors.Join(cleanupErr, bundleErr)
}

func validateR013Receipt(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > 4*1024*1024 {
		return "", errors.New("bounded R-013 regression receipt is required")
	}
	var value struct {
		SchemaVersion       string `json:"schema_version"`
		Status              string `json:"status"`
		Decision            string `json:"decision"`
		InputManifestSHA256 string `json:"input_manifest_sha256"`
		ExperimentSHA256    string `json:"experiment_sha256"`
	}
	if json.Unmarshal(data, &value) != nil || value.SchemaVersion != "carrier-lab-route-experiment-receipt/v1" || value.Status != "completed" || value.Decision != "advance" || len(value.InputManifestSHA256) != 64 || len(value.ExperimentSHA256) != 64 {
		return "", errors.New("R-013 receipt is invalid")
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
