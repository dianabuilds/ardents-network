package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// SourceInputs retains original request/input descriptors and direct ancestors.
// Copies share the same original custody, cancellation and refusal latch.
type SourceInputs struct{ original *sourceInputs }
type sourceInputs struct {
	mu                 sync.Mutex
	ioMu               sync.Mutex
	uses               sync.WaitGroup
	closeOnce          sync.Once
	ctx                context.Context
	request            SourceRequest
	files              []*sourceFile
	ancestors          map[string]os.FileInfo
	namespaceAncestors map[string]os.FileInfo
	closed             bool
	terminal           error
	managerIdentity    string
	requestPath        string
	// Synchronous test barrier after original physical reads. It cannot supply
	// bytes or an accepting fact; production acquisition always leaves it nil.
	observe func(*os.File)
}
type sourceFile struct {
	path string
	file *os.File
	info os.FileInfo
	raw  []byte
}

// ReadSourceInputs opens only existing inputs. Future destinations stay absent;
// private bytes are retained solely until joined Close, never in public facts.
// The caller serializes use; this finite operation starts no background worker.
func ReadSourceInputs(ctx context.Context, path string) (result SourceInputs, err error) {
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || runtime.GOARCH != "amd64" || !sourcePath(path) {
		return result, ErrInput
	}
	s := &sourceInputs{ctx: ctx, ancestors: map[string]os.FileInfo{}, namespaceAncestors: map[string]os.FileInfo{}, requestPath: path}
	result.original = s
	defer func() {
		if err != nil {
			s.terminal = err
		}
	}()
	raw, err := s.open(path, 16<<10, true)
	if err != nil {
		return result, err
	}
	s.request, err = DecodeSourceRequest(raw)
	if err != nil {
		return result, err
	}
	for _, p := range s.request.paths() {
		if contains(p, path) || contains(path, p) {
			return result, ErrBinding
		}
	}
	for _, input := range []struct {
		path    string
		maximum int64
		private bool
	}{
		{s.request.Plan, 16 << 10, true}, {s.request.Certificate, 64 << 10, false}, {s.request.PrivateKey, 16 << 10, true}, {s.request.ClientCA, 64 << 10, false},
	} {
		if _, err = s.open(input.path, input.maximum, input.private); err != nil {
			return result, err
		}
	}
	if err = s.request.checkPlan(s.files[1].raw); err != nil {
		return result, err
	}
	return result, result.Check()
}

// Declaration returns detached data; successful checks require original
// descriptors and caller. No decoded struct or receipt can recreate custody.
func (s SourceInputs) Declaration() SourceRequest {
	if s.original == nil {
		return SourceRequest{}
	}
	return s.original.request
}

// IndependentPaths is checked before Release opens history. Authentication's
// separately owned floor effects must never create a future runtime resource.
func (s SourceInputs) IndependentPaths(distribution, history string) error {
	if s.original == nil || !sourcePath(distribution) || !sourcePath(history) {
		return ErrInput
	}
	paths := append(s.Declaration().paths(), s.original.requestPath)
	for _, p := range paths {
		if contains(p, distribution) || contains(distribution, p) || contains(p, history) || contains(history, p) {
			return ErrBinding
		}
	}
	return s.Check()
}
func (s *sourceInputs) begin() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.terminal != nil {
		return s.terminal
	}
	if s.closed {
		return ErrInput
	}
	if err := s.ctx.Err(); err != nil {
		s.terminal = err
		return err
	}
	s.uses.Add(1)
	return nil
}

func (s *sourceInputs) local() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.terminal != nil {
		return s.terminal
	}
	if s.closed {
		return ErrInput
	}
	return s.ctx.Err()
}

