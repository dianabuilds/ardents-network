package release

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Reopening does not establish durability by reading cached committed bytes.
// Reconfirm the exact validated current generation under the native lease;
// neither residue nor another retained generation may become current here.
func (s *floorStore) confirmDurability() error {
	before, err := s.ReadFloors()
	if err != nil {
		return err
	}
	for _, name := range []string{floorStoreMarkerName} {
		if err = syncRetainedFile(filepath.Join(s.path, name), maximumMetadataFileBytes); err != nil {
			return err
		}
	}
	genRoot := filepath.Join(s.path, "generations")
	if before.RootVersion > 0 {
		pointer, err := readBoundedFloorFile(filepath.Join(s.path, "current"), 65)
		if err != nil {
			return err
		}
		if len(pointer) != 65 || pointer[64] != '\n' || !floorGenerationName.Match(pointer[:64]) {
			return errors.New("release: changed history pointer before flush")
		}
		directory := filepath.Join(genRoot, string(pointer[:64]))
		if err = syncRetainedFile(filepath.Join(directory, "state.bin"), floorFileSizeLimit); err != nil {
			return err
		}
		rootDirectory := filepath.Join(directory, "roots")
		roots, err := readFloorStoreDirectory(rootDirectory, int(maximumRootRotations)+1)
		if err != nil {
			return err
		}
		for _, entry := range roots {
			if err = syncRetainedFile(filepath.Join(rootDirectory, entry.Name()), maximumMetadataFileBytes); err != nil {
				return err
			}
		}
		for _, path := range []string{rootDirectory, directory} {
			if err = s.flush(path); err != nil {
				return err
			}
		}
		if err = syncRetainedFile(filepath.Join(s.path, "current"), 65); err != nil {
			return err
		}
	}
	for _, path := range []string{genRoot, s.path} {
		if err = s.flush(path); err != nil {
			return err
		}
	}
	after, err := s.ReadFloors()
	if err != nil {
		return err
	}
	if !floorSetEqual(before, after) {
		return errors.New("release: history changed while confirming durability")
	}
	return nil
}

func syncRetainedFile(path string, bound int64) error {
	before, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !before.Mode().IsRegular() || before.Size() > bound {
		return errors.New("release: invalid retained file for flush")
	}
	// Windows FlushFileBuffers requires a write-capable handle. This opens an
	// existing owned file without creation, truncation or byte mutation.
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	opened, statErr := file.Stat()
	if statErr != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return errors.Join(errors.New("release: retained file changed before flush"), statErr, file.Close())
	}
	syncErr := file.Sync()
	finished, finishErr := file.Stat()
	pathAfter, pathErr := os.Lstat(path)
	var changed error
	if finishErr != nil || pathErr != nil || !os.SameFile(before, finished) || !os.SameFile(before, pathAfter) || before.Size() != finished.Size() || !before.ModTime().Equal(finished.ModTime()) {
		changed = errors.New("release: retained file changed during flush")
	}
	return errors.Join(syncErr, finishErr, pathErr, changed, file.Close())
}

func writeSyncedFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("release: create %s: %w", filepath.Base(path), err)
	}
	written, err := file.Write(contents)
	if err == nil && written != len(contents) {
		err = errors.New("short write")
	}
	if err != nil {
		return errors.Join(fmt.Errorf("release: write %s: %w", filepath.Base(path), err), file.Close())
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(syncErr, closeErr); err != nil {
		return fmt.Errorf("release: finish %s: %w", filepath.Base(path), err)
	}
	return nil
}
