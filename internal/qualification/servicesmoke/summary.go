package servicesmoke

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func finalize(input Config, result Result) Result {
	if input.EvidenceRoot == "" || !filepath.IsAbs(input.EvidenceRoot) {
		return result
	}
	marker, err := os.ReadFile(filepath.Join(input.EvidenceRoot, ".ardents-owned"))
	if err != nil || string(marker) != evidenceMarker {
		result.Verdict, result.Reason = "invalid", "Stage 3 evidence ownership is missing"
		return result
	}
	if !result.DockerCleanup || !result.FixtureCleanup {
		result.Verdict, result.Reason = "invalid", "Stage 3 cleanup did not satisfy every terminal conjunct"
	}
	cleanup := map[string]any{"schema": "ardents-h3-service-cleanup-v1",
		"docker_cleanup_verified": result.DockerCleanup, "private_fixture_cleanup_verified": result.FixtureCleanup}
	if err := byteio.WriteJSON(filepath.Join(input.EvidenceRoot, "cleanup.json"), cleanup, 64<<10); err != nil {
		result.Verdict, result.Reason = "invalid", err.Error()
		return result
	}
	terminal := map[string]any{"schema": "ardents-h3-service-terminal-v1", "verdict": result.Verdict,
		"reason": result.Reason, "attempts": result.Attempts, "requested_duration": input.Duration.String(),
		"elapsed": result.Elapsed.String(), "source_commit": result.SourceCommit, "image_id": result.ImageID,
		"docker_cleanup_verified": result.DockerCleanup, "private_fixture_cleanup_verified": result.FixtureCleanup,
		"claim": "local development evidence only"}
	if err := byteio.WriteJSON(filepath.Join(input.EvidenceRoot, "terminal.json"), terminal, 64<<10); err != nil {
		result.Verdict, result.Reason = "invalid", err.Error()
		return result
	}
	digest, err := evidenceBundleDigest(input.EvidenceRoot)
	if err != nil {
		result.Verdict, result.Reason = "invalid", err.Error()
	} else {
		result.EvidenceDigest = hex.EncodeToString(digest[:])
	}
	summary := map[string]any{"schema": "ardents-h3-service-summary-v1", "verdict": result.Verdict,
		"reason": result.Reason, "attempts": result.Attempts, "requested_duration": input.Duration.String(),
		"elapsed": result.Elapsed.String(), "source_commit": result.SourceCommit, "image_id": result.ImageID,
		"evidence_digest": result.EvidenceDigest, "claim": "local development evidence only"}
	if err := byteio.WriteJSON(filepath.Join(input.EvidenceRoot, "summary.json"), summary, 64<<10); err != nil {
		result.Verdict, result.Reason = "invalid", err.Error()
	}
	return result
}

func evidenceBundleDigest(root string) ([32]byte, error) {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("stage 3 evidence contains a symbolic link")
		}
		if entry.IsDir() || entry.Name() == "summary.json" {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.Size() > 4<<20 {
			return errors.New("stage 3 evidence file exceeds 4 MiB")
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
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil {
			return errors.Join(copyErr, closeErr)
		}
		_, _ = hash.Write([]byte{0})
		return nil
	})
	var digest [32]byte
	if err == nil {
		copy(digest[:], hash.Sum(nil))
	}
	return digest, err
}
