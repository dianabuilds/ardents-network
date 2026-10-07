package installation

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

type installedInspection struct {
	lease              *installationLease
	gid                uint32
	files              map[string]stagedFile
	directories        map[string]os.FileInfo
	mutableDirectories map[string]os.FileInfo
}

func checkInstalled(ctx context.Context, directory string) (result CheckResult, returnedErr error) {
	if err := observeInstallationPlatform(ctx); err != nil {
		return CheckResult{}, err
	}
	reader, err := openInstalledInspection(ctx, directory)
	if err != nil {
		return CheckResult{}, err
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, reader.lease.close(), ctx.Err())
		if returnedErr != nil {
			result = CheckResult{}
		}
	}()
	for _, name := range []string{"transition.json", "start-guard.json", "preparation/failure.json"} {
		if _, err := reader.lease.root.Lstat(name); !os.IsNotExist(err) {
			return CheckResult{}, errors.Join(ErrBinding, err)
		}
	}
	selectedRaw, err := reader.read(ctx, filepath.Join(directory, "selection.json"), 4<<10, 0640, reader.gid)
	if err != nil {
		return CheckResult{}, err
	}
	var selected generationSelection
	if decodeCanonical(selectedRaw, 4<<10, &selected) != nil || !canonicalDigest(selected.GenerationDigest) {
		return CheckResult{}, ErrBinding
	}
	generationPath := filepath.Join(directory, "generations", selected.GenerationDigest)
	for _, parent := range []string{filepath.Join(directory, "generations"), generationPath} {
		if err := reader.pinGenerationDirectory(parent); err != nil {
			return CheckResult{}, err
		}
	}
	if _, err := reader.lease.root.Lstat(filepath.Join("journals", selected.GenerationDigest, "original-transition-failure.json")); !os.IsNotExist(err) {
		return CheckResult{}, errors.Join(ErrBinding, err)
	}
	if _, err := reader.lease.root.Lstat(filepath.Join("journals", selected.GenerationDigest, "recovery-failure.json")); !os.IsNotExist(err) {
		return CheckResult{}, errors.Join(ErrBinding, err)
	}
	bindingRaw, err := reader.read(ctx, filepath.Join(generationPath, "binding.json"), 32<<10, 0640, reader.gid)
	if err != nil {
		return CheckResult{}, err
	}
	var binding generationBinding
	if decodeCanonical(bindingRaw, 32<<10, &binding) != nil || len(binding.Files) != 14 {
		return CheckResult{}, ErrBinding
	}
	files := make(map[string][]byte, len(binding.Files))
	for name := range binding.Files {
		if filepath.Base(name) != name || name == "." || name == ".." {
			return CheckResult{}, ErrBinding
		}
		mode, maximum := os.FileMode(0640), int64(64<<10)
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			mode, maximum = 0555, 64<<20
		}
		body, err := reader.read(ctx, filepath.Join(generationPath, name), maximum, mode, reader.gid)
		if err != nil {
			return CheckResult{}, err
		}
		files[name] = body
	}
	checked, err := inspectGeneration(directory, selectedRaw, bindingRaw, files)
	if err != nil || checked.binding.GID != reader.gid {
		return CheckResult{}, errors.Join(ErrBinding, err)
	}
	if err := reader.observeAccountAndRoots(checked); err != nil {
		return CheckResult{}, err
	}
	if err := reader.inspectFixedResources(ctx, checked); err != nil {
		return CheckResult{}, err
	}
	if err := reader.observe(ctx); err != nil {
		return CheckResult{}, err
	}
	if err := reader.observeAccountAndRoots(checked); err != nil {
		return CheckResult{}, err
	}
	role := checked.request.Headless.Role
	if role == "" {
		role = "publisher"
	}
	return CheckResult{Status: "local-integrity-verified", GenerationDigest: selected.GenerationDigest, Role: role}, ctx.Err()
}

func openInstalledInspection(ctx context.Context, directory string) (result *installedInspection, returnedErr error) {
	if !canonicalPath(directory) || directory == "/" {
		return nil, ErrInput
	}
	ancestors, err := rootDirectoryAncestors(filepath.Dir(directory))
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(directory)
	if err != nil || info == nil {
		return nil, errors.Join(ErrBinding, err)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid == 0 || info.Mode() != os.ModeDir|0750 {
		return nil, ErrBinding
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	lease := &installationLease{root: root, path: directory, identity: info}
	defer func() {
		if returnedErr != nil {
			returnedErr = errors.Join(returnedErr, lease.close())
		}
	}()
	lease.writer, err = root.OpenFile("writer.lock", os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	if err := lease.observe(); err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lease.writer.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, err
	}
	if err := lease.observe(); err != nil {
		return nil, err
	}
	ancestors[directory] = info
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &installedInspection{lease: lease, gid: native.Gid, files: make(map[string]stagedFile), directories: ancestors, mutableDirectories: make(map[string]os.FileInfo)}, nil
}

func (reader *installedInspection) pinGenerationDirectory(directory string) error {
	info, err := os.Lstat(directory)
	if err != nil || info == nil {
		return errors.Join(ErrBinding, err)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid != reader.gid || info.Mode() != os.ModeDir|0750 {
		return ErrBinding
	}
	reader.directories[directory] = info
	return nil
}

func (reader *installedInspection) read(ctx context.Context, filename string, maximum int64, mode os.FileMode, gid uint32) (body []byte, returnedErr error) {
	return reader.readObserved(ctx, filename, maximum, mode, gid, false)
}

// Recovery may observe an empty or partial owned birth. Ordinary inspection
// still requires complete nonempty bytes and never repairs a file.
func (reader *installedInspection) readObserved(ctx context.Context, filename string, maximum int64, mode os.FileMode, gid uint32, allowEmpty bool) (body []byte, returnedErr error) {
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

func (reader *installedInspection) observe(ctx context.Context) error {
	if err := reader.lease.observe(); err != nil {
		return err
	}
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

func (reader *installedInspection) observeAccountAndRoots(checked inspectedGeneration) error {
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

func (reader *installedInspection) retainMutableDirectory(name string, info os.FileInfo) error {
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

func (reader *installedInspection) inspectFixedResources(ctx context.Context, checked inspectedGeneration) error {
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
