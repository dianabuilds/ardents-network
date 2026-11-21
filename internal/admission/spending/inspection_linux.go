package spending

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
)

// Inspection independently retains complete existing receiving history under
// its original lease. It creates, repairs, prunes and spends nothing, lends no
// Ledger or descriptor, and grants no Network, runtime or recovery authority.
// Copies share local denial, original physical users and the retained Close.
type Inspection struct{ original *spendInspection }

type spendInspection struct {
	mu, ioMu             sync.Mutex
	users                sync.WaitGroup
	once                 sync.Once
	ctx                  context.Context
	binding              Binding
	uid, gid             uint32
	parents              []*spendInspectionFile
	root, lease, journal *spendInspectionFile
	closed               bool
	failure, closeErr    error
	// Failure/hold only, after real reads or before real descriptor Close.
	observe func(string, *os.File)
}

type spendInspectionFile struct {
	path string
	file *os.File
	info os.FileInfo
	hash [32]byte
}

var errInspectionClosed = errors.New("retained spending inspection closed")

// OpenInspection is root-only Linux/ext4 observation of the exact original
// numeric principal and receiving binding. Missing files or a crash tail refuse
// without creating a lease, changing bytes or granting a fresh-root fact.
func OpenInspection(ctx context.Context, root string, binding Binding, uid, gid uint32) (Inspection, error) {
	return openInspection(ctx, root, binding, uid, gid, nil)
}

func openInspection(ctx context.Context, root string, binding Binding, uid, gid uint32, observe func(string, *os.File)) (result Inspection, returned error) {
	if ctx == nil || os.Geteuid() != 0 || !filepath.IsAbs(root) || filepath.Clean(root) != root || root == "/" || !validBinding(binding) {
		return result, errors.New("invalid retained spending inspection")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if _, finite := ctx.Deadline(); !finite {
		return result, errors.New("spending inspection requires finite caller")
	}
	s := &spendInspection{ctx: ctx, binding: binding, uid: uid, gid: gid, observe: observe}
	defer func() {
		if returned != nil {
			s.failure = returned
			returned = errors.Join(returned, s.close())
			result = Inspection{}
		}
	}()
	for path := filepath.Dir(root); ; path = filepath.Dir(path) {
		entry, err := s.openDirectory(path, true)
		if entry != nil {
			s.parents = append(s.parents, entry)
		}
		if err != nil {
			return result, err
		}
		if path == "/" {
			break
		}
	}
	var err error
	s.root, err = s.openDirectory(root, false)
	if err != nil {
		return result, err
	}
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(s.root.file.Fd()), &fs); err != nil || fs.Type != 0xef53 || fs.Flags&1 != 0 {
		return result, errors.Join(errors.New("spending inspection requires original writable ext4 root"), err)
	}
	s.lease, err = s.openLeaf(closedSpendLockName, 0)
	if err != nil {
		return result, err
	}
	if err := syscall.Flock(int(s.lease.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return result, err
	}
	s.journal, err = s.openLeaf(closedSpendLedgerName, closedSpendLedgerHeaderSize+(maximumClosedSpends+1)*closedSpendRecordSize)
	if err != nil {
		return result, err
	}
	raw, err := s.readJournal()
	defer clear(raw)
	if err != nil {
		return result, err
	}
	decoded, err := decodeLedgerWithRepair(s.journal.path, binding, raw, func(string, int64) error { return errors.New("spending inspection refuses incomplete history") })
	if err != nil {
		return result, err
	}
	clear(decoded.spent)
	s.journal.hash = sha256.Sum256(raw)
	if err := s.check(); err != nil {
		return result, err
	}
	return Inspection{original: s}, nil
}

func (s *spendInspection) openDirectory(path string, ancestor bool) (*spendInspectionFile, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	entry := &spendInspectionFile{path: path, file: file}
	entry.info, err = file.Stat()
	if err != nil {
		return entry, err
	}
	native, ok := entry.info.Sys().(*syscall.Stat_t)
	if !ok || !entry.info.IsDir() || entry.info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || ancestor && (native.Uid != 0 || entry.info.Mode().Perm()&0022 != 0) || !ancestor && (native.Uid != s.uid || native.Gid != s.gid || entry.info.Mode().Perm() != 0700) {
		return entry, errors.New("retained spending directory access differs")
	}
	return entry, nil
}

func (s *spendInspection) openLeaf(name string, maximum int) (*spendInspectionFile, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	fd, err := syscall.Openat(int(s.root.file.Fd()), name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	entry := &spendInspectionFile{path: filepath.Join(s.root.path, name), file: os.NewFile(uintptr(fd), name)}
	entry.info, err = entry.file.Stat()
	if err != nil {
		return entry, err
	}
	native, ok := entry.info.Sys().(*syscall.Stat_t)
	if !ok || !entry.info.Mode().IsRegular() || entry.info.Mode().Perm() != 0600 || entry.info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || native.Uid != s.uid || native.Gid != s.gid || native.Nlink != 1 || entry.info.Size() < 0 || entry.info.Size() > int64(maximum) {
		return entry, errors.New("retained spending leaf access or bound differs")
	}
	return entry, nil
}

func (s *spendInspection) local() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		if err := errors.Join(s.failure, s.ctx.Err()); err != nil {
			return errors.Join(errInspectionClosed, err)
		}
		return errInspectionClosed
	}
	return errors.Join(s.failure, s.ctx.Err())
}

