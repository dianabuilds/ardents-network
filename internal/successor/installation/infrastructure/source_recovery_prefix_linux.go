package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// sourceRecoveryPrefix owns a new read-only descriptor and exclusive original
// journal flock. This excludes cooperating live writers; it proves no account
// birth, prior sync/Close, stopped process, domain join or recovery permission.
// Close seals admission before joining every admitted physical observation.
type sourceRecoveryPrefix struct {
	ctx        context.Context
	mu         sync.Mutex
	ioMu       sync.Mutex
	users      sync.WaitGroup
	closeOnce  sync.Once
	closed     bool
	terminal   error
	operation  string
	exclusive  bool
	journal    *sourceFile
	completion *sourceFile
	cutControl *sourceFile
	cutResult  *sourceFile
	cut        *sourceCutResult
	cutGuard   string
	parents    map[string]*os.File
	identities map[string]os.FileInfo
	records    []sourceRecord
	// A native test may hold after actual original file I/O. It supplies no
	// bytes, successful observation, native identity or accepting authority.
	observed func(*os.File)
}

func openSourceRecoveryPrefix(ctx context.Context, path, operation string) (result *sourceRecoveryPrefix, err error) {
	return openSourceRecoveryPrefixCustody(ctx, path, operation, true)
}

// The original controller has independent read-only custody while its genuine
// coordinator is frozen. This constructor grants no writer/recovery lease.
// Only after the actual original scope joins may that controller take EX on
// its own descriptor. A later RecoverSource still opens a separate descriptor.
func openSourceRecoveryPrefixCustody(ctx context.Context, path, operation string, exclusive bool) (result *sourceRecoveryPrefix, err error) {
	maximum := 0
	switch operation {
	case "initial":
		maximum = sourceRecoverableInitialRecords
	case "replacement":
		maximum = 55
	default:
		return nil, ErrInput
	}
	if ctx == nil || os.Geteuid() != 0 || !sourcePath(path) {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s := &sourceRecoveryPrefix{ctx: ctx, operation: operation, parents: map[string]*os.File{}, identities: map[string]os.FileInfo{}}
	defer func() {
		if err != nil {
			s.record(err)
			err = s.close()
			result = nil
		}
	}()
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		info, e := os.Lstat(parent)
		if e != nil || !trustedDirectory(info) {
			return nil, errors.Join(ErrBinding, e)
		}
		file, original, e := openSourceDirectory(parent, info.Mode().Perm(), 0, 0)
		if file != nil {
			s.parents[parent] = file
		}
		if e != nil {
			return nil, e
		}
		s.identities[parent] = original
		if parent == "/" {
			break
		}
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	s.journal = &sourceFile{path: path, file: file}
	if exclusive {
		if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			return nil, err
		}
		s.exclusive = true
	}
	s.journal.info, err = file.Stat()
	if err != nil || !sourceAccess(s.journal.info, 0600, 0, 0, false) || s.journal.info.Size() <= 0 || s.journal.info.Size() > 4<<20 {
		return nil, errors.Join(ErrBinding, err)
	}
	var fs syscall.Statfs_t
	if err = syscall.Fstatfs(int(file.Fd()), &fs); err != nil || fs.Type != 0xef53 {
		return nil, errors.Join(ErrBinding, err)
	}
	s.journal.raw, err = io.ReadAll(io.LimitReader(file, (4<<20)+1))
	if err != nil || int64(len(s.journal.raw)) != s.journal.info.Size() {
		return nil, errors.Join(ErrBinding, err)
	}
	s.records, err = decodeSourceRecoveryPrefix(s.journal.raw, maximum)
	if err != nil {
		return nil, err
	}
	native := s.journal.info.Sys().(*syscall.Stat_t)
	first := sourceRecord{Phase: s.records[0].Phase, Path: path, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}
	count := sourceRecoveryCompleteCount(operation, first.Phase)
	if count == 0 || len(s.records) > count || s.records[0] != first {
		return nil, ErrBinding
	}
	return s, s.check()
}

func (s *sourceRecoveryPrefix) begin() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrInput
	}
	if s.terminal != nil {
		return s.terminal
	}
	if err := s.ctx.Err(); err != nil {
		s.terminal = err
		return err
	}
	s.users.Add(1)
	return nil
}

