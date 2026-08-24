package enrollment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/release"
)

const (
	manifestName   = "SHA256SUMS"
	descriptorName = "RELEASE"
	maximumFiles   = 32
	maximumFileLen = 64 << 20
)

// Pin is the independently delivered, one-release closed-alpha enrollment
// fact. It authorizes one exact manifest, not a signing key or successor.
type Pin struct {
	Cohort, Release, Platform string
	ManifestSHA256            string
}

// Request identifies one local bundle and the matching local Release binding.
// The caller supplies a fixed reference time for deterministic Release checks.
type Request struct {
	BundleRoot     string
	ExecutablePath string
	Pin            Pin
	Environment    string
	Network        string
	TargetPath     string
	Architecture   string
	ReferenceTime  time.Time
}

// Verified is the bounded result of authenticating an enrolled bundle. Inputs
// is passed unchanged to Release Decision; it does not grant execution.
type Verified struct {
	Inputs release.Inputs
}

// Verify checks the manifest pin before parsing it, then accepts only a
// single-directory, regular-file inventory whose descriptor and artifact bind
// the requested first-install facts. It never runs a bundle artifact.
func Verify(request Request) (Verified, error) {
	if !validRequest(request) {
		return Verified{}, errors.New("alpha enrollment request is incomplete")
	}
	root, err := filepath.Abs(request.BundleRoot)
	if err != nil {
		return Verified{}, fmt.Errorf("resolve alpha bundle: %w", err)
	}
	manifest, err := readRegular(filepath.Join(root, manifestName))
	if err != nil {
		return Verified{}, fmt.Errorf("read alpha manifest: %w", err)
	}
	if !equalDigest(manifest, request.Pin.ManifestSHA256) {
		return Verified{}, errors.New("alpha enrollment manifest does not match the independent pin")
	}
	entries, err := parseManifest(manifest)
	if err != nil {
		return Verified{}, err
	}
	if err := exactInventory(root, entries); err != nil {
		return Verified{}, err
	}
	files := make(map[string][]byte, len(entries))
	for name, expected := range entries {
		contents, readErr := readRegular(filepath.Join(root, name))
		if readErr != nil {
			return Verified{}, fmt.Errorf("read alpha bundle entry %q: %w", name, readErr)
		}
		if !bytes.Equal(digest(contents), expected) {
			return Verified{}, fmt.Errorf("alpha bundle entry %q does not match manifest", name)
		}
		files[name] = contents
	}
	descriptor, err := parseDescriptor(files[descriptorName])
	if err != nil {
		return Verified{}, err
	}
	if err := descriptor.matches(request); err != nil {
		return Verified{}, err
	}
	artifact, found := files[descriptor.artifact]
	if !found {
		return Verified{}, errors.New("alpha descriptor artifact is absent from the manifest")
	}
	if err := exactExecutable(request.ExecutablePath, filepath.Join(root, descriptor.artifact), artifact); err != nil {
		return Verified{}, err
	}
	trustedRoot, found := files[descriptor.trustedRoot]
	if !found {
		return Verified{}, errors.New("alpha descriptor trusted root is absent from the manifest")
	}
	metadata := make(map[string][]byte, len(files))
	for name, contents := range files {
		if name == descriptorName || name == descriptor.artifact || name == descriptor.trustedRoot {
			continue
		}
		metadata[release.MetadataURL(name)] = contents
	}
	return Verified{Inputs: release.Inputs{RootBytes: trustedRoot, Files: metadata, TargetPath: request.TargetPath,
		Artifact: artifact, Local: release.LocalEnvironment{Environment: request.Environment, Network: request.Network,
			Platform: request.Pin.Platform, Architecture: request.Architecture, RefTime: request.ReferenceTime.UTC()}}}, nil
}

func validRequest(request Request) bool {
	return request.BundleRoot != "" && request.ExecutablePath != "" && request.Pin.Cohort != "" &&
		request.Pin.Release != "" && request.Pin.Platform != "" && len(request.Pin.ManifestSHA256) == 64 &&
		request.Environment != "" && request.Network != "" && request.TargetPath != "" && request.Architecture != "" &&
		!request.ReferenceTime.IsZero()
}

