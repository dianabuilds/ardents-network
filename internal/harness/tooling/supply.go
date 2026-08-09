package tooling

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/qualification"
)

const toolBundleSchema = "carrier-lab-tool-bundle/v1"

var toolComponentName = regexp.MustCompile(`^[a-z0-9][a-z0-9.+-]*$`)

type toolBundle struct {
	Schema     string
	Platform   string
	BaseImage  string
	LockSHA256 string
	BundlePath string
	Tools      map[string]toolDefinition
	Packages   []toolPackage
}

type toolDefinition struct {
	Name    string
	Version string
	Path    string
	SHA256  string
}

type toolPackage struct {
	Name     string
	Version  string
	Filename string
	SHA256   string
	Source   string
	License  string
}

type toolObservation struct {
	Name    string
	Version string
	Path    string
	SHA256  string
}

// VerifyInputs parses the committed identity lock, verifies that the
// external directory contains exactly the named package artifacts, and proves
// the locked base image is already present before a build can be requested.
// The returned source digest binds code, tests, and qualification infrastructure.
func VerifyInputs(lockPath, bundlePath, repositoryRoot string) (lockSHA256 string, packageCount int, baseImageID, sourceSHA256 string, err error) {
	identity, err := verifyToolBundle(lockPath, bundlePath, repositoryRoot)
	if err != nil {
		return "", 0, "", "", err
	}
	baseImageID, err = verifyLocalToolingBaseWithDocker(identity)
	if err != nil {
		return "", 0, "", "", err
	}
	sourceSHA256, err = qualification.SourceSHA256(repositoryRoot)
	if err != nil {
		return "", 0, "", "", fmt.Errorf("qualification source snapshot: %w", err)
	}
	return identity.LockSHA256, len(identity.Packages), baseImageID, sourceSHA256, nil
}

func verifyToolBundle(lockPath, bundlePath, repositoryRoot string) (toolBundle, error) {
	repository, err := canonicalDirectory(repositoryRoot)
	if err != nil {
		return toolBundle{}, fmt.Errorf("repository root: %w", err)
	}
	bundleDirectory, err := canonicalDirectory(bundlePath)
	if err != nil {
		return toolBundle{}, fmt.Errorf("tool bundle: %w", err)
	}
	if pathWithinOrSame(bundleDirectory, repository) || pathWithinOrSame(repository, bundleDirectory) {
		return toolBundle{}, errors.New("tool bundle must be outside the repository")
	}
	expectedLock := carrierLabToolLockPath(repository)
	if !sameFilesystemPath(lockPath, expectedLock) {
		return toolBundle{}, errors.New("tool lock must be the repository carrier-lab/tools.lock")
	}
	identity, err := readToolLock(lockPath)
	if err != nil {
		return toolBundle{}, err
	}
	identity.BundlePath = bundleDirectory
	if err := verifyPackageFiles(identity.Packages, bundleDirectory); err != nil {
		return toolBundle{}, err
	}
	return identity, nil
}

// ReadToolLock reads a strict lock without requiring the external package
// bundle. Tooling roles use it to bind their runtime observations to the same
// committed identity that the offline build verified.
func readToolLock(lockPath string) (toolBundle, error) {
	lock, err := canonicalRegularFile(lockPath)
	if err != nil {
		return toolBundle{}, fmt.Errorf("tool lock: %w", err)
	}
	file, err := os.Open(lock)
	if err != nil {
		return toolBundle{}, err
	}
	defer file.Close()
	identity, err := parseToolLock(file)
	if err != nil {
		return toolBundle{}, err
	}
	identity.LockSHA256, err = hashRegularFile(lock)
	if err != nil {
		return toolBundle{}, err
	}
	return identity, nil
}

