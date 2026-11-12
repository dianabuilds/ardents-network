//go:build linux

package request

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
)

// Files retains original protected delivery bytes and every traversed ancestor.
// Copies share only this physical custody; it decodes no role declaration and
// grants no Release, installation mutation, account or startup authority.
type Files struct{ original *deliveryFiles }

type deliveryFiles struct {
	mu, ioMu sync.Mutex
	uses     sync.WaitGroup
	once     sync.Once
	ctx      context.Context
	closed   bool
	failure  error
	dirs     []*deliveryDirectory
	byPath   map[string]*deliveryDirectory
	files    []*deliveryFile
	// Native controls delay actual reads/Close; no production facts are supplied.
	observe func(string, *os.File)
}

type deliveryDirectory struct {
	path, name string
	file       *os.File
	info       os.FileInfo
	parent     *deliveryDirectory
}
type deliveryFile struct {
	path, name string
	file       *os.File
	info       os.FileInfo
	parent     *deliveryDirectory
	raw        []byte
}

func deliveryPath(path string) bool {
	return canonicalPath(path) && path != "/" && len(path) <= 4096 && len(strings.Split(path, "/")) <= 128
}

var errFilesClosed = errors.New("installation request: original delivery files closed")

// OpenFiles creates one empty original root caller's custody. Read and
// RetainParents admit bounded existing inputs only; they create no files.
func OpenFiles(ctx context.Context) (Files, error) {
	if ctx == nil || os.Geteuid() != 0 || runtime.GOARCH != "amd64" {
		return Files{}, ErrNativeUnavailable
	}
	if err := ctx.Err(); err != nil {
		return Files{}, err
	}
	return Files{original: &deliveryFiles{ctx: ctx, byPath: map[string]*deliveryDirectory{}}}, nil
}

// Read retains one direct root-owned ext4 regular file, at most 1 MiB. The
// returned copy belongs to the caller and must be cleared for private material.
// Repeated reads use the retained original; a changed path never rebinds it.
func (h Files) Read(path string, maximum int64, private bool) (raw []byte, returned error) {
	p := h.original
	if p == nil || !deliveryPath(path) || maximum < 1 || maximum > 1<<20 {
		return nil, ErrInput
	}
	if err := p.begin(); err != nil {
		return nil, err
	}
	defer p.uses.Done()
	p.ioMu.Lock()
	defer p.ioMu.Unlock()
	defer func() {
		p.record(returned)
		if returned != nil {
			clear(raw)
			raw = nil
		}
	}()
	if err := p.local(); err != nil {
		return nil, err
	}
	for _, original := range p.files {
		if original.path == path {
			// Repetition cannot weaken the original access/size declaration.
			if original.info.Size() > maximum || private && original.info.Mode().Perm() != 0600 {
				return nil, ErrChanged
			}
			if err := p.check(); err != nil {
				return nil, err
			}
			return append([]byte(nil), original.raw...), nil
		}
	}
	if len(p.files) >= 16 {
		return nil, ErrInput
	}
	body, err := p.open(path, maximum, private)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), body...), nil
}

// RetainParents pins existing trusted ancestors of a declared leaf. The leaf
// itself is neither opened nor required absent, and no creation fact is returned.
func (h Files) RetainParents(path string) (returned error) {
	p := h.original
	if p == nil || !deliveryPath(path) {
		return ErrInput
	}
	if err := p.begin(); err != nil {
		return err
	}
	defer p.uses.Done()
	p.ioMu.Lock()
	defer p.ioMu.Unlock()
	defer func() { p.record(returned) }()
	if err := p.local(); err != nil {
		return err
	}
	if _, err := p.parents(path); err != nil {
		return err
	}
	if err := p.links(); err != nil {
		return err
	}
	return p.local()
}

