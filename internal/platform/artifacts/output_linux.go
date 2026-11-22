package artifacts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

type Content struct {
	Name string
	Data []byte
}

// This offline native mechanism grants no signing, Release or Installation
// authority. Composition supplies the fixed public inventory. Original input,
// parent and born file descriptors stay retained through durable readback and
// Close; residue is never removed or adopted. observe is a synchronous barrier
// around actual effects for negative controls, not an I/O replacement.
func Publish(ctx context.Context, path string, files []Content, check func() error, observe func(string, *os.File)) (result error) {
	return publish(ctx, path, files, 8, false, check, observe)
}

// secret changes only temporary-memory retirement. It supplies no authority or
// new output namespace; each finite caller admits its own fixed purpose/files.
// PublishPrivate publishes the selected artifacts and clears verification buffers.
func PublishPrivate(ctx context.Context, path string, files []Content, check func() error, observe func(string, *os.File)) error {
	return publish(ctx, path, files, 8, true, check, observe)
}

// PublishInventory admits the larger bounded Network inventory. The caller
// selects its file names and per-file bounds before borrowing the writer.
// Sensitive inventories clear all owned readback copies.
func PublishInventory(ctx context.Context, path string, files []Content, sensitive bool, check func() error, observe func(string, *os.File)) error {
	return publish(ctx, path, files, 129, sensitive, check, observe)
}

