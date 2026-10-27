package forwarding

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// initialRecord belongs only to the distinct Forwarding initial transcript.
// Complete or partial bytes confer no adoption, continuation or startup right.
type initialRecord struct {
	Sequence int    `json:"sequence"`
	Phase    string `json:"phase"`
	Path     string `json:"path"`
	Device   uint64 `json:"device"`
	Inode    uint64 `json:"inode"`
	SHA256   string `json:"sha256"`
	Mode     uint32 `json:"mode"`
	UID      uint32 `json:"uid"`
	GID      uint32 `json:"gid"`
}

type initialFile struct {
	path string
	file *os.File
	info os.FileInfo
	body []byte
}

// This serialized creator owns only its original native resources. The root
// operation supplies exact allowed paths and rechecks authority around effects.
// No background writer, supplied completion or recovery/adoption path exists.
type initialBirths struct {
	ctx        context.Context
	journal    *initialFile
	sequence   int
	allowed    map[string]bool
	files      []*initialFile
	dirs       map[string]*os.File
	identities map[string]os.FileInfo
	terminal   error
	closed     bool
	// Failure-only observer after actual sync, never an accepting fact.
	afterSync func(*os.File) error
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func nodeAccess(info os.FileInfo, mode os.FileMode, uid, gid uint32, directory bool) bool {
	if info == nil || info.IsDir() != directory || info.Mode().Perm() != mode || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky|os.ModeSymlink) != 0 || !directory && !info.Mode().IsRegular() {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == uid && native.Gid == gid && (directory || native.Nlink == 1)
}

func syncInitialParent(path string) (returned error) {
	before, err := os.Lstat(filepath.Dir(path))
	if err != nil || !trustedInputDirectory(before) {
		return errors.Join(ErrBinding, err)
	}
	file, err := os.OpenFile(filepath.Dir(path), os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) || !trustedInputDirectory(opened) || before.Mode() != opened.Mode() {
		return errors.Join(ErrBinding, err)
	}
	if err := file.Sync(); err != nil {
		return err
	}
	after, err := os.Lstat(filepath.Dir(path))
	if err != nil || !os.SameFile(opened, after) || !trustedInputDirectory(after) || opened.Mode() != after.Mode() {
		return errors.Join(ErrBinding, err)
	}
	return nil
}

func openInitialBirths(ctx context.Context, path string, allowed map[string]bool) (*initialBirths, error) {
	return openForwardingBirths(ctx, path, allowed, "forwarding-initial-v1")
}

// Only the two fixed Forwarding creators share physical births. Their distinct
// first records and independent readers never exchange initialization rights.
func openForwardingBirths(ctx context.Context, path string, allowed map[string]bool, first string) (*initialBirths, error) {
	j := &initialBirths{ctx: ctx, allowed: allowed, dirs: map[string]*os.File{}, identities: map[string]os.FileInfo{}}
	if ctx == nil || !validPath(path) || os.Geteuid() != 0 || first != "forwarding-initial-v1" && first != "forwarding-hosting-v1" {
		return j, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return j, err
	}
	parent, err := os.Lstat(filepath.Dir(path))
	if err != nil || !trustedInputDirectory(parent) {
		return j, errors.Join(ErrBinding, err)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return j, err
	}
	j.journal = &initialFile{path: path, file: f}
	j.journal.info, err = f.Stat()
	if err != nil || !nodeAccess(j.journal.info, 0600, 0, 0, false) {
		return j, errors.Join(ErrBinding, err)
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return j, err
	}
	if err = errors.Join(f.Sync(), syncInitialParent(path)); err != nil {
		return j, err
	}
	return j, j.record(first, path, j.journal.info, "", 0600, 0, 0)
}

func (j *initialBirths) fail(err error) error {
	if err != nil && j.terminal == nil {
		j.terminal = err
	}
	return err
}

func (j *initialBirths) record(phase, path string, info os.FileInfo, sha string, mode os.FileMode, uid, gid uint32) (returned error) {
	if j == nil || j.ctx == nil || j.journal == nil {
		return ErrInput
	}
	defer func() { j.fail(returned) }()
	if err := j.check(); err != nil {
		return err
	}
	if path != j.journal.path && !j.allowed[path] {
		return ErrBinding
	}
	valid := phase == "forwarding-hosting-v1"
	for _, p := range []string{"forwarding-initial-v1", "admitted", "account-preimage", "account-auxiliary-preimage", "account-intent", "account-born", "account-auxiliary-result", "account-auxiliary-cleared", "account-ready", "directory-intent", "directory-born", "directory-ready", "file-intent", "file-born", "file-ready", "seal-intent", "sealed", "selected", "reload-intent", "stopped-verified", "completed"} {
		valid = valid || phase == p
	}
	if !valid || len(sha) != 0 && (len(sha) != 64 || len(bytes.Trim([]byte(sha), "0123456789abcdef")) != 0) {
		return ErrBinding
	}
	r := initialRecord{Sequence: j.sequence, Phase: phase, Path: path, SHA256: sha, Mode: uint32(mode), UID: uid, GID: gid}
	if info != nil {
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return ErrBinding
		}
		r.Device, r.Inode = uint64(native.Dev), native.Ino
	}
	raw, err := json.Marshal(r)
	if err != nil || len(raw) > 32<<10 || len(j.journal.body)+len(raw)+1 > 4<<20 {
		return errors.Join(ErrBinding, err)
	}
	raw = append(raw, '\n')
	n, err := j.journal.file.WriteAt(raw, int64(len(j.journal.body)))
	if err != nil || n != len(raw) {
		return errors.Join(io.ErrShortWrite, err)
	}
	if err = j.journal.file.Sync(); err != nil {
		return err
	}
	if j.afterSync != nil {
		if err = j.afterSync(j.journal.file); err != nil {
			return err
		}
	}
	j.journal.body = append(j.journal.body, raw...)
	j.journal.info, err = j.journal.file.Stat()
	if err != nil {
		return err
	}
	j.sequence++
	return j.check()
}

