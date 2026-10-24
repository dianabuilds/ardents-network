package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// These records belong only to the initial stopped Source transition. They are
// evidence for independent prefix classification, never continuation authority.
type sourceRecord struct {
	Sequence int    `json:"sequence"`
	Phase    string `json:"phase"`
	Path     string `json:"path"`
	Device   uint64 `json:"device"`
	Inode    uint64 `json:"inode"`
	SHA256   string `json:"sha256"`
	Mode     uint32 `json:"mode"`
	UID      uint32 `json:"uid"`
	GID      uint32 `json:"gid"`
}

type sourceBirth struct {
	path string
	file *os.File
	info os.FileInfo
	body []byte
}

type sourceBirths struct {
	ctx            context.Context
	journal        *sourceBirth
	firstPhase     string
	sequence       int
	files          []*sourceBirth
	directories    map[string]os.FileInfo
	directoryFiles map[string]*os.File
	// Finite effect inventory, constructed from the admitted Source request.
	allowed  map[string]bool
	terminal error
	closed   bool
	// Failure-only native test seam, invoked after the real fsync completes.
	// It cannot replace physical I/O or supply accepting bytes/provenance.
	afterSync func(*os.File) error
	// A fresh recovery writer owns a separate completion file. It never lends
	// the old journal descriptor to this append path or revives its caller.
	recovery             *sourceRecoveryWriting
	transitionGeneration string
	transitionBinding    string
	transitionManager    string
}

func openSourceBirths(ctx context.Context, journalPath string, allowed map[string]bool) (*sourceBirths, error) {
	return openSourceBirthsWithPhase(ctx, journalPath, allowed, "initial")
}

func openSourceRecoverableInitialBirths(ctx context.Context, journalPath string, allowed map[string]bool) (*sourceBirths, error) {
	return openSourceBirthsWithPhase(ctx, journalPath, allowed, sourceRecoverableInitialPhase)
}

func openSourceBirthsWithPhase(ctx context.Context, journalPath string, allowed map[string]bool, phase string) (*sourceBirths, error) {
	s := &sourceBirths{ctx: ctx, firstPhase: phase, allowed: allowed, directories: map[string]os.FileInfo{}, directoryFiles: map[string]*os.File{}}
	if ctx == nil || ctx.Err() != nil || !sourcePath(journalPath) || phase != "initial" && phase != sourceRecoverableInitialPhase && phase != "source-cut" {
		return s, ErrInput
	}
	parent, err := os.Lstat(filepath.Dir(journalPath))
	if err != nil || !trustedDirectory(parent) {
		return s, errors.Join(ErrBinding, err)
	}
	f, err := os.OpenFile(journalPath, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return s, err
	}
	s.journal = &sourceBirth{path: journalPath, file: f}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return s, err
	}
	s.journal.info, err = f.Stat()
	if err != nil || !sourceAccess(s.journal.info, 0600, 0, 0, false) {
		return s, errors.Join(ErrBinding, err)
	}
	if err = errors.Join(f.Sync(), syncSourceParent(journalPath)); err != nil {
		return s, err
	}
	return s, s.record(phase, journalPath, s.journal.info, "", 0600, 0, 0)
}

func syncSourceParent(path string) (err error) {
	p := filepath.Dir(path)
	before, err := os.Lstat(p)
	if err != nil || before == nil || !before.IsDir() || before.Mode().Perm()&0022 != 0 {
		return errors.Join(ErrBinding, err)
	}
	f, err := os.OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return errors.Join(ErrBinding, err)
	}
	if err = f.Sync(); err != nil {
		return err
	}
	after, err := os.Lstat(p)
	if err != nil || !os.SameFile(opened, after) || opened.Mode() != after.Mode() {
		return errors.Join(ErrBinding, err)
	}
	return nil
}

