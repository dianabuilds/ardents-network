package directory

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

var (
	ErrInput             = errors.New("installation directory: invalid input")
	ErrBinding           = errors.New("installation directory: original binding changed")
	ErrNativeUnavailable = errors.New("installation directory: native custody unavailable")
)

// Creation retains the original caller and private birth identities of one
// serialized creation sequence. A second Creation cannot adopt those roots.
type Creation struct {
	ctx      context.Context
	uid, gid uint32
	original map[string]os.FileInfo
}

// Identity is detached physical data, not creation or installation authority.
type Identity struct {
	Device uint64
	Inode  uint64
}

// New prepares independent birth observation state without filesystem effects.
func New(ctx context.Context, uid, gid uint32) (*Creation, error) {
	if ctx == nil || uid == 0 || gid == 0 {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &Creation{ctx: ctx, uid: uid, gid: gid, original: make(map[string]os.FileInfo)}, nil
}

// Create exclusively creates a root or reobserves this sequence's own sibling.
// The caller selects every path and admits the effect before entering here.
func (created *Creation) Create(name string) error {
	if created == nil {
		return ErrInput
	}
	return createDirectory(created.ctx, name, created.uid, created.gid, created.original)
}

// Identity reobserves the original inode and account/mode before copying facts.
// It performs no mutation and does not renew the creation caller.
func (created *Creation) Identity(name string) (Identity, error) {
	if created == nil || created.original == nil {
		return Identity{}, ErrInput
	}
	original, own := created.original[name]
	if !own {
		return Identity{}, ErrBinding
	}
	current, err := os.Lstat(name)
	if err != nil || !Matches(current, created.uid, created.gid) || !os.SameFile(original, current) {
		return Identity{}, errors.Join(ErrBinding, err)
	}
	native := current.Sys().(*syscall.Stat_t)
	return Identity{Device: uint64(native.Dev), Inode: native.Ino}, nil
}

// Observe checks private original roots. Installation separately checks its
// caller/account/lease/journal before and after these read-only observations.
func (created *Creation) Observe() error {
	if created == nil || created.original == nil {
		return ErrInput
	}
	for name := range created.original {
		if _, err := created.Identity(name); err != nil {
			return err
		}
	}
	return nil
}

// Matches checks native account/mode properties without original birth provenance.
// Independent Installation inspection must separately match its bound inode.
func Matches(info os.FileInfo, uid, gid uint32) bool {
	if info == nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && uid != 0 && gid != 0 && native.Uid == uid && native.Gid == gid
}

// Only directories born in this operation can be reused by a sibling path.
// Successful chown is not an adoption or ownership record for another inode.
func createDirectory(ctx context.Context, path string, uid, gid uint32, created map[string]os.FileInfo) error {
	if ctx == nil || !canonicalPath(path) || path == "/" || uid == 0 || gid == 0 || created == nil {
		return ErrInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if original, own := created[path]; own {
		current, err := os.Lstat(path)
		if err != nil || !Matches(current, uid, gid) || !os.SameFile(original, current) {
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
			if !own || !Matches(info, uid, gid) || !os.SameFile(original, info) {
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
		if err := syncParentDirectory(missing[index]); err != nil {
			return err
		}
		if err := syncParentDirectory(filepath.Dir(missing[index])); err != nil {
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
	return finishDirectory(ctx, path, file, uid, gid, created)
}

func finishDirectory(ctx context.Context, path string, file *os.File, uid, gid uint32, created map[string]os.FileInfo) (returnedErr error) {
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	birth, err := file.Stat()
	pathBirth, pathErr := os.Lstat(path)
	if err := errors.Join(err, pathErr); err != nil || !privateBirthDirectory(birth) || !os.SameFile(birth, pathBirth) {
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
	if err := errors.Join(err, pathErr); err != nil || !Matches(current, uid, gid) ||
		!Matches(pathInfo, uid, gid) || !os.SameFile(birth, current) || !os.SameFile(current, pathInfo) {
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

func privateBirthDirectory(info os.FileInfo) bool {
	return trustedDirectory(info) && info.Mode().Perm() == 0700
}

func syncParentDirectory(directory string) (returnedErr error) {
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

func trustedDirectory(info os.FileInfo) bool {
	if info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0
}

func canonicalPath(value string) bool {
	return path.IsAbs(value) && path.Clean(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}
