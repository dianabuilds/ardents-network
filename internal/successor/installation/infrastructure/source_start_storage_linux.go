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

// sourceStartStorage is one attempt's distinct exclusive journal and original
// born resources. First/subsequent constructors never reopen an existing prefix.
// Its records are private evidence, never permission to replay Start or ACK.
type sourceStartStorage struct {
	ctx           context.Context
	journal       *sourceBirth
	pending       *sourceBirth
	files         []*sourceBirth
	directory     *os.File
	directoryInfo os.FileInfo
	path          string
	sequence      uint32
	number        sourceAttemptNumber
	terminal      error
	closed        bool
}

type sourceStartEntry struct {
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

func openSourceStartStorage(ctx context.Context, selection *sourceSelection) (s *sourceStartStorage, err error) {
	return createSourceStartStorage(ctx, selection, 0)
}

func openSourceInvocationStorage(ctx context.Context, selection *sourceSelection, number sourceAttemptNumber) (*sourceStartStorage, error) {
	if number == 0 || number >= sourceAttemptLimit {
		return nil, ErrInput
	}
	return createSourceStartStorage(ctx, selection, number)
}

func createSourceStartStorage(ctx context.Context, selection *sourceSelection, number sourceAttemptNumber) (s *sourceStartStorage, err error) {
	if ctx == nil || selection == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	root := selection.binding.Request.InstallationRoot
	s = &sourceStartStorage{ctx: ctx, path: number.directory(root), number: number}
	path := number.journal(root)
	if err = selection.pinParents(path); err != nil {
		return s, err
	}
	if err = selection.observe(); err != nil {
		return s, err
	}
	file, e := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return s, e
	}
	s.journal = &sourceBirth{path: path, file: file}
	s.journal.info, err = file.Stat()
	if err != nil || !sourceAccess(s.journal.info, 0600, 0, 0, false) {
		return s, errors.Join(ErrBinding, err)
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return s, err
	}
	var fs syscall.Statfs_t
	if err = syscall.Fstatfs(int(file.Fd()), &fs); err != nil || fs.Type != 0xef53 {
		return s, errors.Join(ErrBinding, err)
	}
	if err = errors.Join(file.Sync(), syncSourceParent(path)); err != nil {
		return s, err
	}
	_, phase, _, _ := number.schema()
	err = s.record(phase, path, s.journal.info, digest(selection.files[filepath.Join(root, "binding.json")].raw))
	return s, err
}

func (s *sourceStartStorage) record(phase, path string, info os.FileInfo, sha string) (err error) {
	if s == nil || s.closed || s.journal == nil {
		return ErrInput
	}
	if s.terminal != nil {
		return s.terminal
	}
	defer func() {
		err = errors.Join(err, s.ctx.Err())
		if err != nil {
			s.terminal = err
		}
	}()
	if err = s.ctx.Err(); err != nil {
		return err
	}
	switch phase {
	case "first-start":
		if s.number != 0 {
			return ErrBinding
		}
	case "invocation":
		if s.number == 0 {
			return ErrBinding
		}
	case "directory-intent", "directory-born", "directory-ready", "file-intent", "file-born", "file-ready", "socket-intent", "socket-born", "socket-ready", "start-attempt", "invocation-observed", "completion-received", "archived", "ack-attempt", "ack-written":
	default:
		return ErrBinding
	}
	if err = s.check(); err != nil {
		return err
	}
	schema, _, _, _ := s.number.schema()
	entry := sourceStartEntry{Schema: schema, Sequence: s.sequence, Phase: phase, Path: path, SHA256: sha}
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
	if len(raw) > 32<<10 || len(s.journal.body)+len(raw) > 128<<10 {
		return ErrBinding
	}
	file := s.journal.file
	if _, err = file.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	count, err := file.Write(raw)
	s.journal.body = append(s.journal.body, raw[:count]...)
	if err != nil || count != len(raw) {
		return errors.Join(io.ErrShortWrite, err)
	}
	if err = file.Sync(); err != nil {
		return err
	}
	s.journal.info, err = file.Stat()
	if err != nil {
		return err
	}
	if err = s.journal.check(); err != nil {
		return err
	}
	s.sequence++
	return s.ctx.Err()
}

