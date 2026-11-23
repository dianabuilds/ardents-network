package durable

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RetainedReader holds the existing exclusive State lease without a writable
// probe, cleanup or pointer repair. It grants no mutation surface.
type RetainedReader struct{ root *Root }

func OpenRetainedReader(path string, limits Limits) (*RetainedReader, error) {
	root, err := openRetained(path, limits, false)
	if err != nil {
		return nil, err
	}
	if err := verifyRetainedLayout(root.path); err != nil {
		return nil, errors.Join(err, root.Close())
	}
	return &RetainedReader{root: root}, nil
}

func (r *RetainedReader) ReadState() (string, []Generation, error) { return r.root.LoadState() }
func (r *RetainedReader) ReadControl() (string, []byte, error)     { return r.root.LoadControl() }
func (r *RetainedReader) Close() error                             { return r.root.Close() }

func retainedLayoutEntry(relative string, directory bool) bool {
	parts := strings.Split(relative, string(filepath.Separator))
	if relative == "." {
		return directory
	}
	if len(parts) == 1 {
		name := parts[0]
		if name == "generations" || name == "distribution" {
			return directory
		}
		if directory {
			return false
		}
		if name == rootLockName || name == rootMarkerName || name == "current" {
			return true
		}
		if after, ok := strings.CutPrefix(name, "closed-profile-state-"); ok {
			return generationName.MatchString(after)
		}
		return strings.HasPrefix(name, "closed-profile-") && strings.HasSuffix(name, ".bin") && generationName.MatchString(strings.TrimSuffix(strings.TrimPrefix(name, "closed-profile-"), ".bin"))
	}
	if parts[0] == "distribution" {
		if len(parts) == 2 {
			return parts[1] == "generations" && directory || parts[1] == "current" && !directory
		}
		if parts[1] != "generations" || !generationName.MatchString(parts[2]) {
			return false
		}
		return len(parts) == 3 && directory || len(parts) == 4 && parts[3] == "state.bin" && !directory
	}
	if parts[0] != "generations" || !generationName.MatchString(parts[1]) {
		return false
	}
	if len(parts) == 2 {
		return directory
	}
	if len(parts) == 3 {
		return parts[2] == "epoch.bin" && !directory || parts[2] == "inputs" && directory
	}
	if len(parts) != 4 || parts[2] != "inputs" || directory {
		return false
	}
	index, err := strconv.Atoi(strings.TrimSuffix(parts[3], ".bin"))
	return err == nil && index >= 0 && index < 64 && parts[3] == fmt.Sprintf("%04d.bin", index)
}

func verifyRetainedLayout(root string) error {
	const maximumEntries = maximumRootStableEntries + maximumStateGenerations*70 + maximumControlGenerations*2 + 32
	count := 0
	var visit func(string) error
	visit = func(path string) error {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		count++
		if count > maximumEntries {
			return errors.New("retained State inventory exceeds bound")
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || !retainedLayoutEntry(relative, info.IsDir()) || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("retained State has an unknown or uncertain entry")
		}
		if !info.IsDir() {
			if !info.Mode().IsRegular() {
				return errors.New("retained State record is not regular")
			}
			return nil
		}
		entries, err := readBoundedDirectory(path, maximumEntries-count)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := visit(filepath.Join(path, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	return visit(root)
}
