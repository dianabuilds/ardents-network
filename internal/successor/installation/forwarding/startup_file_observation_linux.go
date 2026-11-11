package forwarding

import (
	"errors"
)

// observeStartupFiles retains one original physical use across the complete
// read-only Startup observation. Every byte and exact EOF is checked before and
// after its native reads; each running round additionally checks original/named
// metadata, mounts, principal, search paths, process and scope. Original file
// ctime/mtime bindings prevent an in-place change-and-restore from reviving it.
// The callback is Startup's native observation, never State or domain authority.
func (s *runtimeSelection) observeStartupFiles(observe func() error) (returned error) {
	if s == nil || s.files == nil || observe == nil {
		return ErrInput
	}
	f := s.files
	if err := f.begin(); err != nil {
		return err
	}
	defer f.uses.Done()
	f.ioMu.Lock()
	defer f.ioMu.Unlock()
	defer func() { f.record(returned) }()
	if err := f.check(); err != nil {
		return err
	}
	err := observe()
	after := f.check()
	if local := f.local(); local != nil && local != after {
		if after == nil {
			after = local
		} else {
			after = errors.Join(after, local)
		}
	}
	// Preserve an unchanged original error identity, including bare caller
	// cancellation. Independent physical failures remain separate and retained.
	if after == nil || after == err {
		return err
	}
	if err == nil {
		return after
	}
	return errors.Join(err, after)
}

// observeStartupPaths is called only within observeStartupFiles' original
// physical-use lock. Original/named metadata surrounds one compound sequence
// of fresh access, absence and fresh access observations. No domain effect or
// accepting result occurs between these reads; no previous verdict is reused.
func (s *runtimeSelection) observeStartupPaths(absent []string) error {
	f := s.files
	if err := f.paths(); err != nil {
		return err
	}
	if err := s.checkAccess(); err != nil {
		return err
	}
	if f.observe != nil {
		f.observe("startup-first-access", nil)
	}
	if err := observeStartupAbsences(absent); err != nil {
		return err
	}
	if f.observe != nil {
		f.observe("startup-absence", nil)
	}
	if err := s.checkAccess(); err != nil {
		return err
	}
	if f.observe != nil {
		f.observe("startup-final-access", nil)
	}
	err := f.paths()
	if local := f.local(); local != nil && local != err {
		if err == nil {
			return local
		}
		return errors.Join(err, local)
	}
	return err
}