// CheckClock reopens a live root-written 0644 clock leaf under retained
// ancestors. It freezes neither time, contents nor inode; State owns time
// acceptance and floors. The original parent must already be retained.
func (h Files) CheckClock(path string) (returned error) {
	p := h.original
	if p == nil || !deliveryPath(path) {
		return ErrInput
	}
	if err := p.begin(); err != nil {
		return err
	}
	defer p.uses.Done()
	p.ioMu.Lock()
	defer p.ioMu.Unlock()
	defer func() { p.record(returned) }()
	if err := errors.Join(p.local(), p.links()); err != nil {
		return err
	}
	parent := p.byPath[filepath.Dir(path)]
	if parent == nil {
		return ErrChanged
	}
	file, err := openDeliveryAt(parent, filepath.Base(path), false)
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm() != 0644 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || native.Uid != 0 || native.Gid != 0 || native.Nlink != 1 {
		return ErrChanged
	}
	named, err := os.Lstat(path)
	if err != nil || !os.SameFile(info, named) || info.Mode() != named.Mode() {
		return errors.Join(ErrChanged, err)
	}
	if err := p.links(); err != nil {
		return err
	}
	return p.local()
}

func (p *deliveryFiles) begin() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failure != nil {
		return p.failure
	}
	if err := p.ctx.Err(); err != nil {
		// A refusal after completed Close cannot rewrite its retained result.
		// Closing users still record their own original failures before join.
		if !p.closed {
			p.failure = err
		}
		return err
	}
	if p.closed {
		return errFilesClosed
	}
	p.uses.Add(1)
	return nil
}

func (p *deliveryFiles) local() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failure != nil {
		return p.failure
	}
	if err := p.ctx.Err(); err != nil {
		return err
	}
	if p.closed {
		return errFilesClosed
	}
	return nil
}

func (p *deliveryFiles) record(err error) {
	if err == nil || err == errFilesClosed {
		return
	}
	p.mu.Lock()
	if p.failure == nil {
		p.failure = err
	}
	p.mu.Unlock()
}

func (h Files) Check() (returned error) {
	p := h.original
	if p == nil {
		return ErrInput
	}
	if err := p.begin(); err != nil {
		return err
	}
	defer p.uses.Done()
	p.ioMu.Lock()
	defer p.ioMu.Unlock()
	defer func() { p.record(returned) }()
	return p.check()
}

func (p *deliveryFiles) check() error {
	if err := p.local(); err != nil {
		return err
	}
	if err := p.links(); err != nil {
		return err
	}
	for _, original := range p.files {
		raw := make([]byte, len(original.raw)+1)
		count, err := original.file.ReadAt(raw, 0)
		matches := (err == nil || err == io.EOF) && count == len(original.raw) && bytes.Equal(raw[:count], original.raw)
		clear(raw)
		if !matches {
			return errors.Join(ErrChanged, err)
		}
		if p.observe != nil {
			p.observe("read", original.file)
		}
		if err := p.local(); err != nil {
			return err
		}
	}
	if err := p.links(); err != nil {
		return err
	}
	return p.local()
}

func (h Files) Close() error {
	p := h.original
	if p == nil {
		return nil
	}
	p.once.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()
		if p.observe != nil {
			p.observe("denied", nil)
		}
		p.uses.Wait()
		var physical error
		for i := len(p.files) - 1; i >= 0; i-- {
			original := p.files[i]
			if p.observe != nil {
				p.observe("before-close", original.file)
			}
			physical = errors.Join(physical, original.file.Close())
			clear(original.raw)
		}
		for i := len(p.dirs) - 1; i >= 0; i-- {
			physical = errors.Join(physical, p.dirs[i].file.Close())
		}
		p.mu.Lock()
		p.failure = errors.Join(p.failure, p.ctx.Err(), physical)
		p.mu.Unlock()
	})
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.failure
}

