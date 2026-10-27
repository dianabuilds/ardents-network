package forwarding

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

// startStorage is one serialized root attempt's physical custody. Creation is
// exclusive even after a failed or uncertain attempt. Neither its records nor
// its constructors authorize Start, ACK, retirement or a retained restart.
// The root producer must independently retain Completed, fresh Release and all
// original manager/process/scope users until their actual joined completion.
type startStorage struct {
	ctx      context.Context
	parents  *inputs
	journal  *initialFile
	files    []*initialFile
	dir      *os.File
	dirInfo  os.FileInfo
	path     string
	number   attemptNumber
	sequence uint32
	terminal error
	closed   bool
	// Negative controls run after real Sync or before real Close, never supply
	// successful observations. This owner has no concurrent background writer.
	observe func(string, *os.File) error
}

type startEntry struct {
	Schema   string `json:"schema"`
	Sequence uint32 `json:"sequence"`
	Phase    string `json:"phase"`
	Path     string `json:"path"`
	Device   uint64 `json:"device"`
	Inode    uint64 `json:"inode"`
	SHA256   string `json:"sha256"`
	Mode     uint32 `json:"mode"`
	UID      uint32 `json:"uid"`
	GID      uint32 `json:"gid"`
}

func createStartStorage(ctx context.Context, root string, number attemptNumber, bindingSHA string) (s *startStorage, returned error) {
	s = &startStorage{ctx: ctx, path: number.directory(root), number: number}
	defer func() { s.retain(returned) }()
	if ctx == nil || os.Geteuid() != 0 || !validPath(root) || number >= attemptLimit || !validStartDigest(bindingSHA) {
		return s, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return s, err
	}
	path := number.journal(root)
	s.parents = &inputs{ctx: ctx, byPath: map[string]*inputDirectory{}}
	if _, err := s.parents.parents(path); err != nil {
		return s, err
	}
	directoryParent, err := s.parents.parents(s.path)
	if err != nil {
		return s, err
	}
	var directoryFS syscall.Statfs_t
	if err := syscall.Fstatfs(int(directoryParent.file.Fd()), &directoryFS); err != nil || directoryFS.Type != 0xef53 || directoryFS.Flags&1 != 0 {
		return s, errors.Join(ErrBinding, err)
	}
	if err := s.parents.links(); err != nil {
		return s, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return s, err
	}
	s.journal = &initialFile{path: path, file: f}
	s.journal.info, err = f.Stat()
	if err != nil || !nodeAccess(s.journal.info, 0600, 0, 0, false) {
		return s, errors.Join(ErrBinding, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return s, err
	}
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(f.Fd()), &fs); err != nil || fs.Type != 0xef53 || fs.Flags&1 != 0 {
		return s, errors.Join(ErrBinding, err)
	}
	if err := errors.Join(f.Sync(), syncInitialParent(path)); err != nil {
		return s, err
	}
	phase := "first-start"
	if number != 0 {
		phase = "invocation"
	}
	return s, s.record(phase, path, s.journal.info, bindingSHA)
}

func validStartDigest(value string) bool {
	return len(value) == 64 && len(bytes.Trim([]byte(value), "0123456789abcdef")) == 0
}

func (s *startStorage) retain(err error) {
	if s != nil && err != nil && s.terminal == nil {
		s.terminal = err
	}
}

