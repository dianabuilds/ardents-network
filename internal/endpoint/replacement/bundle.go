package replacement

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/release"
)

const (
	replacementDescriptorName = "REPLACEMENT"
	maximumBundleFiles        = 32
)

// LoadBundle decodes one explicit local replacement bundle into the byte-only
// Release input it names. It makes no network request and grants no authority:
// Release must still authenticate every supplied byte against its durable
// floors before a caller can commit a replacement.
func LoadBundle(root string, referenceTime time.Time) (release.Inputs, error) {
	if root == "" || referenceTime.IsZero() {
		return release.Inputs{}, errors.New("endpoint replacement bundle request is incomplete")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return release.Inputs{}, fmt.Errorf("resolve Endpoint replacement bundle: %w", err)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return release.Inputs{}, fmt.Errorf("read Endpoint replacement bundle: %w", err)
	}
	if len(entries) < 3 || len(entries) > maximumBundleFiles {
		return release.Inputs{}, errors.New("endpoint replacement bundle inventory is out of bounds")
	}
	files := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		if !validBundleName(entry.Name()) {
			return release.Inputs{}, errors.New("endpoint replacement bundle has an invalid entry name")
		}
		contents, readErr := readDirectFile(filepath.Join(abs, entry.Name()), maximumProgramBytes)
		if readErr != nil {
			return release.Inputs{}, fmt.Errorf("read Endpoint replacement bundle entry %q: %w", entry.Name(), readErr)
		}
		files[entry.Name()] = contents
	}
	descriptor, found := files[replacementDescriptorName]
	if !found {
		return release.Inputs{}, errors.New("endpoint replacement bundle lacks its descriptor")
	}
	parsed, err := parseBundleDescriptor(descriptor)
	if err != nil {
		return release.Inputs{}, err
	}
	if parsed.platform != runtime.GOOS+"-"+runtime.GOARCH || parsed.architecture != runtime.GOARCH {
		return release.Inputs{}, errors.New("endpoint replacement bundle does not match this platform")
	}
	artifact, found := files[parsed.artifact]
	if !found {
		return release.Inputs{}, errors.New("endpoint replacement bundle lacks its artifact")
	}
	trustedRoot, found := files[parsed.trustedRoot]
	if !found {
		return release.Inputs{}, errors.New("endpoint replacement bundle lacks its trusted root")
	}
	metadata := make(map[string][]byte, len(files)-3)
	for name, contents := range files {
		if name == replacementDescriptorName || name == parsed.artifact || name == parsed.trustedRoot {
			continue
		}
		metadata[release.MetadataURL(name)] = contents
	}
	return release.Inputs{RootBytes: trustedRoot, Files: metadata, TargetPath: parsed.targetPath, Artifact: artifact,
		Local: release.LocalEnvironment{Environment: parsed.environment, Network: parsed.network, Platform: parsed.platform,
			Architecture: parsed.architecture, RefTime: referenceTime.UTC()}}, nil
}

type bundleDescriptor struct {
	targetPath, artifact, trustedRoot   string
	platform, architecture, environment string
	network                             string
}

func parseBundleDescriptor(raw []byte) (bundleDescriptor, error) {
	keys := []string{"schema", "target_path", "artifact", "trusted_root", "platform", "architecture", "environment", "network"}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return bundleDescriptor{}, errors.New("endpoint replacement descriptor is not canonical")
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != len(keys) {
		return bundleDescriptor{}, errors.New("endpoint replacement descriptor is not canonical")
	}
	values := make(map[string]string, len(keys))
	for index, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || parts[0] != keys[index] || parts[1] == "" {
			return bundleDescriptor{}, errors.New("endpoint replacement descriptor is not canonical")
		}
		values[parts[0]] = parts[1]
	}
	if values["schema"] != "ardents-offline-replacement-bundle-v1" ||
		!validBundleName(values["artifact"]) || !validBundleName(values["trusted_root"]) ||
		values["artifact"] == values["trusted_root"] || values["artifact"] == replacementDescriptorName ||
		values["trusted_root"] == replacementDescriptorName {
		return bundleDescriptor{}, errors.New("endpoint replacement descriptor is invalid")
	}
	for _, key := range []string{"target_path", "platform", "architecture", "environment", "network"} {
		if strings.ContainsAny(values[key], "\r\n") {
			return bundleDescriptor{}, errors.New("endpoint replacement descriptor is invalid")
		}
	}
	return bundleDescriptor{targetPath: values["target_path"], artifact: values["artifact"], trustedRoot: values["trusted_root"],
		platform: values["platform"], architecture: values["architecture"], environment: values["environment"], network: values["network"]}, nil
}

func validBundleName(value string) bool {
	return value != "" && filepath.Base(value) == value && !strings.ContainsAny(value, "\\/\t\r\n ")
}