func (s *spendInspection) readJournal() ([]byte, error) {
	if err := s.local(); err != nil {
		return nil, err
	}
	maximum := closedSpendLedgerHeaderSize + (maximumClosedSpends+1)*closedSpendRecordSize
	raw := make([]byte, int(s.journal.info.Size()))
	if len(raw) > maximum {
		return nil, errors.New("retained spending size exceeds bound")
	}
	for offset := 0; offset < len(raw); {
		if err := s.local(); err != nil {
			clear(raw)
			return nil, err
		}
		end := offset + min(32<<10, len(raw)-offset)
		n, err := s.journal.file.ReadAt(raw[offset:end], int64(offset))
		if err != nil || n != end-offset {
			clear(raw)
			return nil, errors.Join(io.ErrUnexpectedEOF, err)
		}
		offset = end
	}
	var extra [1]byte
	if n, err := s.journal.file.ReadAt(extra[:], int64(len(raw))); n != 0 || err != io.EOF {
		clear(raw)
		return nil, errors.New("retained spending journal grew")
	}
	if s.observe != nil {
		s.observe("read", s.journal.file)
	}
	if err := s.local(); err != nil {
		clear(raw)
		return nil, err
	}
	return raw, nil
}

func sameSpendInspectionFile(original, actual os.FileInfo) bool {
	if original == nil || actual == nil || !os.SameFile(original, actual) || original.Mode() != actual.Mode() {
		return false
	}
	a, aok := original.Sys().(*syscall.Stat_t)
	b, bok := actual.Sys().(*syscall.Stat_t)
	return aok && bok && a.Uid == b.Uid && a.Gid == b.Gid && (original.IsDir() || original.Size() == actual.Size() && a.Nlink == b.Nlink && a.Ctim == b.Ctim && a.Mtim == b.Mtim)
}

