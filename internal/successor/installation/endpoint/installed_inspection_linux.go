package endpoint

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	manageddirectory "github.com/dianabuilds/ardents-network/internal/successor/installation/directory"
)

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
			continue
		}
		body, err := reader.read(ctx, filename, 64<<20, mode, 0)
		if err != nil || !bytes.Equal(body, checked.files[name]) {
			return errors.Join(ErrBinding, err)
		}
		if name != "ardents-text.conf" {
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