func parseToolLock(reader io.Reader) (toolBundle, error) {
	identity := toolBundle{Tools: make(map[string]toolDefinition)}
	metadata := make(map[string]string)
	packages := make(map[string]toolPackage)
	scanner := bufio.NewScanner(io.LimitReader(reader, 256*1024))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if line == "" {
			return toolBundle{}, fmt.Errorf("tool lock line %d is empty", lineNumber)
		}
		fields := strings.Split(line, "\t")
		switch fields[0] {
		case "meta":
			if len(fields) != 3 || fields[1] == "" || fields[2] == "" {
				return toolBundle{}, fmt.Errorf("malformed metadata at line %d", lineNumber)
			}
			if _, found := metadata[fields[1]]; found {
				return toolBundle{}, fmt.Errorf("duplicate metadata %q", fields[1])
			}
			metadata[fields[1]] = fields[2]
		case "tool":
			if len(fields) != 5 || !toolComponentName.MatchString(fields[1]) || fields[2] == "" || !validToolPath(fields[3]) || !validSHA256(fields[4]) {
				return toolBundle{}, fmt.Errorf("malformed tool identity at line %d", lineNumber)
			}
			if _, found := identity.Tools[fields[1]]; found {
				return toolBundle{}, fmt.Errorf("duplicate tool %q", fields[1])
			}
			identity.Tools[fields[1]] = toolDefinition{Name: fields[1], Version: fields[2], Path: fields[3], SHA256: fields[4]}
		case "package":
			if len(fields) != 7 || !toolComponentName.MatchString(fields[1]) || fields[2] == "" || filepath.Base(fields[3]) != fields[3] || !strings.HasSuffix(fields[3], ".deb") || !validSHA256(fields[4]) || !strings.HasPrefix(fields[5], "https://archive.ubuntu.com/ubuntu/") || fields[6] == "" {
				return toolBundle{}, fmt.Errorf("malformed package identity at line %d", lineNumber)
			}
			if _, found := packages[fields[1]]; found {
				return toolBundle{}, fmt.Errorf("duplicate package %q", fields[1])
			}
			packages[fields[1]] = toolPackage{Name: fields[1], Version: fields[2], Filename: fields[3], SHA256: fields[4], Source: fields[5], License: fields[6]}
		default:
			return toolBundle{}, fmt.Errorf("unknown tool lock record at line %d", lineNumber)
		}
	}
	if err := scanner.Err(); err != nil {
		return toolBundle{}, err
	}
	if len(metadata) != 3 || metadata["schema"] != toolBundleSchema || metadata["platform"] != "linux/amd64" || !validImageReference(metadata["base_image"]) {
		return toolBundle{}, errors.New("tool lock metadata is missing or unsupported")
	}
	if len(identity.Tools) == 0 || len(packages) == 0 {
		return toolBundle{}, errors.New("tool lock must name tools and packages")
	}
	identity.Schema = metadata["schema"]
	identity.Platform = metadata["platform"]
	identity.BaseImage = metadata["base_image"]
	for _, item := range packages {
		identity.Packages = append(identity.Packages, item)
	}
	slices.SortFunc(identity.Packages, func(left, right toolPackage) int { return strings.Compare(left.Name, right.Name) })
	return identity, nil
}

// VerifyObservation binds runtime version output and the on-image file digest
// back to the committed tool identity.
func (identity toolBundle) verifyObservation(observed toolObservation) error {
	expected, found := identity.Tools[observed.Name]
	if !found {
		return fmt.Errorf("unexpected tool %q", observed.Name)
	}
	if observed.Version != expected.Version {
		return fmt.Errorf("%s version %q does not match %q", observed.Name, observed.Version, expected.Version)
	}
	if observed.Path != expected.Path {
		return fmt.Errorf("%s path %q does not match %q", observed.Name, observed.Path, expected.Path)
	}
	if !strings.EqualFold(observed.SHA256, expected.SHA256) {
		return fmt.Errorf("%s digest does not match the lock", observed.Name)
	}
	return nil
}

func verifyPackageFiles(packages []toolPackage, bundleDirectory string) error {
	entries, err := os.ReadDir(bundleDirectory)
	if err != nil {
		return err
	}
	expected := make(map[string]toolPackage, len(packages))
	for _, item := range packages {
		expected[item.Filename] = item
	}
	if len(entries) != len(expected) {
		return fmt.Errorf("tool bundle contains %d entries, want exactly %d", len(entries), len(expected))
	}
	for _, entry := range entries {
		item, found := expected[entry.Name()]
		if !found || entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("unexpected tool bundle entry %q", entry.Name())
		}
		digest, err := hashRegularFile(filepath.Join(bundleDirectory, entry.Name()))
		if err != nil {
			return err
		}
		if !strings.EqualFold(digest, item.SHA256) {
			return fmt.Errorf("package %s digest does not match the lock", item.Name)
		}
	}
	return nil
}

func validSHA256(value string) bool {
	digest, err := hex.DecodeString(value)
	return err == nil && len(digest) == 32 && value == strings.ToLower(value)
}

func validImageReference(value string) bool {
	name, digest, found := strings.Cut(value, "@sha256:")
	return found && name != "" && validSHA256(digest)
}

func validToolPath(value string) bool {
	return strings.HasPrefix(value, "/") && !strings.Contains(value, "\\") && value == filepath.ToSlash(filepath.Clean(value))
}
