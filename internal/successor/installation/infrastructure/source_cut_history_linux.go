package infrastructure

import (
	"errors"
	"io"
	"os"
	"syscall"
)

// The caller owns this prefix's original journal lease. This reader takes
// separate shared leases on both immutable original-result files and retains
// every partial acquisition through this prefix's joined Close.
func (s *sourceRecoveryPrefix) readOriginalCut(generation, binding, program string) (err error) {
	if s == nil || s.ctx == nil {
		return ErrInput
	}
	if err = s.begin(); err != nil {
		return err
	}
	defer s.users.Done()
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	s.mu.Lock()
	prior := s.terminal
	s.mu.Unlock()
	if prior != nil {
		return prior
	}
	defer func() { err = s.record(errors.Join(err, s.ctx.Err())) }()
	if s.cut != nil || s.cutControl != nil || s.cutResult != nil {
		return ErrInput
	}
	control, result, guard := sourceCutPaths(s.journal.path)
	s.cutGuard = guard
	if err = s.observe(); err != nil {
		return err
	}
	for _, row := range []struct {
		path  string
		leaf  **sourceFile
		limit int64
	}{{control, &s.cutControl, 64 << 10}, {result, &s.cutResult, 64 << 10}} {
		file, e := os.OpenFile(row.path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if e != nil {
			return e
		}
		leaf := &sourceFile{path: row.path, file: file}
		*row.leaf = leaf
		if err = syscall.Flock(int(file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
			return err
		}
		leaf.info, err = file.Stat()
		if err != nil || !sourceAccess(leaf.info, 0600, 0, 0, false) || leaf.info.Size() <= 0 || leaf.info.Size() > row.limit {
			return errors.Join(ErrBinding, err)
		}
		var fs syscall.Statfs_t
		if err = syscall.Fstatfs(int(file.Fd()), &fs); err != nil || fs.Type != 0xef53 {
			return errors.Join(ErrBinding, err)
		}
		leaf.raw, err = io.ReadAll(io.LimitReader(file, row.limit+1))
		if err != nil || int64(len(leaf.raw)) != leaf.info.Size() {
			return errors.Join(ErrBinding, err)
		}
	}
	decoded, err := decodeSourceCutResult(s.cutControl.raw, s.cutResult.raw, s.journal.raw, s.cutControl.info, s.cutResult.info, s.journal.info,
		s.journal.path, s.operation, generation, binding, program)
	if err != nil {
		return err
	}
	s.cut = &decoded
	return s.observe()
}

// Fresh domain admission supplies the actual independently observed broker
// instance and manager unique owner. Empty native file-test facts cannot pass.
func (s *sourceRecoveryPrefix) checkCutManager(manager string) error {
	if s == nil || s.cut == nil || manager == "" || s.cut.Boundary.ManagerIdentity != manager {
		return ErrBinding
	}
	native, err := sourceManagerIdentity()
	if err != nil || native != s.cut.Origin.NativeManager {
		return errors.Join(ErrBinding, err)
	}
	return s.check()
}
