package endpoint

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
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
	observationBytes   []byte
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
func (reader *installedFiles) readObserved(ctx context.Context, filename string, maximum int64, mode os.FileMode, gid uint32, allowEmpty bool) ([]byte, error) {
	return reader.readInto(ctx, filename, maximum, mode, gid, allowEmpty, nil, nil)
}

func (reader *installedFiles) readInto(ctx context.Context, filename string, maximum int64, mode os.FileMode, gid uint32, allowEmpty bool, buffer []byte, expected *fileObservation) (body []byte, returnedErr error) {
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
	return reader.readFile(ctx, root, filename, maximum, mode, gid, allowEmpty, buffer, expected)
}

// readFile uses only the freshly opened original parent. It checks that parent
// by both descriptor and name before each leaf; its caller checks all ancestors
// and owns the root's joined close. Detached directory facts cannot replace it.
func (reader *installedFiles) readFile(ctx context.Context, root *os.Root, filename string, maximum int64, mode os.FileMode, gid uint32, allowEmpty bool, buffer []byte, expected *fileObservation) (body []byte, returnedErr error) {
	if root == nil || !canonicalPath(filename) || maximum < 1 {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parent := filepath.Dir(filename)
	original, known := reader.directories[parent]
	openedParent, fdErr := root.Stat(".")
	pathParent, pathErr := os.Lstat(parent)
	if !known || fdErr != nil || pathErr != nil || !sameObservedDirectory(original, openedParent) || !sameObservedDirectory(original, pathParent) {
		return nil, errors.Join(ErrBinding, fdErr, pathErr)
	}
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
	if expected != nil {
		if before.Size() != int64(len(expected.body)) || len(buffer) == 0 {
			return nil, ErrBinding
		}
		// Each observation reads and compares every byte; the finite scratch
		// block is not an accepting cache or a replacement for original custody.
		for offset := 0; offset < len(expected.body); {
			n := min(len(buffer), len(expected.body)-offset)
			if _, err := io.ReadFull(file, buffer[:n]); err != nil {
				return nil, errors.Join(ErrBinding, err)
			}
			if !bytes.Equal(buffer[:n], expected.body[offset:offset+n]) {
				return nil, ErrBinding
			}
			offset += n
		}
	} else {
		body = make([]byte, before.Size())
		if _, err := io.ReadFull(file, body); err != nil {
			return nil, errors.Join(ErrBinding, err)
		}
	}
	var trailing [1]byte
	if n, err := file.Read(trailing[:]); n != 0 || err != io.EOF {
		return nil, errors.Join(ErrBinding, err)
	}
	final, fileErr := file.Stat()
	pathFinal, pathErr := root.Lstat(name)
	if fileErr != nil || pathErr != nil || !sameObservedFile(before, final) || !sameObservedFile(before, pathFinal) {
		return nil, errors.Join(ErrBinding, fileErr, pathErr)
	}
	if original, known := reader.files[filename]; known {
		if !sameObservedFile(original.identity, before) || expected == nil && !bytes.Equal(original.body, body) {
			return nil, ErrBinding
		}
	} else {
		reader.files[filename] = fileObservation{identity: before, body: bytes.Clone(body), gid: gid, mode: mode}
	}
	return body, ctx.Err()
}

func (reader *installedFiles) observeDirectories(ctx context.Context) error {
	for directory, original := range reader.directories {
		current, err := os.Lstat(directory)
		if err != nil || !sameObservedDirectory(original, current) {
			return errors.Join(ErrBinding, err)
		}
	}
	return ctx.Err()
}

func (reader *installedFiles) observe(ctx context.Context) (returnedErr error) {
	if err := reader.observeDirectories(ctx); err != nil {
		return err
	}
	// Roots are fresh custody for this one observation only. Every file is
	// still fully read and checked, and all original ancestors are reobserved
	// after the reads and joined closes before success can escape.
	roots := make(map[string]*os.Root)
	defer func() {
		for _, root := range roots {
			returnedErr = errors.Join(returnedErr, root.Close())
		}
		returnedErr = errors.Join(returnedErr, reader.observeDirectories(ctx))
	}()
	names := make([]string, 0, len(reader.files))
	for filename := range reader.files {
		if !canonicalPath(filename) {
			return ErrInput
		}
		parent := filepath.Dir(filename)
		root := roots[parent]
		if root == nil {
			var err error
			root, err = os.OpenRoot(parent)
			if err != nil {
				return err
			}
			roots[parent] = root
		}
		names = append(names, filename)
	}
	sort.Slice(names, func(i, j int) bool { return len(reader.files[names[i]].body) > len(reader.files[names[j]].body) })
	if reader.observationBytes == nil {
		reader.observationBytes = make([]byte, 2*(256<<10))
	}
	// The file map and fresh parent roots are immutable throughout these two
	// bounded readers. Each owns its scratch block and exact leaf descriptor;
	// all readers join before the deferred parent closes and final observations.
	failures := make([]error, len(names))
	var joined sync.WaitGroup
	for worker := range 2 {
		joined.Go(func() {
			buffer := reader.observationBytes[worker*(256<<10) : (worker+1)*(256<<10)]
			for index := worker; index < len(names); index += 2 {
				filename := names[index]
				expected := reader.files[filename]
				maximum := max(int64(len(expected.body)), 1)
				_, failures[index] = reader.readFile(ctx, roots[filepath.Dir(filename)], filename, maximum, expected.mode, expected.gid, len(expected.body) == 0, buffer, &expected)
			}
		})
	}
	joined.Wait()
	if err := errors.Join(failures...); err != nil {
		return err
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