func (s *startStorage) record(phase, path string, info os.FileInfo, sha string) (returned error) {
	if s == nil {
		return ErrInput
	}
	defer func() { s.retain(returned) }()
	if err := s.check(); err != nil {
		return err
	}
	if !s.recordPath(path) || sha != "" && !validStartDigest(sha) {
		return ErrBinding
	}
	switch phase {
	case "first-start":
		if s.number != 0 || s.sequence != 0 || path != s.journal.path {
			return ErrBinding
		}
	case "invocation":
		if s.number == 0 || s.sequence != 0 || path != s.journal.path {
			return ErrBinding
		}
	case "directory-intent", "directory-born", "directory-ready", "file-intent", "file-born", "file-ready", "socket-intent", "socket-born", "socket-ready", "start-attempt", "invocation-observed", "completion-received", "archived", "ack-attempt", "ack-written":
		if s.sequence == 0 {
			return ErrBinding
		}
	default:
		return ErrBinding
	}
	entry := startEntry{Schema: "ardents-forwarding-start-journal-v1", Sequence: s.sequence, Phase: phase, Path: path, SHA256: sha}
	if info != nil {
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return ErrBinding
		}
		entry.Device, entry.Inode = uint64(native.Dev), native.Ino
		entry.Mode, entry.UID, entry.GID = uint32(info.Mode().Perm()), native.Uid, native.Gid
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if len(raw) > 32<<10 || len(s.journal.body)+len(raw) > 128<<10 || s.sequence >= 256 {
		return ErrBinding
	}
	n, err := s.journal.file.WriteAt(raw, int64(len(s.journal.body)))
	if err != nil || n != len(raw) {
		return errors.Join(io.ErrShortWrite, err)
	}
	if err := s.journal.file.Sync(); err != nil {
		return err
	}
	if s.observe != nil {
		if err := s.observe("synced", s.journal.file); err != nil {
			return err
		}
	}
	s.journal.body = append(s.journal.body, raw...)
	s.journal.info, err = s.journal.file.Stat()
	if err != nil {
		return err
	}
	s.sequence++
	return s.check()
}

func (s *startStorage) recordPath(path string) bool {
	if path == s.journal.path || path == s.path {
		return true
	}
	for _, name := range []string{"start-guard.json", "start-socket.json", "start-archived.json", "start-completion.socket", "forwarding-start-pending"} {
		if path == filepath.Join(s.path, name) {
			return true
		}
	}
	return false
}

