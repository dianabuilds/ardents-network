package generation

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Identity is detached directory provenance, never adoption or startup authority.
type Identity struct {
	Device, Inode uint64
	GID           uint32
	Mode          os.FileMode
}

// Owner retains the original created directory and files, including partial
// birth or failed writes. It never removes residue or adopts an existing name.
type Owner struct {
	parent, root             *os.Root
	file                     *os.File
	parentPath, name         string
	parentIdentity, identity os.FileInfo
	files                    map[string]fileObservation
	gid                      uint32
	sealed                   bool
	terminal                 error
}

// Create returns partial custody together with any post-birth error. The caller
// must retain that nonnil Owner through its original join and eventually Close.
// Parent identity comes from the caller's still-live Installation transaction;
// it conveys no authorization and does not lend a root or lease.
func Create(ctx context.Context, parentPath string, expectedParent os.FileInfo, name string, gid uint32) (result *Owner, returnedErr error) {
	digest, err := hex.DecodeString(name)
	if ctx == nil || os.Geteuid() != 0 || gid == 0 || gid == ^uint32(0) ||
		!filepath.IsAbs(parentPath) || filepath.Clean(parentPath) != parentPath ||
		filepath.Base(parentPath) != "generations" || expectedParent == nil ||
		err != nil || len(digest) != 32 || hex.EncodeToString(digest) != name {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	before, err := os.Lstat(parentPath)
	if err != nil || !sameDirectory(expectedParent, before) || !containerAccess(before, gid) {
		return nil, errors.Join(ErrBinding, err)
	}
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return nil, err
	}
	actual, err := parent.Stat(".")
	if err != nil || !sameDirectory(expectedParent, actual) {
		return nil, errors.Join(ErrBinding, err, parent.Close())
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(err, parent.Close())
	}
	if err := parent.Mkdir(name, 0700); err != nil {
		return nil, errors.Join(err, parent.Close())
	}
	owned := &Owner{parent: parent, parentPath: parentPath, parentIdentity: actual,
		name: name, gid: gid, files: make(map[string]fileObservation)}
	defer func() {
		if returnedErr != nil {
			owned.terminal = returnedErr
			result = owned
		}
	}()
	owned.file, err = parent.OpenFile(name, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return owned, err
	}
	owned.identity, err = owned.file.Stat()
	if err != nil || !privateDirectory(owned.identity) {
		return owned, errors.Join(ErrBinding, err)
	}
	owned.root, err = parent.OpenRoot(name)
	if err != nil {
		return owned, err
	}
	if err := owned.Observe(); err != nil {
		return owned, err
	}
	return owned, errors.Join(owned.file.Sync(), syncRoot(parent), ctx.Err())
}

// CreateFile exclusively births and syncs an empty closed leaf before its first
// byte mutation. Installation records this original identity durably before Write.
// A failed birth retains its original descriptor until Close.
func (owned *Owner) CreateFile(ctx context.Context, name string) (result os.FileInfo, returnedErr error) {
	if owned == nil || ctx == nil {
		return nil, ErrInput
	}
	if owned.terminal != nil {
		return nil, owned.terminal
	}
	defer func() {
		if returnedErr != nil {
			owned.terminal = returnedErr
		}
	}()
	if _, allowed := fileMode(name); !allowed || owned.sealed {
		return nil, ErrBinding
	}
	if _, present := owned.files[name]; present {
		return nil, ErrBinding
	}
	if err := errors.Join(ctx.Err(), owned.Observe()); err != nil {
		return nil, err
	}
	file, err := owned.root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	// Register before Stat/Sync can fail: failure cannot forget physical custody.
	owned.files[name] = fileObservation{file: file, mode: 0600}
	birth, err := file.Stat()
	if err != nil || !ownedFile(birth) || birth.Mode() != 0600 || birth.Size() != 0 {
		return nil, errors.Join(ErrBinding, err)
	}
	owned.files[name] = fileObservation{file: file, identity: birth, mode: 0600}
	if err := errors.Join(file.Sync(), owned.file.Sync(), ctx.Err()); err != nil {
		return birth, err
	}
	return birth, errors.Join(owned.Observe(), ctx.Err())
}

// Write mutates only an already born original empty leaf. Installation first
// resynchronizes its exact birth record and rechecks transaction admission.
func (owned *Owner) Write(ctx context.Context, name string, body []byte) (returnedErr error) {
	if owned == nil || ctx == nil {
		return ErrInput
	}
	if owned.terminal != nil {
		return owned.terminal
	}
	defer func() {
		if returnedErr != nil {
			owned.terminal = returnedErr
		}
	}()
	mode, allowed := fileMode(name)
	original, present := owned.files[name]
	if !allowed || owned.sealed || !present || original.complete || original.file == nil || len(body) == 0 || len(body) > 64<<20 {
		return ErrBinding
	}
	if err := errors.Join(ctx.Err(), owned.Observe()); err != nil {
		return err
	}
	record, err := writeFile(ctx, owned.root, name, original, body, mode, owned.gid)
	// Preserve the original descriptor even if the write/access/Sync failed.
	if record.identity != nil {
		owned.files[name] = record
	}
	return errors.Join(err, owned.file.Sync(), ctx.Err())
}

