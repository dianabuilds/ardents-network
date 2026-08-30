package enrollment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
	// ArtifactPath is empty for the Portable bundle, where the manifest's
	// artifact is a direct child of BundleRoot. The Installed profile sets it
	// to the one package-owned direct executable that the static manifest
	// binds; no other external inventory entry is permitted.
	ArtifactPath  string
	Pin           Pin
	Environment   string
	Network       string
	TargetPath    string
	Architecture  string
	ReferenceTime time.Time
}

// Verified is the bounded result of authenticating an enrolled bundle. Inputs
// is passed unchanged to Release Decision; it does not grant execution.
type Verified struct {
	Inputs release.Inputs
	// ControlCatalog and DisclosureRoot are enrollment-pinned alpha-control
	// disclosure companions. They are deliberately not Release metadata.
	ControlCatalog, DisclosureRoot []byte
	ControlRelease                 []byte
	ControlNetwork                 []byte
	ControlCompatibility           []byte
	ControlReleaseRoot             []byte
	ControlNetworkRoot             []byte
	ControlCompatibilityRoot       []byte
	// CorpusAuthority is the optional independently pinned Alpha Name Corpus
	// authority from an enrollment-v2-or-later bundle. It is not Release metadata.
	CorpusAuthority []byte
	// ControlArtifact is the exact separately executable alpha-control command
	// from an enrollment-v3 bundle. It is intentionally not Release metadata.
	ControlArtifactName string
	ControlArtifact     []byte
	// BrowserAdapterArtifact, BrowserEntryArtifact, and BrowserEntryExtension
	// are the exact Adapter, native host, and Mozilla-signed XPI companions from
	// an enrollment-v4 bundle. They are participant-delivery bytes, never
	// Release metadata.
	BrowserAdapterArtifactName string
	BrowserAdapterArtifact     []byte
	BrowserEntryArtifactName   string
	BrowserEntryArtifact       []byte
	BrowserEntryExtensionName  string
	BrowserEntryExtension      []byte
}

// Verify authenticates the Network enrollment inventory. Browser enrollment-v4
// is deliberately rejected and must enter through VerifyBrowser.
func Verify(request Request) (Verified, error) {
	return verify(request, false)
}

// VerifyBrowser authenticates the distinct Browser Adapter enrollment-v4
// inventory. It never broadens the Network enrollment inventory.
func VerifyBrowser(request Request) (Verified, error) {
	return verify(request, true)
}

