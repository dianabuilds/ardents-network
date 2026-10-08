package installation

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
	files              map[string]stagedFile
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
	if original, known := reader.directories[directory]; known && !sameStagingDirectory(original, info) {
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
			if !sameStagingDirectory(original, info) {
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
	if err != nil || !sameReadIdentity(before, opened) {
		return nil, errors.Join(ErrBinding, err)
	}
	body, err = io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(body)) != before.Size() {
		return nil, errors.Join(ErrBinding, err)
	}
	final, fileErr := file.Stat()
	pathFinal, pathErr := root.Lstat(name)
	if fileErr != nil || pathErr != nil || !sameReadIdentity(before, final) || !sameReadIdentity(before, pathFinal) {
		return nil, errors.Join(ErrBinding, fileErr, pathErr)
	}
	if original, known := reader.files[filename]; known {
		if !sameReadIdentity(original.identity, before) || !bytes.Equal(original.body, body) {
			return nil, ErrBinding
		}
	} else {
		reader.files[filename] = stagedFile{identity: before, body: bytes.Clone(body), gid: gid, mode: mode}
	}
	return body, ctx.Err()
}

func sameReadIdentity(before, after os.FileInfo) bool {
	if before == nil || after == nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return false
	}
	a, aOK := before.Sys().(*syscall.Stat_t)
	b, bOK := after.Sys().(*syscall.Stat_t)
	return aOK && bOK && a.Uid == b.Uid && a.Gid == b.Gid && a.Nlink == 1 && b.Nlink == 1 && a.Ctim == b.Ctim
}

func (reader *installedFiles) observe(ctx context.Context) error {
	for directory, original := range reader.directories {
		current, err := os.Lstat(directory)
		if err != nil || !sameStagingDirectory(original, current) {
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

func (reader *installedFiles) observeAccountAndRoots(checked inspectedGeneration) error {
	uid, gid, err := observeEndpointAccount()
	if err != nil || uid != checked.binding.UID || gid != checked.binding.GID {
		return errors.Join(ErrBinding, err)
	}
	for _, root := range checked.binding.MutableRoots {
		info, err := os.Lstat(root.Path)
		if err != nil || !managedDirectory(info, uid, gid) {
			return errors.Join(ErrBinding, err)
		}
		native := info.Sys().(*syscall.Stat_t)
		if uint64(native.Dev) != root.Device || native.Ino != root.Inode {
			return ErrBinding
		}
		if err := reader.retainMutableDirectory(root.Path, info); err != nil {
			return err
		}
		for parent := filepath.Dir(root.Path); ; parent = filepath.Dir(parent) {
			info, err := os.Lstat(parent)
			if err != nil || info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
				return errors.Join(ErrBinding, err)
			}
			native, ok := info.Sys().(*syscall.Stat_t)
			if !ok || (native.Uid != 0 && native.Uid != uid) {
				return ErrBinding
			}
			if err := reader.retainMutableDirectory(parent, info); err != nil {
				return err
			}
			if filepath.Dir(parent) == parent {
				break
			}
		}
	}
	return nil
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

func (reader *installedFiles) inspectFixedResources(ctx context.Context, checked inspectedGeneration) error {
	digests := make(map[string]string)
	for filename, name := range fixedResourceNames() {
		mode := os.FileMode(0644)
		if name == "ardents-text-linux-amd64" {
			mode = 0555
		}
		if name == "ardents-endpoint.service" {
			name = "endpoint-unit.service"
		}
		body, err := reader.read(ctx, filename, 64<<20, mode, 0)
		if err != nil || !bytes.Equal(body, checked.files[name]) {
			return errors.Join(ErrBinding, err)
		}
		if name != "endpoint-unit.service" && name != "ardents-text.conf" {
			digests[filename] = digestHex(body)
		}
	}
	manifest, err := canonicalJSON(struct {
		Schema string            `json:"schema"`
		Files  map[string]string `json:"files"`
	}{"ardents-text-worker-artifact-v1", digests})
	if err != nil {
		return err
	}
	actual, err := reader.read(ctx, "/etc/ardents/text-worker-artifact.json", 64<<10, 0644, 0)
	if err != nil || !bytes.Equal(actual, manifest) {
		return errors.Join(ErrBinding, err)
	}
	return ctx.Err()
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
			if !sameReadIdentity(original.identity, info) || !bytes.Equal(original.body, body) {
				return nil, nil, ErrBinding
			}
		} else {
			reader.files[filename] = stagedFile{identity: info, body: bytes.Clone(body), gid: reader.gid, mode: info.Mode()}
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
