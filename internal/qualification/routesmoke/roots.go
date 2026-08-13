package routesmoke

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func validateRoots(input Config) error {
	fixture, err := canonicalNewPath(input.FixtureRoot)
	if err != nil {
		return err
	}
	evidence, err := canonicalNewPath(input.EvidenceRoot)
	if err != nil {
		return err
	}
	source, err := filepath.EvalSymlinks(input.SourceRoot)
	if err != nil {
		return err
	}
	if pathWithin(fixture, evidence) || pathWithin(evidence, fixture) ||
		pathWithin(source, fixture) || pathWithin(source, evidence) {
		return errors.New("route smoke fixture and evidence roots must be separate and outside the source repository")
	}
	return nil
}

func canonicalNewPath(path string) (string, error) {
	current, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	missing := make([]string, 0, 2)
	for {
		if _, statErr := os.Lstat(current); statErr == nil {
			break
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("route smoke path has no existing ancestor")
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
	current, err = filepath.EvalSymlinks(current)
	if err != nil {
		return "", err
	}
	for index := len(missing) - 1; index >= 0; index-- {
		current = filepath.Join(current, missing[index])
	}
	return filepath.Clean(current), nil
}

func pathWithin(parent, candidate string) bool {
	relative, err := filepath.Rel(parent, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}
