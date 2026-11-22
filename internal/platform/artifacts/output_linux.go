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

type outputAccess uint8

const (
	protectedOutput outputAccess = iota
	callerOwnedOutput
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
	return publish(ctx, path, files, 8, false, protectedOutput, check, observe)
}

// secret changes only temporary-memory retirement. It supplies no authority or
// new output namespace; each finite caller admits its own fixed purpose/files.
// PublishPrivate publishes the selected artifacts and clears verification buffers.
func PublishPrivate(ctx context.Context, path string, files []Content, check func() error, observe func(string, *os.File)) error {
	return publish(ctx, path, files, 8, true, protectedOutput, check, observe)
}

// PublishInventory admits the larger bounded Network inventory. The caller
// selects its file names and per-file bounds before borrowing the writer.
// Sensitive inventories clear all owned readback copies.
func PublishInventory(ctx context.Context, path string, files []Content, sensitive bool, check func() error, observe func(string, *os.File)) error {
	return publish(ctx, path, files, 129, sensitive, protectedOutput, check, observe)
}

// PublishUnsigned publishes one public development artifact below a private
// directory owned by the calling user. It retains the unsigned command's
// caller custody and does not admit a protected signing inventory.
func PublishUnsigned(ctx context.Context, path string, file Content, observe func(string, *os.File)) error {
	return publish(ctx, path, []Content{file}, 1, false, callerOwnedOutput, func() error { return nil }, observe)
}

func publish(ctx context.Context, path string, files []Content, maximum int, secret bool, access outputAccess, check func() error, observe func(string, *os.File)) (result error) {
	owner := os.Geteuid()
	if ctx == nil || access == protectedOutput && owner != 0 || !AbsolutePath(path) || len(files) == 0 || len(files) > maximum || check == nil {
		return ErrInput
	}
	if err := ctx.Err(); err != nil {
		return err
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
	openParent := parents.Directory
	if access == callerOwnedOutput {
		openParent = parents.openDirectory
	}
	parent, err := openParent(filepath.Dir(path))
	if err != nil {
		return err
	}
	if !access.privateDirectory(parent.info, owner) {
		return ErrInput
	}
	barrier := func(label string, file *os.File) error {
		if observe != nil {
			observe(label, file)
		}
		if os.Geteuid() != owner {
			return ErrInput
		}
		if access == protectedOutput {
			for _, directory := range parents.files {
				info, err := directory.file.Stat()
				if err != nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 || info.Sys().(*syscall.Stat_t).Uid != 0 {
					return errors.Join(ErrInput, err)
				}
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
	if !access.privateDirectory(d.info, owner) {
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
		if err != nil || !access.sameOutput(output.info, info, owner) {
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
		if err != nil || !access.sameOutput(info, final, owner) {
			return errors.Join(ErrInput, err)
		}
		linked, err := OpenChild(d, output.artifact.Name, false)
		if err != nil {
			return err
		}
		linkedInfo, err := linked.Stat()
		err = errors.Join(err, linked.Close())
		if err != nil || !access.sameOutput(info, linkedInfo, owner) {
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
		if err != nil || !access.fileAllowed(birth, 0, owner) {
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
		if statErr == nil && (!os.SameFile(birth, finished) || !access.fileAllowed(finished, int64(len(artifact.Data)), owner)) {
			statErr = ErrInput
		}
		writeErr = errors.Join(writeErr, statErr)
		reader, readErr := OpenChild(d, artifact.Name, false)
		closeErr := writer.Close()
		if reader != nil {
			info, statErr := reader.Stat()
			if statErr != nil || finished == nil || !access.sameOutput(finished, info, owner) {
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
		if access == callerOwnedOutput {
			if err := barrier("readback", d.file); err != nil {
				return err
			}
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

func (access outputAccess) privateDirectory(info os.FileInfo, owner int) bool {
	if access == protectedOutput {
		return info.Mode() == os.ModeDir|0700
	}
	return info.IsDir() && info.Mode().Perm() == 0700 && info.Sys().(*syscall.Stat_t).Uid == uint32(owner)
}

func (access outputAccess) fileAllowed(info os.FileInfo, size int64, owner int) bool {
	stat := info.Sys().(*syscall.Stat_t)
	if info.Size() != size || stat.Nlink != 1 || stat.Uid != uint32(owner) {
		return false
	}
	if access == protectedOutput {
		return info.Mode() == 0600 && stat.Gid == 0
	}
	return info.Mode().IsRegular() && info.Mode().Perm() == 0600
}

func (access outputAccess) sameOutput(original, current os.FileInfo, owner int) bool {
	if !os.SameFile(original, current) || !access.fileAllowed(current, original.Size(), owner) {
		return false
	}
	before, after := original.Sys().(*syscall.Stat_t), current.Sys().(*syscall.Stat_t)
	return before.Ctim == after.Ctim && before.Mtim == after.Mtim
}
