package preflight

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func writeEvidence(layout RunLayout, record manifest, cleanup cleanupReport) error {
	if record.RunID != layout.runID || cleanup.RunID != layout.runID {
		return errors.New("evidence run ID does not match the run layout")
	}
	if err := layout.validateOwnedPaths(false, true); err != nil {
		return fmt.Errorf("validate run layout before evidence write: %w", err)
	}
	manifestPath := filepath.Join(layout.evidenceDir, manifestFilename)
	if err := writeJSON(manifestPath, record); err != nil {
		return err
	}
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(manifestData)
	result := verdict{
		SchemaVersion:  verdictSchemaVersion,
		RunID:          record.RunID,
		Status:         record.Status,
		ManifestSHA256: hex.EncodeToString(digest[:]),
		FailureReasons: record.FailureReasons,
		Cleanup:        record.Cleanup,
	}
	if err := writeJSON(filepath.Join(layout.evidenceDir, verdictFilename), result); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(layout.evidenceDir, cleanupFilename), cleanup); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(layout.evidenceDir, humanFilename), []byte(humanReport(record, cleanup)), 0o600)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(data, '\n'), 0o600)
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return errors.New("refusing to replace a non-regular evidence file")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".preflight-evidence-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	removeTemporary = false
	return nil
}

func humanReport(record manifest, cleanup cleanupReport) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# Carrier Lab preflight report\n\nRun: `%s`  \nSeed: `%s`  \nStatus: **%s**\n\n", record.RunID, record.Seed, record.Status)
	if len(record.FailureReasons) > 0 {
		builder.WriteString("## Failure reasons\n\n")
		for _, reason := range record.FailureReasons {
			fmt.Fprintf(&builder, "- `%s`: %s\n", reason.Code, reason.Message)
		}
		builder.WriteString("\n")
	}
	fmt.Fprintf(&builder, "Pinned image manifest: `%s`  \nPinned Go archive SHA-256: `%s`\n\n", record.Image.ObservedManifestDigest, record.Toolchain.ObservedArchiveSHA256)
	fmt.Fprintf(&builder, "Cleanup: **%s**. See `%s` for the machine-readable cleanup result.\n\n", cleanup.Status, cleanupFilename)
	builder.WriteString("This preflight did not execute a Route, topology, network protocol, TLS/HPKE flow, or production code. It makes no compatibility, privacy, anonymity, security, availability, or production claim.\n")
	return builder.String()
}

func readManifest(layout RunLayout) (manifest, error) {
	if err := layout.validateOwnedPaths(false, true); err != nil {
		return manifest{}, err
	}
	path := filepath.Join(layout.evidenceDir, manifestFilename)
	info, err := os.Lstat(path)
	if err != nil {
		return manifest{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return manifest{}, errors.New("manifest must be a regular file, not a symlink")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest{}, err
	}
	var record manifest
	if err := json.Unmarshal(data, &record); err != nil {
		return manifest{}, err
	}
	if record.SchemaVersion != manifestSchemaVersion {
		return manifest{}, errors.New("cannot finalize an unknown manifest schema")
	}
	return record, nil
}
