package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

type sourceArtifactFile struct {
	name string
	raw  []byte
}

// This offline native mechanism grants no signing, Release or Installation
// authority. Composition supplies the fixed public inventory. Original input,
// parent and born file descriptors stay retained through durable readback and
// Close; residue is never removed or adopted. observe is a synchronous barrier
// around actual effects for negative controls, not an I/O replacement.
func persistSourceArtifacts(ctx context.Context, path string, files []sourceArtifactFile, check func() error, observe func(string, *os.File)) (result error) {
	return persistSourceOutput(ctx, path, files, false, check, observe)
}

// secret changes only temporary-memory retirement. It supplies no authority or
// new output namespace; each finite caller admits its own fixed purpose/files.
func persistSourceOutput(ctx context.Context, path string, files []sourceArtifactFile, secret bool, check func() error, observe func(string, *os.File)) (result error) {
	if ctx == nil || os.Geteuid() != 0 || !sourceControlPath(path) || len(files) == 0 || len(files) > 8 || check == nil {
		return errSourceControlInput
	}
	seen := make(map[string]bool, len(files))
	for _, file := range files {
		if file.name == "." || file.name == ".." || filepath.IsAbs(file.name) || filepath.Base(file.name) != file.name || seen[file.name] || len(file.raw) == 0 || len(file.raw) > 64<<20 {
			return errSourceControlInput
		}
		seen[file.name] = true
	}
	parents := &sourceControlInputs{ctx: ctx}
	defer func() { result = errors.Join(result, parents.close(), ctx.Err()) }()
	parent, err := parents.directory(filepath.Dir(path))
	if err != nil {
		return err
	}
	if parent.info.Mode() != os.ModeDir|0700 {
		return errSourceControlInput
	}
	barrier := func(label string, file *os.File) error {
		if observe != nil {
			observe(label, file)
		}
		if os.Geteuid() != 0 {
			return errSourceControlInput
		}
		for _, directory := range parents.files {
			info, err := directory.file.Stat()
			if err != nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 || info.Sys().(*syscall.Stat_t).Uid != 0 {
				return errors.Join(errSourceControlInput, err)
			}
		}
		return errors.Join(parents.check(), check(), ctx.Err())
	}
	var filesystem syscall.Statfs_t
	if err := syscall.Fstatfs(int(parent.file.Fd()), &filesystem); err != nil || filesystem.Type != 0xef53 {
		return errors.Join(errSourceControlInput, err)
	}
	if err := errors.Join(parent.file.Sync(), barrier("before-directory", parent.file)); err != nil {
		return err
	}
	if err := syscall.Mkdirat(int(parent.file.Fd()), filepath.Base(path), 0700); err != nil {
		return err
	}
	directory, err := sourceControlOpen(parent, filepath.Base(path), true)
	if err != nil {
		return err
	}
	d, err := parents.retain(directory, parent, filepath.Base(path))
	if err != nil {
		return err
	}
	if d.info.Mode() != os.ModeDir|0700 {
		return errSourceControlInput
	}
	if err := errors.Join(parent.file.Sync(), barrier("directory-created", d.file)); err != nil {
		return err
	}
	type retainedOutput struct {
		file     *os.File
		info     os.FileInfo
		artifact sourceArtifactFile
	}
	var outputs []retainedOutput
	defer func() {
		for _, output := range outputs {
			result = errors.Join(result, output.file.Close())
		}
	}()
	verify := func(output retainedOutput) error {
		info, err := output.file.Stat()
		if err != nil || !os.SameFile(output.info, info) || info.Mode() != 0600 || info.Size() != int64(len(output.artifact.raw)) {
			return errors.Join(errSourceControlInput, err)
		}
		before := info.Sys().(*syscall.Stat_t)
		original := output.info.Sys().(*syscall.Stat_t)
		if before.Uid != 0 || before.Gid != 0 || before.Nlink != 1 || before.Ctim != original.Ctim || before.Mtim != original.Mtim {
			return errSourceControlInput
		}
		raw := make([]byte, len(output.artifact.raw)+1)
		count, err := output.file.ReadAt(raw, 0)
		matches := (err == nil || err == io.EOF) && count == len(output.artifact.raw) && bytes.Equal(raw[:count], output.artifact.raw)
		if secret {
			clear(raw)
		}
		if !matches {
			return errors.Join(errSourceControlInput, err)
		}
		final, err := output.file.Stat()
		if err != nil || final.Mode() != info.Mode() || final.Sys().(*syscall.Stat_t).Ctim != before.Ctim {
			return errors.Join(errSourceControlInput, err)
		}
		linked, err := sourceControlOpen(d, output.artifact.name, false)
		if err != nil {
			return err
		}
		linkedInfo, err := linked.Stat()
		err = errors.Join(err, linked.Close())
		if err != nil || !os.SameFile(info, linkedInfo) {
			return errors.Join(errSourceControlInput, err)
		}
		return nil
	}
	for _, artifact := range files {
		if err := barrier("before-file", d.file); err != nil {
			return err
		}
		fd, err := syscall.Openat(int(d.file.Fd()), artifact.name, syscall.O_RDWR|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
		if err != nil {
			return err
		}
		writer := os.NewFile(uintptr(fd), artifact.name)
		birth, err := writer.Stat()
		if err != nil || birth.Mode() != 0600 || birth.Size() != 0 || birth.Sys().(*syscall.Stat_t).Nlink != 1 || birth.Sys().(*syscall.Stat_t).Uid != 0 || birth.Sys().(*syscall.Stat_t).Gid != 0 {
			return errors.Join(errSourceControlInput, err, writer.Close())
		}
		if err := barrier("file-created", writer); err != nil {
			return errors.Join(err, writer.Close())
		}
		count, writeErr := writer.Write(artifact.raw)
		if writeErr == nil && count != len(artifact.raw) {
			writeErr = io.ErrShortWrite
		}
		writeErr = errors.Join(writeErr, writer.Sync(), barrier("written", writer))
		reader, readErr := sourceControlOpen(d, artifact.name, false)
		closeErr := writer.Close()
		if reader != nil {
			info, statErr := reader.Stat()
			if statErr != nil || !os.SameFile(birth, info) {
				return errors.Join(errSourceControlInput, writeErr, readErr, closeErr, statErr, reader.Close())
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
			return errSourceControlInput
		}
		for _, name := range names {
			if !seen[name] {
				return errSourceControlInput
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
	if err := errors.Join(d.file.Sync(), parent.file.Sync(), barrier("synced", d.file)); err != nil {
		return err
	}
	if err := barrier("commit", d.file); err != nil {
		return err
	}
	return errors.Join(inventory(), ctx.Err())
}