func (s *sourceBirths) record(phase, path string, info os.FileInfo, sha string, mode os.FileMode, uid, gid uint32) (err error) {
	if s == nil {
		return ErrInput
	}
	defer func() {
		if err != nil && s.terminal == nil {
			s.terminal = err
		}
	}()
	if s.closed || s.journal == nil || s.terminal != nil {
		return errors.Join(ErrBinding, s.terminal)
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	r := sourceRecord{Sequence: s.sequence, Phase: phase, Path: path, SHA256: sha, Mode: uint32(mode), UID: uid, GID: gid}
	if info != nil {
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return ErrBinding
		}
		r.Device, r.Inode = uint64(st.Dev), st.Ino
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if len(raw)+1 > 32<<10 {
		return ErrBinding
	}
	canonical := raw
	if s.recovery != nil {
		if err := s.recovery.beforeRecord(s); err != nil {
			return err
		}
		if len(s.recovery.records) != 0 {
			raw = append([]byte{','}, raw...)
		}
	} else {
		raw = append(raw, '\n')
	}
	if err := s.appendJournal(raw); err != nil {
		return err
	}
	if s.recovery != nil {
		s.recovery.records = append(s.recovery.records, r)
		s.recovery.transcript = append(s.recovery.transcript, canonical...)
		s.recovery.transcript = append(s.recovery.transcript, '\n')
	}
	s.sequence++
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if s.recovery == nil && s.transitionGeneration != "" {
		if coordinator, ok := s.ctx.Value(sourceTransitionContextKey{}).(*SourceTransitionCoordinator); ok {
			return coordinator.report(s)
		}
	}
	return nil
}

func (s *sourceBirths) bindTransition(generation, binding, operation, manager string) error {
	if s == nil || s.closed || s.terminal != nil || !sourceInitialSHA(generation) || !sourceInitialSHA(binding) {
		return ErrInput
	}
	if coordinator, ok := s.ctx.Value(sourceTransitionContextKey{}).(*SourceTransitionCoordinator); ok && coordinator.operation != operation {
		return ErrBinding
	}
	s.transitionGeneration, s.transitionBinding = generation, binding
	s.transitionManager = manager
	return nil
}

// Append to this operation's actual file, preserving visible partial bytes on
// any error. The caller owns record grammar and never retries uncertain I/O.
func (s *sourceBirths) appendJournal(raw []byte) (err error) {
	defer func() {
		if err != nil && s.terminal == nil {
			s.terminal = err
		}
	}()
	if s.closed || s.terminal != nil || s.journal == nil || len(raw) == 0 || len(s.journal.body)+len(raw) > 4<<20 {
		return errors.Join(ErrBinding, s.terminal)
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if err := s.journal.check(); err != nil {
		return err
	}
	f := s.journal.file
	if _, err = f.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	n, err := f.Write(raw)
	// Retain visible bytes even on an uncertain write; this owner never retries.
	s.journal.body = append(s.journal.body, raw[:n]...)
	if err != nil || n != len(raw) {
		s.terminal = errors.Join(io.ErrShortWrite, err)
		return s.terminal
	}
	if err = s.sync(f); err != nil {
		s.terminal = err
		return err
	}
	s.journal.info, err = f.Stat()
	if err != nil {
		s.terminal = err
		return err
	}
	if err = s.journal.check(); err != nil {
		s.terminal = err
		return err
	}
	return s.ctx.Err()
}

func (s *sourceBirths) directory(path string, mode os.FileMode, uid, gid uint32, admit func() error) (err error) {
	defer func() {
		if err != nil && s.terminal == nil {
			s.terminal = err
		}
	}()
	if !s.allowed[path] {
		return ErrBinding
	}
	if err := s.record("directory-intent", path, nil, "", mode, uid, gid); err != nil {
		return err
	}
	if err := admit(); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	birth, err := f.Stat()
	// Retain the original directory descriptor even on a partial birth. An
	// inode number alone could be reused after a substituted directory is unlinked.
	s.directoryFiles[path] = f
	named, ne := os.Lstat(path)
	if err != nil || ne != nil || !trustedDirectory(birth) || !os.SameFile(birth, named) {
		return errors.Join(ErrBinding, err, ne)
	}
	if err = s.record("directory-born", path, birth, "", mode, uid, gid); err != nil {
		return err
	}
	if err = admit(); err != nil {
		return err
	}
	named, ne = os.Lstat(path)
	if ne != nil || !os.SameFile(birth, named) || !sourceAccess(named, 0700, 0, 0, true) {
		return errors.Join(ErrBinding, ne)
	}
	if err = errors.Join(f.Chown(int(uid), int(gid)), f.Chmod(mode), f.Sync(), syncSourceParent(path)); err != nil {
		return err
	}
	info, err := f.Stat()
	named, ne = os.Lstat(path)
	if err != nil || ne != nil || !os.SameFile(birth, info) || !os.SameFile(info, named) || !sourceAccess(info, mode, uid, gid, true) {
		return errors.Join(ErrBinding, err, ne)
	}
	s.directories[path] = info
	return s.record("directory-ready", path, info, "", mode, uid, gid)
}

func sourceAccess(info os.FileInfo, mode os.FileMode, uid, gid uint32, directory bool) bool {
	if info == nil || info.IsDir() != directory || info.Mode().Perm() != mode || info.Mode()&(os.ModeSymlink|os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Uid == uid && st.Gid == gid && (directory || info.Mode().IsRegular() && st.Nlink == 1)
}

func (s *sourceBirths) write(path string, body []byte, mode os.FileMode, uid, gid uint32, admit func() error) (err error) {
	defer func() {
		if err != nil && s.terminal == nil {
			s.terminal = err
		}
	}()
	if !s.allowed[path] || len(body) == 0 || len(body) > 64<<20 {
		return ErrBinding
	}
	if err := admit(); err != nil {
		return err
	}
	if err := s.record("file-intent", path, nil, digest(body), mode, uid, gid); err != nil {
		return err
	}
	if err := admit(); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	b := &sourceBirth{path: path, file: f}
	s.files = append(s.files, b)
	b.info, err = f.Stat()
	named, ne := os.Lstat(path)
	if err != nil || ne != nil || !sourceAccess(b.info, 0600, 0, 0, false) || !os.SameFile(b.info, named) {
		return errors.Join(ErrBinding, err, ne)
	}
	if err = errors.Join(f.Sync(), syncSourceParent(path)); err != nil {
		return err
	}
	if err = s.record("file-born", path, b.info, digest(body), mode, uid, gid); err != nil {
		return err
	}
	if err = admit(); err != nil {
		return err
	}
	if err = b.check(); err != nil {
		return err
	}
	n, err := f.Write(body)
	b.body = bytes.Clone(body[:n])
	if err != nil || n != len(body) {
		return errors.Join(io.ErrShortWrite, err)
	}
	b.info, err = f.Stat()
	if err != nil || !sourceAccess(b.info, 0600, 0, 0, false) {
		return errors.Join(ErrBinding, err)
	}
	if err = b.check(); err != nil {
		return err
	}
	if err = admit(); err != nil {
		return err
	}
	if err = errors.Join(f.Chown(int(uid), int(gid)), f.Chmod(mode), s.sync(f), syncSourceParent(path)); err != nil {
		return err
	}
	b.info, err = f.Stat()
	if err != nil || !sourceAccess(b.info, mode, uid, gid, false) {
		return errors.Join(ErrBinding, err)
	}
	if err = b.check(); err != nil {
		return err
	}
	if err = s.record("file-ready", path, b.info, digest(body), mode, uid, gid); err != nil {
		return err
	}
	return admit()
}

func (b *sourceBirth) check() error {
	if b == nil || b.file == nil || b.info == nil {
		return ErrBinding
	}
	info, err := b.file.Stat()
	named, ne := os.Lstat(b.path)
	if err != nil || ne != nil || !sameSourceFile(b.info, info) || !sameSourceFile(b.info, named) {
		return errors.Join(ErrBinding, err, ne)
	}
	if _, err = b.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	raw, err := io.ReadAll(io.LimitReader(b.file, int64(len(b.body))+1))
	equal := bytes.Equal(raw, b.body)
	clear(raw)
	if err != nil || !equal {
		return errors.Join(ErrBinding, err)
	}
	after, err := b.file.Stat()
	named, ne = os.Lstat(b.path)
	if err != nil || ne != nil || !sameSourceFile(b.info, after) || !sameSourceFile(b.info, named) {
		return errors.Join(ErrBinding, err, ne)
	}
	return nil
}

func (s *sourceBirths) check() (err error) {
	defer func() {
		if err != nil && s.terminal == nil {
			s.terminal = err
		}
	}()
	if s.closed {
		return errors.Join(ErrInput, s.terminal)
	}
	if s.terminal != nil {
		return s.terminal
	}
	if err := s.journal.check(); err != nil {
		return err
	}
	if s.recovery != nil {
		if err := s.recovery.prefix.check(); err != nil {
			return err
		}
	}
	for path, before := range s.directories {
		now, err := os.Lstat(path)
		original, oe := s.directoryFiles[path].Stat()
		if err != nil || oe != nil || !os.SameFile(before, original) || !os.SameFile(before, now) || before.Mode() != now.Mode() {
			return errors.Join(ErrBinding, err, oe)
		}
		a, b := before.Sys().(*syscall.Stat_t), now.Sys().(*syscall.Stat_t)
		if a.Uid != b.Uid || a.Gid != b.Gid {
			return ErrBinding
		}
	}
	for _, b := range s.files {
		// An exclusive empty birth is recorded before writing; it remains part
		// of original custody during the journal's own I/O.
		if err := b.check(); err != nil {
			return err
		}
	}
	// A later read can yield while an earlier name/ancestor is replaced.
	// Reobserve the entire retained namespace after all byte reads have joined.
	for _, b := range s.files {
		named, e := os.Lstat(b.path)
		if e != nil || !sameSourceFile(b.info, named) {
			return errors.Join(ErrBinding, e)
		}
	}
	for path, before := range s.directories {
		now, e := os.Lstat(path)
		if e != nil || !os.SameFile(before, now) || before.Mode() != now.Mode() {
			return errors.Join(ErrBinding, e)
		}
		a, b := before.Sys().(*syscall.Stat_t), now.Sys().(*syscall.Stat_t)
		if a.Uid != b.Uid || a.Gid != b.Gid {
			return ErrBinding
		}
	}
	return s.ctx.Err()
}

func (s *sourceBirths) close() error {
	if s == nil {
		return nil
	}
	if s.closed {
		return s.terminal
	}
	s.closed = true
	for _, f := range s.directoryFiles {
		s.terminal = errors.Join(s.terminal, f.Close())
	}
	for i := len(s.files) - 1; i >= 0; i-- {
		b := s.files[i]
		s.terminal = errors.Join(s.terminal, b.file.Close())
		clear(b.body)
	}
	if s.journal != nil {
		s.terminal = errors.Join(s.terminal, s.journal.file.Close())
	}
	if s.ctx != nil {
		s.terminal = errors.Join(s.terminal, s.ctx.Err())
	} else {
		s.terminal = errors.Join(s.terminal, ErrInput)
	}
	if s.recovery != nil && s.recovery.borrowed {
		// Keep original-prefix exclusion until every new native writer/user has
		// physically closed, including a failed acquisition or partial envelope.
		s.recovery.prefix.record(s.terminal)
		s.recovery.borrowed = false
		s.recovery.prefix.users.Done()
	}
	return s.terminal
}

func (s *sourceBirths) sync(f *os.File) error {
	if err := f.Sync(); err != nil {
		return err
	}
	if s.afterSync != nil {
		return s.afterSync(f)
	}
	return nil
}
