package preflight

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/lab/runlayout"
)

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// RunLayout is the verified filesystem identity of one preflight run.
// Its owned paths are derived rather than accepted from a caller.
type RunLayout struct {
	identity       runlayout.Layout
	runID          string
	sessionRoot    string
	repositoryRoot string
	tempRoot       string
	runDir         string
	evidenceDir    string
}

// NewRunLayout derives and verifies the only directories one run may own.
func NewRunLayout(sessionRoot, repositoryRoot, tempRoot, runID string) (RunLayout, error) {
	if !runIDPattern.MatchString(runID) {
		return RunLayout{}, errors.New("run ID must contain only letters, digits, dot, underscore, and hyphen")
	}
	identity, err := runlayout.New(sessionRoot, repositoryRoot, tempRoot, runID)
	if err != nil {
		return RunLayout{}, err
	}
	layout := RunLayout{
		identity:       identity,
		runID:          runID,
		sessionRoot:    sessionRoot,
		repositoryRoot: repositoryRoot,
		tempRoot:       tempRoot,
		runDir:         filepath.Join(sessionRoot, runDirectoryPrefix+runID),
		evidenceDir:    filepath.Join(sessionRoot, evidenceDirectoryPrefix+runID),
	}
	if err := layout.validateBase(); err != nil {
		return RunLayout{}, err
	}
	return layout, nil
}

// OwnedPaths revalidates the run identity immediately before another Carrier
// Lab Module creates, writes, or removes its derived paths.
func (layout RunLayout) OwnedPaths(requireRun, requireEvidence bool) (runID, repositoryRoot, runDir, evidenceDir string, err error) {
	delegatedID, delegatedRepository, delegatedRun, delegatedEvidence, err := layout.identity.OwnedPaths(requireRun, requireEvidence)
	if err != nil {
		return "", "", "", "", err
	}
	if err := layout.validateOwnedPaths(requireRun, requireEvidence); err != nil {
		return "", "", "", "", err
	}
	if delegatedID != layout.runID || delegatedRepository != layout.repositoryRoot || delegatedRun != layout.runDir || delegatedEvidence != layout.evidenceDir {
		return "", "", "", "", errors.New("preflight layout does not match experiment run identity")
	}
	return layout.runID, layout.repositoryRoot, layout.runDir, layout.evidenceDir, nil
}

func (layout RunLayout) validateBase() error {
	for name, path := range map[string]string{
		"session root":    layout.sessionRoot,
		"repository root": layout.repositoryRoot,
		"temporary root":  layout.tempRoot,
	} {
		if err := requireCanonicalDirectory(path); err != nil {
			return fmt.Errorf("%s %q: %w", name, path, err)
		}
	}
	if filepath.Base(layout.sessionRoot) != sessionDirectoryPrefix+layout.runID {
		return errors.New("session root does not match the run identity")
	}
	if within, err := pathWithin(layout.sessionRoot, layout.tempRoot); err != nil || !within || samePath(layout.sessionRoot, layout.tempRoot) {
		return errors.New("session root is not an owned child of the system temporary directory")
	}
	if pathsOverlap(layout.sessionRoot, layout.repositoryRoot) {
		return errors.New("session root intersects the repository")
	}
	if layout.runDir != filepath.Join(layout.sessionRoot, runDirectoryPrefix+layout.runID) ||
		layout.evidenceDir != filepath.Join(layout.sessionRoot, evidenceDirectoryPrefix+layout.runID) {
		return errors.New("run paths are not derived from the session root and run identity")
	}
	return nil
}

func (layout RunLayout) validateOwnedPaths(requireRun, requireEvidence bool) error {
	if err := layout.validateBase(); err != nil {
		return err
	}
	for _, check := range []struct {
		name     string
		path     string
		basename string
		required bool
	}{
		{name: "run directory", path: layout.runDir, basename: runDirectoryPrefix + layout.runID, required: requireRun},
		{name: "evidence directory", path: layout.evidenceDir, basename: evidenceDirectoryPrefix + layout.runID, required: requireEvidence},
	} {
		if !filepath.IsAbs(check.path) || filepath.Clean(check.path) != check.path {
			return fmt.Errorf("%s must be an absolute canonical path", check.name)
		}
		if filepath.Base(check.path) != check.basename {
			return fmt.Errorf("%s does not match the run identity", check.name)
		}
		withinSession, err := pathWithin(check.path, layout.sessionRoot)
		if err != nil || !withinSession || samePath(check.path, layout.sessionRoot) {
			return fmt.Errorf("%s is outside the owned session", check.name)
		}
		withinTemp, err := pathWithin(check.path, layout.tempRoot)
		if err != nil || !withinTemp || samePath(check.path, layout.tempRoot) {
			return fmt.Errorf("%s is outside the system temporary directory", check.name)
		}
		if pathsOverlap(check.path, layout.repositoryRoot) {
			return fmt.Errorf("%s intersects the repository", check.name)
		}
		info, err := os.Lstat(check.path)
		switch {
		case err == nil:
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return fmt.Errorf("%s is not a canonical directory", check.name)
			}
			if err := requireCanonicalDirectory(check.path); err != nil {
				return fmt.Errorf("%s: %w", check.name, err)
			}
		case os.IsNotExist(err) && !check.required:
			// A derived directory may be created only after its existing parent
			// has passed the checks above.
		case os.IsNotExist(err):
			return fmt.Errorf("%s does not exist", check.name)
		default:
			return fmt.Errorf("inspect %s: %w", check.name, err)
		}
	}
	return nil
}

func requireCanonicalDirectory(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("path must be absolute and clean")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("path must name a real directory, not a symlink")
	}
	if err := requireNoSymlinkComponents(path); err != nil {
		return err
	}
	return nil
}

func requireNoSymlinkComponents(path string) error {
	volume := filepath.VolumeName(path)
	remainder := strings.TrimPrefix(path, volume)
	current := volume
	if strings.HasPrefix(remainder, string(filepath.Separator)) {
		current += string(filepath.Separator)
		remainder = strings.TrimLeft(remainder, string(filepath.Separator))
	}
	for _, part := range strings.Split(remainder, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("path contains a symbolic link")
		}
	}
	return nil
}

func pathsOverlap(left, right string) bool {
	leftWithinRight, leftErr := pathWithin(left, right)
	rightWithinLeft, rightErr := pathWithin(right, left)
	return leftErr == nil && rightErr == nil && (leftWithinRight || rightWithinLeft)
}
