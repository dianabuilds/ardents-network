package hosting

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
)

// Inspection retains one existing provider period read-only, under its own
// original exclusive lease. It cannot initialize, charge, reserve, refund or
// lend a Budget. Copies share denial, physical users and terminal completion.
type Inspection struct{ original *periodInspection }

type periodInspection struct {
	mu, ioMu          sync.Mutex
	users             sync.WaitGroup
	once              sync.Once
	ctx               context.Context
	root, lease       *periodInspectionFile
	state, pin        *periodInspectionFile
	parents           []*periodInspectionFile
	uid, gid          uint32
	policy            string
	closed            bool
	failure, closeErr error
	// Failure/hold only, after real reads or before real descriptor Close.
	observe func(string, *os.File)
}

type periodInspectionFile struct {
	path string
	file *os.File
	info os.FileInfo
	hash [32]byte
}

var errInspectionClosed = errors.New("retained Hosting inspection closed")

// OpenInspection requires the original immutable policy commitment, complete
// existing history and zero retained reservations. Missing or unfinished files
// refuse without repair. Process exit never supplies a reservation return.
func OpenInspection(ctx context.Context, path, policySHA256 string, uid, gid uint32) (Inspection, error) {
	return openInspection(ctx, path, policySHA256, uid, gid, nil)
}

