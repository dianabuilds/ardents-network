package installation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func managedDirectory(info os.FileInfo, uid, gid uint32) bool {
	if info == nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && uid != 0 && gid != 0 && native.Uid == uid && native.Gid == gid
}

// Only directories born in this operation can be reused by a sibling path.
// Successful chown is not an adoption or ownership record for another inode.
func createManagedDirectory(ctx context.Context, path string, uid, gid uint32, created map[string]os.FileInfo) error {
	if ctx == nil || !canonicalPath(path) || path == "/" || uid == 0 || gid == 0 || created == nil {
		return ErrInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if original, own := created[path]; own {
		current, err := os.Lstat(path)
		if err != nil || !managedDirectory(current, uid, gid) || !os.SameFile(original, current) {
			return errors.Join(ErrBinding, err)
		}
		return ctx.Err()
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	var missing []string
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if os.IsNotExist(err) {
			missing = append(missing, parent)
			continue
		}
		if err != nil {
			return err
		}
		native, nativeOK := info.Sys().(*syscall.Stat_t)
		if !info.IsDir() || !nativeOK || info.Mode().Perm()&0022 != 0 {
			return ErrNativeUnavailable
		}
		if native.Uid != 0 {
			original, own := created[parent]
			if !own || !managedDirectory(info, uid, gid) || !os.SameFile(original, info) {
				return ErrNativeUnavailable
			}
		}
		if parent == filepath.Dir(parent) {
			break
		}
	}
	for index := len(missing) - 1; index >= 0; index-- {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.Mkdir(missing[index], 0755); err != nil {
			return err
		}
		if err := os.Chmod(missing[index], 0755); err != nil {
			return err
		}
		if err := syncDirectDirectory(missing[index]); err != nil {
			return err
		}
		if err := syncDirectDirectory(filepath.Dir(missing[index])); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	return finishManagedDirectory(ctx, path, file, uid, gid, created)
}

func finishManagedDirectory(ctx context.Context, path string, file *os.File, uid, gid uint32, created map[string]os.FileInfo) (returnedErr error) {
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	birth, err := file.Stat()
	pathBirth, pathErr := os.Lstat(path)
	if err := errors.Join(err, pathErr); err != nil || !privateJournalDirectory(birth) || !os.SameFile(birth, pathBirth) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	if err := file.Chown(int(uid), int(gid)); err != nil {
		return err
	}
	if err := file.Chmod(0700); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	current, err := file.Stat()
	pathInfo, pathErr := os.Lstat(path)
	if err := errors.Join(err, pathErr); err != nil || !managedDirectory(current, uid, gid) ||
		!managedDirectory(pathInfo, uid, gid) || !os.SameFile(birth, current) || !os.SameFile(current, pathInfo) {
		return errors.Join(ErrBinding, err)
	}
	parent, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, parent.Close()) }()
	parentInfo, err := parent.Stat()
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode().Perm()&0022 != 0 {
		return errors.Join(ErrNativeUnavailable, err)
	}
	if err := parent.Sync(); err != nil {
		return err
	}
	created[path] = current
	return ctx.Err()
}

func privateJournalDirectory(info os.FileInfo) bool {
	return rootDirectory(info) && info.Mode().Perm() == 0700
}

func syncDirectDirectory(directory string) (returnedErr error) {
	before, err := os.Lstat(directory)
	if err != nil || before == nil || !before.IsDir() || before.Mode().Perm()&0022 != 0 {
		return errors.Join(ErrNativeUnavailable, err)
	}
	owner, ok := before.Sys().(*syscall.Stat_t)
	if !ok || owner.Uid != 0 {
		return ErrNativeUnavailable
	}
	file, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	if err := file.Sync(); err != nil {
		return err
	}
	after, err := os.Lstat(directory)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
		return errors.Join(ErrNativeUnavailable, err)
	}
	return nil
}
