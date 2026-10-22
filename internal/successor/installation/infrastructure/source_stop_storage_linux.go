package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

type sourceStopStorage struct {
	ctx                      context.Context
	selection                *sourceSelection
	directory                string
	journal, result, pending *sourceBirth
	directoryInfo            os.FileInfo
	listener                 *net.UnixListener
	socket                   os.FileInfo
	sequence                 uint32
	terminal                 error
	closed                   bool
}

func openSourceStopStorage(ctx context.Context, selection *sourceSelection, started *sourceStartHistory) (s *sourceStopStorage, err error) {
	if ctx == nil || selection == nil || started == nil || os.Geteuid() != 0 {
		return nil, ErrInput
	}
	s = &sourceStopStorage{ctx: ctx, selection: selection, directory: started.number.directory(selection.binding.Request.InstallationRoot)}
	if err = selection.observe(); err != nil {
		return s, err
	}
	s.directoryInfo = selection.identities[s.directory]
	pendingPath := filepath.Join(s.directory, "source-retirement-pending")
	pending, err := os.OpenFile(pendingPath, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return s, err
	}
	s.pending = &sourceBirth{path: pendingPath, file: pending}
	body := []byte("ardents-source-retirement-pending-v1\n" + started.sha256 + "\n")
	count, err := pending.Write(body)
	s.pending.body = bytes.Clone(body[:count])
	if err != nil || count != len(body) {
		return s, errors.Join(io.ErrShortWrite, err)
	}
	if err = errors.Join(pending.Sync(), syncSourceParent(pendingPath)); err != nil {
		return s, err
	}
	s.pending.info, err = pending.Stat()
	if err != nil || !sourceAccess(s.pending.info, 0600, 0, 0, false) {
		return s, errors.Join(ErrBinding, err)
	}
	if err = s.pending.check(); err != nil {
		return s, err
	}
	path := filepath.Join(s.directory, "source-retirement-journal")
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return s, err
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
	return s, s.record("stop", path, s.journal.info, started.sha256)
}

func (s *sourceStopStorage) record(phase, path string, info os.FileInfo, sha string) (err error) {
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
	case "stop", "socket-intent", "socket-born", "socket-ready", "interrupt-attempt", "domain-joined", "native-joined", "native-closed", "file-intent", "file-born", "file-ready", "retired":
	default:
		return ErrBinding
	}
	if err = s.check(); err != nil {
		return err
	}
	r := sourceStartEntry{Schema: "ardents-source-retirement-record-v1", Sequence: s.sequence, Phase: phase, Path: path, SHA256: sha}
	if info != nil {
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return ErrBinding
		}
		r.Device, r.Inode, r.Mode, r.UID, r.GID = uint64(native.Dev), native.Ino, uint32(info.Mode().Perm()), native.Uid, native.Gid
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if len(raw) > 32<<10 || len(s.journal.body)+len(raw) > 128<<10 {
		return ErrBinding
	}
	if _, err = s.journal.file.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	count, err := s.journal.file.Write(raw)
	s.journal.body = append(s.journal.body, raw[:count]...)
	if err != nil || count != len(raw) {
		return errors.Join(io.ErrShortWrite, err)
	}
	if err = s.journal.file.Sync(); err != nil {
		return err
	}
	s.journal.info, err = s.journal.file.Stat()
	if err != nil {
		return err
	}
	if err = s.journal.check(); err != nil {
		return err
	}
	s.sequence++
	return s.ctx.Err()
}

