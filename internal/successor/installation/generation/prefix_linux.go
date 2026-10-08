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

// PrefixFile is an independently observed original leaf image. It conveys no
// birth-record, Release, phase, lease or mutation authority.
type PrefixFile struct {
	Identity os.FileInfo
	Bytes    []byte
}

// Prefix retains reopened original private generation files under one caller.
// Installation separately admits original journal provenance, write order,
// fresh complete preimages and every effect. It is neither a creation Owner nor
// a sealed Snapshot. A failed operation retains all descriptors until Close.
type Prefix struct {
	ctx                      context.Context
	parent, root             *os.Root
	file                     *os.File
	parentPath, name         string
	parentIdentity, identity os.FileInfo
	files                    map[string]fileObservation
	gid                      uint32
	sealed, closed           bool
	terminal                 error
}

// OpenPrefix independently opens the exact supplied image. A nonnil result on
// failure is partial custody which the caller must retain and Close. No live
// Installation root or writer lease is lent to this mechanism.
func OpenPrefix(ctx context.Context, parentPath string, expectedParent, expectedDirectory os.FileInfo, name string, gid uint32, images map[string]PrefixFile) (result *Prefix, returnedErr error) {
	digest, err := hex.DecodeString(name)
	if ctx == nil || os.Geteuid() != 0 || gid == 0 || gid == ^uint32(0) || !filepath.IsAbs(parentPath) || filepath.Clean(parentPath) != parentPath || filepath.Base(parentPath) != "generations" || err != nil || len(digest) != 32 || hex.EncodeToString(digest) != name || !sealedDirectory(expectedParent, gid) || !prefixDirectory(expectedDirectory, gid) || len(images) > len(Names()) {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p := &Prefix{ctx: ctx, parentPath: parentPath, name: name, gid: gid, files: make(map[string]fileObservation)}
	result = p
	defer func() {
		if returnedErr != nil {
			p.terminal = errors.Join(returnedErr, ctx.Err())
		}
	}()
	p.parent, err = os.OpenRoot(parentPath)
	if err != nil {
		return p, err
	}
	p.parentIdentity, err = p.parent.Stat(".")
	if err != nil || !sameDirectory(expectedParent, p.parentIdentity) {
		return p, errors.Join(ErrBinding, err)
	}
	p.file, err = p.parent.OpenFile(name, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return p, err
	}
	p.identity, err = p.file.Stat()
	if err != nil || !sameDirectory(expectedDirectory, p.identity) {
		return p, errors.Join(ErrBinding, err)
	}
	p.root, err = p.parent.OpenRoot(name)
	if err != nil {
		return p, err
	}
	for name, image := range images {
		mode, allowed := fileMode(name)
		info := image.Identity
		if !allowed || info == nil || len(image.Bytes) > 64<<20 || info.Size() != int64(len(image.Bytes)) || !info.Mode().IsRegular() {
			return p, ErrBinding
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || native.Uid != 0 || native.Nlink != 1 || !((info.Mode() == 0600 && (native.Gid == 0 || native.Gid == gid)) || (info.Mode() == mode && native.Gid == gid)) {
			return p, ErrBinding
		}
		expected := fileObservation{identity: info, body: bytes.Clone(image.Bytes), mode: info.Mode(), gid: native.Gid}
		if err := observeFile(p.root, name, expected); err != nil {
			return p, err
		}
		file, err := p.root.OpenFile(name, os.O_RDWR|syscall.O_NOFOLLOW, 0)
		if err != nil {
			return p, err
		}
		// Register the original descriptor before any following observation.
		expected.file = file
		p.files[name] = expected
		actual, err := file.Stat()
		if err != nil || !fileObservationMatches(expected, actual) {
			return p, errors.Join(ErrBinding, err)
		}
		expected.identity = actual
		expected.complete = actual.Mode() == mode && native.Gid == gid && len(expected.body) > 0
		p.files[name] = expected
	}
	return p, p.Observe()
}

func prefixDirectory(info os.FileInfo, gid uint32) bool {
	if info == nil || !info.IsDir() {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && ((info.Mode() == os.ModeDir|0700 && (native.Gid == 0 || native.Gid == gid)) || (info.Mode() == os.ModeDir|0750 && native.Gid == gid))
}

// Observe rechecks original directory/leaf descriptors, paths, access, ctime,
// exact finite inventory and observed bytes; the first refusal cannot renew.
func (p *Prefix) Observe() (returnedErr error) {
	if p == nil {
		return ErrBinding
	}
	if p.terminal != nil {
		return p.terminal
	}
	defer func() {
		if returnedErr != nil {
			p.terminal = returnedErr
			if p.ctx != nil {
				p.terminal = errors.Join(p.terminal, p.ctx.Err())
			}
		}
	}()
	if p.closed || p.ctx == nil || p.parent == nil || p.root == nil || p.file == nil || p.identity == nil {
		return ErrBinding
	}
	if err := p.ctx.Err(); err != nil {
		return err
	}
	pathParent, pathErr := os.Lstat(p.parentPath)
	rootParent, rootErr := p.parent.Stat(".")
	for _, info := range []os.FileInfo{pathParent, rootParent} {
		if !sameDirectory(p.parentIdentity, info) || !sealedDirectory(info, p.gid) {
			return errors.Join(ErrBinding, pathErr, rootErr)
		}
	}
	path, pathErr := p.parent.Lstat(p.name)
	root, rootErr := p.root.Stat(".")
	fd, fdErr := p.file.Stat()
	for _, info := range []os.FileInfo{path, root, fd} {
		if !sameDirectory(p.identity, info) || !prefixDirectory(info, p.gid) {
			return errors.Join(ErrBinding, pathErr, rootErr, fdErr)
		}
	}
	directory, err := p.root.Open(".")
	if err != nil {
		return err
	}
	names, readErr := directory.Readdirnames(len(p.files) + 1)
	closeErr := directory.Close()
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	if err := errors.Join(readErr, closeErr); err != nil {
		return err
	}
	if len(names) != len(p.files) {
		return ErrBinding
	}
	for _, name := range names {
		observed, known := p.files[name]
		if !known {
			return ErrBinding
		}
		if err := observeFile(p.root, name, observed); err != nil {
			return err
		}
	}
	return p.ctx.Err()
}

// Match checks every actual image against full independently supplied preimages.
// Installation obtains those bytes from fresh proofs, never from this Prefix.
func (p *Prefix) Match(full map[string][]byte) (returnedErr error) {
	if err := p.Observe(); err != nil {
		return err
	}
	defer func() {
		if returnedErr != nil {
			p.terminal = errors.Join(returnedErr, p.ctx.Err())
		}
	}()
	if len(full) != len(Names()) {
		return ErrBinding
	}
	for _, name := range Names() {
		body, known := full[name]
		if !known || len(body) == 0 || len(body) > 64<<20 {
			return ErrBinding
		}
		if observed, present := p.files[name]; present {
			mode, _ := fileMode(name)
			if !bytes.HasPrefix(body, observed.body) || observed.mode == mode && !bytes.Equal(body, observed.body) {
				return ErrBinding
			}
		}
	}
	return p.ctx.Err()
}

// CreateFile syncs an exclusive empty leaf and retains its descriptor. The
// caller durably records its original inode BEFORE Repair permits body bytes.
func (p *Prefix) CreateFile(name string) (result os.FileInfo, returnedErr error) {
	if err := p.Observe(); err != nil {
		return nil, err
	}
	defer func() {
		if returnedErr != nil {
			p.terminal = errors.Join(returnedErr, p.ctx.Err())
		}
	}()
	if _, allowed := fileMode(name); !allowed || p.sealed {
		return nil, ErrBinding
	}
	if _, present := p.files[name]; present {
		return nil, ErrBinding
	}
	file, err := p.root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	observed := fileObservation{file: file, mode: 0600}
	p.files[name] = observed
	observed.identity, err = file.Stat()
	p.files[name] = observed
	if err != nil || !ownedFile(observed.identity) || observed.identity.Mode() != 0600 || observed.identity.Size() != 0 {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := errors.Join(file.Sync(), syncRoot(p.root)); err != nil {
		return observed.identity, err
	}
	return observed.identity, p.Observe()
}

// Repair preserves the original inode. Complete bytes are never truncated;
// private/torn bytes may be repaired only as a prefix of the full preimage.
func (p *Prefix) Repair(name string, body []byte) (returnedErr error) {
	if err := p.Observe(); err != nil {
		return err
	}
	defer func() {
		if returnedErr != nil {
			p.terminal = errors.Join(returnedErr, p.ctx.Err())
		}
	}()
	observed, present := p.files[name]
	mode, allowed := fileMode(name)
	if !present || !allowed || p.sealed || observed.file == nil || len(body) == 0 || len(body) > 64<<20 || !bytes.HasPrefix(body, observed.body) || observed.mode == mode && !bytes.Equal(body, observed.body) {
		return ErrBinding
	}
	file := observed.file
	if !bytes.Equal(body, observed.body) {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if err := file.Truncate(0); err != nil {
			return err
		}
		n, err := file.Write(body)
		if err != nil || n != len(body) {
			return errors.Join(io.ErrShortWrite, err)
		}
	}
	if observed.gid != p.gid {
		if err := file.Chown(0, int(p.gid)); err != nil {
			return err
		}
	}
	if observed.mode != mode {
		if err := file.Chmod(mode); err != nil {
			return err
		}
	}
	if err := file.Sync(); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil || !os.SameFile(observed.identity, info) {
		return errors.Join(ErrBinding, err)
	}
	p.files[name] = fileObservation{file: file, identity: info, body: bytes.Clone(body), mode: mode, gid: p.gid, complete: true}
	return p.Observe()
}

// Seal promotes only a complete original fifteen-file image on the same inode.
// It grants no Release, generation selection, startup or recovery authority.
func (p *Prefix) Seal() (returnedErr error) {
	if err := p.Observe(); err != nil {
		return err
	}
	defer func() {
		if returnedErr != nil {
			p.terminal = errors.Join(returnedErr, p.ctx.Err())
		}
	}()
	if p.sealed || len(p.files) != len(Names()) {
		return ErrBinding
	}
	for _, name := range Names() {
		if !p.files[name].complete {
			return ErrBinding
		}
	}
	native := p.identity.Sys().(*syscall.Stat_t)
	if native.Gid != p.gid {
		if err := p.file.Chown(0, int(p.gid)); err != nil {
			return err
		}
	}
	if p.identity.Mode() != os.ModeDir|0750 {
		if err := p.file.Chmod(0750); err != nil {
			return err
		}
	}
	if err := p.file.Sync(); err != nil {
		return err
	}
	info, err := p.file.Stat()
	if err != nil || !os.SameFile(p.identity, info) || !sealedDirectory(info, p.gid) {
		return errors.Join(ErrBinding, err)
	}
	p.identity, p.sealed = info, true
	if err := syncRoot(p.parent); err != nil {
		return err
	}
	return p.Observe()
}

// Identity returns detached physical facts; it cannot recreate an owner.
func (p *Prefix) Identity() Identity {
	if p == nil || p.identity == nil {
		return Identity{}
	}
	native, ok := p.identity.Sys().(*syscall.Stat_t)
	if !ok {
		return Identity{}
	}
	return Identity{Device: uint64(native.Dev), Inode: native.Ino, GID: native.Gid, Mode: p.identity.Mode()}
}

func (p *Prefix) Close() error {
	if p == nil {
		return nil
	}
	if p.closed {
		return p.terminal
	}
	p.closed = true
	if p.ctx != nil {
		p.terminal = errors.Join(p.terminal, p.ctx.Err())
	}
	for name, observed := range p.files {
		if observed.file != nil {
			p.terminal = errors.Join(p.terminal, observed.file.Close())
			observed.file = nil
			p.files[name] = observed
		}
	}
	if p.root != nil {
		p.terminal = errors.Join(p.terminal, p.root.Close())
		p.root = nil
	}
	if p.file != nil {
		p.terminal = errors.Join(p.terminal, p.file.Close())
		p.file = nil
	}
	if p.parent != nil {
		p.terminal = errors.Join(p.terminal, p.parent.Close())
		p.parent = nil
	}
	return p.terminal
}
