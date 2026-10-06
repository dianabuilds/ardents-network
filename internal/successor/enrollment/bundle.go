package enrollment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"
)

// Scope selects the existing companion inventory requirement.
type Scope uint8

const (
	General Scope = iota + 1
	Headless
)

var (
	ErrInput                      = errors.New("invalid enrollment input")
	ErrPin                        = errors.New("manifest differs from independent pin")
	ErrInventory                  = errors.New("invalid enrollment inventory")
	ErrBinding                    = errors.New("enrollment program or generation binding differs")
	ErrLegacyEnrollmentDescriptor = errors.New("retired Network enrollment descriptor version")
)

// Request supplies the independent pin and actual original executable path.
// ExecutablePath comes from the command's os.Executable, never a CLI override.
type Request struct {
	BundleRoot, ExecutablePath, ManifestSHA256 string
	Scope                                      Scope
}

// Facts are authenticated initial descriptor facts, not current authority.
type Facts struct {
	Cohort, Release, Platform, Environment, Network, TargetPath string
	Artifact, TrustedRoot                                       string
	ControlCatalog, DisclosureRoot, ControlArtifact             string
	ManifestSHA256                                              string
	Headless, Protected                                         bool
}

type snapshot struct {
	facts Facts
	files map[string][]byte
}

// Bundle retains private initial provenance. Its zero value is unverified.
// File and Names return copies; no live filesystem owner escapes Verify.
type Bundle struct{ accepted *snapshot }

func (b Bundle) Facts() (Facts, bool) {
	if b.accepted == nil {
		return Facts{}, false
	}
	return b.accepted.facts, true
}

func (b Bundle) File(name string) ([]byte, bool) {
	if b.accepted == nil {
		return nil, false
	}
	data, ok := b.accepted.files[name]
	return append([]byte(nil), data...), ok
}

func (b Bundle) Names() []string {
	if b.accepted == nil {
		return nil
	}
	names := make([]string, 0, len(b.accepted.files))
	for name := range b.accepted.files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// MetadataNames returns the sorted inventory projection excluding the trusted
// root and static companions. It classifies frozen bytes only: the consumer
// constructs metadata URLs and Release independently authenticates them.
func (b Bundle) MetadataNames() []string {
	if b.accepted == nil {
		return nil
	}
	f := b.accepted.facts
	static := map[string]bool{
		"RELEASE": true, f.Artifact: true, f.TrustedRoot: true,
		f.ControlCatalog: true, f.DisclosureRoot: true, f.ControlArtifact: true,
		"release.ac1": true, "network.ac1": true, "compatibility.ac1": true,
		"release.pub": true, "network.pub": true, "compatibility.pub": true, "corpus.pub": true,
	}
	if f.Headless {
		static[artifactName("ardents-node", f.Platform)] = true
		static[artifactName("ardents-custody", f.Platform)] = true
	}
	if f.Protected {
		static["protected-endpoint.json"] = true
		for _, name := range protectedNames() {
			static[name] = true
		}
	}
	var names []string
	for name := range b.accepted.files {
		if !static[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// Verify completes all stages and closes every file before publishing a
// snapshot. It never writes state, interprets Release signatures or executes.
func Verify(ctx context.Context, request Request) (result Bundle, err error) {
	if ctx == nil || request.BundleRoot == "" || request.ExecutablePath == "" ||
		(request.Scope != General && request.Scope != Headless) || !canonicalDigest(request.ManifestSHA256) {
		return Bundle{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return Bundle{}, err
	}
	rootInfo, err := os.Lstat(request.BundleRoot)
	if err != nil {
		return Bundle{}, err
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return Bundle{}, ErrInventory
	}
	root, err := os.OpenRoot(request.BundleRoot)
	if err != nil {
		return Bundle{}, err
	}
	defer func() {
		err = errors.Join(err, root.Close())
		err = errors.Join(err, ctx.Err())
		if err != nil {
			result = Bundle{}
		}
	}()
	manifest, manifestInfo, err := readBundleFile(ctx, root, "SHA256SUMS", maximumFiles*80)
	if err != nil {
		return Bundle{}, err
	}
	if sum := sha256.Sum256(manifest); hex.EncodeToString(sum[:]) != request.ManifestSHA256 {
		return Bundle{}, ErrPin
	}
	entries, err := parseManifest(manifest)
	if err != nil {
		return Bundle{}, err
	}
	raw, descriptorInfo, err := readBundleFile(ctx, root, "RELEASE", maximumFileLen)
	if err != nil {
		return Bundle{}, err
	}
	if !matchesDigest(raw, entries["RELEASE"]) {
		return Bundle{}, ErrBinding
	}
	descriptor, err := parseDescriptor(raw)
	if err != nil {
		return Bundle{}, err
	}
	if err := checkInventory(root, entries); err != nil {
		return Bundle{}, err
	}
	files := map[string][]byte{"RELEASE": raw}
	identities := map[string]os.FileInfo{"SHA256SUMS": manifestInfo, "RELEASE": descriptorInfo}
	for _, name := range sortedEntries(entries) {
		if name == "RELEASE" {
			continue
		}
		limit := int64(maximumFileLen)
		if name == "protected-endpoint.json" {
			limit = 16 << 10
		}
		data, info, err := readBundleFile(ctx, root, name, limit)
		if err != nil {
			return Bundle{}, fmt.Errorf("read %s: %w", name, err)
		}
		if !matchesDigest(data, entries[name]) {
			return Bundle{}, ErrBinding
		}
		files[name], identities[name] = data, info
	}
	facts, err := descriptor.bind(files, request.Scope)
	if err != nil {
		return Bundle{}, err
	}
	actual, err := os.Lstat(request.ExecutablePath)
	if err != nil {
		return Bundle{}, err
	}
	if !actual.Mode().IsRegular() || !sameFileState(actual, identities[facts.Artifact]) {
		return Bundle{}, ErrBinding
	}
	// Check every original path after all I/O. Returned bytes are never reread.
	if err := checkInventory(root, entries); err != nil {
		return Bundle{}, err
	}
	for name, original := range identities {
		current, err := root.Lstat(name)
		if err != nil {
			return Bundle{}, err
		}
		if !sameFileState(original, current) {
			return Bundle{}, ErrBinding
		}
		if err := verifyOwnedFile(current); err != nil {
			return Bundle{}, err
		}
	}
	currentRoot, err := os.Lstat(request.BundleRoot)
	if err != nil {
		return Bundle{}, err
	}
	if !os.SameFile(rootInfo, currentRoot) || currentRoot.Mode()&os.ModeSymlink != 0 {
		return Bundle{}, ErrBinding
	}
	currentProgram, err := os.Lstat(request.ExecutablePath)
	if err != nil {
		return Bundle{}, err
	}
	if !sameFileState(actual, currentProgram) {
		return Bundle{}, ErrBinding
	}
	if err := ctx.Err(); err != nil {
		return Bundle{}, err
	}
	facts.ManifestSHA256 = request.ManifestSHA256
	return Bundle{accepted: &snapshot{facts: facts, files: files}}, nil
}
