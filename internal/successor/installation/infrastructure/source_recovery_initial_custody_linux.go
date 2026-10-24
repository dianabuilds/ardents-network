package infrastructure

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Presence selects mandatory recovered validation, never proof of completion.
// A full115 original may have been interrupted before final Close; marker
// absence cannot hide its cut journal/result/guard or partial recovery suffix.
func sourceInitialRecoveryEvidencePresent(journal string) (bool, error) {
	control, result, guard := sourceCutPaths(journal)
	present := false
	for _, path := range []string{control, result, guard, journal + ".recovery-completion"} {
		_, err := os.Lstat(path)
		if err == nil {
			present = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return present, nil
}

// Retain each recorded resource through a fresh independent descriptor. The
// original prefix was already checked against exact freshly reconstructed
// configuration; a recorded inode number alone never satisfies this opening.
func (s *sourceInitialRecovery) retain() error {
	if err := retainSourceRecoveryBirths(s.prefix, s.writer, s.bodies); err != nil {
		return err
	}
	for _, original := range s.writer.files {
		if original.path == sourceInitialPendingPath(s.binding.Request.InstallationRoot) {
			s.pending = original
		}
	}
	if s.pending == nil {
		return ErrBinding
	}
	s.parent = s.prefix.identities[filepath.Dir(s.pending.path)]
	if s.parent == nil {
		return ErrBinding
	}
	return s.writer.check()
}

func retainSourceRecoveryBirths(prefix *sourceRecoveryPrefix, writer *sourceBirths, bodies map[string][]byte) error {
	directories := map[string]sourceRecord{}
	files := map[string]sourceRecord{}
	for _, record := range prefix.records {
		switch record.Phase {
		case "directory-ready", "sealed":
			directories[record.Path] = record
		case "file-ready":
			files[record.Path] = record
		}
	}
	for path, record := range directories {
		file, info, err := openSourceDirectory(path, os.FileMode(record.Mode), record.UID, record.GID)
		if file != nil {
			writer.directoryFiles[path] = file
		}
		if err != nil {
			return err
		}
		writer.directories[path] = info
		native := info.Sys().(*syscall.Stat_t)
		if uint64(native.Dev) != record.Device || native.Ino != record.Inode {
			return ErrBinding
		}
	}
	// Independently retain every ancestor too, without manufacturing birth
	// provenance for those existing host directories.
	for path := range directories {
		for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
			if writer.directoryFiles[parent] == nil {
				info, err := os.Lstat(parent)
				if err != nil || !trustedDirectory(info) {
					return errors.Join(ErrBinding, err)
				}
				file, original, err := openSourceDirectory(parent, info.Mode().Perm(), 0, 0)
				if file != nil {
					writer.directoryFiles[parent] = file
				}
				if err != nil {
					return err
				}
				writer.directories[parent] = original
			}
			if parent == "/" {
				break
			}
		}
	}
	for path, record := range files {
		body, ok := bodies[path]
		if !ok || digest(body) != record.SHA256 {
			return ErrBinding
		}
		file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if err != nil {
			return err
		}
		original := &sourceBirth{path: path, file: file, body: bytes.Clone(body)}
		writer.files = append(writer.files, original)
		original.info, err = file.Stat()
		if err != nil || !sourceAccess(original.info, os.FileMode(record.Mode), record.UID, record.GID, false) || original.info.Size() != int64(len(body)) {
			return errors.Join(ErrBinding, err)
		}
		native := original.info.Sys().(*syscall.Stat_t)
		if uint64(native.Dev) != record.Device || native.Ino != record.Inode {
			return ErrBinding
		}
		var fs syscall.Statfs_t
		if err = syscall.Fstatfs(int(file.Fd()), &fs); err != nil || fs.Type != 0xef53 {
			return errors.Join(ErrBinding, err)
		}
		if err = original.check(); err != nil {
			return err
		}
	}
	return writer.check()
}

// A completed initial unit may be referenced only after independent selected
// custody matches every recorded birth. This consumes actual descriptors and
// bytes, not a detached inode inventory or a recreated original writer.
func checkSourceRecordedBirths(selected *sourceSelection, prefix *sourceRecoveryPrefix, bodies map[string][]byte) error {
	if selected == nil || prefix == nil {
		return ErrInput
	}
	latest := map[string]sourceRecord{}
	for _, record := range prefix.records {
		switch record.Phase {
		case "directory-ready", "sealed", "file-ready":
			latest[record.Path] = record
		}
	}
	for path, record := range latest {
		directory := record.Phase != "file-ready"
		info := selected.identities[path]
		if !directory {
			body, ok := bodies[path]
			if !ok || digest(body) != record.SHA256 {
				return ErrBinding
			}
			file := selected.files[path]
			if file == nil {
				// The initial pending sibling is outside the generation inventory.
				if path != sourceInitialPendingPath(selected.binding.Request.InstallationRoot) {
					return ErrBinding
				}
				if _, err := selected.read(path, 128, 0600, 0, 0); err != nil {
					return err
				}
				file = selected.files[path]
			}
			if !bytes.Equal(file.raw, body) {
				return ErrBinding
			}
			info = file.info
		}
		if !sourceAccess(info, os.FileMode(record.Mode), record.UID, record.GID, directory) {
			return ErrBinding
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || uint64(native.Dev) != record.Device || native.Ino != record.Inode {
			return ErrBinding
		}
	}
	return errors.Join(selected.observe(), prefix.check())
}

func (s *sourceInitialRecovery) verifyReadback(hashes map[string]string, admitted string) error {
	owned, err := decodeSourceRecoveredRecords(s.writer.recovery.transcript, s.raw, "initial", hashes, admitted, "")
	if err != nil {
		return err
	}
	for path, record := range owned {
		info, directory := s.selected.identities[path]
		if path == s.prefix.journal.path {
			info, directory = s.prefix.journal.info, false
		} else if !directory {
			file := s.selected.files[path]
			if file == nil {
				return ErrBinding
			}
			info = file.info
		}
		if !sourceAccess(info, os.FileMode(record.Mode), record.UID, record.GID, directory) {
			return ErrBinding
		}
		native := info.Sys().(*syscall.Stat_t)
		if uint64(native.Dev) != record.Device || native.Ino != record.Inode {
			return ErrBinding
		}
	}
	return errors.Join(s.selected.observe(), s.prefix.check())
}

func (s *sourceInitialRecovery) publish() error {
	if !s.complete || s.writer == nil || !s.writer.closed || s.writer.terminal != nil || s.writer.recovery == nil || !s.writer.recovery.finished || s.prefix == nil || !s.prefix.closed || s.prefix.terminal != nil || s.pending == nil || s.parent == nil {
		return ErrBinding
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	parent, err := os.Lstat(filepath.Dir(s.pending.path))
	if err != nil || !os.SameFile(s.parent, parent) || !sourceAccess(parent, s.parent.Mode().Perm(), 0, 0, true) {
		return errors.Join(ErrBinding, err)
	}
	named, err := os.Lstat(s.pending.path)
	if err != nil || !sameSourceFile(s.pending.info, named) {
		return errors.Join(ErrBinding, err)
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	return os.Remove(s.pending.path)
}

func (s *sourceInitialRecovery) checkGenerationEntries() error {
	sealed := filepath.Join(s.binding.Request.InstallationRoot, "generations", s.binding.Generation)
	return checkSourceRecoveryGeneration(s.prefix, s.writer, sealed)
}

func checkSourceRecoveryGeneration(prefix *sourceRecoveryPrefix, writer *sourceBirths, sealed string) error {
	known := map[string]bool{}
	records := prefix.records
	if writer != nil {
		records = append(append([]sourceRecord(nil), records...), writer.recovery.records...)
	}
	for _, record := range records {
		if (record.Phase == "file-born" || record.Phase == "file-ready") && filepath.Dir(record.Path) == sealed {
			known[filepath.Base(record.Path)] = true
		}
	}
	directory, err := os.OpenFile(sealed, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	entries, readErr := directory.ReadDir(32)
	closeErr := directory.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return errors.Join(readErr, closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if len(entries) >= 32 {
		return ErrBinding
	}
	for _, entry := range entries {
		if !known[entry.Name()] || !entry.Type().IsRegular() {
			return ErrBinding
		}
	}
	return nil
}
