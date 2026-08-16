package blockedentry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maximumEvidenceFile = 16 << 20

func writeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw) == 0 || len(raw) > maximumEvidenceFile {
		return errors.Join(err, errors.New("blocked-entry artifact is empty or oversized"))
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

func hashFile(path string) (string, int64, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 {
		return "", 0, errors.Join(err, errors.New("blocked-entry input path is invalid"))
	}
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(pathInfo, info) ||
		info.Size() < 1 || info.Size() > maximumEvidenceFile {
		return "", 0, errors.Join(err, errors.New("blocked-entry input is not a bounded regular file"))
	}
	digest := sha256.New()
	written, err := io.Copy(digest, io.LimitReader(file, maximumEvidenceFile+1))
	if err != nil || written != info.Size() {
		return "", 0, errors.Join(err, errors.New("blocked-entry input changed while reading"))
	}
	return hex.EncodeToString(digest.Sum(nil)), info.Size(), nil
}

func commitment(root, path string) (artifactCommitment, error) {
	digest, size, err := hashFile(filepath.Join(root, filepath.FromSlash(path)))
	return artifactCommitment{Path: filepath.ToSlash(path), SHA256: digest, Bytes: size}, err
}

func within(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != "." && relative != "" && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