func publish(ctx context.Context, path string, files []Content, maximum int, secret bool, check func() error, observe func(string, *os.File)) (result error) {
	if ctx == nil || os.Geteuid() != 0 || !AbsolutePath(path) || len(files) == 0 || len(files) > maximum || check == nil {
		return ErrInput
	}
	seen := make(map[string]bool, len(files))
	for _, file := range files {
		if file.Name == "." || file.Name == ".." || filepath.IsAbs(file.Name) || filepath.Base(file.Name) != file.Name || seen[file.Name] || len(file.Data) == 0 || len(file.Data) > 64<<20 {
			return ErrInput
		}
		seen[file.Name] = true
	}
	parents := NewInputs(ctx)
	defer func() { result = errors.Join(result, parents.Close(), ctx.Err()) }()
	parent, err := parents.Directory(filepath.Dir(path))
	if err != nil {
		return err
	}
	if parent.info.Mode() != os.ModeDir|0700 {
		return ErrInput
	}
	barrier := func(label string, file *os.File) error {
		if observe != nil {
			observe(label, file)
		}
		if os.Geteuid() != 0 {
			return ErrInput
		}
		for _, directory := range parents.files {
			info, err := directory.file.Stat()
			if err != nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 || info.Sys().(*syscall.Stat_t).Uid != 0 {
				return errors.Join(ErrInput, err)
			}
		}
		return errors.Join(parents.Check(), check(), ctx.Err())
	}
	var filesystem syscall.Statfs_t
	if err := syscall.Fstatfs(int(parent.file.Fd()), &filesystem); err != nil || filesystem.Type != 0xef53 {
		return errors.Join(ErrInput, err)
	}
	if err := errors.Join(parent.file.Sync(), barrier("before-directory", parent.file)); err != nil {
		return err
	}
	if err := syscall.Mkdirat(int(parent.file.Fd()), filepath.Base(path), 0700); err != nil {
		return err
	}
	directory, err := OpenChild(parent, filepath.Base(path), true)
	if err != nil {
		return err
	}
	d, err := parents.Retain(directory, parent, filepath.Base(path))
	if err != nil {
		return err
	}
	if d.info.Mode() != os.ModeDir|0700 {
		return ErrInput
	}
	if err := errors.Join(parent.file.Sync(), barrier("directory-created", d.file)); err != nil {
		return err
	}
	type retainedOutput struct {
		file     *os.File
		info     os.FileInfo
		artifact Content
	}
	var outputs []retainedOutput
	defer func() {
		for _, output := range outputs {
			result = errors.Join(result, output.file.Close())
		}
	}()
	verify := func(output retainedOutput) error {
		info, err := output.file.Stat()
		if err != nil || !sameOutput(output.info, info) {
			return errors.Join(ErrInput, err)
		}
		raw := make([]byte, len(output.artifact.Data)+1)
		count, err := output.file.ReadAt(raw, 0)
		matches := (err == nil || err == io.EOF) && count == len(output.artifact.Data) && bytes.Equal(raw[:count], output.artifact.Data)
		if secret {
			clear(raw)
		}
		if !matches {
			return errors.Join(ErrInput, err)
		}
		final, err := output.file.Stat()
		if err != nil || !sameOutput(info, final) {
			return errors.Join(ErrInput, err)
		}
		linked, err := OpenChild(d, output.artifact.Name, false)
		if err != nil {
			return err
		}
		linkedInfo, err := linked.Stat()
		err = errors.Join(err, linked.Close())
		if err != nil || !sameOutput(info, linkedInfo) {
			return errors.Join(ErrInput, err)
		}
		return nil
	}
	for _, artifact := range files {
		if err := barrier("before-file", d.file); err != nil {
			return err
		}
		fd, err := syscall.Openat(int(d.file.Fd()), artifact.Name, syscall.O_RDWR|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
		if err != nil {
			return err
		}
		writer := os.NewFile(uintptr(fd), artifact.Name)
		birth, err := writer.Stat()
		if err != nil || !outputAllowed(birth, 0) {
			return errors.Join(ErrInput, err, writer.Close())
		}
		if err := barrier("file-created", writer); err != nil {
			return errors.Join(err, writer.Close())
		}
		count, writeErr := writer.Write(artifact.Data)
		if writeErr == nil && count != len(artifact.Data) {
			writeErr = io.ErrShortWrite
		}
		writeErr = errors.Join(writeErr, barrier("written", writer), writer.Sync())
		finished, statErr := writer.Stat()
		if statErr == nil && (!os.SameFile(birth, finished) || !outputAllowed(finished, int64(len(artifact.Data)))) {
			statErr = ErrInput
		}
		writeErr = errors.Join(writeErr, statErr)
		reader, readErr := OpenChild(d, artifact.Name, false)
		closeErr := writer.Close()
		if reader != nil {
			info, statErr := reader.Stat()
			if statErr != nil || finished == nil || !sameOutput(finished, info) {
				return errors.Join(ErrInput, writeErr, readErr, closeErr, statErr, reader.Close())
			}
			outputs = append(outputs, retainedOutput{reader, info, artifact})
		}
		if err := errors.Join(writeErr, readErr, closeErr, barrier("closed", d.file)); err != nil {
			return err
		}
		if err := verify(outputs[len(outputs)-1]); err != nil {
			return err
		}
	}
	inventory := func() error {
		if _, err := d.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		names, err := d.file.Readdirnames(len(files) + 1)
		if err != nil && err != io.EOF || len(names) != len(files) {
			return ErrInput
		}
		for _, name := range names {
			if !seen[name] {
				return ErrInput
			}
		}
		for _, output := range outputs {
			if err := verify(output); err != nil {
				return err
			}
		}
		return nil
	}
	if err := errors.Join(inventory(), barrier("before-directory-sync", d.file)); err != nil {
		return err
	}
	if observe != nil {
		observe("directory-sync", d.file)
	}
	if err := errors.Join(d.file.Sync(), parent.file.Sync(), barrier("synced", d.file)); err != nil {
		return err
	}
	if err := barrier("commit", d.file); err != nil {
		return err
	}
	return errors.Join(inventory(), ctx.Err())
}

func outputAllowed(info os.FileInfo, size int64) bool {
	stat := info.Sys().(*syscall.Stat_t)
	return info.Mode() == 0600 && info.Size() == size && stat.Nlink == 1 && stat.Uid == 0 && stat.Gid == 0
}

func sameOutput(original, current os.FileInfo) bool {
	if !os.SameFile(original, current) || !outputAllowed(current, original.Size()) {
		return false
	}
	before, after := original.Sys().(*syscall.Stat_t), current.Sys().(*syscall.Stat_t)
	return before.Ctim == after.Ctim && before.Mtim == after.Mtim
}