func trustedDirectory(info os.FileInfo) bool {
	if info == nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Uid == 0 && st.Gid == 0
}
func (s *sourceInputs) parents(path string) error {
	for p := filepath.Dir(path); ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil || !trustedDirectory(info) {
			return errors.Join(ErrBinding, err)
		}
		if original, ok := s.ancestors[p]; ok && (!os.SameFile(original, info) || original.Mode() != info.Mode()) {
			return ErrBinding
		}
		s.ancestors[p] = info
		if p == "/" {
			return nil
		}
	}
}
func (s *sourceInputs) open(path string, maximum int64, private bool) ([]byte, error) {
	if err := s.parents(path); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	f := &sourceFile{path: path, file: file}
	s.files = append(s.files, f)
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || st.Uid != 0 || st.Gid != 0 || st.Nlink != 1 || info.Mode().Perm()&0022 != 0 || private && info.Mode().Perm() != 0600 || info.Size() <= 0 || info.Size() > maximum {
		return nil, ErrBinding
	}
	var fs syscall.Statfs_t
	if err := syscall.Fstatfs(int(file.Fd()), &fs); err != nil || fs.Type != 0xef53 {
		return nil, errors.Join(ErrBinding, err)
	}
	raw, err := io.ReadAll(io.LimitReader(file, maximum+1))
	f.raw = raw // partial private reads remain owned until the original Close
	if err != nil || int64(len(raw)) != info.Size() {
		return nil, errors.Join(ErrBinding, err)
	}
	f.info = info
	return raw, nil
}
func (s SourceInputs) Check() (err error) {
	return s.checkAccountPhase(nil)
}

// Only the initial transition may supply its original successfully created
// account custody. Public copies and read-only preflight retain strict checks.
func (s SourceInputs) checkAccountPhase(account *sourceAccount) (err error) {
	return s.checkSourcePhase(account, nil)
}

func (s SourceInputs) checkSourcePhase(account *sourceAccount, namespace map[string]os.FileInfo) (err error) {
	p := s.original
	if p == nil {
		return ErrInput
	}
	if err = p.begin(); err != nil {
		return err
	}
	defer p.uses.Done()
	p.ioMu.Lock()
	defer p.ioMu.Unlock()
	defer func() {
		if err != nil {
			p.mu.Lock()
			p.terminal = errors.Join(p.terminal, err)
			p.mu.Unlock()
		}
	}()
	if err := p.local(); err != nil {
		return err
	}
	if err := p.ctx.Err(); err != nil {
		return err
	}
	if err := p.checkSourcePaths(account, namespace); err != nil {
		return err
	}
	for _, f := range p.files {
		if account != nil && account.replaces(f.path) {
			if err := account.checkOriginal(f); err != nil {
				return err
			}
			continue
		}
		if _, err = f.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		raw, err := io.ReadAll(io.LimitReader(f.file, int64(len(f.raw))+1))
		equal := bytes.Equal(raw, f.raw)
		clear(raw)
		if err != nil || !equal {
			return errors.Join(ErrBinding, err)
		}
		if p.observe != nil {
			p.observe(f.file)
		}
		current, err := f.file.Stat()
		if err != nil || !sameSourceFile(f.info, current) {
			return errors.Join(ErrBinding, err)
		}
	}
	// A read can yield while a named file or an original ancestor changes.
	// Reobserve the complete retained namespace after every physical read has
	// joined, not only each open fd's metadata before its read.
	if err := p.checkSourcePaths(account, namespace); err != nil {
		return err
	}
	return p.local()
}
func (p *sourceInputs) checkSourcePaths(account *sourceAccount, namespace map[string]os.FileInfo) error {
	for path, before := range p.ancestors {
		now, err := os.Lstat(path)
		if err != nil || !trustedDirectory(now) || !os.SameFile(before, now) || before.Mode() != now.Mode() {
			return fmt.Errorf("original Source ancestor %s: %w", path, errors.Join(ErrBinding, err))
		}
	}
	if namespace == nil {
		namespace = p.namespaceAncestors
	}
	for path, before := range namespace {
		now, err := os.Lstat(path)
		if before == nil && os.IsNotExist(err) {
			continue
		}
		if err != nil || before == nil || !trustedDirectory(now) || !os.SameFile(before, now) || before.Mode() != now.Mode() {
			return fmt.Errorf("source unit namespace ancestor %s: %w", path, errors.Join(ErrBinding, err))
		}
	}
	for _, f := range p.files {
		if account != nil && account.replaces(f.path) {
			continue
		}
		current, err := f.file.Stat()
		named, namedErr := os.Lstat(f.path)
		if err != nil || namedErr != nil || !sameSourceFile(f.info, current) || !sameSourceFile(f.info, named) {
			return errors.Join(ErrBinding, err, namedErr)
		}
	}
	if account != nil {
		return account.check()
	}
	return nil
}
func sameSourceFile(a, b os.FileInfo) bool {
	if a == nil || b == nil || !os.SameFile(a, b) || a.Mode() != b.Mode() || a.Size() != b.Size() {
		return false
	}
	x, xok := a.Sys().(*syscall.Stat_t)
	y, yok := b.Sys().(*syscall.Stat_t)
	return xok && yok && x.Uid == y.Uid && x.Gid == y.Gid && x.Nlink == y.Nlink && x.Ctim == y.Ctim && x.Mtim == y.Mtim
}
func (s SourceInputs) Close() error {
	p := s.original
	if p == nil {
		return nil
	}
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()
		p.uses.Wait()
		var closeErr error
		for i := len(p.files) - 1; i >= 0; i-- {
			f := p.files[i]
			closeErr = errors.Join(closeErr, f.file.Close())
			clear(f.raw)
		}
		p.mu.Lock()
		p.terminal = errors.Join(p.terminal, closeErr)
		p.mu.Unlock()
	})
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.terminal
}