func observeInitialFile(original *initialFile) error {
	current, err := original.file.Stat()
	named, nameErr := os.Lstat(original.path)
	if err != nil || nameErr != nil || !sameInputFile(original.info, current) || !sameInputFile(original.info, named) {
		return errors.Join(ErrBinding, err, nameErr)
	}
	raw := make([]byte, len(original.body)+1)
	n, err := original.file.ReadAt(raw, 0)
	equal := (err == nil || err == io.EOF) && n == len(original.body) && bytes.Equal(raw[:n], original.body)
	clear(raw)
	if !equal {
		return errors.Join(ErrBinding, err)
	}
	after, err := original.file.Stat()
	if err != nil || !sameInputFile(original.info, after) {
		return errors.Join(ErrBinding, err)
	}
	return nil
}

func (j *initialBirths) check() error {
	if j == nil || j.journal == nil || j.ctx == nil || j.closed {
		return ErrInput
	}
	if j.terminal != nil {
		return j.terminal
	}
	if err := j.ctx.Err(); err != nil {
		return err
	}
	if err := observeInitialFile(j.journal); err != nil {
		return err
	}
	for _, f := range j.files {
		if err := observeInitialFile(f); err != nil {
			return err
		}
	}
	for path, f := range j.dirs {
		info, err := f.Stat()
		named, nameErr := os.Lstat(path)
		before := j.identities[path]
		a, aok := infoSys(before)
		b, bok := infoSys(info)
		c, cok := infoSys(named)
		if err != nil || nameErr != nil || !aok || !bok || !cok || !os.SameFile(before, info) || !os.SameFile(before, named) || before.Mode() != info.Mode() || before.Mode() != named.Mode() || a.Uid != b.Uid || a.Gid != b.Gid || a.Uid != c.Uid || a.Gid != c.Gid {
			return errors.Join(ErrBinding, err, nameErr)
		}
	}
	return j.ctx.Err()
}

