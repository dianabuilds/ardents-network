package endpoint

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/generation"
)

// installedFiles retains native file/directory provenance for one serialized
// read operation. It owns no writer lease and supplies no startup authority.
// Root inspection and service startup each construct their OWN observations.
type installedFiles struct {
	directory          string
	gid                uint32
	files              map[string]fileObservation
	directories        map[string]os.FileInfo
	mutableDirectories map[string]os.FileInfo
}

func (reader *installedFiles) pinGenerationDirectory(directory string) error {
	info, err := os.Lstat(directory)
	if err != nil || info == nil {
		return errors.Join(ErrBinding, err)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid != reader.gid || info.Mode() != os.ModeDir|0750 {
		return ErrBinding
	}
	if original, known := reader.directories[directory]; known && !sameObservedDirectory(original, info) {
		return ErrBinding
	}
	reader.directories[directory] = info
	return nil
}

func (reader *installedFiles) read(ctx context.Context, filename string, maximum int64, mode os.FileMode, gid uint32) (body []byte, returnedErr error) {
	return reader.readObserved(ctx, filename, maximum, mode, gid, false)
}

// Recovery may observe an empty or partial owned birth. Ordinary inspection
// still requires complete nonempty bytes and never repairs a file.
func (reader *installedFiles) readObserved(ctx context.Context, filename string, maximum int64, mode os.FileMode, gid uint32, allowEmpty bool) (body []byte, returnedErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !canonicalPath(filename) || maximum < 1 {
		return nil, ErrInput
	}
	for directory := filepath.Dir(filename); ; directory = filepath.Dir(directory) {
		info, err := os.Lstat(directory)
		if err != nil {
			return nil, err
		}
		if original, known := reader.directories[directory]; known {
			if !sameObservedDirectory(original, info) {
				return nil, ErrBinding
			}
		} else {
			if !rootDirectory(info) {
				return nil, ErrBinding
			}
			reader.directories[directory] = info
		}
		if filepath.Dir(directory) == directory {
			break
		}
	}
	root, err := os.OpenRoot(filepath.Dir(filename))
	if err != nil {
		return nil, err
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, root.Close(), ctx.Err())
		if returnedErr != nil {
			body = nil
		}
	}()
	name := filepath.Base(filename)
	before, err := root.Lstat(name)
	if err != nil || before == nil || !before.Mode().IsRegular() || before.Mode() != mode || before.Size() < 0 || (!allowEmpty && before.Size() == 0) || before.Size() > maximum {
		return nil, errors.Join(ErrBinding, err)
	}
	native, ok := before.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid != gid || native.Nlink != 1 {
		return nil, ErrBinding
	}
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, file.Close())
		if returnedErr != nil {
			body = nil
		}
	}()
	opened, err := file.Stat()
	if err != nil || !sameObservedFile(before, opened) {
		return nil, errors.Join(ErrBinding, err)
	}
	body, err = io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(body)) != before.Size() {
		return nil, errors.Join(ErrBinding, err)
	}
	final, fileErr := file.Stat()
	pathFinal, pathErr := root.Lstat(name)
	if fileErr != nil || pathErr != nil || !sameObservedFile(before, final) || !sameObservedFile(before, pathFinal) {
		return nil, errors.Join(ErrBinding, fileErr, pathErr)
	}
	if original, known := reader.files[filename]; known {
		if !sameObservedFile(original.identity, before) || !bytes.Equal(original.body, body) {
			return nil, ErrBinding
		}
	} else {
		reader.files[filename] = fileObservation{identity: before, body: bytes.Clone(body), gid: gid, mode: mode}
	}
	return body, ctx.Err()
}

func (reader *installedFiles) observe(ctx context.Context) error {
	for directory, original := range reader.directories {
		current, err := os.Lstat(directory)
		if err != nil || !sameObservedDirectory(original, current) {
			return errors.Join(ErrBinding, err)
		}
	}
	for filename, expected := range reader.files {
		maximum := int64(len(expected.body))
		if maximum == 0 {
			maximum = 1
		}
		if _, err := reader.readObserved(ctx, filename, maximum, expected.mode, expected.gid, len(expected.body) == 0); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (reader *installedFiles) retainMutableDirectory(name string, info os.FileInfo) error {
	if original, known := reader.mutableDirectories[name]; known {
		if !os.SameFile(original, info) || original.Mode() != info.Mode() {
			return ErrBinding
		}
		a := original.Sys().(*syscall.Stat_t)
		b := info.Sys().(*syscall.Stat_t)
		if a.Uid != b.Uid || a.Gid != b.Gid {
			return ErrBinding
		}
	} else {
		reader.mutableDirectories[name] = info
	}
	return nil
}

// A sealed generation has independent read custody. The root inspection keeps
// its own lease and detached original file observations for subsequent checks.
func (reader *installedFiles) readSealedGeneration(ctx context.Context, digest string) (binding []byte, files map[string][]byte, returnedErr error) {
	parent := filepath.Join(reader.directory, "generations")
	directory := filepath.Join(parent, digest)
	snapshot, err := generation.OpenSnapshot(ctx, parent, reader.directories[parent], reader.directories[directory], digest, reader.gid)
	if err != nil {
		return nil, nil, errors.Join(ErrBinding, err)
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, snapshot.Close(), ctx.Err())
		if returnedErr != nil {
			binding = nil
			files = nil
		}
	}()
	files = make(map[string][]byte)
	for _, name := range generation.Names() {
		filename := filepath.Join(directory, name)
		info, err := os.Lstat(filename)
		body := snapshot.Bytes(name)
		if err != nil || !snapshot.Matches(name, info) || len(body) == 0 {
			return nil, nil, errors.Join(ErrBinding, err)
		}
		if original, known := reader.files[filename]; known {
			if !sameObservedFile(original.identity, info) || !bytes.Equal(original.body, body) {
				return nil, nil, ErrBinding
			}
		} else {
			reader.files[filename] = fileObservation{identity: info, body: bytes.Clone(body), gid: reader.gid, mode: info.Mode()}
		}
		if name == "binding.json" {
			binding = body
		} else {
			files[name] = body
		}
	}
	if err := snapshot.Observe(); err != nil {
		return nil, nil, errors.Join(ErrBinding, err)
	}
	return binding, files, ctx.Err()
}

// Recovery callers supply the exact required/optional roster. This bounded
// native read grants no continuation, mutation or journal authority.
func recoveryInventory(directory string, allowed map[string]bool) (returnedErr error) {
	info, err := os.Lstat(directory)
	if err != nil || info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return errors.Join(ErrBinding, err)
	}
	file, err := os.OpenFile(directory, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return errors.Join(ErrBinding, err)
	}
	names, err := file.Readdirnames(len(allowed) + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	seen := make(map[string]bool)
	for _, name := range names {
		if _, known := allowed[name]; !known {
			return ErrBinding
		}
		seen[name] = true
	}
	for name, required := range allowed {
		if required && !seen[name] {
			return ErrBinding
		}
	}
	return nil
}
