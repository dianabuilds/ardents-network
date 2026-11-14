package descriptor

import (
	"bytes"
	"errors"
	"io"
	"runtime"
)

func (s *runtimeFiles) begin() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return s.failure
	}
	if err := s.ctx.Err(); err != nil {
		if !s.closed {
			s.failure = err
		}
		return err
	}
	if s.closed {
		return errRuntimeFilesClosed
	}
	s.uses.Add(1)
	return nil
}

func (s *runtimeFiles) local() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return s.failure
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if s.closed {
		return errRuntimeFilesClosed
	}
	return nil
}

func (s *runtimeFiles) record(err error) {
	if err == nil || err == errRuntimeFilesClosed {
		return
	}
	s.mu.Lock()
	if s.failure == nil {
		s.failure = err
	}
	s.mu.Unlock()
}

// Original file observation and joined erasure share this custody owner.
// Directory construction and namespace inventories remain in completed_files.
// All original before/after path checks and round boundaries remain mandatory.
func (s *runtimeFiles) check() error {
	if err := s.local(); err != nil {
		return err
	}
	if err := s.paths(); err != nil {
		return err
	}
	// This bounded buffer remains on the stack with the selected Go toolchain;
	// a 256 KiB local array instead allocates on every complete observation.
	var scratch [64 << 10]byte
	defer clear(scratch[:])
	for _, original := range s.fileOrder {
		if err := s.checkFileBytes(original, scratch[:]); err != nil {
			return err
		}
		if s.observe != nil {
			s.observe("read", original.file)
		}
		if err := s.local(); err != nil {
			return err
		}
	}
	return errors.Join(s.paths(), s.local())
}

// checkFileBytes compares every byte of the original file, including an
// independent exact-EOF read through its retained descriptor. Only the actual
// executing inode may use its original read-only view. The caller owns the
// physical-use lock and scratch; no observation grants runtime authority.
func (s *runtimeFiles) checkFileBytes(original *initialFile, scratch []byte) error {
	if original == nil || original.file == nil || len(scratch) == 0 {
		return ErrInput
	}
	var executable []byte
	if s.executable != nil && s.executable.original == original {
		if err := s.local(); err != nil {
			return err
		}
		executable = s.executable.body
		if len(executable) != len(original.body) {
			return ErrBinding
		}
	}
	for offset := 0; offset < len(original.body); {
		if err := s.local(); err != nil {
			return err
		}
		step := len(scratch)
		if executable != nil {
			// The actual executing image needs no scratch copy or syscall.
			// Compare bounded mapped spans with the same complete byte/EOF and
			// before/after native checks, retaining cancellation between spans.
			// Fewer scheduler yields avoid repeated observer queue churn here.
			step *= 8
		}
		end := offset + min(step, len(original.body)-offset)
		if executable != nil {
			if !bytes.Equal(executable[offset:end], original.body[offset:end]) {
				return ErrBinding
			}
			if s.observe != nil {
				s.observe("executable-chunk", original.file)
			}
		} else {
			n, err := original.file.ReadAt(scratch[:end-offset], int64(offset))
			if err != nil || n != end-offset || !bytes.Equal(scratch[:n], original.body[offset:end]) {
				return errors.Join(ErrBinding, err)
			}
		}
		offset = end
		// Let queued Go work run between chunks. Ordinary descriptor reads
		// otherwise invite syscall handoffs to additional OS threads competing
		// for the same physical CPU when there is one Go processor.
		runtime.Gosched()
	}
	n, err := original.file.ReadAt(scratch[:1], int64(len(original.body)))
	if n != 0 || err != io.EOF {
		return errors.Join(ErrBinding, err)
	}
	return nil
}

func (s *runtimeFiles) close() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		if s.observe != nil {
			s.observe("denied", nil)
		}
		s.uses.Wait()
		physical := s.executable.close()
		closeFile := func(original *initialFile) {
			if s.observe != nil {
				s.observe("before-close", original.file)
			}
			physical = errors.Join(physical, original.file.Close())
			clear(original.body)
		}
		for i := len(s.fileOrder) - 1; i >= 0; i-- {
			if s.fileOrder[i] != s.journal {
				closeFile(s.fileOrder[i])
			}
		}
		for i := len(s.dirOrder) - 1; i >= 0; i-- {
			physical = errors.Join(physical, s.dirOrder[i].file.Close())
		}
		if s.journal != nil {
			closeFile(s.journal)
		}
		s.mu.Lock()
		s.failure = errors.Join(s.failure, physical, s.ctx.Err())
		s.mu.Unlock()
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failure
}
