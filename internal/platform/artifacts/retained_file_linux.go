//go:build linux

package artifacts

import (
	"bytes"
	"errors"
	"io"
	"os"
	"syscall"
)

// ReadRetainedFile reads one original descriptor after the caller admits its
// owner and mode. The caller retains the descriptor and must reobserve it with
// CheckRetainedFile before using the bytes. A partial read is returned so its
// owner can erase it during the joined cleanup.
func ReadRetainedFile(file *os.File, maximum int64, admit func(os.FileMode, uint32) bool, afterRead func(*os.File)) (os.FileInfo, []byte, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maximum {
		return nil, nil, ErrInput
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 || !admit(info.Mode(), stat.Uid) {
		return nil, nil, ErrInput
	}
	raw, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if afterRead != nil {
		afterRead(file)
	}
	if err == nil && int64(len(raw)) != info.Size() {
		err = ErrInput
	}
	return info, raw, err
}

// CheckRetainedFile reobserves an admitted original descriptor, its direct
// link and its bytes under this directory chain. The caller supplies its
// previously admitted metadata and contents and retains ownership of both
// the file and the chain. afterRead observes completion of the physical read.
func (d *DirectoryChain) CheckRetainedFile(file *os.File, original os.FileInfo, contents []byte, afterRead func(*os.File)) error {
	if file == nil || original == nil || d.CheckOriginalAccess() != nil {
		return ErrInput
	}
	before, err := file.Stat()
	if err != nil || !sameRetainedFileMetadata(original, before) {
		return ErrInput
	}
	if err := d.CheckRetainedFileLink(file.Name(), original); err != nil {
		return err
	}

	raw := make([]byte, len(contents)+1)
	defer clear(raw)
	n, err := file.ReadAt(raw, 0)
	if afterRead != nil {
		afterRead(file)
	}
	if err != nil && err != io.EOF || !bytes.Equal(raw[:n], contents) {
		return ErrInput
	}

	after, err := file.Stat()
	if err != nil || !sameRetainedFileMetadata(original, after) {
		return ErrInput
	}
	// The descriptor may survive a path or ancestor substitution during the
	// read. Reobserve both before returning the retained result.
	if err := d.CheckOriginalAccess(); err != nil {
		return ErrInput
	}
	return d.CheckRetainedFileLink(file.Name(), original)
}

// CheckRetainedFileLink reobserves the original direct entry after a later
// input observation. The caller checks the retained ancestor access separately.
func (d *DirectoryChain) CheckRetainedFileLink(name string, original os.FileInfo) error {
	if original == nil {
		return ErrInput
	}
	linked, err := d.OpenFile(name)
	if err != nil {
		return err
	}
	observed, statErr := linked.Stat()
	if err := errors.Join(statErr, linked.Close()); err != nil {
		return err
	}
	if !sameRetainedFileMetadata(original, observed) {
		return ErrInput
	}
	return nil
}

func sameRetainedFileMetadata(original, current os.FileInfo) bool {
	a, aok := original.Sys().(*syscall.Stat_t)
	b, bok := current.Sys().(*syscall.Stat_t)
	if !aok || !bok || !os.SameFile(original, current) || original.Mode() != current.Mode() || a.Uid != b.Uid || a.Gid != b.Gid {
		return false
	}
	return a.Nlink == b.Nlink && a.Ctim == b.Ctim && a.Mtim == b.Mtim && original.Size() == current.Size()
}