func openInspection(ctx context.Context, path, policy string, uid, gid uint32, observe func(string, *os.File)) (result Inspection, returned error) {
	pin, err := hex.DecodeString(policy)
	if ctx == nil || os.Geteuid() != 0 || !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" || err != nil || len(pin) != sha256.Size || hex.EncodeToString(pin) != policy {
		return result, errors.New("invalid retained Hosting inspection")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if _, finite := ctx.Deadline(); !finite {
		return result, errors.New("Hosting inspection requires finite caller")
	}
	s := &periodInspection{ctx: ctx, uid: uid, gid: gid, policy: policy, observe: observe}
	defer func() {
		if returned != nil {
			s.failure = returned
			returned = errors.Join(returned, s.close())
			result = Inspection{}
		}
	}()
	for ancestor := filepath.Dir(path); ; ancestor = filepath.Dir(ancestor) {
		entry, err := s.openDirectory(ancestor, true)
		if entry != nil {
			s.parents = append(s.parents, entry)
		}
		if err != nil {
			return result, err
		}
		if ancestor == "/" {
			break
		}
	}
	s.root, err = s.openDirectory(path, false)
	if err != nil {
		return result, err
	}
	s.lease, err = s.openLeaf("budget.lock", 0)
	if err != nil {
		return result, err
	}
	if err := syscall.Flock(int(s.lease.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return result, err
	}
	s.pin, err = s.openLeaf("budget.pin", 64)
	if err != nil {
		return result, err
	}
	s.state, err = s.openLeaf("budget.json", 16<<10)
	if err != nil {
		return result, err
	}
	for _, entry := range []*periodInspectionFile{s.pin, s.state} {
		raw, err := s.read(entry)
		if err != nil {
			return result, err
		}
		entry.hash = sha256.Sum256(raw)
	}
	if err := s.check(); err != nil {
		return result, err
	}
	return Inspection{original: s}, nil
}

func (s *periodInspection) openDirectory(path string, ancestor bool) (*periodInspectionFile, error) {
	if err := s.local(); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	entry := &periodInspectionFile{path: path, file: file}
	entry.info, err = file.Stat()
	if err != nil {
		return entry, err
	}
	native, ok := entry.info.Sys().(*syscall.Stat_t)
	if !ok || !entry.info.IsDir() || entry.info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || ancestor && (native.Uid != 0 || entry.info.Mode().Perm()&0022 != 0) || !ancestor && (native.Uid != s.uid || native.Gid != s.gid || entry.info.Mode().Perm() != 0700) {
		return entry, errors.New("retained Hosting directory access differs")
	}
	return entry, nil
}

func (s *periodInspection) openLeaf(name string, maximum int64) (*periodInspectionFile, error) {
	if err := s.local(); err != nil {
		return nil, err
	}
	fd, err := syscall.Openat(int(s.root.file.Fd()), name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	entry := &periodInspectionFile{path: filepath.Join(s.root.path, name), file: os.NewFile(uintptr(fd), name)}
	entry.info, err = entry.file.Stat()
	if err != nil {
		return entry, err
	}
	native, ok := entry.info.Sys().(*syscall.Stat_t)
	if !ok || !entry.info.Mode().IsRegular() || entry.info.Mode().Perm() != 0600 || entry.info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || native.Uid != s.uid || native.Gid != s.gid || native.Nlink != 1 || entry.info.Size() < 0 || entry.info.Size() > maximum {
		return entry, errors.New("retained Hosting leaf access or bound differs")
	}
	return entry, nil
}

func (s *periodInspection) local() error {
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

func (s *periodInspection) read(entry *periodInspectionFile) ([]byte, error) {
	if err := s.local(); err != nil {
		return nil, err
	}
	raw := make([]byte, int(entry.info.Size()))
	n, err := entry.file.ReadAt(raw, 0)
	if err != nil || n != len(raw) {
		return nil, errors.Join(io.ErrUnexpectedEOF, err)
	}
	var extra [1]byte
	if n, err := entry.file.ReadAt(extra[:], int64(len(raw))); n != 0 || err != io.EOF {
		return nil, errors.New("retained Hosting file grew")
	}
	if s.observe != nil {
		s.observe("read", entry.file)
	}
	return raw, s.local()
}

func samePeriodInspectionFile(original, actual os.FileInfo) bool {
	if original == nil || actual == nil || !os.SameFile(original, actual) || original.Mode() != actual.Mode() {
		return false
	}
	a, aok := original.Sys().(*syscall.Stat_t)
	b, bok := actual.Sys().(*syscall.Stat_t)
	return aok && bok && a.Uid == b.Uid && a.Gid == b.Gid && (original.IsDir() || original.Size() == actual.Size() && a.Nlink == b.Nlink && a.Ctim == b.Ctim && a.Mtim == b.Mtim)
}

func (s *periodInspection) identities() error {
	for _, entry := range append([]*periodInspectionFile{s.root, s.lease, s.pin, s.state}, s.parents...) {
		opened, err := entry.file.Stat()
		named, nameErr := os.Lstat(entry.path)
		if err != nil || nameErr != nil || !samePeriodInspectionFile(entry.info, opened) || !samePeriodInspectionFile(entry.info, named) {
			return errors.Join(errors.New("retained Hosting original identity differs"), err, nameErr)
		}
	}
	return s.local()
}

func (s *periodInspection) nativeRoot() (returned error) {
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(s.root.file.Fd()), &fs); err != nil || fs.Type != 0xef53 || fs.Flags&1 != 0 {
		return errors.Join(errors.New("retained Hosting filesystem differs"), err)
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
	if err != nil || fsErr != nil || !samePeriodInspectionFile(s.root.info, info) || fs.Type != 0xef53 || fs.Flags&1 != 0 {
		return errors.Join(errors.New("retained Hosting named filesystem differs"), err, fsErr)
	}
	return s.local()
}

func (s *periodInspection) check() error {
	if err := s.identities(); err != nil {
		return err
	}
	if err := s.nativeRoot(); err != nil {
		return err
	}
	if _, err := s.root.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	names, err := s.root.file.Readdirnames(4)
	if err != nil && err != io.EOF {
		return err
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"budget.json", "budget.lock", "budget.pin"}) {
		return errors.New("retained Hosting inventory differs")
	}
	pin, err := s.read(s.pin)
	if err != nil {
		return err
	}
	raw, err := s.read(s.state)
	if err != nil {
		return err
	}
	if sha256.Sum256(pin) != s.pin.hash || sha256.Sum256(raw) != s.state.hash || string(pin) != s.policy {
		return errors.New("retained Hosting bytes or policy differ")
	}
	state, err := decodeCommittedHostingState(raw, pin)
	if err != nil || state.Reserved != 0 {
		return errors.Join(errors.New("retained Hosting history or original reservations unavailable"), err)
	}
	if err := s.identities(); err != nil {
		return err
	}
	return s.nativeRoot()
}

// Check independently reobserves the original complete period. A real failure
// permanently denies this inspection even if another actor restores bytes.
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

// Close denies new checks, joins original readers and descriptor closes, then
// returns the original allowance lease last. It never returns reservations.
func (inspection Inspection) Close() error {
	if inspection.original == nil {
		return nil
	}
	return inspection.original.close()
}

func (s *periodInspection) close() error {
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.users.Wait()
		var physical error
		closeFile := func(entry *periodInspectionFile) {
			if entry == nil || entry.file == nil {
				return
			}
			if s.observe != nil {
				s.observe("before-close", entry.file)
			}
			physical = errors.Join(physical, entry.file.Close())
		}
		closeFile(s.state)
		closeFile(s.pin)
		closeFile(s.root)
		for i := len(s.parents) - 1; i >= 0; i-- {
			closeFile(s.parents[i])
		}
		closeFile(s.lease)
		s.closeErr = errors.Join(s.failure, physical, s.ctx.Err())
	})
	return s.closeErr
}
