package durable

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

// Inspection owns independent read-only descriptors and an existing exclusive
// State lease. It exposes no mutation, initialization, recovery or role policy.
// State alone authenticates returned bytes and decides currentness; its owning
// inspection serializes these reads and Check before joining this physical Close.
type Inspection struct {
	ctx        context.Context
	root       *Root
	lease      *os.File
	entries    map[string]*inspectionEntry
	order      []*inspectionEntry
	parents    map[string]*os.File
	parentInfo map[string]os.FileInfo
	uid, gid   uint32
	terminal   error
}

type inspectionEntry struct {
	path   string
	file   *os.File
	info   os.FileInfo
	digest [32]byte
	names  []string
}

// OpenInspection never creates a missing marker, lease, pointer or file.
// Every retained leaf/ancestor has its own descriptor through joined Close;
// descriptor/resource exhaustion refuses without deleting or pruning history.
func OpenInspection(ctx context.Context, path string, limits Limits) (result *Inspection, err error) {
	if ctx == nil || ctx.Err() != nil || !filepath.IsAbs(path) || filepath.Clean(path) != path || os.Geteuid() != 0 || limits.EpochBytes <= 0 || limits.RecordBytes <= 0 || limits.ClosedProfileBytes <= 0 {
		return nil, errors.New("invalid retained State inspection")
	}
	s := &Inspection{ctx: ctx, root: &Root{path: path, limits: limits}, entries: map[string]*inspectionEntry{}, parents: map[string]*os.File{}, parentInfo: map[string]os.FileInfo{}}
	defer func() {
		if err != nil {
			err = errors.Join(err, s.Close())
			result = nil
		}
	}()
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		f, e := os.OpenFile(parent, os.O_RDONLY|syscall.O_CLOEXEC|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
		if e != nil {
			return nil, e
		}
		s.parents[parent] = f
		info, e := f.Stat()
		named, ne := os.Lstat(parent)
		if e != nil || ne != nil || !os.SameFile(info, named) || !info.IsDir() || info.Mode().Perm()&0022 != 0 || info.Sys().(*syscall.Stat_t).Uid != 0 {
			return nil, errors.Join(errors.New("retained State ancestor differs"), e, ne)
		}
		s.parentInfo[parent] = info
		if parent == "/" {
			break
		}
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_CLOEXEC|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	rootEntry := &inspectionEntry{path: path, file: f}
	s.order = append(s.order, rootEntry)
	rootEntry.info, err = f.Stat()
	if err != nil || rootEntry.info.Mode() != os.ModeDir|0700 {
		return nil, errors.Join(errors.New("retained State root access differs"), err)
	}
	native := rootEntry.info.Sys().(*syscall.Stat_t)
	s.uid, s.gid = native.Uid, native.Gid
	fd, err := syscall.Openat(int(f.Fd()), rootLockName, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	s.lease = os.NewFile(uintptr(fd), filepath.Join(path, rootLockName))
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, err
	}
	if err := inspectRetainedRoot(path); err != nil {
		return nil, err
	}
	// Capture after acquiring the existing owner lease. No writable probe runs.
	rootEntry.info, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if err := s.capture(rootEntry); err != nil {
		return nil, err
	}
	lock := s.entries[filepath.Join(path, rootLockName)]
	leaseInfo, err := s.lease.Stat()
	if err != nil || lock == nil || !os.SameFile(lock.info, leaseInfo) {
		return nil, errors.Join(errors.New("retained State original lease differs"), err)
	}
	return s, s.Check()
}

func (s *Inspection) capture(entry *inspectionEntry) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if len(s.entries) >= maximumRootStableEntries+maximumStateGenerations*70+maximumControlGenerations*2+32 {
		return errors.New("retained State inspection inventory bound exceeded")
	}
	s.entries[entry.path] = entry
	info, err := entry.file.Stat()
	if err != nil {
		return err
	}
	entry.info = info
	if !s.knownEntry(entry.path, info.IsDir()) {
		return errors.New("retained State contains an unknown retained entry")
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != s.uid || native.Gid != s.gid {
		return errors.New("retained State owner differs")
	}
	if info.IsDir() {
		if info.Mode() != os.ModeDir|0700 {
			return errors.New("retained State directory access differs")
		}
		names, err := entry.file.Readdirnames(maximumControlGenerations*2 + maximumStateGenerations*70 + maximumRootStableEntries + 33)
		if err != nil && err != io.EOF {
			return err
		}
		slices.Sort(names)
		entry.names = names
		for _, name := range names {
			if strings.HasPrefix(name, ".stage-") || strings.HasPrefix(name, ".current-") || strings.HasPrefix(name, ".closed-profile-") {
				return errors.New("retained State has an uncertain retained prefix")
			}
			path := filepath.Join(entry.path, name)
			named, err := os.Lstat(path)
			if err != nil || named.Mode()&os.ModeSymlink != 0 {
				return errors.Join(errors.New("retained State entry differs"), err)
			}
			flags := syscall.O_RDONLY | syscall.O_CLOEXEC | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
			if named.IsDir() {
				flags |= syscall.O_DIRECTORY
			}
			fd, err := syscall.Openat(int(entry.file.Fd()), name, flags, 0)
			if err != nil {
				return err
			}
			child := &inspectionEntry{path: path, file: os.NewFile(uintptr(fd), path)}
			s.order = append(s.order, child)
			if err := s.capture(child); err != nil {
				return err
			}
		}
		return nil
	}
	if info.Mode() != 0600 || native.Nlink != 1 || info.Size() < 0 || info.Size() > max(s.root.limits.EpochBytes, s.root.limits.RecordBytes, s.root.limits.ClosedProfileBytes, 4096) {
		return errors.New("retained State leaf access or size differs")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(entry.file, info.Size()+1))
	if err != nil || n != info.Size() {
		return errors.Join(errors.New("retained State bounded read differs"), err)
	}
	copy(entry.digest[:], h.Sum(nil))
	return nil
}

// This reader admits only the existing durable layout. An unknown directory
// must not cause recursive traversal or become an implicitly accepted floor.
func (s *Inspection) knownEntry(path string, directory bool) bool {
	relative, err := filepath.Rel(s.root.path, path)
	if err != nil {
		return false
	}
	parts := strings.Split(relative, string(filepath.Separator))
	if relative == "." {
		return directory
	}
	if len(parts) == 1 {
		name := parts[0]
		if name == "generations" || name == "distribution" {
			return directory
		}
		if directory {
			return false
		}
		if name == rootLockName || name == rootMarkerName || name == "current" {
			return true
		}
		if strings.HasPrefix(name, "closed-profile-state-") {
			return generationName.MatchString(strings.TrimPrefix(name, "closed-profile-state-"))
		}
		return strings.HasPrefix(name, "closed-profile-") && strings.HasSuffix(name, ".bin") && generationName.MatchString(strings.TrimSuffix(strings.TrimPrefix(name, "closed-profile-"), ".bin"))
	}
	if parts[0] == "distribution" {
		if len(parts) == 2 {
			return parts[1] == "generations" && directory || parts[1] == "current" && !directory
		}
		if parts[1] != "generations" || !generationName.MatchString(parts[2]) {
			return false
		}
		return len(parts) == 3 && directory || len(parts) == 4 && parts[3] == "state.bin" && !directory
	}
	if parts[0] != "generations" || !generationName.MatchString(parts[1]) {
		return false
	}
	if len(parts) == 2 {
		return directory
	}
	if len(parts) == 3 {
		return parts[2] == "epoch.bin" && !directory || parts[2] == "inputs" && directory
	}
	if len(parts) != 4 || parts[2] != "inputs" || directory {
		return false
	}
	index, err := strconv.Atoi(strings.TrimSuffix(parts[3], ".bin"))
	return err == nil && index >= 0 && index < 64 && parts[3] == fmt.Sprintf("%04d.bin", index)
}

func (s *Inspection) Principal() (uint32, uint32) {
	if s == nil {
		return 0, 0
	}
	return s.uid, s.gid
}
func (s *Inspection) ReadState() (string, []Generation, error) {
	if s == nil || s.root == nil {
		return "", nil, errors.New("retained State inspection absent")
	}
	return s.root.LoadState()
}
func (s *Inspection) ReadControl() (string, []byte, error) {
	if s == nil || s.root == nil {
		return "", nil, errors.New("retained State inspection absent")
	}
	return s.root.LoadControl()
}

func (s *Inspection) ReadClosedProfile(generation [32]byte) (ClosedProfileState, []byte, error) {
	if s == nil || s.root == nil {
		return ClosedProfileState{}, nil, errors.New("retained State inspection absent")
	}
	return s.root.LoadClosedProfile(generation)
}

func (s *Inspection) Check() (err error) {
	if s == nil || s.root == nil {
		return errors.New("retained State inspection absent")
	}
	s.root.mu.Lock()
	defer s.root.mu.Unlock()
	defer func() {
		if err != nil {
			s.terminal = errors.Join(s.terminal, err)
		}
	}()
	if s.root.closed || s.terminal != nil {
		return errors.Join(errors.New("retained State inspection closed"), s.terminal)
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if err := s.nativeRoot(); err != nil {
		return err
	}
	for path, file := range s.parents {
		opened, e := file.Stat()
		named, ne := os.Lstat(path)
		original := s.parentInfo[path]
		if e != nil || ne != nil || !os.SameFile(original, opened) || !os.SameFile(original, named) || original.Mode() != named.Mode() || named.Sys().(*syscall.Stat_t).Uid != 0 || named.Sys().(*syscall.Stat_t).Gid != original.Sys().(*syscall.Stat_t).Gid {
			return errors.Join(errors.New("retained State original ancestor lost"), e, ne)
		}
	}
	for _, entry := range s.order {
		opened, e := entry.file.Stat()
		named, ne := os.Lstat(entry.path)
		if e != nil || ne != nil || !sameInspectionFile(entry.info, opened) || !sameInspectionFile(entry.info, named) {
			return errors.Join(errors.New("retained State original entry lost"), e, ne)
		}
		if _, err := entry.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if entry.info.IsDir() {
			names, e := entry.file.Readdirnames(len(entry.names) + 1)
			if e != nil && e != io.EOF {
				return e
			}
			slices.Sort(names)
			if !slices.Equal(names, entry.names) {
				return errors.New("retained State original inventory changed")
			}
		} else {
			h := sha256.New()
			n, e := io.Copy(h, io.LimitReader(entry.file, entry.info.Size()+1))
			if e != nil || n != entry.info.Size() || !bytes.Equal(h.Sum(nil), entry.digest[:]) {
				return errors.Join(errors.New("retained State original bytes changed"), e)
			}
		}
		opened, e = entry.file.Stat()
		named, ne = os.Lstat(entry.path)
		if e != nil || ne != nil || !sameInspectionFile(entry.info, opened) || !sameInspectionFile(entry.info, named) {
			return errors.Join(errors.New("retained State late entry changed"), e, ne)
		}
	}
	for _, entry := range s.order {
		opened, e := entry.file.Stat()
		named, ne := os.Lstat(entry.path)
		if e != nil || ne != nil || !sameInspectionFile(entry.info, opened) || !sameInspectionFile(entry.info, named) {
			return errors.Join(errors.New("retained State final namespace changed"), e, ne)
		}
	}
	return s.nativeRoot()
}

// A same-inode bind mount can change pathname flags without changing the
// retained descriptor. Both observations and the fresh reader's Close matter.
func (s *Inspection) nativeRoot() (returned error) {
	original := s.order[0]
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(original.file.Fd()), &fs); err != nil || fs.Type != 0xef53 || fs.Flags&1 != 0 {
		return errors.Join(errors.New("retained State filesystem differs"), err)
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
	if err != nil || fsErr != nil || !sameInspectionFile(original.info, info) || fs.Type != 0xef53 || fs.Flags&1 != 0 {
		return errors.Join(errors.New("retained State named filesystem differs"), err, fsErr)
	}
	return s.ctx.Err()
}

func sameInspectionFile(a, b os.FileInfo) bool {
	if a == nil || b == nil || !os.SameFile(a, b) || a.Mode() != b.Mode() || a.Size() != b.Size() {
		return false
	}
	x, xok := a.Sys().(*syscall.Stat_t)
	y, yok := b.Sys().(*syscall.Stat_t)
	return xok && yok && x.Uid == y.Uid && x.Gid == y.Gid && x.Nlink == y.Nlink && x.Ctim == y.Ctim && x.Mtim == y.Mtim
}

func (s *Inspection) Close() error {
	if s == nil || s.root == nil {
		return nil
	}
	s.root.mu.Lock()
	defer s.root.mu.Unlock()
	if s.root.closed {
		return s.terminal
	}
	s.root.closed = true
	for i := len(s.order) - 1; i >= 0; i-- {
		s.terminal = errors.Join(s.terminal, s.order[i].file.Close())
	}
	for _, file := range s.parents {
		s.terminal = errors.Join(s.terminal, file.Close())
	}
	if s.lease != nil {
		s.terminal = errors.Join(s.terminal, syscall.Flock(int(s.lease.Fd()), syscall.LOCK_UN), s.lease.Close())
	}
	return s.terminal
}
