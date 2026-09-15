//go:build linux

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

const (
	qualificationBinaryPath   = "/usr/lib/ardents/qualification/ardents-qualification"
	qualificationPlanPath     = "/etc/ardents/qualification-plan.json"
	qualificationUnitPath     = "/etc/systemd/system/ardents-endpoint.service"
	qualificationManifestPath = "/etc/ardents/qualification-endpoint-artifact.json"
)

type qualificationEndpointArtifact struct {
	ManifestSHA256 string
	Files          map[string]string
}

// verifyQualificationEndpointArtifact binds the running Endpoint to the exact
// root-installed binary, plan and unit before State, permission or worker work.
func verifyQualificationEndpointArtifact(plan string) (qualificationEndpointArtifact, error) {
	var artifact qualificationEndpointArtifact
	if plan != qualificationPlanPath {
		return artifact, errors.New("qualification requires the fixed installed plan")
	}
	manifestBody, err := readQualificationInstalledFile(qualificationManifestPath, 16<<10, 0o644)
	if err != nil {
		return artifact, err
	}
	var manifest struct {
		Schema string            `json:"schema"`
		Files  map[string]string `json:"files"`
	}
	decoder := json.NewDecoder(bytes.NewReader(manifestBody))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil || manifest.Schema != "ardents-qualification-endpoint-artifact-v1" || len(manifest.Files) != 3 {
		return artifact, errors.New("qualification Endpoint artifact manifest is invalid")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return artifact, errors.New("qualification Endpoint artifact manifest has trailing data")
	}
	canonical, err := json.Marshal(map[string]any{"files": manifest.Files, "schema": manifest.Schema})
	if err != nil || !bytes.Equal(bytes.TrimSpace(manifestBody), canonical) {
		return artifact, errors.New("qualification Endpoint artifact manifest is not canonical")
	}
	limits := map[string]struct {
		maximum int64
		mode    os.FileMode
	}{
		qualificationBinaryPath: {64 << 20, 0o555},
		qualificationPlanPath:   {256 << 10, 0o640},
		qualificationUnitPath:   {64 << 10, 0o644},
	}
	artifact.Files = make(map[string]string, len(limits))
	for path, expected := range limits {
		spelling, ok := manifest.Files[path]
		decoded, decodeErr := hex.DecodeString(spelling)
		if !ok || decodeErr != nil || len(decoded) != sha256.Size || hex.EncodeToString(decoded) != spelling {
			return qualificationEndpointArtifact{}, errors.New("qualification Endpoint artifact digest is invalid")
		}
		body, readErr := readQualificationInstalledFile(path, expected.maximum, expected.mode)
		actual := sha256.Sum256(body)
		if readErr != nil || !bytes.Equal(actual[:], decoded) {
			return qualificationEndpointArtifact{}, errors.New("qualification Endpoint artifact was substituted")
		}
		artifact.Files[path] = spelling
	}
	running, err := os.Stat("/proc/self/exe")
	installed, installedErr := os.Stat(qualificationBinaryPath)
	if err != nil || installedErr != nil || !os.SameFile(running, installed) {
		return qualificationEndpointArtifact{}, errors.New("qualification process is not the installed Endpoint artifact")
	}
	digest := sha256.Sum256(manifestBody)
	artifact.ManifestSHA256 = hex.EncodeToString(digest[:])
	return artifact, nil
}

func readQualificationInstalledFile(path string, maximum int64, mode os.FileMode) ([]byte, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("qualification Endpoint artifact path is invalid")
	}
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		identity, ok := infoIdentity(info)
		if err != nil || !ok || identity.Uid != 0 || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o022 != 0 || current != path && !info.IsDir() {
			return nil, errors.New("qualification Endpoint artifact path is not root-controlled")
		}
		if current == "/" {
			break
		}
	}
	before, err := os.Lstat(path)
	identity, ok := infoIdentity(before)
	if err != nil || !ok || identity.Nlink != 1 || !before.Mode().IsRegular() || before.Mode().Perm() != mode || before.Size() <= 0 || before.Size() > maximum {
		return nil, errors.New("qualification Endpoint artifact file is invalid")
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.New("qualification Endpoint artifact file is unavailable")
	}
	file := os.NewFile(uintptr(fd), path)
	body, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	after, statErr := file.Stat()
	closeErr := file.Close()
	if readErr != nil || statErr != nil || closeErr != nil || int64(len(body)) != before.Size() || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		return nil, errors.New("qualification Endpoint artifact changed during read")
	}
	return body, nil
}

func infoIdentity(info os.FileInfo) (*syscall.Stat_t, bool) {
	if info == nil {
		return nil, false
	}
	identity, ok := info.Sys().(*syscall.Stat_t)
	return identity, ok
}