// ObserveSource performs a bounded read-only check for a fresh stopped Source.
// Release-owned floor authentication happened separately; no installation,
// account, generation, credential, runtime root or listener is created here.
func ObserveSource(v *release.Verifier, a Authorization, inputs SourceInputs, distribution, history string) (err error) {
	if inputs.original == nil {
		return ErrInput
	}
	if err = inputs.original.begin(); err != nil {
		return err
	}
	defer inputs.original.uses.Done()
	ctx := inputs.original.ctx
	r := inputs.Declaration()
	if err = inputs.IndependentPaths(distribution, history); err != nil {
		return err
	}
	if err = a.Check(ctx, v); err != nil {
		return err
	}
	if _, err = RenderSourceUnit(a.Template(), r, digest(a.Descriptor())); err != nil {
		return err
	}
	manager, err := systemd.OpenSourcePreflight(ctx)
	defer func() { err = errors.Join(err, manager.Close()) }()
	if err != nil {
		return fmt.Errorf("source preflight manager acquisition: %w", err)
	}
	for round := 0; round < 2; round++ {
		if err = inputs.Check(); err != nil {
			return err
		}
		paths, e := manager.Check()
		if e != nil {
			return fmt.Errorf("source preflight manager observation: %w", e)
		}
		if err = inputs.observeFresh(paths); err != nil {
			return fmt.Errorf("source preflight native namespace: %w", err)
		}
		if err = inputs.Check(); err != nil {
			return err
		}
		if err = a.Check(ctx, v); err != nil {
			return err
		}
	}
	// The complete second manager round follows all independent native I/O;
	// original file/caller/Release checks still precede the final handoff.
	if _, err = manager.Check(); err != nil {
		return err
	}
	return errors.Join(inputs.Check(), a.Check(ctx, v), ctx.Err())
}

