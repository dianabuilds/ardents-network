package installation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

type installedInspection struct {
	lease *installationLease
	*installedFiles
}

// Read custody is independent of creation custody. Retain the candidate's
// original sealed files before manager start, under this same writer lease;
// a fresh path or copied bytes cannot replace the stage's original directory.
func (reader *installedInspection) retainStagedGeneration(ctx context.Context, stage *installationTransaction) (binding []byte, files map[string][]byte, returnedErr error) {
	if ctx == nil || reader == nil || reader.installedFiles == nil || reader.lease == nil ||
		stage == nil || stage.lease != reader.lease || stage.generation == nil && stage.sealed == nil || !canonicalDigest(stage.selected.GenerationDigest) {
		return nil, nil, ErrBinding
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, ctx.Err())
		if returnedErr != nil {
			binding, files = nil, nil
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if err := stage.observe(); err != nil {
		return nil, nil, err
	}
	parent := filepath.Join(reader.lease.path, "generations")
	directory := filepath.Join(parent, stage.selected.GenerationDigest)
	if err := reader.pinGenerationDirectory(parent); err != nil {
		return nil, nil, err
	}
	info, err := os.Lstat(directory)
	if err != nil || info == nil {
		return nil, nil, errors.Join(ErrBinding, err)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	original := stage.generationIdentity()
	if !ok || uint64(native.Dev) != original.Device || native.Ino != original.Inode ||
		native.Uid != 0 || native.Gid != original.GID || info.Mode() != original.Mode {
		return nil, nil, ErrBinding
	}
	if err := reader.pinGenerationDirectory(directory); err != nil {
		return nil, nil, err
	}
	binding, files, err = reader.readSealedGeneration(ctx, stage.selected.GenerationDigest)
	if err != nil {
		return nil, nil, err
	}
	if !bytes.Equal(binding, stage.generationBytes("binding.json")) {
		return nil, nil, ErrBinding
	}
	for name, body := range files {
		if !bytes.Equal(body, stage.generationBytes(name)) {
			return nil, nil, ErrBinding
		}
	}
	return binding, files, errors.Join(stage.observe(), reader.observe(ctx))
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
	checked, err := reader.inspectSelected(ctx)
	if err != nil {
		return CheckResult{}, err
	}
	role := checked.request.Headless.Role
	if role == "" {
		role = "publisher"
	}
	return CheckResult{Status: "local-integrity-verified", GenerationDigest: checked.selected.GenerationDigest, Role: role}, ctx.Err()
}

// inspectSelected retains observations under the caller's original writer lease.
// A transition can keep this same reader through fresh Release authentication;
// the returned local bytes and binding cannot recreate private authorization.
func (reader *installedInspection) inspectSelected(ctx context.Context) (inspectedGeneration, error) {
	if ctx == nil || reader == nil || reader.lease == nil {
		return inspectedGeneration{}, ErrInput
	}
	if err := reader.lease.observe(); err != nil {
		return inspectedGeneration{}, err
	}
	directory := reader.lease.path
	for _, name := range []string{"transition.json", "start-guard.json", "preparation/failure.json"} {
		if _, err := reader.lease.root.Lstat(name); !os.IsNotExist(err) {
			return inspectedGeneration{}, errors.Join(ErrBinding, err)
		}
	}
	selectedRaw, err := reader.read(ctx, filepath.Join(directory, "selection.json"), 4<<10, 0640, reader.gid)
	if err != nil {
		return inspectedGeneration{}, err
	}
	var selected generationSelection
	if decodeCanonical(selectedRaw, 4<<10, &selected) != nil || !canonicalDigest(selected.GenerationDigest) {
		return inspectedGeneration{}, ErrBinding
	}
	generationPath := filepath.Join(directory, "generations", selected.GenerationDigest)
	for _, parent := range []string{filepath.Join(directory, "generations"), generationPath} {
		if err := reader.pinGenerationDirectory(parent); err != nil {
			return inspectedGeneration{}, err
		}
	}
	if _, err := reader.lease.root.Lstat(filepath.Join("journals", selected.GenerationDigest, "original-transition-failure.json")); !os.IsNotExist(err) {
		return inspectedGeneration{}, errors.Join(ErrBinding, err)
	}
	if _, err := reader.lease.root.Lstat(filepath.Join("journals", selected.GenerationDigest, "recovery-failure.json")); !os.IsNotExist(err) {
		return inspectedGeneration{}, errors.Join(ErrBinding, err)
	}
	bindingRaw, files, err := reader.readSealedGeneration(ctx, selected.GenerationDigest)
	if err != nil {
		return inspectedGeneration{}, err
	}
	checked, err := inspectGeneration(directory, selectedRaw, bindingRaw, files)
	if err != nil || checked.binding.GID != reader.gid {
		return inspectedGeneration{}, errors.Join(ErrBinding, err)
	}
	if err := reader.observeAccountAndRoots(checked); err != nil {
		return inspectedGeneration{}, err
	}
	if err := reader.inspectFixedResources(ctx, checked); err != nil {
		return inspectedGeneration{}, err
	}
	if err := reader.observe(ctx); err != nil {
		return inspectedGeneration{}, err
	}
	if err := reader.observeAccountAndRoots(checked); err != nil {
		return inspectedGeneration{}, err
	}
	return checked, ctx.Err()
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
	return &installedInspection{lease: lease, installedFiles: &installedFiles{directory: directory, gid: native.Gid, files: make(map[string]stagedFile), directories: ancestors, mutableDirectories: make(map[string]os.FileInfo)}}, nil
}

func (reader *installedInspection) observe(ctx context.Context) error {
	if reader == nil || reader.lease == nil || reader.installedFiles == nil {
		return ErrInput
	}
	if err := reader.lease.observe(); err != nil {
		return err
	}
	if err := reader.installedFiles.observe(ctx); err != nil {
		return err
	}
	return errors.Join(reader.lease.observe(), ctx.Err())
}

// Retained private records stay under the same Installation writer and original
// inode observations. Durability or visibility never supplies recovery authority.
func (reader *installedInspection) syncObserved(ctx context.Context, filename string) (returnedErr error) {
	observed, exists := reader.files[filename]
	if !exists {
		return ErrBinding
	}
	if _, err := reader.readObserved(ctx, filename, int64(len(observed.body))+1, observed.mode, observed.gid, len(observed.body) == 0); err != nil {
		return err
	}
	file, err := os.OpenFile(filename, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close(), ctx.Err()) }()
	info, err := file.Stat()
	if err != nil || !sameReadIdentity(observed.identity, info) {
		return errors.Join(ErrBinding, err)
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return syncDirectDirectory(filepath.Dir(filename))
}

func (reader *installedInspection) writePrivate(ctx context.Context, filename string, body []byte) (returnedErr error) {
	if existing, retained := reader.files[filename]; retained {
		if !bytes.Equal(existing.body, body) {
			return ErrBinding
		}
		return reader.syncObserved(ctx, filename)
	}
	if err := reader.lease.observe(); err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(filename))
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, root.Close()) }()
	info, err := root.Stat(".")
	if err != nil || !sameStagingDirectory(reader.directories[filepath.Dir(filename)], info) || !privateJournalDirectory(info) {
		return errors.Join(ErrBinding, err)
	}
	created, err := writeStagedFile(ctx, root, filepath.Base(filename), body, 0600, 0)
	if created.identity != nil {
		reader.files[filename] = created
	}
	return errors.Join(err, syncStagingRoot(root), ctx.Err())
}

func (reader *installedInspection) removeObserved(ctx context.Context, filename string) (returnedErr error) {
	if err := reader.syncObserved(ctx, filename); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(filename))
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, root.Close(), ctx.Err()) }()
	info, err := root.Lstat(filepath.Base(filename))
	if err != nil || !sameReadIdentity(reader.files[filename].identity, info) {
		return errors.Join(ErrBinding, err)
	}
	if err := root.Remove(filepath.Base(filename)); err != nil {
		return err
	}
	delete(reader.files, filename)
	return syncStagingRoot(root)
}
