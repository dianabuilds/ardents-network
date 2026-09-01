package credential

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func inspectIssuerRoot(root string, create bool) error {
	info, err := os.Lstat(root)
	if os.IsNotExist(err) && create {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(root)
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("transit grant issuer root is not an owned directory")
	}
	return validateIssuerRootPermissions(root, info)
}

func prepareIssuerRoot(root string, create bool) error {
	markerPath := filepath.Join(root, issuerRootMarkerName)
	marker, err := readIssuerFile(markerPath, int64(len(issuerRootMarker)))
	if err == nil && !bytes.Equal(marker, []byte(issuerRootMarker)) || err != nil && !os.IsNotExist(err) {
		return errors.New("transit grant issuer ownership marker is invalid")
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil || len(entries) > 9 {
		return errors.New("transit grant issuer root exceeds its entry bound")
	}
	if os.IsNotExist(err) {
		if !create || len(entries) != 1 || entries[0].Name() != issuerRootLockName {
			return errors.New("refusing to claim an uninitialized transit grant issuer root")
		}
		if err := writeIssuerExclusive(markerPath, []byte(issuerRootMarker)); err != nil {
			return err
		}
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".stage-") || strings.HasPrefix(name, ".current-") || strings.HasPrefix(name, ".watermark-") {
			if entry.IsDir() || os.Remove(filepath.Join(root, name)) != nil {
				return errors.New("transit grant issuer staging entry is invalid")
			}
			continue
		}
		if entry.IsDir() || name != issuerRootMarkerName && name != issuerRootLockName && name != "current" && name != "watermark" && !strings.HasPrefix(name, "state-") {
			return fmt.Errorf("unknown transit grant issuer entry %q", name)
		}
	}
	return syncIssuerDirectory(root)
}

func writeIssuerGeneration(root, name string, raw []byte) error {
	final := filepath.Join(root, "state-"+name)
	if existing, err := readIssuerFile(final, maximumIssuerState); err == nil {
		if bytes.Equal(existing, raw) {
			return nil
		}
		return errors.New("immutable transit grant issuer generation disagrees")
	} else if !os.IsNotExist(err) {
		return err
	}
	temporary, err := os.CreateTemp(root, ".stage-")
	if err != nil {
		return err
	}
	path := temporary.Name()
	defer func() { _ = os.Remove(path) }()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(raw)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(path, final); err != nil {
		return err
	}
	return syncIssuerDirectory(root)
}

func replaceIssuerFile(root, prefix, name, content string) error {
	temporary, err := os.CreateTemp(root, "."+prefix+"-")
	if err != nil {
		return err
	}
	path := temporary.Name()
	defer func() { _ = os.Remove(path) }()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.WriteString(content)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(path, filepath.Join(root, name)); err != nil {
		return err
	}
	return syncIssuerDirectory(root)
}

func cleanupIssuerGenerations(root, current, previous string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "state-") {
			continue
		}
		name := strings.TrimPrefix(entry.Name(), "state-")
		if name != current && name != previous {
			if err := os.Remove(filepath.Join(root, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return syncIssuerDirectory(root)
}

func writeIssuerExclusive(path string, raw []byte) error {
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

func readIssuerFile(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if int64(len(raw)) > maximum {
		_ = file.Close()
		return nil, fmt.Errorf("bounded transit grant issuer file exceeds %d bytes", maximum)
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return raw, nil
}
