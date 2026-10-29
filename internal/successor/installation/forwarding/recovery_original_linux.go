package forwarding

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// Fresh independent historical custody, never recovery permission. The later
// operation still needs fresh Release, exact request, native/semantic admission
// and its closed safe-prefix classification before creating a completion suffix.
type recoveryOriginal struct {
	ctx        context.Context
	mu, ioMu   sync.Mutex
	users      sync.WaitGroup
	once       sync.Once
	closed     bool
	terminal   error
	prefix     *transitionPrefix
	parents    *inputs
	files      []*initialFile
	control    *initialFile
	result     *initialFile
	guard      *initialFile
	cut        transitionCutResult
	guardPath  string
	knownGuard *initialFile
	// Failure/hold-only controls after actual I/O and before actual Close.
	observe func(string, *os.File) error
}

func readRecoveryOriginal(ctx context.Context, journal string) (*recoveryOriginal, error) {
	return openRecoveryOriginal(ctx, journal, nil)
}

// Only the actual publisher supplies its own still-present guard. Ordinary
// recovery requires absence; no public option can waive a pending cut proof.
func readOriginalCutBack(ctx context.Context, journal string, guard *initialFile) (*recoveryOriginal, error) {
	if guard == nil {
		return nil, ErrInput
	}
	return openRecoveryOriginal(ctx, journal, guard)
}

func openRecoveryOriginal(ctx context.Context, journal string, guard *initialFile) (result *recoveryOriginal, returned error) {
	if ctx == nil || os.Geteuid() != 0 || os.Getegid() != 0 || !validPath(journal) {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s := &recoveryOriginal{ctx: ctx, parents: &inputs{ctx: ctx, byPath: map[string]*inputDirectory{}}, knownGuard: guard}
	defer func() {
		if returned != nil {
			s.record(returned)
			returned = s.close()
			result = nil
		}
	}()
	controlPath, resultPath, pending := transitionCutPaths(journal)
	s.guardPath = pending
	if guard != nil && (guard.path != pending || guard.info == nil || !nodeAccess(guard.info, 0600, 0, 0, false)) {
		return nil, ErrBinding
	}
	var err error
	s.result, err = s.read(resultPath, 256<<10)
	if err != nil {
		return nil, err
	}
	// This untrusted header chooses only a separately verified original file.
	// All fields, canonical bytes and native relations are checked below.
	var header transitionCutResult
	if json.Unmarshal(s.result.body, &header) != nil || header.Boundary.Journal != journal {
		return nil, ErrBinding
	}
	s.prefix, err = openTransitionPrefix(ctx, header.Boundary)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(s.prefix.journal.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, err
	}
	s.prefix.exclusive = true
	s.control, err = s.read(controlPath, 64<<10)
	if err != nil {
		return nil, err
	}
	if guard != nil {
		s.guard, err = s.read(pending, 4096)
		if err != nil {
			return nil, err
		}
	}
	s.cut, err = decodeTransitionCut(s.control.body, s.result.body, s.prefix.journal.body, s.control.info, s.result.info, s.prefix.journal.info)
	if err != nil {
		return nil, err
	}
	identities := map[[2]uint64]bool{}
	for _, file := range append([]*initialFile{s.prefix.journal}, s.files...) {
		native, ok := infoSys(file.info)
		if !ok {
			return nil, ErrBinding
		}
		key := [2]uint64{uint64(native.Dev), native.Ino}
		if identities[key] {
			return nil, ErrBinding
		}
		identities[key] = true
	}
	if err := s.check(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *recoveryOriginal) read(path string, maximum int64) (*initialFile, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	parent, err := s.parents.parents(path)
	if err != nil {
		return nil, err
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	file, err := openInputAt(parent, filepath.Base(path), false)
	if err != nil {
		return nil, err
	}
	original := &initialFile{path: path, file: file}
	s.files = append(s.files, original)
	original.info, err = file.Stat()
	if err != nil || !nodeAccess(original.info, 0600, 0, 0, false) || original.info.Size() <= 0 || original.info.Size() > maximum {
		return nil, errors.Join(ErrBinding, err)
	}
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(file.Fd()), &fs); err != nil || fs.Type != 0xef53 || fs.Flags&1 != 0 {
		return nil, errors.Join(ErrBinding, err)
	}
	original.body, err = io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(original.body)) != original.info.Size() {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := errors.Join(observeInitialFile(original), s.ctx.Err()); err != nil {
		return nil, err
	}
	return original, nil
}

func (s *recoveryOriginal) record(err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil && (s.terminal == nil || !errors.Is(s.terminal, err)) {
		s.terminal = errors.Join(s.terminal, err)
	}
	return s.terminal
}

func (s *recoveryOriginal) check() (returned error) {
	if s == nil {
		return ErrInput
	}
	s.mu.Lock()
	if s.closed || s.terminal != nil || s.ctx.Err() != nil {
		err := errors.Join(ErrInput, s.terminal, s.ctx.Err())
		s.mu.Unlock()
		return err
	}
	s.users.Add(1)
	s.mu.Unlock()
	defer s.users.Done()
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	defer func() { returned = s.record(errors.Join(returned, s.ctx.Err())) }()
	s.mu.Lock()
	prior := errors.Join(s.terminal, s.ctx.Err())
	if s.closed {
		prior = errors.Join(ErrInput, prior)
	}
	s.mu.Unlock()
	if prior != nil {
		return prior
	}
	if s.prefix == nil || !s.prefix.exclusive {
		return ErrInput
	}
	if err := errors.Join(s.parents.links(), s.prefix.check(), s.checkGuard()); err != nil {
		return err
	}
	for _, original := range s.files {
		if err := observeInitialFile(original); err != nil {
			return err
		}
		if s.observe != nil {
			if err := s.observe("read", original.file); err != nil {
				return err
			}
		}
	}
	if err := errors.Join(s.parents.links(), s.prefix.check(), s.checkGuard()); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.Join(ErrInput, s.terminal, s.ctx.Err())
	}
	return errors.Join(s.terminal, s.ctx.Err())
}

func (s *recoveryOriginal) checkGuard() error {
	if s.knownGuard == nil {
		if _, err := os.Lstat(s.guardPath); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
		return nil
	}
	if s.guard == nil || !sameInputFile(s.knownGuard.info, s.guard.info) || string(s.guard.body) != string(transitionCutPending(s.cut.Boundary, s.cut.Origin)) {
		return ErrBinding
	}
	return observeInitialFile(s.guard)
}

func (s *recoveryOriginal) close() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.users.Wait()
		var physical error
		for n := len(s.files) - 1; n >= 0; n-- {
			file := s.files[n].file
			if s.observe != nil {
				physical = errors.Join(physical, s.observe("before-close", file))
			}
			physical = errors.Join(physical, file.Close())
		}
		physical = errors.Join(physical, (Inputs{original: s.parents}).Close(), s.prefix.close(), s.ctx.Err())
		s.record(physical)
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.terminal
}
