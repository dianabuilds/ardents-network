package installation

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	manageddirectory "github.com/dianabuilds/ardents-network/internal/successor/installation/directory"
)

// Read custody is independent of creation custody. Retain the candidate's
// original sealed files before manager start, under this same writer lease;
// a fresh path or copied bytes cannot replace the stage's original directory.
func (reader *installedRoot) retainStagedGeneration(ctx context.Context, stage *installationTransaction) (binding []byte, files map[string][]byte, returnedErr error) {
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
	reader, err := openInstalledRoot(ctx, directory)
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
func (reader *installedRoot) inspectSelected(ctx context.Context) (inspectedGeneration, error) {
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

// Shared installed-integrity policy consumes each caller's independent read
// observations. Account and bound resource checks grant no lease or authority.
func (reader *installedFiles) observeAccountAndRoots(checked inspectedGeneration) error {
	uid, gid, err := observeEndpointAccount()
	if err != nil || uid != checked.binding.UID || gid != checked.binding.GID {
		return errors.Join(ErrBinding, err)
	}
	for _, root := range checked.binding.MutableRoots {
		info, err := os.Lstat(root.Path)
		if err != nil || !manageddirectory.Matches(info, uid, gid) {
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
