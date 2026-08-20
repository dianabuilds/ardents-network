package stage6verify

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

const maximumIndexBytes = 1 << 20
const maximumEvidenceBytes = 256 << 20

func readStableFile(path string, maximum int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 ||
		before.Size() <= 0 || before.Size() > maximum {
		return nil, errors.New("artifact is not a bounded regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		_ = file.Close()
		return nil, errors.New("artifact identity changed before read")
	}
	raw, err := io.ReadAll(io.LimitReader(file, maximum+1))
	closeErr := file.Close()
	after, statErr := os.Lstat(path)
	if err != nil || closeErr != nil || statErr != nil || int64(len(raw)) != before.Size() ||
		!os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return nil, errors.New("artifact changed during read")
	}
	return raw, nil
}

func readCanonical(root, relative string, maximum int64, value any, jsonl bool) ([]byte, error) {
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == "." || filepath.IsAbs(clean) || clean != filepath.FromSlash(relative) ||
		clean == ".." || len(clean) > 3 && clean[:3] == ".."+string(filepath.Separator) {
		return nil, errors.New("artifact path escapes its root")
	}
	raw, err := readStableFile(filepath.Join(root, clean), maximum)
	if err != nil {
		return nil, err
	}
	payload := raw
	if jsonl {
		if len(raw) < 2 || raw[len(raw)-1] != '\n' || bytes.Contains(raw[:len(raw)-1], []byte{'\n'}) {
			return nil, errors.New("JSONL artifact is non-canonical")
		}
		payload = raw[:len(raw)-1]
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, errors.New("artifact contains a trailing JSON value")
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(canonical, payload) {
		return nil, errors.New("artifact JSON is non-canonical")
	}
	return raw, nil
}

func verifyArtifact(root string, reference artifact, expectedPath, expectedSchema string, maximum int64,
	value any, jsonl bool,
) ([]byte, error) {
	if reference.Path != expectedPath || reference.Schema != expectedSchema || reference.Size <= 0 ||
		reference.Size > maximum || len(reference.SHA256) != 64 || reference.SHA256 != string(bytes.ToLower([]byte(reference.SHA256))) {
		return nil, errors.New("artifact commitment is invalid")
	}
	raw, err := readCanonical(root, expectedPath, maximum, value, jsonl)
	if err != nil || int64(len(raw)) != reference.Size {
		return nil, errors.New("artifact commitment does not match its file")
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != reference.SHA256 {
		return nil, errors.New("artifact digest mismatch")
	}
	return raw, nil
}

func verifyObservation(root string, reference observationArtifact, expectedPath string,
	value any,
) ([]byte, error) {
	if reference.Role != "cell-worker" || reference.EpisodeOrdinal != 0 || reference.StreamOrdinal != 0 ||
		reference.ObservationStart < 0 || reference.ObservationEnd < reference.ObservationStart {
		return nil, errors.New("observation identity is invalid")
	}
	artifactReference := artifact{Path: reference.Path, Schema: reference.Schema, Size: reference.Size, SHA256: reference.SHA256}
	return verifyArtifact(root, artifactReference, expectedPath, "ardents-stage-6-trace-v1", 4<<20, value, true)
}

func inspectRoot(root string) error {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	info, err := os.Lstat(absolute)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("S6E1 root is invalid")
	}
	return nil
}
