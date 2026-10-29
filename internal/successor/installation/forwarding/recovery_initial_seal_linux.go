package forwarding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// The same Installation operation owns this directory's descriptor in files.
// This physical continuation never owns a foreign root or grants admission.
type initialSealContinuation struct {
	directory *inputDirectory
	record    initialRecord
	attempted bool
	sealed    bool
	terminal  error
	// Failure/hold-only control after the actual directory Sync.
	afterSync func() error
}

func validInitialSealDirectory(record *initialRecord, path string) bool {
	return record != nil && record.Sequence == 50 && record.Phase == "directory-ready" && record.Path == path && record.Device != 0 && record.Inode != 0 && record.Mode == 0755 && record.UID == 0 && record.GID == 0 && record.SHA256 == ""
}

func (u *initialSealContinuation) native(mode os.FileMode) (os.FileInfo, error) {
	if u == nil || u.directory == nil || u.directory.file == nil || !validInitialSealDirectory(&u.record, u.directory.path) {
		return nil, ErrBinding
	}
	d := u.directory
	current, err := d.file.Stat()
	named, nameErr := os.Lstat(d.path)
	native, ok := infoSys(current)
	var fs syscall.Statfs_t
	fsErr := syscall.Fstatfs(int(d.file.Fd()), &fs)
	if err != nil || nameErr != nil || fsErr != nil || !ok || fs.Type != 0xef53 || fs.Flags&1 != 0 || !nodeAccess(current, mode, 0, 0, true) || !nodeAccess(named, mode, 0, 0, true) || !os.SameFile(d.info, current) || !os.SameFile(current, named) || uint64(native.Dev) != u.record.Device || native.Ino != u.record.Inode {
		return nil, errors.Join(ErrBinding, err, nameErr, fsErr)
	}
	return current, nil
}

// The actor has already recorded its guarded intent and rechecked admission.
// Retain the original FD even on failure; files joins its actual Close later.
func (u *initialSealContinuation) commit(ctx context.Context, files map[string][]byte) (returned error) {
	if u == nil || ctx == nil || u.attempted || u.sealed || len(files) != 12 {
		if u != nil {
			return errors.Join(ErrBinding, u.terminal)
		}
		return ErrInput
	}
	u.attempted = true
	defer func() { u.terminal = errors.Join(u.terminal, returned) }()
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := u.native(0755); err != nil {
		return err
	}
	if err := verifyInitialInventory(u.directory.path, files); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := u.directory.file.Chmod(0555); err != nil {
		return err
	}
	if err := u.directory.file.Sync(); err != nil {
		return err
	}
	if u.afterSync != nil {
		if err := u.afterSync(); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := u.native(0555); err != nil {
		return err
	}
	if err := verifyInitialInventory(u.directory.path, files); err != nil {
		return err
	}
	current, err := u.native(0555)
	if err != nil {
		return err
	}
	u.directory.info = current
	u.sealed = true
	return ctx.Err()
}

func (s *recoveryInitialReloadAttempt) sealGeneration() error {
	u := s.seal
	b := s.candidate.selection.binding
	path := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	if u == nil || u.directory == nil || u.directory != s.files.files.dirs[path] || u.attempted || u.sealed || s.proof.generationDirectory != &u.record || !initialSealCut(s.original.cut.Boundary.Records) || s.publication.births == nil {
		return ErrBinding
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.publication.births.record("continued-directory-seal-intent", path, u.directory.info, b.Generation, 0555, 0, 0); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := u.commit(s.ctx, s.candidate.files); err != nil {
		return err
	}
	if err := s.observe(); err != nil {
		return err
	}
	if err := s.publication.births.record("continued-directory-sealed", path, u.directory.info, b.Generation, 0555, 0, 0); err != nil {
		return err
	}
	return s.observe()
}
