package namedsite

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type runResult struct {
	SchemaVersion string             `json:"schema_version"`
	RunID         string             `json:"run_id"`
	SourceSHA256  string             `json:"source_sha256"`
	Matrix        matrixResult       `json:"matrix"`
	Measurements  measurementSummary `json:"measurements"`
	Cleanup       bool               `json:"cleanup_complete"`
	CompletedUnix int64              `json:"completed_unix"`
}

func writeGateCBundle(directory string, manifest runManifest, matrix matrixResult, measurements measurementSummary, cleanup bool, completed time.Time) error {
	result := runResult{
		SchemaVersion: "gatec-result/v1", RunID: manifest.RunID, SourceSHA256: manifest.SourceSHA256,
		Matrix: matrix, Measurements: measurements, Cleanup: cleanup, CompletedUnix: completed.Unix(),
	}
	manifestPath := filepath.Join(directory, "manifest.json")
	resultPath := filepath.Join(directory, "result.json")
	if err := writeBoundedJSON(manifestPath, manifest); err != nil {
		return err
	}
	if err := writeBoundedJSON(resultPath, result); err != nil {
		return err
	}
	if err := writeBoundedJSON(filepath.Join(directory, "migration.json"), map[string]any{
		"schema_version": "gatec-migration-evidence/v1", "episodes": matrix.Migrations,
	}); err != nil {
		return err
	}
	if err := writeRoleViewSummary(directory); err != nil {
		return err
	}
	manifestSHA, manifestErr := fileSHA256(manifestPath)
	resultSHA, resultErr := fileSHA256(resultPath)
	if manifestErr != nil || resultErr != nil {
		return errors.Join(manifestErr, resultErr)
	}
	if err := writeBoundedJSON(filepath.Join(directory, "receipt.json"), map[string]any{
		"schema_version": "gatec-receipt/v1", "run_id": manifest.RunID, "source_sha256": manifest.SourceSHA256,
		"manifest_sha256": manifestSHA, "result_sha256": resultSHA, "verdict": matrix.Verdict,
	}); err != nil {
		return err
	}
	report := fmt.Sprintf("# Gate C terminal report\n\nRun: `%s`\n\nVerdict: **%s**\n\nPositive attempts: %d/%d. Required failures: %d/%d. Migration episodes: %d/5. Cleanup complete: %t.\n\nThis is bounded laboratory evidence, not a public privacy, anonymity, availability, or Route Qualification claim.\n",
		manifest.RunID, matrix.Verdict, matrix.PositivePassed, matrix.PositiveTotal, passedFailures(matrix.Failures), len(fixedFailureCases), len(matrix.Migrations), cleanup)
	if len(report) > 64*1024 {
		return errors.New("gate C report exceeds its bound")
	}
	return os.WriteFile(filepath.Join(directory, "report.md"), []byte(report), 0o600)
}

func writeRoleViewSummary(directory string) error {
	reference := filepath.Join(directory, "attempts", "001", "reference")
	var relay, gateway, isolation map[string]any
	for path, destination := range map[string]*map[string]any{
		filepath.Join(reference, "relay", "relay.json"):     &relay,
		filepath.Join(reference, "gateway", "gateway.json"): &gateway,
		filepath.Join(reference, "isolation.json"):          &isolation,
	} {
		if err := readStrictEvidence(path, destination); err != nil {
			return err
		}
	}
	return writeBoundedJSON(filepath.Join(directory, "role-views.json"), map[string]any{
		"schema_version": "gatec-role-view-summary/v1", "relay": relay, "gateway": gateway, "isolation": isolation,
	})
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > 16*1024*1024 {
		return "", errors.New("bounded file required for SHA-256")
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func passedFailures(failures map[string]bool) int {
	passed := 0
	for _, value := range failures {
		if value {
			passed++
		}
	}
	return passed
}