func (s *sourceRecoveryPrefix) record(err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.terminal == nil {
		s.terminal = err
		return err
	}
	if err == nil || errors.Is(s.terminal, err) {
		return s.terminal
	}
	s.terminal = errors.Join(s.terminal, err)
	return s.terminal
}

func (s *sourceRecoveryPrefix) check() (err error) {
	if s == nil || s.ctx == nil {
		return ErrInput
	}
	if err = s.begin(); err != nil {
		return err
	}
	defer s.users.Done()
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	// A previous admitted observer can fail while this one waits for I/O.
	s.mu.Lock()
	prior := s.terminal
	s.mu.Unlock()
	if prior != nil {
		return prior
	}
	defer func() { err = s.record(errors.Join(err, s.ctx.Err())) }()
	return s.observe()
}

// observe runs only under this owner's physical I/O serializer and an admitted
// user. It performs no foreign authority call and does not hold the state mu.
func (s *sourceRecoveryPrefix) observe() (err error) {
	if err = s.ctx.Err(); err != nil {
		return err
	}
	parents := func() error {
		for path, original := range s.identities {
			if err := observeSourceDirectory(path, s.parents[path], original); err != nil {
				return err
			}
		}
		return nil
	}
	if err = parents(); err != nil {
		return err
	}
	if err = s.observeCutGuard(); err != nil {
		return err
	}
	for _, original := range []*sourceFile{s.journal, s.completion, s.cutControl, s.cutResult} {
		if original == nil {
			continue
		}
		if err := s.observeFile(original); err != nil {
			return err
		}
	}
	// The completion read can yield while an earlier original name changes.
	// Reobserve every original file after all byte reads have joined.
	for _, original := range []*sourceFile{s.journal, s.completion, s.cutControl, s.cutResult} {
		if original == nil {
			continue
		}
		opened, e := original.file.Stat()
		named, ne := os.Lstat(original.path)
		if e != nil || ne != nil || !sameSourceFile(original.info, opened) || !sameSourceFile(opened, named) {
			return errors.Join(ErrBinding, e, ne)
		}
	}
	return errors.Join(parents(), s.observeCutGuard())
}

func (s *sourceRecoveryPrefix) observeCutGuard() error {
	if s.cutGuard != "" {
		if _, err := os.Lstat(s.cutGuard); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	return nil
}

func (s *sourceRecoveryPrefix) observeFile(original *sourceFile) error {
	before, err := original.file.Stat()
	named, ne := os.Lstat(original.path)
	if err != nil || ne != nil || !sameSourceFile(original.info, before) || !sameSourceFile(before, named) {
		return errors.Join(ErrBinding, err, ne)
	}
	if _, err = original.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(original.file, int64(len(original.raw))+1))
	if err != nil || !bytes.Equal(raw, original.raw) {
		return errors.Join(ErrBinding, err)
	}
	if s.observed != nil {
		s.observed(original.file)
	}
	after, err := original.file.Stat()
	named, ne = os.Lstat(original.path)
	if err != nil || ne != nil || !sameSourceFile(before, after) || !sameSourceFile(after, named) {
		return errors.Join(ErrBinding, err, ne)
	}
	return nil
}

func (s *sourceRecoveryPrefix) close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.users.Wait()
		var cleanup error
		for _, original := range []*sourceFile{s.cutResult, s.cutControl} {
			if original != nil {
				cleanup = errors.Join(cleanup, original.file.Close())
			}
		}
		if s.completion != nil {
			cleanup = errors.Join(cleanup, s.completion.file.Close())
		}
		for _, file := range s.parents {
			cleanup = errors.Join(cleanup, file.Close())
		}
		// Keep the original exclusion until every independent ancestor/user has
		// joined; release the actual journal descriptor last, including failure.
		if s.journal != nil {
			cleanup = errors.Join(cleanup, s.journal.file.Close())
		}
		s.mu.Lock()
		if s.ctx == nil {
			cleanup = errors.Join(cleanup, ErrInput)
		} else {
			cleanup = errors.Join(cleanup, s.ctx.Err())
		}
		s.terminal = errors.Join(s.terminal, cleanup)
		s.mu.Unlock()
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.terminal
}
