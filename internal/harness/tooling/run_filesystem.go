package tooling

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/preflight"
)

const smokeEvidenceCap = 64 * 1024

type runLayout struct {
	identity       preflight.RunLayout
	runID          string
	repositoryRoot string
	runDir         string
	evidenceDir    string
}

func ownedLayout(identity preflight.RunLayout, requireRun, requireEvidence bool) (runLayout, error) {
	runID, repositoryRoot, runDir, evidenceDir, err := identity.OwnedPaths(requireRun, requireEvidence)
	if err != nil {
		return runLayout{}, err
	}
	return runLayout{identity: identity, runID: runID, repositoryRoot: repositoryRoot, runDir: runDir, evidenceDir: evidenceDir}, nil
}

func prepareSmokeWorkspace(layout runLayout) error {
	if _, err := ownedLayout(layout.identity, false, false); err != nil {
		return err
	}
	for _, directory := range []string{layout.runDir, layout.evidenceDir} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return fmt.Errorf("create tooling workspace: %w", err)
		}
	}
	_, err := ownedLayout(layout.identity, true, true)
	return err
}

func carrierLabComposePath(repositoryRoot string) string {
	return filepath.Join(repositoryRoot, "carrier-lab", "compose.yaml")
}

func carrierLabToolLockPath(repositoryRoot string) string {
	return filepath.Join(repositoryRoot, "carrier-lab", "tools.lock")
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

func writeAtomic(path string, data []byte, mode os.FileMode) (runErr error) {
	if err := requireCanonicalDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".carrier-lab-tooling-evidence-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		if cleanupErr := os.Remove(temporaryPath); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			runErr = errors.Join(runErr, cleanupErr)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func writeBoundedJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > smokeEvidenceCap {
		return errors.New("tooling evidence exceeds bounded size")
	}
	return writeAtomic(path, data, 0o644)
}

func removeSmokeRunDirectory(layout runLayout) error {
	if _, err := ownedLayout(layout.identity, false, false); err != nil {
		return err
	}
	if info, err := os.Lstat(layout.runDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("tooling run path is not an owned directory")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.RemoveAll(layout.runDir); err != nil {
		return err
	}
	if _, err := os.Stat(layout.runDir); !os.IsNotExist(err) {
		return errors.New("tooling run directory remains after cleanup")
	}
	return nil
}