func (s *startStorage) birthDirectory(gid uint32, admit func() error) (returned error) {
	if s == nil || s.dir != nil || gid < 100 || gid > 999 || admit == nil {
		return ErrInput
	}
	defer func() { s.retain(returned) }()
	if err := s.record("directory-intent", s.path, nil, ""); err != nil {
		return err
	}
	if err := admit(); err != nil {
		return err
	}
	if err := s.check(); err != nil {
		return err
	}
	if err := os.Mkdir(s.path, 0700); err != nil {
		return err
	}
	var err error
	s.dir, err = os.OpenFile(s.path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	s.dirInfo, err = s.dir.Stat()
	if err != nil || !nodeAccess(s.dirInfo, 0700, 0, 0, true) {
		return errors.Join(ErrBinding, err)
	}
	if err := errors.Join(s.dir.Sync(), syncInitialParent(s.path)); err != nil {
		return err
	}
	if err := s.record("directory-born", s.path, s.dirInfo, ""); err != nil {
		return err
	}
	if err := admit(); err != nil {
		return err
	}
	if err := s.check(); err != nil {
		return err
	}
	if err := s.dir.Chown(0, int(gid)); err != nil {
		return err
	}
	if err := s.dir.Chmod(0750); err != nil {
		return err
	}
	if err := s.dir.Sync(); err != nil {
		return err
	}
	born := s.dirInfo
	s.dirInfo, err = s.dir.Stat()
	if err != nil || !os.SameFile(born, s.dirInfo) || !nodeAccess(s.dirInfo, 0750, 0, gid, true) {
		return errors.Join(ErrBinding, err)
	}
	if err := s.record("directory-ready", s.path, s.dirInfo, ""); err != nil {
		return err
	}
	return admit()
}

func (s *startStorage) write(name string, body []byte, admit func() error) (result *initialFile, returned error) {
	if s == nil || s.dir == nil || admit == nil || len(body) == 0 || len(body) > 64<<10 {
		return nil, ErrInput
	}
	defer func() { s.retain(returned) }()
	switch name {
	case "start-guard.json", "start-socket.json", "start-archived.json", "forwarding-start-pending":
	default:
		return nil, ErrInput
	}
	path := filepath.Join(s.path, name)
	if err := s.record("file-intent", path, nil, digest(body)); err != nil {
		return nil, err
	}
	if err := admit(); err != nil {
		return nil, err
	}
	if err := s.check(); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	born := &initialFile{path: path, file: file}
	s.files = append(s.files, born)
	born.info, err = file.Stat()
	if err != nil || !nodeAccess(born.info, 0600, 0, 0, false) {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := errors.Join(file.Sync(), s.dir.Sync()); err != nil {
		return nil, err
	}
	if err := s.record("file-born", path, born.info, digest(body)); err != nil {
		return nil, err
	}
	if err := admit(); err != nil {
		return nil, err
	}
	if err := s.check(); err != nil {
		return nil, err
	}
	n, err := file.Write(body)
	if err != nil || n != len(body) {
		return nil, errors.Join(io.ErrShortWrite, err)
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	if s.observe != nil {
		if err := s.observe("synced", file); err != nil {
			return nil, err
		}
	}
	born.body = bytes.Clone(body)
	born.info, err = file.Stat()
	if err != nil || !nodeAccess(born.info, 0600, 0, 0, false) {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := s.record("file-ready", path, born.info, digest(body)); err != nil {
		return nil, err
	}
	if err := admit(); err != nil {
		return nil, err
	}
	return born, s.check()
}

func (s *startStorage) check() (returned error) {
	if s == nil || s.ctx == nil || s.closed || s.journal == nil || s.parents == nil {
		return ErrInput
	}
	if s.terminal != nil {
		return s.terminal
	}
	defer func() { s.retain(returned) }()
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if err := s.parents.links(); err != nil {
		return err
	}
	if err := observeInitialFile(s.journal); err != nil {
		return err
	}
	if s.dir != nil {
		current, err := s.dir.Stat()
		named, nameErr := os.Lstat(s.path)
		before, ok := infoSys(s.dirInfo)
		if err != nil || nameErr != nil || !ok || !os.SameFile(s.dirInfo, current) || !os.SameFile(s.dirInfo, named) || !nodeAccess(current, s.dirInfo.Mode().Perm(), before.Uid, before.Gid, true) || !nodeAccess(named, s.dirInfo.Mode().Perm(), before.Uid, before.Gid, true) {
			return errors.Join(ErrBinding, err, nameErr)
		}
	}
	for _, file := range s.files {
		if err := observeInitialFile(file); err != nil {
			return err
		}
	}
	return s.ctx.Err()
}

func (s *startStorage) close() error {
	if s == nil {
		return nil
	}
	if !s.closed {
		s.closed = true
		closeFile := func(file *initialFile) {
			if s.observe != nil {
				s.terminal = errors.Join(s.terminal, s.observe("before-close", file.file))
			}
			s.terminal = errors.Join(s.terminal, file.file.Close())
			clear(file.body)
		}
		for i := len(s.files) - 1; i >= 0; i-- {
			closeFile(s.files[i])
		}
		if s.dir != nil {
			s.terminal = errors.Join(s.terminal, s.dir.Close())
		}
		if s.parents != nil {
			s.terminal = errors.Join(s.terminal, (Inputs{original: s.parents}).Close())
		}
		// The journal's own exclusive lease outlives every owned file/ancestor.
		if s.journal != nil {
			closeFile(s.journal)
		}
		if s.ctx != nil {
			s.terminal = errors.Join(s.terminal, s.ctx.Err())
		}
	}
	return s.terminal
}

// Only the fully joined root producer publishes its completed attempt. This
// final one-use unlink cannot turn an uncertain journal/Close into success.
func (s *startStorage) publish() error {
	if s == nil || !s.closed || s.terminal != nil || s.dirInfo == nil {
		return ErrBinding
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	var pending *initialFile
	for _, file := range s.files {
		if file.path == filepath.Join(s.path, "forwarding-start-pending") {
			pending = file
		}
	}
	if pending == nil {
		return ErrBinding
	}
	directory, err := os.Lstat(s.path)
	native, ok := infoSys(s.dirInfo)
	if err != nil || !ok || !os.SameFile(s.dirInfo, directory) || !nodeAccess(directory, 0750, 0, native.Gid, true) {
		return errors.Join(ErrBinding, err)
	}
	current, err := os.Lstat(pending.path)
	if err != nil || !sameInputFile(pending.info, current) {
		return errors.Join(ErrBinding, err)
	}
	return os.Remove(pending.path)
}
