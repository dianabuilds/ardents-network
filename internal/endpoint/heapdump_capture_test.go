//go:build heapdumpcapture

package endpoint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHeapDumpObservation(t *testing.T) {
	root, output := os.Getenv("ARDENTS_HEAPDUMP_INPUT_ROOT"), os.Getenv("ARDENTS_HEAPDUMP_REPORT")
	if root == "" && output == "" {
		return
	}
	if !filepath.IsAbs(root) || !filepath.IsAbs(output) {
		t.Fatal("heap dump input root and report path must be absolute")
	}
	selected := filepath.Join("ardents-carrier-quic-v2-2245479829", "05-resolution", "published.heap")
	receiptHash, targetPresenceHash, err := verifyObservationInputs(root, selected)
	if err != nil {
		t.Fatal(err)
	}
	target, err := observationTarget(filepath.Join(root, "ardents-carrier-quic-v2-2245479829", "reader-input.json"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, selected))
	if err != nil {
		t.Fatal(err)
	}
	report, err := parseHeapDump(raw, target[:])
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Matches) == 0 {
		t.Fatal("expected Target match in resolution published heap")
	}
	report.ReceiptSHA256, report.TargetPresenceSHA256 = receiptHash, targetPresenceHash
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, append(encoded, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func verifyObservationInputs(root, selected string) (string, string, error) {
	manifest, receiptDigest, err := verifiedObservationManifest(root, false)
	if err != nil {
		return "", "", err
	}
	presenceRaw, err := os.ReadFile(filepath.Join(root, "target-presence.json"))
	if err != nil {
		return "", "", err
	}
	presenceDigest := sha256.Sum256(presenceRaw)
	presenceBound := false
	for _, artifact := range manifest.Artifacts {
		if filepath.Base(artifact.Path) == "target-presence.json" {
			presenceBound = true
			if !strings.EqualFold(hex.EncodeToString(presenceDigest[:]), artifact.SHA256) {
				return "", "", errors.New("target-presence hash differs from receipt")
			}
		}
	}
	if !presenceBound {
		return "", "", errors.New("receipt does not bind target-presence")
	}
	readerInput := filepath.Join(root, "ardents-carrier-quic-v2-2245479829", "reader-input.json")
	readerRaw, err := os.ReadFile(readerInput)
	if err != nil {
		return "", "", err
	}
	readerDigest := sha256.Sum256(readerRaw)
	readerBound := false
	for _, artifact := range manifest.Artifacts {
		if filepath.Clean(artifact.Path) == filepath.Clean(readerInput) {
			readerBound = true
			if !strings.EqualFold(hex.EncodeToString(readerDigest[:]), artifact.SHA256) {
				return "", "", errors.New("reader-input hash differs from receipt")
			}
		}
	}
	if !readerBound {
		return "", "", errors.New("receipt does not bind reader-input")
	}
	var presence struct {
		Rows []struct {
			Path          string
			SHA256        string
			TargetPresent bool
		}
	}
	if err := json.Unmarshal(presenceRaw, &presence); err != nil {
		return "", "", err
	}
	for _, row := range presence.Rows {
		if row.Path == selected {
			if !row.TargetPresent {
				return "", "", errors.New("selected heap has no declared Target match")
			}
			raw, err := os.ReadFile(filepath.Join(root, selected))
			if err != nil {
				return "", "", err
			}
			actual := sha256.Sum256(raw)
			if !strings.EqualFold(hex.EncodeToString(actual[:]), row.SHA256) {
				return "", "", fmt.Errorf("heap hash mismatch for %s", selected)
			}
			return receiptDigest, hex.EncodeToString(presenceDigest[:]), nil
		}
	}
	return "", "", fmt.Errorf("selected heap missing from target-presence manifest: %s", selected)
}

type observationFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type observationManifest struct {
	SourceFiles []observationFile `json:"sourceFiles"`
	Artifacts   []observationFile `json:"artifacts"`
}

// verifiedObservationManifest binds the locally retained observation files to
// the receipt before any report consumes their contents. allArtifacts is for
// maps that use the full captured set rather than T01's one selected heap.
func verifiedObservationManifest(root string, allArtifacts bool) (observationManifest, string, error) {
	receipt, err := os.ReadFile(filepath.Join(root, "receipt.json"))
	if err != nil {
		return observationManifest{}, "", err
	}
	var manifest observationManifest
	if err := json.Unmarshal(receipt, &manifest); err != nil {
		return observationManifest{}, "", err
	}
	repository, err := heapDumpRepositoryRoot()
	if err != nil {
		return observationManifest{}, "", err
	}
	for _, source := range manifest.SourceFiles {
		raw, err := os.ReadFile(filepath.Join(repository, source.Path))
		if err != nil {
			return observationManifest{}, "", err
		}
		if actual := sha256.Sum256(raw); !strings.EqualFold(hex.EncodeToString(actual[:]), source.SHA256) {
			return observationManifest{}, "", fmt.Errorf("source hash mismatch for %s", source.Path)
		}
	}
	if allArtifacts {
		for _, artifact := range manifest.Artifacts {
			path, err := observationArtifactPath(root, artifact.Path)
			if err != nil {
				return observationManifest{}, "", err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return observationManifest{}, "", err
			}
			if actual := sha256.Sum256(raw); !strings.EqualFold(hex.EncodeToString(actual[:]), artifact.SHA256) {
				return observationManifest{}, "", fmt.Errorf("artifact hash mismatch for %s", artifact.Path)
			}
		}
	}
	digest := sha256.Sum256(receipt)
	return manifest, hex.EncodeToString(digest[:]), nil
}

func observationArtifactPath(root, artifact string) (string, error) {
	relative, err := filepath.Rel(root, artifact)
	if err != nil || relative == "." || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("receipt artifact is outside observation root: %s", artifact)
	}
	return filepath.Join(root, relative), nil
}

func heapDumpRepositoryRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("repository root not found")
		}
		directory = parent
	}
}

func observationTarget(path string) ([32]byte, error) {
	var input struct{ Target [32]byte }
	raw, err := os.ReadFile(path)
	if err != nil {
		return input.Target, err
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return input.Target, err
	}
	return input.Target, nil
}
