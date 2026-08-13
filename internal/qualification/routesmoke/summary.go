package routesmoke

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func finalizeEvidence(input Config, result Result) Result {
	if input.EvidenceRoot == "" || !filepath.IsAbs(input.EvidenceRoot) {
		return result
	}
	marker, err := os.ReadFile(filepath.Join(input.EvidenceRoot, ".ardents-route-smoke-evidence"))
	if err != nil || string(marker) != evidenceOwner {
		result.Verdict = "invalid"
		result.Reason = "current route smoke evidence ownership is missing"
		result.EvidenceDigest = ""
		return result
	}
	cleanup := map[string]any{"schema": "ardents-h3-route-smoke-cleanup-v1",
		"docker_cleanup_verified": result.DockerCleanup, "fixture_cleanup_verified": result.FixtureCleanup}
	if err := byteio.WriteJSON(filepath.Join(input.EvidenceRoot, "cleanup.json"), cleanup, 64<<10); err != nil {
		result.Verdict, result.Reason = "invalid", err.Error()
		return result
	}
	terminal := map[string]any{"schema": "ardents-h3-route-smoke-terminal-v1", "verdict": result.Verdict,
		"reason": result.Reason, "attempts": result.Attempts, "requested_duration": input.Duration.String(),
		"elapsed": result.Elapsed.String(), "source_digest": result.SourceDigest, "image_id": result.ImageID,
		"docker_cleanup_verified": result.DockerCleanup, "fixture_cleanup_verified": result.FixtureCleanup,
		"claim": "local development evidence only"}
	if err := byteio.WriteJSON(filepath.Join(input.EvidenceRoot, "terminal.json"), terminal, 64<<10); err != nil {
		result.Verdict, result.Reason = "invalid", err.Error()
		return result
	}
	digest, err := bundleDigest(input.EvidenceRoot)
	if err != nil {
		result.Verdict, result.Reason = "invalid", err.Error()
	} else {
		result.EvidenceDigest = hex.EncodeToString(digest[:])
	}
	summary := map[string]any{"schema": "ardents-h3-route-smoke-summary-v1", "verdict": result.Verdict,
		"reason": result.Reason, "attempts": result.Attempts, "requested_duration": input.Duration.String(),
		"elapsed": result.Elapsed.String(), "source_digest": result.SourceDigest, "image_id": result.ImageID,
		"evidence_digest": result.EvidenceDigest, "claim": "local development evidence only"}
	if writeErr := byteio.WriteJSON(filepath.Join(input.EvidenceRoot, "summary.json"), summary, 64<<10); writeErr != nil {
		result.Verdict, result.Reason = "invalid", writeErr.Error()
	}
	return result
}

func bundleDigest(root string) ([32]byte, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("route smoke evidence contains a symbolic link")
		}
		if entry.IsDir() || entry.Name() == "summary.json" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		_, _ = io.WriteString(hash, filepath.ToSlash(relative)+"\x00")
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(hash, io.LimitReader(file, 2<<20))
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		_, _ = hash.Write([]byte{0})
		return nil
	})
	var digest [32]byte
	if err != nil {
		return digest, err
	}
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}