func infoSys(info os.FileInfo) (*syscall.Stat_t, bool) {
	if info == nil {
		return nil, false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return native, ok
}

func (j *initialBirths) directory(path string, mode os.FileMode, uid, gid uint32, admit func() error) (returned error) {
	defer func() { j.fail(returned) }()
	if !j.allowed[path] || !validPath(path) || mode != 0700 && mode != 0755 {
		return ErrBinding
	}
	if err := admit(); err != nil {
		return err
	}
	if err := j.record("directory-intent", path, nil, "", mode, uid, gid); err != nil {
		return err
	}
	if err := admit(); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if f != nil {
		j.dirs[path] = f
	}
	if err != nil {
		return err
	}
	j.identities[path], err = f.Stat()
	if err != nil || !nodeAccess(j.identities[path], 0700, 0, 0, true) {
		return errors.Join(ErrBinding, err)
	}
	if err = errors.Join(f.Sync(), syncInitialParent(path)); err != nil {
		return err
	}
	if err = j.record("directory-born", path, j.identities[path], "", 0700, 0, 0); err != nil {
		return err
	}
	if err = admit(); err != nil {
		return err
	}
	if err = f.Chown(int(uid), int(gid)); err != nil {
		return err
	}
	if err = f.Chmod(mode); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	j.identities[path], err = f.Stat()
	if err != nil || !nodeAccess(j.identities[path], mode, uid, gid, true) {
		return errors.Join(ErrBinding, err)
	}
	return j.record("directory-ready", path, j.identities[path], "", mode, uid, gid)
}

func (j *initialBirths) write(path string, body []byte, mode os.FileMode, uid, gid uint32, admit func() error) (returned error) {
	defer func() { j.fail(returned) }()
	if !j.allowed[path] || !validPath(path) || len(body) == 0 || len(body) > 64<<20 {
		return ErrBinding
	}
	if err := admit(); err != nil {
		return err
	}
	if err := j.record("file-intent", path, nil, digest(body), mode, uid, gid); err != nil {
		return err
	}
	if err := admit(); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	own := &initialFile{path: path, file: f}
	j.files = append(j.files, own)
	own.info, err = f.Stat()
	if err != nil || !nodeAccess(own.info, 0600, 0, 0, false) {
		return errors.Join(ErrBinding, err)
	}
	if err = errors.Join(f.Sync(), syncInitialParent(path)); err != nil {
		return err
	}
	if err = j.record("file-born", path, own.info, "", 0600, 0, 0); err != nil {
		return err
	}
	if err = admit(); err != nil {
		return err
	}
	n, err := f.Write(body)
	if err != nil || n != len(body) {
		return errors.Join(io.ErrShortWrite, err)
	}
	if err = f.Chown(int(uid), int(gid)); err != nil {
		return err
	}
	if err = f.Chmod(mode); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if j.afterSync != nil {
		if err = j.afterSync(f); err != nil {
			return err
		}
	}
	own.body = bytes.Clone(body)
	own.info, err = f.Stat()
	if err != nil || !nodeAccess(own.info, mode, uid, gid, false) {
		return errors.Join(ErrBinding, err)
	}
	return j.record("file-ready", path, own.info, digest(body), mode, uid, gid)
}

func (j *initialBirths) close() error {
	if j == nil {
		return nil
	}
	if !j.closed {
		j.closed = true
		for i := len(j.files) - 1; i >= 0; i-- {
			j.terminal = errors.Join(j.terminal, j.files[i].file.Close())
			clear(j.files[i].body)
		}
		for _, f := range j.dirs {
			j.terminal = errors.Join(j.terminal, f.Close())
		}
		if j.journal != nil {
			j.terminal = errors.Join(j.terminal, j.journal.file.Close())
		}
		if j.ctx != nil {
			j.terminal = errors.Join(j.terminal, j.ctx.Err())
		}
	}
	return j.terminal
}
