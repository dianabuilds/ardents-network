package blockedverify

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maximumInput = 16 << 20

func decodeStrict(path string, value any) ([]byte, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 {
		return nil, errors.Join(err, errors.New("verifier input path is invalid"))
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Join(err, errors.New("verifier input cannot be opened"))
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(pathInfo, info) ||
		info.Size() < 1 || info.Size() > maximumInput {
		return nil, errors.Join(err, errors.New("verifier input is missing, empty, or oversized"))
	}
	raw, err := io.ReadAll(io.LimitReader(file, maximumInput+1))
	if err != nil || int64(len(raw)) != info.Size() {
		return nil, errors.Join(err, errors.New("verifier input changed or exceeded its bound while reading"))
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, errors.New("verifier input is not one canonical schema value")
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return nil, errors.New("verifier input does not use canonical JSON encoding")
	}
	return raw, nil
}

func digest(raw []byte) string {
	value := sha256.Sum256(raw)
	return hex.EncodeToString(value[:])
}

func hashFile(path string) (string, int64, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 {
		return "", 0, errors.Join(err, errors.New("committed secret artifact path is invalid"))
	}
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(pathInfo, info) ||
		info.Size() < 1 || info.Size() > maximumInput {
		return "", 0, errors.Join(err, errors.New("committed secret artifact is invalid"))
	}
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, maximumInput+1))
	if err != nil || written != info.Size() {
		return "", 0, errors.Join(err, errors.New("committed secret artifact changed while reading"))
	}
	return hex.EncodeToString(hash.Sum(nil)), info.Size(), nil
}

func readStableFile(path string) ([]byte, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 {
		return nil, errors.Join(err, errors.New("publishable artifact path is invalid"))
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(pathInfo, info) || info.Size() > maximumInput {
		return nil, errors.Join(err, errors.New("publishable artifact handle is invalid"))
	}
	raw, err := io.ReadAll(io.LimitReader(file, maximumInput+1))
	if err != nil || int64(len(raw)) != info.Size() {
		return nil, errors.Join(err, errors.New("publishable artifact changed while reading"))
	}
	return raw, nil
}

func safeArtifactPath(root, relative string) (string, bool) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", false
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	path := filepath.Join(absoluteRoot, filepath.FromSlash(relative))
	withinRoot, err := filepath.Rel(absoluteRoot, path)
	if err != nil || withinRoot == ".." || withinRoot == "" ||
		strings.HasPrefix(withinRoot, ".."+string(filepath.Separator)) {
		return "", false
	}
	rootAliased, rootErr := pathHasSymlink(absoluteRoot)
	pathAliased, pathErr := pathHasSymlink(path)
	if rootErr != nil || pathErr != nil || rootAliased || pathAliased {
		return "", false
	}
	return path, true
}

func pathHasSymlink(path string) (bool, error) {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
	}
}
