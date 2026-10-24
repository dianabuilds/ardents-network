package infrastructure

import (
	"errors"
	"io"
	"os"
	"syscall"
)

// readCompletion independently opens and retains the immutable completion
// envelope next to the original journal. The two actual flocks exclude both
// writers until joined Close. The returned canonical transcript still requires
// the complete operation grammar, native resources, fresh Release and domain
// join checks; this read neither publishes completion nor grants continuation.
func (s *sourceRecoveryPrefix) readCompletion(generation, binding string) (transcript []byte, err error) {
	if s == nil || s.ctx == nil {
		return nil, ErrInput
	}
	if err = s.begin(); err != nil {
		return nil, err
	}
	defer s.users.Done()
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	s.mu.Lock()
	prior := s.terminal
	s.mu.Unlock()
	if prior != nil {
		return nil, prior
	}
	defer func() {
		err = s.record(errors.Join(err, s.ctx.Err()))
		if err != nil {
			transcript = nil
		}
	}()
	if !sourceInitialSHA(generation) || !sourceInitialSHA(binding) {
		return nil, ErrInput
	}
	if err = s.observe(); err != nil {
		return nil, err
	}
	if s.completion == nil {
		path := s.journal.path + ".recovery-completion"
		file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
		// Retain even a failed acquisition until the operation joins Close;
		// another attempt cannot reuse an owner that has latched a failure.
		s.completion = &sourceFile{path: path, file: file}
		if err = syscall.Flock(int(file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
			return nil, err
		}
		s.completion.info, err = file.Stat()
		if err != nil || !sourceAccess(s.completion.info, 0600, 0, 0, false) || s.completion.info.Size() <= 0 || s.completion.info.Size() > 4<<20 {
			return nil, errors.Join(ErrBinding, err)
		}
		var fs syscall.Statfs_t
		if err = syscall.Fstatfs(int(file.Fd()), &fs); err != nil || fs.Type != 0xef53 {
			return nil, errors.Join(ErrBinding, err)
		}
		s.completion.raw, err = io.ReadAll(io.LimitReader(file, (4<<20)+1))
		if err != nil || int64(len(s.completion.raw)) != s.completion.info.Size() {
			return nil, errors.Join(ErrBinding, err)
		}
	}
	if err = s.observe(); err != nil {
		return nil, err
	}
	transcript, err = decodeSourceRecoveryCompletion(s.completion.raw, s.journal.raw, s.journal.info,
		s.journal.path, s.operation, generation, binding)
	if err != nil {
		return nil, err
	}
	return transcript, s.observe()
}
