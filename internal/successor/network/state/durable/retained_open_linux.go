package durable

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// OpenRetained is the selected native consumer's existing-history acquisition.
// It creates no ownership marker, semantic directory or security journal and
// does not clean a partial prefix into a fresh root. State verifies meaning.
func OpenRetained(path string, limits Limits) (openedRoot *Root, resultErr error) {
	if limits.EpochBytes <= 0 || limits.RecordBytes <= 0 || limits.ClosedProfileBytes <= 0 {
		return nil, errors.New("state storage bounds are invalid")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err = inspectRetainedRoot(absolute); err != nil {
		return nil, err
	}
	lease, err := acquireRootLease(absolute)
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			resultErr = errors.Join(resultErr, lease.release())
		}
	}()
	if err = inspectRetainedRoot(absolute); err != nil {
		return nil, err
	}
	if err = verifyRootWritable(absolute); err != nil {
		return nil, err
	}
	root := &Root{path: absolute, lease: lease, limits: limits}
	opened = true
	return root, nil
}

func inspectRetainedRoot(root string) error {
	for _, name := range []string{"", "generations", "distribution", filepath.Join("distribution", "generations")} {
		info, err := os.Lstat(filepath.Join(root, name))
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("retained state directory is missing or invalid")
		}
	}
	for _, name := range []string{rootMarkerName, rootLockName, filepath.Join("distribution", "current")} {
		info, err := os.Lstat(filepath.Join(root, name))
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("retained state record is missing or invalid")
		}
	}
	marker, err := readBoundedFile(filepath.Join(root, rootMarkerName), int64(len(rootMarker)))
	if err != nil || !bytes.Equal(marker, []byte(rootMarker)) {
		return errors.New("retained state ownership marker is invalid")
	}
	pointer, err := readBoundedFile(filepath.Join(root, "distribution", "current"), 65)
	name := strings.TrimSuffix(string(pointer), "\n")
	if err != nil || string(pointer) != name+"\n" || !generationName.MatchString(name) {
		return errors.New("retained distribution pointer is invalid")
	}
	info, err := os.Lstat(filepath.Join(root, "distribution", "generations", name, "state.bin"))
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() <= 0 || info.Size() > 4096 {
		return errors.New("retained distribution generation is missing or invalid")
	}
	return nil
}
