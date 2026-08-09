package directcontrol

import (
	"errors"
	"fmt"
	"os"

	"github.com/dianabuilds/ardents-network/internal/preflight"
)

type controlLayout struct {
	identity    preflight.RunLayout
	runID       string
	runDir      string
	evidenceDir string
}

func openControlLayout(identity preflight.RunLayout, requireRun, requireEvidence bool) (controlLayout, error) {
	runID, _, runDir, evidenceDir, err := identity.OwnedPaths(requireRun, requireEvidence)
	if err != nil {
		return controlLayout{}, err
	}
	return controlLayout{identity: identity, runID: runID, runDir: runDir, evidenceDir: evidenceDir}, nil
}

func prepareControlWorkspace(layout controlLayout) error {
	if _, err := openControlLayout(layout.identity, false, false); err != nil {
		return err
	}
	for _, directory := range []string{layout.runDir, layout.evidenceDir} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			return fmt.Errorf("create Direct-control workspace: %w", err)
		}
	}
	_, err := openControlLayout(layout.identity, true, true)
	return err
}

func removeControlRunDirectory(layout controlLayout) error {
	if _, err := openControlLayout(layout.identity, false, false); err != nil {
		return err
	}
	if info, err := os.Lstat(layout.runDir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("direct-control run path is not an owned directory")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.RemoveAll(layout.runDir); err != nil {
		return err
	}
	if _, err := os.Stat(layout.runDir); !os.IsNotExist(err) {
		return errors.New("direct-control run directory remains after cleanup")
	}
	return nil
}

func controlChecksPassed(checks map[string]bool, ignored string) bool {
	for name, passed := range checks {
		if name != ignored && !passed {
			return false
		}
	}
	return true
}
