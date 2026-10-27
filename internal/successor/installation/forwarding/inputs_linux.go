package forwarding

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
)

// Inputs retains one root caller's original request, plan and delivery material.
// Copies share original native custody; detached Request/Plan values grant none.
// Independent non-root startup must acquire its own installed observations.
type Inputs struct{ original *inputs }

type inputs struct {
	mu, ioMu   sync.Mutex
	uses       sync.WaitGroup
	once       sync.Once
	ctx        context.Context
	closed     bool
	installing bool
	failure    error
	request    Request
	plan       Plan
	path       string
	dirs       []*inputDirectory
	byPath     map[string]*inputDirectory
	files      []*inputFile
	// Native controls hold actual reads/Close; production never supplies facts.
	observe func(string, *os.File)
}

type inputDirectory struct {
	path, name string
	file       *os.File
	info       os.FileInfo
	parent     *inputDirectory
}

type inputFile struct {
	path, name string
	file       *os.File
	info       os.FileInfo
	parent     *inputDirectory
	raw        []byte
}

var errInputsClosed = errors.New("forwarding installation: original inputs closed")

// ReadInputs opens only existing direct root-controlled inputs on native ext4.
// Every rejected partial acquisition joins before return. It initializes no
// directory, semantic history, account, Release verifier or manager connection.
func ReadInputs(ctx context.Context, path string) (result Inputs, returned error) {
	if ctx == nil || os.Geteuid() != 0 || runtime.GOARCH != "amd64" || !validPath(path) {
		return result, ErrInput
	}
	p := &inputs{ctx: ctx, path: path, byPath: map[string]*inputDirectory{}}
	result.original = p
	defer func() {
		if returned != nil {
			p.record(returned)
			returned = result.Close()
			result = Inputs{}
		}
	}()
	if err := ctx.Err(); err != nil {
		return result, err
	}
	raw, err := p.open(path, 16<<10, true)
	if err != nil {
		return result, err
	}
	p.request, err = DecodeRequest(raw)
	if err != nil {
		return result, err
	}
	for _, declared := range p.request.ownedPaths() {
		if contains(path, declared) || contains(declared, path) {
			return result, ErrBinding
		}
	}
	for _, input := range []struct {
		path    string
		maximum int64
		private bool
	}{{p.request.Plan, 16 << 10, true}, {p.request.Certificate, 8192, false}, {p.request.PrivateKey, 8192, true}} {
		if _, err = p.open(input.path, input.maximum, input.private); err != nil {
			return result, err
		}
	}
	p.plan, err = DecodePlan(p.files[1].raw)
	if err != nil {
		return result, err
	}
	if err := p.request.MatchPlan(p.plan); err != nil {
		return result, err
	}
	pair, err := tls.X509KeyPair(p.files[2].raw, p.files[3].raw)
	if err != nil {
		return result, err
	}
	key, ok := pair.PrivateKey.(ed25519.PrivateKey)
	if !ok || len(key) != ed25519.PrivateKeySize {
		pair.PrivateKey = nil
		return result, ErrBinding
	}
	clear(key)
	pair.PrivateKey = nil
	if _, err := p.parents(p.request.Clock); err != nil {
		return result, err
	}
	return result, result.Check()
}

func (r Request) paths() []string {
	return []string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.CredentialDirectory, r.Plan, r.Certificate, r.PrivateKey, r.Clock}
}

func (h Inputs) Request() Request {
	if h.original == nil {
		return Request{}
	}
	return h.original.request
}

func (h Inputs) IndependentPaths(distribution, history string) error {
	if h.original == nil || !validPath(distribution) || !validPath(history) {
		return ErrInput
	}
	if contains(distribution, history) || contains(history, distribution) {
		return ErrBinding
	}
	paths := append(h.Request().ownedPaths(), h.original.path)
	for _, path := range paths {
		if contains(distribution, path) || contains(path, distribution) || contains(history, path) || contains(path, history) {
			return ErrBinding
		}
	}
	return h.Check()
}

func (p *inputs) begin() error {
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
		return errInputsClosed
	}
	p.uses.Add(1)
	return nil
}

func (p *inputs) local() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failure != nil {
		return p.failure
	}
	if err := p.ctx.Err(); err != nil {
		return err
	}
	if p.closed {
		return errInputsClosed
	}
	return nil
}

func (p *inputs) record(err error) {
	if err == nil || err == errInputsClosed {
		return
	}
	p.mu.Lock()
	if p.failure == nil {
		p.failure = err
	}
	p.mu.Unlock()
}