func (s *spendInspection) check() error {
	if err := s.local(); err != nil {
		return err
	}
	entries := append([]*spendInspectionFile{s.root, s.lease, s.journal}, s.parents...)
	for _, entry := range entries {
		opened, err := entry.file.Stat()
		named, nameErr := os.Lstat(entry.path)
		if err != nil || nameErr != nil || !sameSpendInspectionFile(entry.info, opened) || !sameSpendInspectionFile(entry.info, named) {
			return errors.Join(errors.New("retained spending original identity differs"), err, nameErr)
		}
	}
	if err := s.checkNativeRoot(); err != nil {
		return err
	}
	if _, err := s.root.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	names, err := s.root.file.Readdirnames(3)
	if err != nil && err != io.EOF {
		return err
	}
	slices.Sort(names)
	wanted := []string{closedSpendLockName, closedSpendLedgerName}
	slices.Sort(wanted)
	if !slices.Equal(names, wanted) {
		return errors.New("retained spending inventory differs")
	}
	raw, err := s.readJournal()
	defer clear(raw)
	if err != nil {
		return err
	}
	if sha256.Sum256(raw) != s.journal.hash {
		return errors.New("retained spending bytes differ")
	}
	for _, entry := range entries {
		opened, err := entry.file.Stat()
		named, nameErr := os.Lstat(entry.path)
		if err != nil || nameErr != nil || !sameSpendInspectionFile(entry.info, opened) || !sameSpendInspectionFile(entry.info, named) {
			return errors.Join(errors.New("retained spending late identity differs"), err, nameErr)
		}
	}
	return s.local()
}

// A same-inode bind mount may change the path's filesystem flags without
// changing the retained descriptor. Reobserve both, and join the fresh reader
// before accepting its observation; a failed physical Close remains a failure.
func (s *spendInspection) checkNativeRoot() (returned error) {
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(s.root.file.Fd()), &fs); err != nil || fs.Type != 0xef53 || fs.Flags&1 != 0 {
		return errors.Join(errors.New("retained spending filesystem differs"), err)
	}
	named, err := os.OpenFile(s.root.path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer func() {
		if err := named.Close(); err != nil {
			returned = errors.Join(returned, err)
		}
	}()
	info, err := named.Stat()
	fsErr := syscall.Fstatfs(int(named.Fd()), &fs)
	if err != nil || fsErr != nil || !sameSpendInspectionFile(s.root.info, info) || fs.Type != 0xef53 || fs.Flags&1 != 0 {
		return errors.Join(errors.New("retained spending named filesystem differs"), err, fsErr)
	}
	return s.local()
}

// Check reobserves the original complete bytes, inventory and native identities.
// Actual failure permanently denies this inspection; restored files cannot
// revive it. Successful read-only observation grants no spending permission.
func (inspection Inspection) Check() (returned error) {
	s := inspection.original
	if s == nil {
		return errInspectionClosed
	}
	s.mu.Lock()
	if s.closed || s.failure != nil || s.ctx.Err() != nil {
		returned = errors.Join(errInspectionClosed, s.failure, s.ctx.Err())
		s.mu.Unlock()
		return returned
	}
	s.users.Add(1)
	s.mu.Unlock()
	defer s.users.Done()
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	returned = s.check()
	s.mu.Lock()
	defer s.mu.Unlock()
	if returned == nil && s.closed {
		returned = errInspectionClosed
	}
	if returned == nil {
		returned = s.ctx.Err()
	}
	if returned != nil && returned != errInspectionClosed {
		s.failure = errors.Join(s.failure, returned)
	}
	return returned
}

// Close denies new checks, joins original readers and physical closes, then
// returns its exclusive spend lease last. Copies retain the same terminal result.
func (inspection Inspection) Close() error {
	if inspection.original == nil {
		return nil
	}
	return inspection.original.close()
}

func (s *spendInspection) close() error {
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.users.Wait()
		var physical error
		closeFile := func(entry *spendInspectionFile) {
			if entry == nil || entry.file == nil {
				return
			}
			if s.observe != nil {
				s.observe("before-close", entry.file)
			}
			physical = errors.Join(physical, entry.file.Close())
		}
		closeFile(s.journal)
		closeFile(s.root)
		for i := len(s.parents) - 1; i >= 0; i-- {
			closeFile(s.parents[i])
		}
		closeFile(s.lease)
		s.closeErr = errors.Join(s.failure, physical, s.ctx.Err())
	})
	return s.closeErr
}