func (p *deliveryFiles) parents(path string) (*deliveryDirectory, error) {
	if len(p.dirs) >= 4096 {
		return nil, ErrInput
	}
	directory := filepath.Dir(path)
	var parent *deliveryDirectory
	parts := append([]string{""}, strings.Split(strings.TrimPrefix(directory, "/"), "/")...)
	current := "/"
	for i, name := range parts {
		if i > 0 && name == "" {
			continue
		}
		if i > 0 {
			current = filepath.Join(current, name)
		}
		if retained := p.byPath[current]; retained != nil {
			parent = retained
			continue
		}
		var file *os.File
		var err error
		if parent == nil {
			file, err = os.OpenFile("/", os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
		} else {
			file, err = openDeliveryAt(parent, name, true)
		}
		if err != nil {
			return nil, err
		}
		original := &deliveryDirectory{path: current, name: name, file: file, parent: parent}
		p.dirs = append(p.dirs, original)
		original.info, err = file.Stat()
		if err != nil || !trustedDeliveryDirectory(original.info) {
			return nil, errors.Join(ErrChanged, err)
		}
		p.byPath[current] = original
		parent = original
	}
	return parent, nil
}

func openDeliveryAt(parent *deliveryDirectory, name string, directory bool) (*os.File, error) {
	flags := syscall.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC
	if directory {
		flags |= syscall.O_DIRECTORY
	}
	fd, err := syscall.Openat(int(parent.file.Fd()), name, flags, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}

func (p *deliveryFiles) open(path string, maximum int64, private bool) ([]byte, error) {
	parent, err := p.parents(path)
	if err != nil {
		return nil, err
	}
	file, err := openDeliveryAt(parent, filepath.Base(path), false)
	if err != nil {
		return nil, err
	}
	original := &deliveryFile{path: path, name: filepath.Base(path), file: file, parent: parent}
	p.files = append(p.files, original)
	original.info, err = file.Stat()
	if err != nil {
		return nil, err
	}
	info := original.info
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || native.Uid != 0 || native.Gid != 0 || native.Nlink != 1 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || info.Mode().Perm()&0022 != 0 || private && info.Mode().Perm() != 0600 || info.Size() < 1 || info.Size() > maximum {
		return nil, ErrChanged
	}
	var filesystem syscall.Statfs_t
	if err := syscall.Fstatfs(int(file.Fd()), &filesystem); err != nil || filesystem.Type != 0xef53 {
		return nil, errors.Join(ErrChanged, err)
	}
	raw := make([]byte, int(info.Size())+1)
	count, err := file.ReadAt(raw, 0)
	original.raw = raw[:count]
	if err != nil && err != io.EOF || int64(count) != info.Size() {
		return nil, errors.Join(ErrChanged, err)
	}
	if err := p.links(); err != nil {
		return nil, err
	}
	return original.raw, p.local()
}

func trustedDeliveryDirectory(info os.FileInfo) bool {
	if info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0
}

func sameDeliveryFile(before, after os.FileInfo) bool {
	if before == nil || after == nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() {
		return false
	}
	a, aok := before.Sys().(*syscall.Stat_t)
	b, bok := after.Sys().(*syscall.Stat_t)
	return aok && bok && a.Uid == b.Uid && a.Gid == b.Gid && a.Nlink == b.Nlink && a.Ctim == b.Ctim && a.Mtim == b.Mtim
}

func (p *deliveryFiles) links() error {
	for _, original := range p.dirs {
		current, err := original.file.Stat()
		if err != nil || !trustedDeliveryDirectory(current) || !os.SameFile(original.info, current) || original.info.Mode() != current.Mode() {
			return errors.Join(ErrChanged, err)
		}
		if original.parent != nil {
			linked, err := openDeliveryAt(original.parent, original.name, true)
			if err != nil {
				return err
			}
			named, statErr := linked.Stat()
			if err := errors.Join(statErr, linked.Close()); err != nil || !trustedDeliveryDirectory(named) || !os.SameFile(original.info, named) || original.info.Mode() != named.Mode() {
				return errors.Join(ErrChanged, err)
			}
		}
	}
	for _, original := range p.files {
		current, err := original.file.Stat()
		if err != nil || !sameDeliveryFile(original.info, current) {
			return errors.Join(ErrChanged, err)
		}
		linked, err := openDeliveryAt(original.parent, original.name, false)
		if err != nil {
			return err
		}
		named, statErr := linked.Stat()
		if err := errors.Join(statErr, linked.Close()); err != nil || !sameDeliveryFile(original.info, named) {
			return errors.Join(ErrChanged, err)
		}
	}
	return nil
}