// Seal promotes only the same complete original directory to bound read access.
// This grants no selection, manager or startup right.
func (owned *Owner) Seal(ctx context.Context) (returnedErr error) {
	if owned == nil || ctx == nil {
		return ErrInput
	}
	if owned.terminal != nil {
		return owned.terminal
	}
	defer func() {
		if returnedErr != nil {
			owned.terminal = returnedErr
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	if owned.sealed || len(owned.files) != len(Names()) {
		return ErrBinding
	}
	for _, record := range owned.files {
		if !record.complete {
			return ErrBinding
		}
	}
	if err := owned.Observe(); err != nil {
		return err
	}
	if err := owned.file.Chown(0, int(owned.gid)); err != nil {
		return err
	}
	if err := owned.file.Chmod(0750); err != nil {
		return err
	}
	if err := owned.file.Sync(); err != nil {
		return err
	}
	promoted, err := owned.file.Stat()
	if err != nil || !os.SameFile(owned.identity, promoted) || promoted.Mode() != os.ModeDir|0750 {
		return errors.Join(ErrBinding, err)
	}
	native, ok := promoted.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Gid != owned.gid {
		return ErrBinding
	}
	owned.identity = promoted
	owned.sealed = true
	if err := syncRoot(owned.parent); err != nil {
		return err
	}
	return errors.Join(owned.Observe(), ctx.Err())
}

// Observe checks original directory identity, exact inventory, original file
// inodes/access and bytes. Closed or failed custody never becomes valid again.
func (owned *Owner) Observe() (returnedErr error) {
	if owned == nil {
		return ErrBinding
	}
	if owned.terminal != nil {
		return owned.terminal
	}
	defer func() {
		if returnedErr != nil {
			owned.terminal = returnedErr
		}
	}()
	if owned.parent == nil || owned.root == nil || owned.file == nil || owned.identity == nil {
		return ErrBinding
	}
	parentPath, pathErr := os.Lstat(owned.parentPath)
	parentRoot, rootErr := owned.parent.Stat(".")
	for _, info := range []os.FileInfo{parentPath, parentRoot} {
		// Installation owns exact container access promotion. This owner
		// retains its original container inode and the closed trusted access
		// states; the caller independently checks phase-appropriate metadata.
		if info == nil || !os.SameFile(owned.parentIdentity, info) || !containerAccess(info, owned.gid) {
			return errors.Join(ErrBinding, pathErr, rootErr)
		}
	}
	pathInfo, pathErr := owned.parent.Lstat(owned.name)
	rootInfo, rootErr := owned.root.Stat(".")
	fdInfo, fdErr := owned.file.Stat()
	for _, info := range []os.FileInfo{pathInfo, rootInfo, fdInfo} {
		if !sameDirectory(owned.identity, info) {
			return errors.Join(ErrBinding, pathErr, rootErr, fdErr)
		}
	}
	reader, err := owned.root.Open(".")
	if err != nil {
		return err
	}
	names, readErr := reader.Readdirnames(len(owned.files) + 1)
	closeErr := reader.Close()
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	if err := errors.Join(readErr, closeErr); err != nil {
		return err
	}
	if len(names) != len(owned.files) {
		return ErrBinding
	}
	for _, name := range names {
		expected, own := owned.files[name]
		if !own {
			return ErrBinding
		}
		if err := observeFile(owned.root, name, expected); err != nil {
			return err
		}
	}
	for name, expected := range owned.files {
		if err := observeFile(owned.root, name, expected); err != nil {
			return err
		}
	}
	return nil
}

// Identity returns only copied physical facts, never internal handles or maps.
func (owned *Owner) Identity() Identity {
	if owned == nil || owned.identity == nil {
		return Identity{}
	}
	native, ok := owned.identity.Sys().(*syscall.Stat_t)
	if !ok {
		return Identity{}
	}
	return Identity{Device: uint64(native.Dev), Inode: native.Ino, GID: native.Gid, Mode: owned.identity.Mode()}
}

// Bytes returns detached bytes only from a sealed, unfailed, still-open owner.
// Installation must Observe before using them for any physical effect.
func (owned *Owner) Bytes(name string) []byte {
	if owned == nil || !owned.sealed || owned.terminal != nil || owned.root == nil {
		return nil
	}
	return bytes.Clone(owned.files[name].body)
}

func (owned *Owner) Close() error {
	if owned == nil {
		return nil
	}
	for name, record := range owned.files {
		if record.file != nil {
			owned.terminal = errors.Join(owned.terminal, record.file.Close())
			record.file = nil
			owned.files[name] = record
		}
	}
	if owned.root != nil {
		owned.terminal = errors.Join(owned.terminal, owned.root.Close())
		owned.root = nil
	}
	if owned.file != nil {
		owned.terminal = errors.Join(owned.terminal, owned.file.Close())
		owned.file = nil
	}
	if owned.parent != nil {
		owned.terminal = errors.Join(owned.terminal, owned.parent.Close())
		owned.parent = nil
	}
	return owned.terminal
}

func sameDirectory(original, current os.FileInfo) bool {
	if original == nil || current == nil || !current.IsDir() || !os.SameFile(original, current) || original.Mode() != current.Mode() {
		return false
	}
	a, aOK := original.Sys().(*syscall.Stat_t)
	b, bOK := current.Sys().(*syscall.Stat_t)
	return aOK && bOK && a.Uid == 0 && b.Uid == 0 && a.Gid == b.Gid && current.Mode().Perm()&0022 == 0
}

func privateDirectory(info os.FileInfo) bool {
	if info == nil || info.Mode() != os.ModeDir|0700 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0
}

func containerAccess(info os.FileInfo, gid uint32) bool {
	if info == nil || !info.IsDir() {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && ((info.Mode() == os.ModeDir|0700 && native.Gid == 0) || (info.Mode() == os.ModeDir|0750 && native.Gid == gid))
}

func syncRoot(root *os.Root) (returnedErr error) {
	file, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close()) }()
	return file.Sync()
}
