//go:build linux

package main

import (
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

func requirePreparationExt4(file *os.File) error {
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(file.Fd()), &fs); err != nil || fs.Type != 0xef53 {
		return errors.Join(errNetworkPreparation, err)
	}
	return nil
}

func openPreparationParent(path string) (*preparationPath, error) {
	return openPreparationDirectories(context.Background(), path, nil)
}

func openPreparationDirectories(ctx context.Context, path string, owner *uint32) (p *preparationPath, err error) {
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
	if !preparationDirectoryOwner(info, owner) {
		return p, errNetworkPreparation
	}
	for _, name := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if name == "" {
			continue
		}
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return p, err
			}
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
		if !preparationDirectoryOwner(info, owner) {
			return p, errNetworkPreparation
		}
	}
	return p, nil
}

func preparationDirectoryOwner(info os.FileInfo, owner *uint32) bool {
	if owner == nil {
		return true
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && (stat.Uid == 0 || stat.Uid == *owner)
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

func writeNetworkPreparationReceipt(ctx context.Context, out io.Writer, raw []byte) error {
	return writeSourceReportContext(ctx, out, raw)
}