func (s *sourceStopStorage) prepareSocket(admit func() error) error {
	path := filepath.Join(s.directory, "stop-completion.socket")
	if err := s.record("socket-intent", path, nil, ""); err != nil {
		return err
	}
	if err := admit(); err != nil {
		return err
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(ErrBinding, err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return err
	}
	s.listener = listener
	listener.SetUnlinkOnClose(false)
	born, err := os.Lstat(path)
	if err != nil || !sameSourceSocket(born, born, born.Mode().Perm(), 0) {
		return errors.Join(ErrBinding, err)
	}
	if err = s.record("socket-born", path, born, ""); err != nil {
		return err
	}
	if err = admit(); err != nil {
		return err
	}
	current, err := os.Lstat(path)
	if err != nil || !sameSourceSocket(born, current, born.Mode().Perm(), 0) {
		return errors.Join(ErrBinding, err)
	}
	if err = os.Lchown(path, 0, int(s.selection.binding.GID)); err != nil {
		return err
	}
	current, err = os.Lstat(path)
	if err != nil || !os.SameFile(born, current) {
		return errors.Join(ErrBinding, err)
	}
	if err = os.Chmod(path, 0660); err != nil {
		return err
	}
	s.socket, err = os.Lstat(path)
	if err != nil || !os.SameFile(born, s.socket) || !sameSourceSocket(s.socket, s.socket, 0660, s.selection.binding.GID) {
		return errors.Join(ErrBinding, err)
	}
	if err = syncSourceParent(path); err != nil {
		return err
	}
	return s.record("socket-ready", path, s.socket, "")
}

func (s *sourceStopStorage) writeResult(raw []byte, admit func() error) error {
	if len(raw) == 0 || len(raw) > 64<<10 || s.result != nil {
		return ErrInput
	}
	path := filepath.Join(s.directory, "source-retirement.json")
	sha := digest(raw)
	if err := s.record("file-intent", path, nil, sha); err != nil {
		return err
	}
	if err := admit(); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	s.result = &sourceBirth{path: path, file: file}
	s.result.info, err = file.Stat()
	if err != nil || !sourceAccess(s.result.info, 0600, 0, 0, false) {
		return errors.Join(ErrBinding, err)
	}
	if err = errors.Join(file.Sync(), syncSourceParent(path)); err != nil {
		return err
	}
	if err = s.record("file-born", path, s.result.info, sha); err != nil {
		return err
	}
	if err = admit(); err != nil {
		return err
	}
	if err = s.result.check(); err != nil {
		return err
	}
	count, err := file.Write(raw)
	s.result.body = bytes.Clone(raw[:count])
	if err != nil || count != len(raw) {
		return errors.Join(io.ErrShortWrite, err)
	}
	if err = file.Sync(); err != nil {
		return err
	}
	s.result.info, err = file.Stat()
	if err != nil {
		return err
	}
	if err = s.record("file-ready", path, s.result.info, sha); err != nil {
		return err
	}
	if err = admit(); err != nil {
		return err
	}
	return s.record("retired", path, s.result.info, sha)
}

func (s *sourceStopStorage) check() error {
	if s == nil || s.closed || s.journal == nil {
		return ErrInput
	}
	if s.terminal != nil {
		return s.terminal
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if err := s.selection.observe(); err != nil {
		return err
	}
	if err := s.journal.check(); err != nil {
		return err
	}
	if s.pending == nil {
		return ErrBinding
	}
	if err := s.pending.check(); err != nil {
		return err
	}
	if s.result != nil {
		if err := s.result.check(); err != nil {
			return err
		}
	}
	if s.socket != nil {
		current, err := os.Lstat(filepath.Join(s.directory, "stop-completion.socket"))
		if err != nil || !sameSourceSocket(s.socket, current, 0660, s.selection.binding.GID) {
			return errors.Join(ErrBinding, err)
		}
	}
	return s.ctx.Err()
}

func (s *sourceStopStorage) closeListener() error {
	if s == nil || s.listener == nil {
		return nil
	}
	err := s.listener.Close()
	s.listener = nil
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func (s *sourceStopStorage) close() error {
	if s == nil {
		return nil
	}
	if !s.closed {
		s.closed = true
		s.terminal = errors.Join(s.terminal, s.closeListener())
		if s.result != nil {
			s.terminal = errors.Join(s.terminal, s.result.file.Close())
		}
		if s.journal != nil {
			s.terminal = errors.Join(s.terminal, s.journal.file.Close())
		}
		if s.pending != nil {
			s.terminal = errors.Join(s.terminal, s.pending.file.Close())
		}
		s.terminal = errors.Join(s.terminal, s.ctx.Err())
	}
	return s.terminal
}

// Removing the original blocker is the final commit effect. Every proof sync
// and physical Close must already have succeeded. There is deliberately no
// required sync after removal: loss of the unlink can only restore refusal,
// never make an incomplete/uncertain transcript accepting after a crash.
func (s *sourceStopStorage) publishRetirement() error {
	if s == nil || !s.closed || s.terminal != nil || s.pending == nil || s.directoryInfo == nil {
		return ErrBinding
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	directory, err := os.Lstat(s.directory)
	if err != nil || !os.SameFile(s.directoryInfo, directory) || !sourceAccess(directory, 0750, 0, s.selection.binding.GID, true) {
		return errors.Join(ErrBinding, err)
	}
	current, err := os.Lstat(s.pending.path)
	if err != nil || !sameSourceFile(s.pending.info, current) {
		return errors.Join(ErrBinding, err)
	}
	return os.Remove(s.pending.path)
}
