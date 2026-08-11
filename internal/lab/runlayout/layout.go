package runlayout

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// SessionPrefix identifies the one temporary parent owned by a run.
	SessionPrefix = "ardents-experiment-session."
	// RunPrefix identifies disposable runtime state inside a session.
	RunPrefix = "ardents-experiment-run."
	// EvidencePrefix identifies retained bounded evidence inside a session.
	EvidencePrefix = "ardents-experiment-evidence."
)

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// Layout is the verified filesystem identity of one experiment run. Its owned
// paths are derived rather than accepted from a caller.
type Layout struct {
	runID          string
	sessionRoot    string
	repositoryRoot string
	temporaryRoot  string
	runDirectory   string
	evidenceDir    string
}

// New derives and verifies the only directories one run may own.
func New(sessionRoot, repositoryRoot, temporaryRoot, runID string) (Layout, error) {
	if !runIDPattern.MatchString(runID) {
		return Layout{}, errors.New("run ID must contain only letters, digits, dot, underscore, and hyphen")
	}
	layout := Layout{
		runID:          runID,
		sessionRoot:    sessionRoot,
		repositoryRoot: repositoryRoot,
		temporaryRoot:  temporaryRoot,
		runDirectory:   filepath.Join(sessionRoot, RunPrefix+runID),
		evidenceDir:    filepath.Join(sessionRoot, EvidencePrefix+runID),
	}
	if err := layout.validateBase(); err != nil {
		return Layout{}, err
	}
	return layout, nil
}

// OwnedPaths revalidates the identity immediately before a Module creates,
// writes, or removes its derived paths.
func (layout Layout) OwnedPaths(requireRun, requireEvidence bool) (runID, repositoryRoot, runDirectory, evidenceDirectory string, err error) {
	if err := layout.validateOwnedPaths(requireRun, requireEvidence); err != nil {
		return "", "", "", "", err
	}
	return layout.runID, layout.repositoryRoot, layout.runDirectory, layout.evidenceDir, nil
}

func (layout Layout) validateBase() error {
	for name, path := range map[string]string{
		"session root": layout.sessionRoot, "repository root": layout.repositoryRoot,
		"temporary root": layout.temporaryRoot,
	} {
		if err := requireCanonicalDirectory(path); err != nil {
			return fmt.Errorf("%s %q: %w", name, path, err)
		}
	}
	if filepath.Base(layout.sessionRoot) != SessionPrefix+layout.runID {
		return errors.New("session root does not match the run identity")
	}
	within, err := pathWithin(layout.sessionRoot, layout.temporaryRoot)
	if err != nil || !within || samePath(layout.sessionRoot, layout.temporaryRoot) {
		return errors.New("session root is not an owned child of the system temporary directory")
	}
	if pathsOverlap(layout.sessionRoot, layout.repositoryRoot) {
		return errors.New("session root intersects the repository")
	}
	if layout.runDirectory != filepath.Join(layout.sessionRoot, RunPrefix+layout.runID) ||
		layout.evidenceDir != filepath.Join(layout.sessionRoot, EvidencePrefix+layout.runID) {
		return errors.New("run paths are not derived from the session root and run identity")
	}
	return nil
}

func (layout Layout) validateOwnedPaths(requireRun, requireEvidence bool) error {
	if err := layout.validateBase(); err != nil {
		return err
	}
	for _, check := range []struct {
		name, path, basename string
		required             bool
	}{
		{name: "run directory", path: layout.runDirectory, basename: RunPrefix + layout.runID, required: requireRun},
		{name: "evidence directory", path: layout.evidenceDir, basename: EvidencePrefix + layout.runID, required: requireEvidence},
	} {
		if !filepath.IsAbs(check.path) || filepath.Clean(check.path) != check.path || filepath.Base(check.path) != check.basename {
			return fmt.Errorf("%s does not match the run identity", check.name)
		}
		withinSession, err := pathWithin(check.path, layout.sessionRoot)
		if err != nil || !withinSession || samePath(check.path, layout.sessionRoot) {
			return fmt.Errorf("%s is outside the owned session", check.name)
		}
		withinTemporary, err := pathWithin(check.path, layout.temporaryRoot)
		if err != nil || !withinTemporary || samePath(check.path, layout.temporaryRoot) {
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
	return requireNoSymlinkComponents(path)
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

func pathWithin(path, parent string) (bool, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	absoluteParent, err := filepath.Abs(parent)
	if err != nil {
		return false, err
	}
	relative, err := filepath.Rel(absoluteParent, absolutePath)
	if err != nil {
		return false, err
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)), nil
}

func samePath(left, right string) bool {
	absLeft, leftErr := filepath.Abs(left)
	absRight, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(absLeft) == filepath.Clean(absRight)
}

func pathsOverlap(left, right string) bool {
	leftWithinRight, leftErr := pathWithin(left, right)
	rightWithinLeft, rightErr := pathWithin(right, left)
	return leftErr == nil && rightErr == nil && (leftWithinRight || rightWithinLeft)
}
