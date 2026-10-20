//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Each original directory descriptor stays open through final readback and
// physical Close. All path components are opened relative to retained parents,
// with NOFOLLOW; reobservation never adopts a replacement inode.
type preparationDirectory struct {
	file *os.File
	name string
	info os.FileInfo
}

type preparationPath struct{ directories []preparationDirectory }

func (p *preparationPath) close() (err error) {
	for i := len(p.directories) - 1; i >= 0; i-- {
		err = errors.Join(err, p.directories[i].file.Close())
	}
	return
}

func openPreparationAt(parent *os.File, name string, flags int, mode uint32) (*os.File, error) {
	fd, err := syscall.Openat(int(parent.Fd()), name, flags|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, mode)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}

func openPreparationParent(path string) (p *preparationPath, err error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errNetworkPreparation
	}
	p = &preparationPath{}
	defer func() {
		if err != nil {
			err = errors.Join(err, p.close())
			p = nil
		}
	}()
	f, err := os.Open("/")
	if err != nil {
		return p, err
	}
	info, err := f.Stat()
	if err != nil {
		return p, errors.Join(err, f.Close())
	}
	p.directories = append(p.directories, preparationDirectory{file: f, info: info})
	for _, name := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if name == "" {
			continue
		}
		f, err = openPreparationAt(f, name, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
		if err != nil {
			return p, err
		}
		info, err = f.Stat()
		if err != nil {
			return p, errors.Join(err, f.Close())
		}
		p.directories = append(p.directories, preparationDirectory{file: f, name: name, info: info})
	}
	return p, nil
}

func (p *preparationPath) parent() *os.File { return p.directories[len(p.directories)-1].file }

func (p *preparationPath) check() error {
	for i, d := range p.directories {
		current, err := d.file.Stat()
		if err != nil || !os.SameFile(d.info, current) || !current.IsDir() || current.Mode() != d.info.Mode() {
			return errors.Join(errNetworkPreparation, err)
		}
		if i == 0 {
			continue
		}
		linked, err := openPreparationAt(p.directories[i-1].file, d.name, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
		if err != nil {
			return err
		}
		observed, e := linked.Stat()
		err = errors.Join(e, linked.Close())
		if err != nil || !os.SameFile(d.info, observed) {
			return errors.Join(errNetworkPreparation, err)
		}
	}
	return nil
}

func preparationPrivate(info os.FileInfo, directory bool) bool {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || s.Uid != uint32(os.Geteuid()) {
		return false
	}
	if directory {
		return info.IsDir() && info.Mode().Perm() == 0700
	}
	return info.Mode().IsRegular() && info.Mode().Perm() == 0600 && s.Nlink == 1
}

func readNetworkPreparation(ctx context.Context, path string) (raw []byte, err error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	p, err := openPreparationParent(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, p.close()) }()
	f, err := openPreparationAt(p.parent(), filepath.Base(path), syscall.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	before, err := f.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() > maximumNetworkPreparationPlan {
		return nil, errors.Join(errNetworkPreparation, err)
	}
	initial, ok := before.Sys().(*syscall.Stat_t)
	if !ok || initial.Nlink != 1 {
		return nil, errNetworkPreparation
	}
	raw, err = io.ReadAll(io.LimitReader(f, maximumNetworkPreparationPlan+1))
	if err != nil || len(raw) > maximumNetworkPreparationPlan {
		return nil, errors.Join(errNetworkPreparation, err)
	}
	after, e := f.Stat()
	if e != nil || !os.SameFile(before, after) || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return nil, errors.Join(errNetworkPreparation, e)
	}
	final, ok := after.Sys().(*syscall.Stat_t)
	if !ok || final.Ctim != initial.Ctim || final.Nlink != 1 {
		return nil, errNetworkPreparation
	}
	linked, e := openPreparationAt(p.parent(), filepath.Base(path), syscall.O_RDONLY, 0)
	if e != nil {
		return nil, e
	}
	observed, e := linked.Stat()
	e = errors.Join(e, linked.Close(), p.check(), ctx.Err())
	if e != nil || !os.SameFile(before, observed) {
		return nil, errors.Join(errNetworkPreparation, e)
	}
	return raw, nil
}

func writeNetworkPreparation(ctx context.Context, path, name string, raw []byte) error {
	return persistNetworkPreparation(ctx, path, name, raw, nil)
}

func writeNetworkPreparationReceipt(ctx context.Context, out io.Writer, raw []byte) error {
	return writeSourceReportContext(ctx, out, raw)
}

