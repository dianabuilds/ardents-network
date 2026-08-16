package localroles

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	rootMarkerName = ".ardents-local-roles-v1"
	rootMarker     = "ardents-local-roles-v1\n"
	rootLockName   = ".ardents-local-roles-lock"
)

func inspectRoot(root string, create bool) error {
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		if !create {
			return errors.New("local role state root does not exist")
		}
		if err := os.MkdirAll(root, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(root)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("local role state root is not an owned directory")
	}
	return nil
}

func validateRootPermissions(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	return validateOwnerOnlyRoot(root, info)
}

func verifyRootClaim(root string, create bool) error {
	marker, err := readBounded(filepath.Join(root, rootMarkerName), int64(len(rootMarker)))
	if err == nil && !bytes.Equal(marker, []byte(rootMarker)) || err != nil && !os.IsNotExist(err) {
		return errors.New("local role ownership marker is invalid")
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil || len(entries) > 9 {
		return errors.New("local role root exceeds its entry bound")
	}
	if os.IsNotExist(err) && (!create || len(entries) != 1 || entries[0].Name() != rootLockName) {
		return errors.New("refusing to claim an uninitialized local role root")
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

func verifyRootCandidate(root string, create bool) error {
	marker, err := readBounded(filepath.Join(root, rootMarkerName), int64(len(rootMarker)))
	if err == nil && !bytes.Equal(marker, []byte(rootMarker)) || err != nil && !os.IsNotExist(err) {
		return errors.New("local role ownership marker is invalid")
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil || len(entries) > 9 {
		return errors.New("local role root exceeds its entry bound")
	}
	if os.IsNotExist(err) && (!create || len(entries) != 0) {
		return errors.New("refusing to claim an uninitialized local role root")
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
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