func (h Inputs) Check() (returned error) {
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
			return errors.Join(ErrBinding, err)
		}
		if p.observe != nil {
			p.observe("read", original.file)
		}
		if err := p.local(); err != nil {
			return err
		}
	}
	if err := p.clock(); err != nil {
		return err
	}
	return errors.Join(p.links(), p.local())
}

func (h Inputs) Close() error {
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

func (p *inputs) parents(path string) (*inputDirectory, error) {
	directory := filepath.Dir(path)
	var parent *inputDirectory
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
			file, err = openInputAt(parent, name, true)
		}
		if err != nil {
			return nil, err
		}
		original := &inputDirectory{path: current, name: name, file: file, parent: parent}
		p.dirs = append(p.dirs, original)
		original.info, err = file.Stat()
		if err != nil || !trustedInputDirectory(original.info) {
			return nil, errors.Join(ErrBinding, err)
		}
		p.byPath[current] = original
		parent = original
	}
	return parent, nil
}

func openInputAt(parent *inputDirectory, name string, directory bool) (*os.File, error) {
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

func (p *inputs) open(path string, maximum int64, private bool) ([]byte, error) {
	parent, err := p.parents(path)
	if err != nil {
		return nil, err
	}
	file, err := openInputAt(parent, filepath.Base(path), false)
	if err != nil {
		return nil, err
	}
	original := &inputFile{path: path, name: filepath.Base(path), file: file, parent: parent}
	p.files = append(p.files, original)
	original.info, err = file.Stat()
	if err != nil {
		return nil, err
	}
	info := original.info
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || native.Uid != 0 || native.Gid != 0 || native.Nlink != 1 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || info.Mode().Perm()&0022 != 0 || private && info.Mode().Perm() != 0600 || info.Size() < 1 || info.Size() > maximum {
		return nil, ErrBinding
	}
	var filesystem syscall.Statfs_t
	if err := syscall.Fstatfs(int(file.Fd()), &filesystem); err != nil || filesystem.Type != 0xef53 {
		return nil, errors.Join(ErrBinding, err)
	}
	raw := make([]byte, int(info.Size())+1)
	count, err := file.ReadAt(raw, 0)
	original.raw = raw[:count]
	if err != nil && err != io.EOF || int64(count) != info.Size() {
		return nil, errors.Join(ErrBinding, err)
	}
	return original.raw, p.local()
}

func trustedInputDirectory(info os.FileInfo) bool {
	if info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0
}

func sameInputFile(before, after os.FileInfo) bool {
	if before == nil || after == nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() {
		return false
	}
	a, aok := before.Sys().(*syscall.Stat_t)
	b, bok := after.Sys().(*syscall.Stat_t)
	return aok && bok && a.Uid == b.Uid && a.Gid == b.Gid && a.Nlink == b.Nlink && a.Ctim == b.Ctim && a.Mtim == b.Mtim
}

func (p *inputs) links() error {
	for _, original := range p.dirs {
		current, err := original.file.Stat()
		if err != nil || !trustedInputDirectory(current) || !os.SameFile(original.info, current) || original.info.Mode() != current.Mode() {
			return errors.Join(ErrBinding, err)
		}
		if original.parent != nil {
			linked, err := openInputAt(original.parent, original.name, true)
			if err != nil {
				return err
			}
			named, statErr := linked.Stat()
			if err := errors.Join(statErr, linked.Close()); err != nil || !trustedInputDirectory(named) || !os.SameFile(original.info, named) || original.info.Mode() != named.Mode() {
				return errors.Join(ErrBinding, err)
			}
		}
	}
	for _, original := range p.files {
		current, err := original.file.Stat()
		if err != nil || !sameInputFile(original.info, current) {
			return errors.Join(ErrBinding, err)
		}
		linked, err := openInputAt(original.parent, original.name, false)
		if err != nil {
			return err
		}
		named, statErr := linked.Stat()
		if err := errors.Join(statErr, linked.Close()); err != nil || !sameInputFile(original.info, named) {
			return errors.Join(ErrBinding, err)
		}
	}
	return nil
}

// Clock custody binds its parent/writer/access policy, not content, mtime or an
// inode across observations. State retains live clock semantics and floors.
func (p *inputs) clock() (returned error) {
	parent := p.byPath[filepath.Dir(p.request.Clock)]
	if parent == nil {
		return ErrBinding
	}
	file, err := openInputAt(parent, filepath.Base(p.request.Clock), false)
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
		return ErrBinding
	}
	named, err := os.Lstat(p.request.Clock)
	if err != nil || !os.SameFile(info, named) || info.Mode() != named.Mode() {
		return errors.Join(ErrBinding, err)
	}
	return p.local()
}