// observe is only a synchronous test barrier around real native effects. It
// cannot supply successful file operations, bytes or accepting observations.
func persistNetworkPreparation(ctx context.Context, path, name string, raw []byte, observe func(string, *os.File)) (err error) {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(raw) == 0 || len(raw) > maximumNetworkPreparationPlan || (name != "node-record.signing-input" && name != "epoch.unsigned" && name != "profile.unsigned") || filepath.Base(path) == "/" {
		return errNetworkPreparation
	}
	p, err := openPreparationParent(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, p.close()) }()
	parentInfo, err := p.parent().Stat()
	if err != nil || !preparationPrivate(parentInfo, true) {
		return errors.Join(errNetworkPreparation, err)
	}
	// This development surface supports native ext4 output only. Observing the
	// retained output parent's actual mount is distinct from host OS selection.
	const ext4SuperMagic = 0xef53
	var filesystem syscall.Statfs_t
	if err = syscall.Fstatfs(int(p.parent().Fd()), &filesystem); err != nil || filesystem.Type != ext4SuperMagic {
		return errors.Join(errNetworkPreparation, err)
	}
	// Admission probes actual selected mount durability, before directory birth.
	// A failed directory sync is an unsupported/failed environment, never a skip.
	if err = errors.Join(p.check(), p.parent().Sync(), ctx.Err()); err != nil {
		return err
	}
	if err = syscall.Mkdirat(int(p.parent().Fd()), filepath.Base(path), 0700); err != nil {
		return err
	}
	d, err := openPreparationAt(p.parent(), filepath.Base(path), syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	di, err := d.Stat()
	if err != nil {
		return errors.Join(err, d.Close())
	}
	p.directories = append(p.directories, preparationDirectory{file: d, name: filepath.Base(path), info: di})
	if !preparationPrivate(di, true) {
		return errNetworkPreparation
	}
	if observe != nil {
		observe("directory-created", d)
	}
	// Persist the incomplete directory itself before writing any signing bytes.
	if err = errors.Join(p.directories[len(p.directories)-2].file.Sync(), p.check(), ctx.Err()); err != nil {
		return err
	}
	f, err := openPreparationAt(d, name, syscall.O_RDWR|syscall.O_CREAT|syscall.O_EXCL, 0600)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			err = errors.Join(err, f.Close())
		}
	}()
	original, err := f.Stat()
	if err != nil || !preparationPrivate(original, false) {
		return errors.Join(errNetworkPreparation, err)
	}
	if observe != nil {
		observe("file-created", f)
	}
	if err = errors.Join(p.check(), ctx.Err()); err != nil {
		return err
	}
	n, writeErr := f.Write(raw)
	if writeErr == nil && n != len(raw) {
		writeErr = io.ErrShortWrite
	}
	if observe != nil {
		observe("written", f)
	}
	// Sync and Close errors survive a preceding write error independently.
	syncErr := f.Sync()
	closeErr := f.Close()
	closed = true
	if observe != nil {
		observe("closed", d)
	}
	if err = errors.Join(writeErr, syncErr, closeErr, ctx.Err()); err != nil {
		return err
	}
	readback, err := openPreparationAt(d, name, syscall.O_RDONLY, 0)
	if err != nil {
		return err
	}
	saved, readErr := io.ReadAll(io.LimitReader(readback, int64(len(raw))+1))
	info, statErr := readback.Stat()
	closeErr = readback.Close()
	if observe != nil {
		observe("readback", d)
	}
	if err = errors.Join(readErr, statErr, closeErr); err != nil {
		return err
	}
	if !bytes.Equal(saved, raw) || !os.SameFile(original, info) || !preparationPrivate(info, false) {
		return errNetworkPreparation
	}
	// Closed inventory, exact original link, directory and ancestor durability.
	entries, e := d.Readdirnames(2)
	if e != nil && e != io.EOF {
		return e
	}
	if len(entries) != 1 || entries[0] != name {
		return errNetworkPreparation
	}
	linked, e := openPreparationAt(d, name, syscall.O_RDONLY, 0)
	if e != nil {
		return e
	}
	info, e = linked.Stat()
	e = errors.Join(e, linked.Close())
	if e != nil || !os.SameFile(original, info) || !preparationPrivate(info, false) {
		return errors.Join(errNetworkPreparation, e)
	}
	if observe != nil {
		observe("before-directory-sync", d)
	}
	err = errors.Join(d.Sync(), p.directories[len(p.directories)-2].file.Sync(), p.check(), ctx.Err())
	if err != nil {
		return err
	}
	// Reopen after the durability barrier. A prior read or detached inode is
	// insufficient if the original link/bytes changed during directory I/O.
	final, err := openPreparationAt(d, name, syscall.O_RDONLY, 0)
	if err != nil {
		return err
	}
	saved, readErr = io.ReadAll(io.LimitReader(final, int64(len(raw))+1))
	info, statErr = final.Stat()
	err = errors.Join(readErr, statErr, final.Close(), p.check(), ctx.Err())
	if err != nil || !os.SameFile(original, info) || !preparationPrivate(info, false) || !bytes.Equal(saved, raw) {
		return errors.Join(errNetworkPreparation, err)
	}
	if _, err = d.Seek(0, io.SeekStart); err != nil {
		return err
	}
	entries, e = d.Readdirnames(2)
	if (e != nil && e != io.EOF) || len(entries) != 1 || entries[0] != name {
		return errors.Join(errNetworkPreparation, e)
	}
	return err
}
