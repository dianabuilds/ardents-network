package duty

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/durablefile"
)

const (
	rootMarkerName = ".ardents-local-roles-v1"
	rootMarker     = "ardents-local-roles-v1\n"
	rootLockName   = ".ardents-local-roles-lock"
)

func inspectRoot(root string, create bool) error {
	return durablefile.InspectRoot(root, create, "local role state root does not exist", "local role state root is not an owned directory")
}

func validateRootPermissions(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	return validateOwnerOnlyRoot(root, info)
}

func verifyRootClaim(root string, create bool) error {
	return verifyRootEntries(root, create, true)
}

func verifyRootCandidate(root string, create bool) error {
	return verifyRootEntries(root, create, false)
}

// verifyRootEntries validates the ownership marker and the bounded entry
// inventory in one pass (audit NW-4 twin consolidation). claim selects the
// missing-marker clause: a claimed root may hold only the lock entry, while a
// candidate root must be empty. The pre-lease candidate and post-lease claim
// boundaries and every refusal are unchanged.
func verifyRootEntries(root string, create, claim bool) error {
	marker, err := readBounded(filepath.Join(root, rootMarkerName), int64(len(rootMarker)))
	if err == nil && !bytes.Equal(marker, []byte(rootMarker)) || err != nil && !os.IsNotExist(err) {
		return errors.New("local role ownership marker is invalid")
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil || len(entries) > 9 {
		return errors.New("local role root exceeds its entry bound")
	}
	if os.IsNotExist(err) {
		uninitialized := !create || len(entries) != 0
		if claim {
			uninitialized = !create || len(entries) != 1 || entries[0].Name() != rootLockName
		}
		if uninitialized {
			return errors.New("refusing to claim an uninitialized local role root")
		}
	}
	for _, entry := range entries {
		name := entry.Name()
		allowed := name == rootMarkerName || name == rootLockName || name == "current" || name == "watermark" ||
			strings.HasPrefix(name, "state-") || strings.HasPrefix(name, ".stage-") ||
			strings.HasPrefix(name, ".current-") || strings.HasPrefix(name, ".watermark-")
		if entry.IsDir() || !allowed {
			return fmt.Errorf("unknown local role entry %q", name)
		}
	}
	return nil
}

func prepareRoot(root string, create bool) error {
	markerPath := filepath.Join(root, rootMarkerName)
	marker, err := readBounded(markerPath, int64(len(rootMarker)))
	if err == nil {
		if !bytes.Equal(marker, []byte(rootMarker)) {
			return errors.New("local role ownership marker is invalid")
		}
	} else if !os.IsNotExist(err) {
		return err
	} else {
		if !create {
			return errors.New("local role state root is not initialized")
		}
		entries, readErr := os.ReadDir(root)
		if readErr != nil || len(entries) != 1 || entries[0].Name() != rootLockName {
			return errors.New("refusing to claim a non-empty local role root")
		}
		if err := writeExclusive(markerPath, []byte(rootMarker)); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) > 9 {
		return errors.New("local role root exceeds its entry bound")
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".stage-") || strings.HasPrefix(name, ".current-") || strings.HasPrefix(name, ".watermark-") {
			if entry.IsDir() || os.Remove(filepath.Join(root, name)) != nil {
				return errors.New("local role staging entry is invalid")
			}
			continue
		}
		if name != rootMarkerName && name != rootLockName && name != "current" && name != "watermark" && !strings.HasPrefix(name, "state-") {
			return fmt.Errorf("unknown local role entry %q", name)
		}
	}
	return syncDirectory(root)
}

func writeExclusive(path string, raw []byte) error {
	return durablefile.WriteExclusive(path, raw)
}
