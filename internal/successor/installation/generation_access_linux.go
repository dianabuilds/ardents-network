package installation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// Service traversal does not release the writer or grant installed startup.
// The admitted operation supplies its actual NSS-observed group and rechecks
// original roots and journals before and after both directory changes.
func (owned *initialPreparation) promoteGenerationAccess(ctx context.Context, request Request) error {
	if err := owned.observe(ctx, request); err != nil {
		return err
	}
	if err := owned.stage.promoteAccess(ctx, owned.prepared.gid); err != nil {
		return err
	}
	return owned.observe(ctx, request)
}

func (stage *generationStage) promoteAccess(ctx context.Context, gid uint32) error {
	if ctx == nil || gid == 0 || gid == 1<<32-1 {
		return ErrInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	native, ok := stage.generation.identity.Sys().(*syscall.Stat_t)
	if !ok || native.Gid != gid {
		return ErrBinding
	}
	if _, complete := stage.journal.files["0004.json"]; !complete {
		return ErrBinding
	}
	var generations *stagingDirectory
	for _, directory := range stage.directories {
		if directory.parent == stage.lease.root && directory.name == "generations" {
			generations = directory
		}
	}
	if generations == nil {
		return ErrBinding
	}
	info, err := promoteReadDirectory(ctx, filepath.Join(stage.lease.path, "generations"), generations.root, generations.identity, gid)
	if info != nil {
		generations.identity = info
	}
	if err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	info, err = promoteReadDirectory(ctx, stage.lease.path, stage.lease.root, stage.lease.identity, gid)
	if info != nil {
		stage.lease.identity = info
	}
	if err != nil {
		return err
	}
	if err := stage.observe(); err != nil {
		return err
	}
	return ctx.Err()
}

func promoteReadDirectory(ctx context.Context, path string, root *os.Root, original os.FileInfo, gid uint32) (result os.FileInfo, returnedErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pathInfo, pathErr := os.Lstat(path)
	rootInfo, rootErr := root.Stat(".")
	if pathErr != nil || rootErr != nil || !sameStagingDirectory(original, pathInfo) || !sameStagingDirectory(original, rootInfo) {
		return nil, errors.Join(ErrBinding, pathErr, rootErr)
	}
	native, ok := original.Sys().(*syscall.Stat_t)
	if !ok || gid == 0 || gid == 1<<32-1 {
		return nil, ErrBinding
	}
	if original.Mode() == os.ModeDir|0750 && native.Gid == gid {
		return original, ctx.Err()
	}
	if original.Mode() != os.ModeDir|0700 || native.Gid != 0 {
		return nil, ErrBinding
	}
	file, err := root.OpenFile(".", os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	before, err := file.Stat()
	if err != nil || !sameStagingDirectory(original, before) {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := file.Chown(0, int(gid)); err != nil {
		return nil, err
	}
	if err := file.Chmod(0750); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !os.SameFile(original, info) || info.Mode() != os.ModeDir|0750 {
		return nil, errors.Join(ErrBinding, err)
	}
	actual, ok := info.Sys().(*syscall.Stat_t)
	if !ok || actual.Uid != 0 || actual.Gid != gid {
		return nil, ErrBinding
	}
	pathInfo, pathErr = os.Lstat(path)
	rootInfo, rootErr = root.Stat(".")
	if pathErr != nil || rootErr != nil || !sameStagingDirectory(info, pathInfo) || !sameStagingDirectory(info, rootInfo) {
		return nil, errors.Join(ErrBinding, pathErr, rootErr)
	}
	// Preserve the changed same inode even if the original caller goes after
	// durability. Its owner returns refusal and retains explicit failure intent.
	return info, ctx.Err()
}