func (s *sourceStartStorage) birthDirectory(gid uint32, admit func() error) (err error) {
	if gid < 100 || gid > 999 || s.directory != nil {
		return ErrInput
	}
	if err = s.record("directory-intent", s.path, nil, ""); err != nil {
		return err
	}
	if err = admit(); err != nil {
		return err
	}
	if err = os.Mkdir(s.path, 0700); err != nil {
		return err
	}
	s.directory, err = os.OpenFile(s.path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	s.directoryInfo, err = s.directory.Stat()
	if err != nil || !sourceAccess(s.directoryInfo, 0700, 0, 0, true) {
		return errors.Join(ErrBinding, err)
	}
	if err = observeSourceDirectory(s.path, s.directory, s.directoryInfo); err != nil {
		return err
	}
	if err = s.record("directory-born", s.path, s.directoryInfo, ""); err != nil {
		return err
	}
	if err = admit(); err != nil {
		return err
	}
	if err = observeSourceDirectory(s.path, s.directory, s.directoryInfo); err != nil {
		return err
	}
	if err = errors.Join(s.directory.Chown(0, int(gid)), s.directory.Chmod(0750), s.directory.Sync(), syncSourceParent(s.path)); err != nil {
		return err
	}
	born := s.directoryInfo
	s.directoryInfo, err = s.directory.Stat()
	if err != nil || !os.SameFile(born, s.directoryInfo) || !sourceAccess(s.directoryInfo, 0750, 0, gid, true) {
		return errors.Join(ErrBinding, err)
	}
	return s.record("directory-ready", s.path, s.directoryInfo, "")
}

func (s *sourceStartStorage) write(name string, body []byte, admit func() error) (result *sourceBirth, err error) {
	if s == nil || s.directory == nil || len(body) == 0 || len(body) > 64<<10 {
		return nil, ErrInput
	}
	switch name {
	case "start-guard.json", "start-socket.json", "start-archived.json":
	default:
		return nil, ErrInput
	}
	path := filepath.Join(s.path, name)
	if err = s.record("file-intent", path, nil, digest(body)); err != nil {
		return nil, err
	}
	if err = admit(); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	born := &sourceBirth{path: path, file: file}
	s.files = append(s.files, born)
	born.info, err = file.Stat()
	if err != nil || !sourceAccess(born.info, 0600, 0, 0, false) {
		return nil, errors.Join(ErrBinding, err)
	}
	if err = errors.Join(file.Sync(), s.directory.Sync()); err != nil {
		return nil, err
	}
	if err = s.record("file-born", path, born.info, digest(body)); err != nil {
		return nil, err
	}
	if err = admit(); err != nil {
		return nil, err
	}
	if err = born.check(); err != nil {
		return nil, err
	}
	count, err := file.Write(body)
	born.body = bytes.Clone(body[:count])
	if err != nil || count != len(body) {
		return nil, errors.Join(io.ErrShortWrite, err)
	}
	if err = file.Sync(); err != nil {
		return nil, err
	}
	born.info, err = file.Stat()
	if err != nil || !sourceAccess(born.info, 0600, 0, 0, false) {
		return nil, errors.Join(ErrBinding, err)
	}
	if err = born.check(); err != nil {
		return nil, err
	}
	if err = s.record("file-ready", path, born.info, digest(body)); err != nil {
		return nil, err
	}
	return born, admit()
}

func (s *sourceStartStorage) preparePending(binding string) error {
	if s == nil || s.directory == nil || s.pending != nil || !sourceInitialSHA(binding) {
		return ErrInput
	}
	if err := s.check(); err != nil {
		return err
	}
	path := filepath.Join(s.path, "source-start-pending")
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	s.pending = &sourceBirth{path: path, file: file}
	body := []byte("ardents-source-start-pending-v1\n" + binding + "\n")
	count, err := file.Write(body)
	s.pending.body = bytes.Clone(body[:count])
	if err != nil || count != len(body) {
		return errors.Join(io.ErrShortWrite, err)
	}
	if err = errors.Join(file.Sync(), s.directory.Sync()); err != nil {
		return err
	}
	s.pending.info, err = file.Stat()
	if err != nil || !sourceAccess(s.pending.info, 0600, 0, 0, false) {
		return errors.Join(ErrBinding, err)
	}
	return s.pending.check()
}

func (s *sourceStartStorage) check() (err error) {
	if s == nil || s.ctx == nil || s.closed || s.journal == nil {
		return ErrInput
	}
	if s.terminal != nil {
		return s.terminal
	}
	if err = s.ctx.Err(); err != nil {
		return err
	}
	if err = s.journal.check(); err != nil {
		return err
	}
	if s.directory != nil {
		if err = observeSourceDirectory(s.path, s.directory, s.directoryInfo); err != nil {
			return err
		}
	}
	for _, file := range s.files {
		if err = file.check(); err != nil {
			return err
		}
	}
	if s.pending != nil {
		if err = s.pending.check(); err != nil {
			return err
		}
	}
	if s.directory != nil {
		if err = observeSourceDirectory(s.path, s.directory, s.directoryInfo); err != nil {
			return err
		}
	}
	return s.ctx.Err()
}

func (s *sourceStartStorage) close() error {
	if s == nil {
		return nil
	}
	if !s.closed {
		s.closed = true
		for _, file := range s.files {
			s.terminal = errors.Join(s.terminal, file.file.Close())
			clear(file.body)
		}
		if s.directory != nil {
			s.terminal = errors.Join(s.terminal, s.directory.Close())
		}
		if s.pending != nil {
			s.terminal = errors.Join(s.terminal, s.pending.file.Close())
		}
		// The writer lease is released last, after the owner's other borrowers.
		if s.journal != nil {
			s.terminal = errors.Join(s.terminal, s.journal.file.Close())
		}
		s.terminal = errors.Join(s.terminal, s.ctx.Err())
	}
	return s.terminal
}

func (s *sourceStartStorage) publishStart() error {
	if s == nil || !s.closed || s.terminal != nil || s.pending == nil || s.directoryInfo == nil {
		return ErrBinding
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	directory, err := os.Lstat(s.path)
	native, ok := s.directoryInfo.Sys().(*syscall.Stat_t)
	if err != nil || !ok || !os.SameFile(s.directoryInfo, directory) || !sourceAccess(directory, 0750, 0, native.Gid, true) {
		return errors.Join(ErrBinding, err)
	}
	current, err := os.Lstat(s.pending.path)
	if err != nil || !sameSourceFile(s.pending.info, current) {
		return errors.Join(ErrBinding, err)
	}
	// All accepting proof bytes were synced and all root observers/writers
	// closed before this final unlink. Its loss restores refusal, never Start.
	return os.Remove(s.pending.path)
}
