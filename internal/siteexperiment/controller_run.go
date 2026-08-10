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

const gateCRunSchema = "gatec-run/v1"

type runManifest struct {
	SchemaVersion  string       `json:"schema_version"`
	RunID          string       `json:"run_id"`
	SourceSHA256   string       `json:"source_sha256"`
	Images         []string     `json:"image_ids"`
	OHTTPModule    string       `json:"ohttp_module"`
	R013ReceiptSHA string       `json:"r013_receipt_sha256"`
	Topology       string       `json:"topology"`
	Matrix         matrixResult `json:"matrix"`
	Cleanup        bool         `json:"cleanup_complete"`
	CompletedUnix  int64        `json:"completed_unix"`
}

// Run owns the complete fixed Gate C scenario and writes its terminal bounded
// evidence. Images are immutable inputs and are never built by this function.
func Run(ctx context.Context, identity experimentrun.Layout, applicationImage, toolImage, referenceImage, r013Receipt string) (string, error) {
	if ctx == nil || !validImageID(applicationImage) || !validImageID(toolImage) || !validImageID(referenceImage) {
		return "", errors.New("gate C requires context and three immutable image IDs")
	}
	runID, repositoryRoot, runDirectory, evidenceDirectory, err := identity.OwnedPaths(false, false)
	if err != nil {
		return "", err
	}
	for _, directory := range []string{runDirectory, evidenceDirectory} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return "", err
		}
	}
	if _, _, _, _, err := identity.OwnedPaths(true, true); err != nil {
		return "", err
	}
	receiptSHA, err := validateR013Receipt(r013Receipt)
	if err != nil {
		return evidenceDirectory, err
	}
	sourceSHA, err := experimentidentity.SourceSHA256(repositoryRoot)
	if err != nil {
		return evidenceDirectory, err
	}
	if err := verifyReferenceImage(ctx, referenceImage, sourceSHA); err != nil {
		return evidenceDirectory, err
	}
	sequence := 0
	runner := matrixRunner{
		positive: func(operation context.Context, _ int, _ uint64) error {
			sequence++
			fixture, err := newAuthorityFixture(runID, "gatec-network", time.Now(), rand.Reader)
			if err != nil {
				return err
			}
			return runRouteAttempt(operation, identity, fixture, sequence, applicationImage, toolImage, referenceImage, evidenceDirectory)
		},
		failure: runContractFailure,
		migrate: func(operation context.Context, episode int) (migrationResult, error) {
			fixture, err := newAuthorityFixture(runID, "gatec-network", time.Now(), rand.Reader)
			if err != nil {
				return migrationResult{Episode: episode}, err
			}
			sequence++
			if err := runRouteAttempt(operation, identity, fixture, sequence, applicationImage, toolImage, referenceImage, evidenceDirectory); err != nil {
				return migrationResult{Episode: episode}, err
			}
			oldKey := append([]byte(nil), fixture.instancePublic...)
			if err := fixture.migrate(time.Now(), rand.Reader); err != nil {
				return migrationResult{Episode: episode}, err
			}
			sequence++
			err = runRouteAttempt(operation, identity, fixture, sequence, applicationImage, toolImage, referenceImage, evidenceDirectory)
			return migrationResult{Episode: episode, GenerationOneStopped: true, GenerationTwoPassed: err == nil, OldInstanceRejected: string(oldKey) != string(fixture.instancePublic)}, err
		},
	}
	if err := validateMatrixRunner(runner); err != nil {
		return evidenceDirectory, err
	}
	matrix := runFixedMatrix(ctx, runner)
	manifest := runManifest{
		SchemaVersion: gateCRunSchema, RunID: runID, SourceSHA256: sourceSHA,
		Images: []string{applicationImage, toolImage, referenceImage}, OHTTPModule: "github.com/openpcc/ohttp@v0.0.80",
		R013ReceiptSHA: receiptSHA, Topology: "reference-site/compose.yaml+carrier-lab/compose.yaml", Matrix: matrix,
		CompletedUnix: time.Now().Unix(),
	}
	cleanupErr := cleanupGateCRuntime(identity, runDirectory)
	manifest.Cleanup = cleanupErr == nil
	writeErr := writeBoundedJSON(filepath.Join(evidenceDirectory, "result.json"), manifest)
	receiptErr := writeBoundedJSON(filepath.Join(evidenceDirectory, "receipt.json"), map[string]any{
		"schema_version": gateCRunSchema, "run_id": runID, "source_sha256": sourceSHA, "verdict": matrix.Verdict,
	})
	return evidenceDirectory, errors.Join(cleanupErr, writeErr, receiptErr)
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
