package infrastructure

import (
	"bytes"
	"errors"
	"io"
	"os"
	"syscall"
)

// resumeSourceBornFile completes only the independently reopened empty inode
// named by the original prefix's last exact intent/born pair. Future births
// use the new writer's ordinary exclusive creation path. No intent-only path,
// partially written bytes, already promoted file or copied inode is adopted.
// The recovery coordinator owns full prefix/provenance/authority admission;
// this native primitive grants none and never constructs runtime key custody.
func (s *sourceBirths) resumeSourceBornFile(body []byte, admit func() error) (err error) {
	if s == nil || s.recovery == nil || admit == nil {
		return ErrInput
	}
	defer func() {
		if err != nil && s.terminal == nil {
			s.terminal = err
		}
	}()
	w := s.recovery
	if s.closed || s.terminal != nil || len(w.records) != 0 || len(w.prefix.records) < 2 || len(body) == 0 || len(body) > 64<<20 {
		return errors.Join(ErrBinding, s.terminal)
	}
	born := w.prefix.records[len(w.prefix.records)-1]
	intent := w.prefix.records[len(w.prefix.records)-2]
	wanted := born
	wanted.Sequence, wanted.Phase, wanted.Device, wanted.Inode = intent.Sequence, "file-intent", 0, 0
	if born.Phase != "file-born" || born.Sequence != intent.Sequence+1 || intent != wanted ||
		born.Device == 0 || born.Inode == 0 || !s.allowed[born.Path] || digest(body) != born.SHA256 {
		return ErrBinding
	}
	if err = errors.Join(w.beforeRecord(s), admit()); err != nil {
		return err
	}
	file, err := os.OpenFile(born.Path, os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	b := &sourceBirth{path: born.Path, file: file}
	s.files = append(s.files, b)
	b.info, err = file.Stat()
	if err != nil || !sourceAccess(b.info, 0600, 0, 0, false) || b.info.Size() != 0 {
		return errors.Join(ErrBinding, err)
	}
	native, ok := b.info.Sys().(*syscall.Stat_t)
	if !ok || uint64(native.Dev) != born.Device || native.Ino != born.Inode {
		return ErrBinding
	}
	if err = errors.Join(b.check(), w.prefix.check(), admit()); err != nil {
		return err
	}
	if err = b.check(); err != nil {
		return err
	}
	n, err := file.Write(body)
	b.body = bytes.Clone(body[:n])
	if err != nil || n != len(body) {
		return errors.Join(io.ErrShortWrite, err)
	}
	b.info, err = file.Stat()
	if err != nil || !sourceAccess(b.info, 0600, 0, 0, false) {
		return errors.Join(ErrBinding, err)
	}
	if err = errors.Join(b.check(), w.prefix.check(), admit()); err != nil {
		return err
	}
	if err = b.check(); err != nil {
		return err
	}
	if err = errors.Join(file.Chown(int(born.UID), int(born.GID)), file.Chmod(os.FileMode(born.Mode)), s.sync(file), syncSourceParent(born.Path)); err != nil {
		return err
	}
	b.info, err = file.Stat()
	if err != nil || !sourceAccess(b.info, os.FileMode(born.Mode), born.UID, born.GID, false) {
		return errors.Join(ErrBinding, err)
	}
	if err = b.check(); err != nil {
		return err
	}
	if err = s.record("file-ready", born.Path, b.info, born.SHA256, os.FileMode(born.Mode), born.UID, born.GID); err != nil {
		return err
	}
	return errors.Join(w.prefix.check(), admit())
}
