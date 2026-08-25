package inspection

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	inspectionMarkerName = ".ardents-alpha-control-inspection-v1"
	inspectionMarker     = "ardents-alpha-control-inspection-v1\n"
)

// prepareInspectionRoot claims an empty root or validates one already owned by
// this reader. Its fixed children keep catalog, Release, and Network State
// floors physically distinct from any Endpoint root.
func prepareInspectionRoot(path string) (string, string, string, error) {
	root, err := filepath.Abs(path)
	if err != nil {
		return "", "", "", err
	}
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return "", "", "", fmt.Errorf("create alpha control inspection root: %w", err)
		}
		if err := os.WriteFile(filepath.Join(root, inspectionMarkerName), []byte(inspectionMarker), 0o600); err != nil {
			return "", "", "", fmt.Errorf("mark alpha control inspection root: %w", err)
		}
	} else if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", "", "", errors.New("alpha control inspection root is not an owned directory")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", "", "", err
	}
	markerPath := filepath.Join(root, inspectionMarkerName)
	marker, err := os.ReadFile(markerPath)
	if os.IsNotExist(err) && len(entries) == 0 {
		if err := os.WriteFile(markerPath, []byte(inspectionMarker), 0o600); err != nil {
			return "", "", "", fmt.Errorf("mark alpha control inspection root: %w", err)
		}
		entries, err = os.ReadDir(root)
		if err != nil {
			return "", "", "", err
		}
	} else if err != nil || string(marker) != inspectionMarker {
		return "", "", "", errors.New("alpha control inspection root marker is invalid")
	}
	allowed := map[string]bool{inspectionMarkerName: true, "catalog": true, "release": true, "network": true}
	for _, entry := range entries {
		if !allowed[entry.Name()] || entry.Name() != inspectionMarkerName && (!entry.IsDir() || entry.Type()&os.ModeSymlink != 0) {
			return "", "", "", errors.New("alpha control inspection root has an unknown entry")
		}
	}
	return filepath.Join(root, "catalog"), filepath.Join(root, "release"), filepath.Join(root, "network"), nil
}
