package blockedentry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

func finalArtifactPath(directory, cell string) string {
	digest := sha256.Sum256([]byte(cell))
	return filepath.ToSlash(filepath.Join(directory, hex.EncodeToString(digest[:])+".json"))
}

func readFinalHandoffArtifact(root, directory, cell string, artifact artifactCommitment, value any) error {
	expected := finalArtifactPath(directory, cell)
	if artifact.Path != expected || !hexDigest(artifact.SHA256, 32) ||
		artifact.Bytes < 1 || artifact.Bytes > maximumEvidenceFile {
		return errors.New("final handoff commitment is invalid")
	}
	path := filepath.Join(root, filepath.FromSlash(expected))
	aliased, aliasErr := pathHasSymlink(path)
	if aliasErr != nil || aliased {
		return errors.Join(aliasErr, errors.New("final handoff artifact path is aliased"))
	}
	raw, err := readStableFinalArtifact(path)
	if err != nil || int64(len(raw)) != artifact.Bytes {
		return errors.Join(err, errors.New("final handoff artifact size differs from its commitment"))
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != artifact.SHA256 {
		return errors.New("final handoff artifact hash differs from its commitment")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("final handoff artifact is not one strict schema value")
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("final handoff artifact is not canonical JSON")
	}
	return nil
}

func readStableFinalArtifact(path string) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() ||
		before.Size() < 1 || before.Size() > maximumEvidenceFile {
		return nil, errors.Join(err, errors.New("final handoff artifact is not a bounded regular file"))
	}
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(input, maximumEvidenceFile+1))
	after, statErr := input.Stat()
	closeErr := input.Close()
	if readErr != nil || statErr != nil || closeErr != nil || len(raw) > maximumEvidenceFile ||
		!os.SameFile(before, after) || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return nil, errors.Join(readErr, statErr, closeErr, errors.New("final handoff artifact changed while reading"))
	}
	return raw, nil
}