func (input SourceInputs) observeFresh(unitPaths []string) (err error) {
	s := input.original
	if s == nil {
		return ErrInput
	}
	if err = s.begin(); err != nil {
		return err
	}
	defer s.uses.Done()
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	defer func() {
		if err != nil {
			s.mu.Lock()
			s.terminal = errors.Join(s.terminal, err)
			s.mu.Unlock()
		}
	}()
	if err := s.ctx.Err(); err != nil {
		return err
	}
	for _, p := range []string{s.request.InstallationRoot, s.request.StateRoot, s.request.ParticipationRoot, s.request.CredentialDirectory} {
		if err := s.absent(p); err != nil {
			return err
		}
		for parent := filepath.Dir(p); ; parent = filepath.Dir(parent) {
			info, e := os.Lstat(parent)
			if os.IsNotExist(e) {
				continue
			}
			if e != nil || !trustedDirectory(info) || info.Mode().Perm()&0001 == 0 {
				return errors.Join(ErrBinding, e)
			}
			var storage syscall.Statfs_t
			if e = syscall.Statfs(parent, &storage); e != nil || storage.Type != 0xef53 || storage.Flags&1 != 0 {
				return errors.Join(ErrBinding, e)
			}
			break
		}
		for parent := filepath.Dir(p); parent != "/"; parent = filepath.Dir(parent) {
			if info, e := os.Lstat(parent); e == nil && info.Mode().Perm()&0001 == 0 {
				return ErrBinding
			}
		}
	}
	// NSS is constrained to the local files on this stand. The systemd module
	// additionally exposes dynamic accounts, so its presence is not silently
	// treated as an absent local identity.
	for _, p := range []string{"/etc/passwd", "/etc/group", "/etc/nsswitch.conf", "/usr/lib/os-release", "/usr/bin/getent"} {
		if !s.has(p) {
			if _, err := s.open(p, 1<<20, false); err != nil {
				return err
			}
		}
	}
	for _, f := range s.files {
		if f.path == "/etc/passwd" || f.path == "/etc/group" {
			for _, line := range strings.Split(string(f.raw), "\n") {
				if strings.HasPrefix(line, "ardents-source:") {
					return ErrBinding
				}
			}
		}
		if f.path == "/usr/lib/os-release" {
			fields := map[string]string{}
			for _, line := range strings.Split(string(f.raw), "\n") {
				k, v, ok := strings.Cut(line, "=")
				if ok {
					if _, duplicate := fields[k]; duplicate {
						return ErrBinding
					}
					fields[k] = strings.Trim(v, "\"")
				}
			}
			if fields["ID"] != "ubuntu" || fields["VERSION_ID"] != "24.04" {
				return ErrBinding
			}
		}
		if f.path == "/etc/nsswitch.conf" {
			if err := validateSourceNSS(f.raw); err != nil {
				return err
			}
		}
	}
	for _, database := range []string{"passwd", "group"} {
		ctx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
		cmd := exec.CommandContext(ctx, "/usr/bin/getent", database, "ardents-source")
		cmd.Env = []string{"LANG=C", "PATH=/usr/bin:/bin"}
		// No output is needed: an existing identity, including a dynamic one,
		// refuses. Run joins the original helper even after deadline/kill.
		runtime.LockOSThread()
		cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
		err := cmd.Run()
		runtime.UnlockOSThread()
		contextErr := ctx.Err()
		cancel()
		var exit *exec.ExitError
		if contextErr != nil || !errors.As(err, &exit) || exit.ExitCode() != 2 {
			return errors.Join(ErrBinding, err, contextErr)
		}
	}
	if err := s.observeClock(); err != nil {
		return err
	}
	identity, err := sourceManagerIdentity()
	if err != nil {
		return err
	}
	if s.managerIdentity != "" && s.managerIdentity != identity {
		return ErrBinding
	}
	s.managerIdentity = identity
	var fs syscall.Statfs_t
	if err := syscall.Statfs("/sys/fs/cgroup", &fs); err != nil || fs.Type != 0x63677270 {
		return errors.Join(ErrBinding, err)
	}
	controllers, e := os.ReadFile("/sys/fs/cgroup/cgroup.controllers")
	if e != nil || len(controllers) > 4096 {
		return errors.Join(ErrBinding, e)
	}
	available := strings.Fields(string(controllers))
	for _, required := range []string{"cpu", "memory", "pids"} {
		found := false
		for _, name := range available {
			if name == required {
				found = true
			}
		}
		if !found {
			return ErrBinding
		}
	}
	for _, dir := range unitPaths {
		if !sourcePath(dir) {
			return ErrBinding
		}
		for _, leaf := range []string{"ardents-source.service", "ardents-source.service.d", "ardents-source.socket", "ardents-source.socket.d", "ardents-.service.d", "service.d", "socket.d"} {
			if err := s.absentUnit(filepath.Join(dir, leaf)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *sourceInputs) observeClock() (err error) {
	p := s.request.Clock
	if err = s.parents(p); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || st.Uid != 0 || st.Gid != 0 || st.Nlink != 1 || info.Mode().Perm() != 0644 {
		return ErrBinding
	}
	named, err := os.Lstat(p)
	if err != nil || !os.SameFile(info, named) {
		return errors.Join(ErrBinding, err)
	}
	// Deliberately retain neither inode, mtime nor content across rounds. This
	// is a root-writer/read-only runtime declaration; State owns live evidence.
	return s.ctx.Err()
}
func sourceManagerIdentity() (identity string, err error) {
	// The caller's filesystem and process observations must describe the same
	// original manager namespace, not a host bus borrowed through another root.
	for _, namespace := range []string{"mnt", "pid", "cgroup", "uts"} {
		manager, e := os.Stat("/proc/1/ns/" + namespace)
		caller, ce := os.Stat("/proc/self/ns/" + namespace)
		if e != nil || ce != nil || !os.SameFile(manager, caller) {
			return "", errors.Join(ErrBinding, e, ce)
		}
		st, ok := manager.Sys().(*syscall.Stat_t)
		if !ok {
			return "", ErrBinding
		}
		identity += fmt.Sprintf("%s:%d:%d;", namespace, st.Dev, st.Ino)
	}
	for _, p := range []string{"/proc/sys/kernel/random/boot_id", "/proc/1/stat"} {
		f, e := os.OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if e != nil {
			return "", e
		}
		raw, e := io.ReadAll(io.LimitReader(f, 4097))
		e = errors.Join(e, f.Close())
		if e != nil || len(raw) > 4096 {
			return "", errors.Join(ErrBinding, e)
		}
		if p == "/proc/1/stat" {
			_, tail, ok := strings.Cut(string(raw), ") ")
			fields := strings.Fields(tail)
			if !ok || len(fields) < 20 {
				return "", ErrBinding
			}
			identity += fields[19]
		} else {
			if len(strings.TrimSpace(string(raw))) != 36 {
				return "", ErrBinding
			}
			identity += string(raw)
		}
	}
	return identity, nil
}
func (s *sourceInputs) has(path string) bool {
	for _, f := range s.files {
		if f.path == path {
			return true
		}
	}
	return false
}

// Installation's phase-specific native observations retain the same original
// input serialization. They never change read-only preflight's absence policy.
func (input SourceInputs) prepareInstallAncestors() error {
	s := input.original
	if s == nil {
		return ErrInput
	}
	if err := s.begin(); err != nil {
		return err
	}
	defer s.uses.Done()
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	for _, path := range []string{s.request.InstallationRoot, s.request.StateRoot, s.request.ParticipationRoot, s.request.CredentialDirectory, "/etc/shadow", "/etc/gshadow", "/etc/default/useradd", "/etc/login.defs", "/usr/sbin/useradd", "/usr/sbin/nologin"} {
		if err := s.parents(path); err != nil {
			return err
		}
	}
	return s.local()
}

func (input SourceInputs) observeInstallEnvironment() error {
	s := input.original
	if s == nil {
		return ErrInput
	}
	if err := s.begin(); err != nil {
		return err
	}
	defer s.uses.Done()
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	identity, err := sourceManagerIdentity()
	if err != nil || identity != s.managerIdentity {
		return errors.Join(ErrBinding, err)
	}
	if err = s.observeClock(); err != nil {
		return err
	}
	return s.local()
}
func (s *sourceInputs) absent(path string) error {
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return errors.Join(ErrBinding, err)
	}
	// Missing suffixes are future resources. Pin every existing ancestor and
	// reject foreign/symlink ancestors without requiring a completed install.
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !trustedDirectory(info) {
			return errors.Join(ErrBinding, err)
		}
		if before, ok := s.ancestors[parent]; ok && (!os.SameFile(before, info) || before.Mode() != info.Mode()) {
			return ErrBinding
		}
		s.ancestors[parent] = info
		if parent == "/" {
			break
		}
	}
	return s.ctx.Err()
}

// Unit search locations are manager observations, distinct from original input
// and destination ancestors. Read-only preflight pins every original location.
func (s *sourceInputs) absentUnit(path string) error {
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return errors.Join(ErrBinding, err)
	}
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !trustedDirectory(info) {
			return errors.Join(ErrBinding, err)
		}
		before, originalInput := s.ancestors[parent]
		if !originalInput {
			before = s.namespaceAncestors[parent]
		}
		if before != nil && (!os.SameFile(before, info) || before.Mode() != info.Mode()) {
			return ErrBinding
		}
		if !originalInput {
			s.namespaceAncestors[parent] = info
		}
		if parent == "/" {
			return s.ctx.Err()
		}
	}
}

// Only the initial transition calls this after its successful one-shot reload
// on the original manager. systemd recreates precisely these generator output
// directories on reload. Input/destination pins and every other namespace pin
// remain original; this returns separate phase custody and never updates them.
func (input SourceInputs) observeReloadedNamespace() (map[string]os.FileInfo, error) {
	p := input.original
	if p == nil {
		return nil, ErrInput
	}
	if err := p.begin(); err != nil {
		return nil, err
	}
	defer p.uses.Done()
	p.ioMu.Lock()
	defer p.ioMu.Unlock()
	result := make(map[string]os.FileInfo, len(p.namespaceAncestors))
	for path, before := range p.namespaceAncestors {
		now, err := os.Lstat(path)
		transient := path == "/run/systemd/generator" || path == "/run/systemd/generator.early" || path == "/run/systemd/generator.late"
		if transient && os.IsNotExist(err) {
			result[path] = nil
			continue
		}
		if err != nil || !trustedDirectory(now) || !transient && (!os.SameFile(before, now) || before.Mode() != now.Mode()) {
			return nil, errors.Join(ErrBinding, err)
		}
		result[path] = now
	}
	return result, p.local()
}