// verify checks the manifest pin before parsing it, then accepts only a
// single-directory, regular-file inventory whose descriptor and artifact bind
// the requested first-install facts. It never runs a bundle artifact.
func verify(request Request, browser bool) (Verified, error) {
	if !validRequest(request) {
		return Verified{}, errors.New("alpha enrollment request is incomplete")
	}
	root, err := filepath.Abs(request.BundleRoot)
	if err != nil {
		return Verified{}, fmt.Errorf("resolve alpha bundle: %w", err)
	}
	if request.ArtifactPath != "" {
		if err := verifyPackageStaticRoot(root); err != nil {
			return Verified{}, err
		}
	}
	manifest, err := readEnrollmentFile(filepath.Join(root, manifestName), request.ArtifactPath != "")
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
	descriptorBytes, found := entries[descriptorName]
	if !found {
		return Verified{}, errors.New("alpha manifest lacks its descriptor")
	}
	descriptorContents, readErr := readEnrollmentFile(filepath.Join(root, descriptorName), request.ArtifactPath != "")
	if readErr != nil || !bytes.Equal(digest(descriptorContents), descriptorBytes) {
		return Verified{}, errors.New("alpha bundle descriptor does not match manifest")
	}
	descriptor, err := parseDescriptor(descriptorContents)
	if err != nil {
		return Verified{}, err
	}
	if browser != (descriptor.schema == "ardents-closed-alpha-enrollment-v4") {
		if browser {
			return Verified{}, errors.New("browser Adapter enrollment requires the v4 inventory")
		}
		return Verified{}, errors.New("network enrollment does not accept the Browser Adapter v4 inventory")
	}
	if err := exactInventory(root, entries, descriptor.artifact, request.ArtifactPath != ""); err != nil {
		return Verified{}, err
	}
	files := make(map[string][]byte, len(entries))
	for name, expected := range entries {
		path := filepath.Join(root, name)
		packageArtifact := false
		if name == descriptor.artifact && request.ArtifactPath != "" {
			path = request.ArtifactPath
			packageArtifact = true
		}
		contents, readErr := readEnrollmentFile(path, request.ArtifactPath != "")
		if packageArtifact {
			contents, readErr = readPackageArtifact(path)
		}
		if readErr != nil {
			return Verified{}, fmt.Errorf("read alpha bundle entry %q: %w", name, readErr)
		}
		if !bytes.Equal(digest(contents), expected) {
			return Verified{}, fmt.Errorf("alpha bundle entry %q does not match manifest", name)
		}
		files[name] = contents
	}
	if err := descriptor.matches(request); err != nil {
		return Verified{}, err
	}
	artifact, found := files[descriptor.artifact]
	if !found {
		return Verified{}, errors.New("alpha descriptor artifact is absent from the manifest")
	}
	artifactPath := filepath.Join(root, descriptor.artifact)
	if request.ArtifactPath != "" {
		artifactPath = request.ArtifactPath
	}
	if err := exactExecutable(request.ExecutablePath, artifactPath, artifact, request.ArtifactPath != ""); err != nil {
		return Verified{}, err
	}
	trustedRoot, found := files[descriptor.trustedRoot]
	if !found {
		return Verified{}, errors.New("alpha descriptor trusted root is absent from the manifest")
	}
	controlCatalog, found := files[descriptor.controlCatalog]
	if !found {
		return Verified{}, errors.New("alpha descriptor control catalog is absent from the manifest")
	}
	disclosureRoot, found := files[descriptor.disclosureRoot]
	if !found {
		return Verified{}, errors.New("alpha descriptor disclosure root is absent from the manifest")
	}
	controlRelease, found := files[descriptor.controlRelease]
	if !found {
		return Verified{}, errors.New("alpha descriptor release control is absent from the manifest")
	}
	controlNetwork, found := files[descriptor.controlNetwork]
	if !found {
		return Verified{}, errors.New("alpha descriptor network control is absent from the manifest")
	}
	controlCompatibility, found := files[descriptor.controlCompatibility]
	if !found {
		return Verified{}, errors.New("alpha descriptor compatibility control is absent from the manifest")
	}
	controlReleaseRoot, found := files[descriptor.controlReleaseRoot]
	if !found {
		return Verified{}, errors.New("alpha descriptor release control root is absent from the manifest")
	}
	controlNetworkRoot, found := files[descriptor.controlNetworkRoot]
	if !found {
		return Verified{}, errors.New("alpha descriptor network control root is absent from the manifest")
	}
	controlCompatibilityRoot, found := files[descriptor.controlCompatibilityRoot]
	if !found {
		return Verified{}, errors.New("alpha descriptor compatibility control root is absent from the manifest")
	}
	var corpusAuthority []byte
	if descriptor.corpusAuthority != "" {
		corpusAuthority, found = files[descriptor.corpusAuthority]
		if !found {
			return Verified{}, errors.New("alpha descriptor corpus authority is absent from the manifest")
		}
	}
	var controlArtifact []byte
	if descriptor.controlArtifact != "" {
		controlArtifact, found = files[descriptor.controlArtifact]
		if !found {
			return Verified{}, errors.New("alpha descriptor control artifact is absent from the manifest")
		}
	}
	var browserAdapterArtifact, browserEntryArtifact, browserEntryExtension []byte
	if descriptor.browserAdapterArtifact != "" {
		browserAdapterArtifact, found = files[descriptor.browserAdapterArtifact]
		if !found {
			return Verified{}, errors.New("alpha descriptor Browser Adapter is absent from the manifest")
		}
	}
	if descriptor.browserEntryArtifact != "" {
		browserEntryArtifact, found = files[descriptor.browserEntryArtifact]
		if !found {
			return Verified{}, errors.New("alpha descriptor Browser Entry host is absent from the manifest")
		}
		browserEntryExtension, found = files[descriptor.browserEntryExtension]
		if !found {
			return Verified{}, errors.New("alpha descriptor Browser Entry extension is absent from the manifest")
		}
	}
	metadata := make(map[string][]byte, len(files))
	for name, contents := range files {
		if name == descriptorName || name == descriptor.artifact || name == descriptor.trustedRoot ||
			name == descriptor.controlCatalog || name == descriptor.disclosureRoot ||
			name == descriptor.controlRelease || name == descriptor.controlNetwork || name == descriptor.controlCompatibility ||
			name == descriptor.controlReleaseRoot || name == descriptor.controlNetworkRoot || name == descriptor.controlCompatibilityRoot {
			continue
		}
		if name == descriptor.corpusAuthority {
			continue
		}
		if name == descriptor.controlArtifact {
			continue
		}
		if name == descriptor.browserAdapterArtifact || name == descriptor.browserEntryArtifact || name == descriptor.browserEntryExtension {
			continue
		}
		metadata[release.MetadataURL(name)] = contents
	}
	return Verified{Inputs: release.Inputs{RootBytes: trustedRoot, Files: metadata, TargetPath: request.TargetPath,
		Artifact: artifact, Local: release.LocalEnvironment{Environment: request.Environment, Network: request.Network,
			Platform: request.Pin.Platform, Architecture: request.Architecture, RefTime: request.ReferenceTime.UTC()}},
		ControlCatalog: append([]byte(nil), controlCatalog...), DisclosureRoot: append([]byte(nil), disclosureRoot...),
		ControlRelease: append([]byte(nil), controlRelease...), ControlNetwork: append([]byte(nil), controlNetwork...),
		ControlCompatibility: append([]byte(nil), controlCompatibility...),
		ControlReleaseRoot:   append([]byte(nil), controlReleaseRoot...), ControlNetworkRoot: append([]byte(nil), controlNetworkRoot...),
		ControlCompatibilityRoot: append([]byte(nil), controlCompatibilityRoot...), CorpusAuthority: append([]byte(nil), corpusAuthority...),
		ControlArtifactName: descriptor.controlArtifact, ControlArtifact: append([]byte(nil), controlArtifact...),
		BrowserAdapterArtifactName: descriptor.browserAdapterArtifact, BrowserAdapterArtifact: append([]byte(nil), browserAdapterArtifact...),
		BrowserEntryArtifactName: descriptor.browserEntryArtifact, BrowserEntryArtifact: append([]byte(nil), browserEntryArtifact...),
		BrowserEntryExtensionName: descriptor.browserEntryExtension, BrowserEntryExtension: append([]byte(nil), browserEntryExtension...)}, nil
}

func validRequest(request Request) bool {
	return request.BundleRoot != "" && request.ExecutablePath != "" && request.Pin.Cohort != "" &&
		request.Pin.Release != "" && request.Pin.Platform != "" && len(request.Pin.ManifestSHA256) == 64 &&
		request.Environment != "" && request.Network != "" && request.TargetPath != "" && request.Architecture != "" &&
		!request.ReferenceTime.IsZero()
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