func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 1 || info.Size() > maximumFileLen {
		return nil, errors.New("entry is not a bounded regular file")
	}
	if err := verifyOwnedRegular(info); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func parseManifest(raw []byte) (map[string][]byte, error) {
	if len(raw) == 0 || len(raw) > maximumFiles*80 || raw[len(raw)-1] != '\n' {
		return nil, errors.New("alpha manifest is not canonical")
	}
	result := make(map[string][]byte)
	previous := ""
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		parts := strings.Split(line, "  ")
		if len(parts) != 2 || len(parts[0]) != 64 || !validName(parts[1]) || parts[1] == manifestName ||
			previous >= parts[1] {
			return nil, errors.New("alpha manifest is not canonical")
		}
		digest, err := hex.DecodeString(parts[0])
		if err != nil || strings.ToLower(parts[0]) != parts[0] {
			return nil, errors.New("alpha manifest digest is invalid")
		}
		if _, exists := result[parts[1]]; exists || len(result) == maximumFiles {
			return nil, errors.New("alpha manifest inventory is invalid")
		}
		result[parts[1]], previous = digest, parts[1]
	}
	if _, found := result[descriptorName]; !found {
		return nil, errors.New("alpha manifest lacks its descriptor")
	}
	return result, nil
}

func exactInventory(root string, entries map[string][]byte) error {
	directory, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	if len(directory) != len(entries)+1 {
		return errors.New("alpha bundle inventory has unknown or missing entries")
	}
	for _, entry := range directory {
		if entry.Name() == manifestName {
			continue
		}
		if _, found := entries[entry.Name()]; !found {
			return errors.New("alpha bundle inventory has an unknown entry")
		}
	}
	return nil
}

type descriptor struct {
	cohort, release, platform, environment, network, targetPath string
	artifact, trustedRoot                                       string
}

func parseDescriptor(raw []byte) (descriptor, error) {
	keys := []string{"schema", "cohort", "release", "platform", "environment", "network", "target_path", "artifact", "trusted_root"}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != len(keys) || len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return descriptor{}, errors.New("alpha descriptor is not canonical")
	}
	values := make(map[string]string, len(keys))
	for index, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || parts[0] != keys[index] || parts[1] == "" {
			return descriptor{}, errors.New("alpha descriptor is not canonical")
		}
		values[parts[0]] = parts[1]
	}
	if values["schema"] != "ardents-closed-alpha-enrollment-v1" || !validName(values["artifact"]) || !validName(values["trusted_root"]) ||
		values["artifact"] == values["trusted_root"] || values["artifact"] == descriptorName || values["trusted_root"] == descriptorName {
		return descriptor{}, errors.New("alpha descriptor is invalid")
	}
	return descriptor{cohort: values["cohort"], release: values["release"], platform: values["platform"],
		environment: values["environment"], network: values["network"], targetPath: values["target_path"],
		artifact: values["artifact"], trustedRoot: values["trusted_root"]}, nil
}

func (descriptor descriptor) matches(request Request) error {
	if descriptor.cohort != request.Pin.Cohort || descriptor.release != request.Pin.Release ||
		descriptor.platform != request.Pin.Platform || descriptor.environment != request.Environment ||
		descriptor.network != request.Network || descriptor.targetPath != request.TargetPath {
		return errors.New("alpha descriptor does not match the enrollment request")
	}
	return nil
}

func exactExecutable(actual, expected string, artifact []byte) error {
	actualInfo, err := os.Stat(actual)
	if err != nil {
		return fmt.Errorf("inspect running executable: %w", err)
	}
	expectedInfo, err := os.Stat(expected)
	if err != nil {
		return fmt.Errorf("inspect enrolled executable: %w", err)
	}
	if !os.SameFile(actualInfo, expectedInfo) {
		return errors.New("running executable is not the enrolled artifact")
	}
	running, err := readRegular(actual)
	if err != nil || !bytes.Equal(running, artifact) {
		return errors.New("running executable does not match the enrolled artifact")
	}
	return nil
}

func equalDigest(data []byte, expected string) bool {
	decoded, err := hex.DecodeString(expected)
	return err == nil && strings.ToLower(expected) == expected && bytes.Equal(digest(data), decoded)
}

func digest(data []byte) []byte {
	value := sha256.Sum256(data)
	return value[:]
}

func validName(value string) bool {
	return value != "" && filepath.Base(value) == value && !strings.ContainsAny(value, "\\/\t\r\n ")
}
