package infrastructure

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/process"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

// SourceStartup owns independent observations of one actual non-root Source
// invocation. Root completion is transaction completion only: State, TLS and
// processresource still admit their own work. Copies retain the same lifetime.
type SourceStartup struct{ original *sourceStartup }

type sourceStartup struct {
	mu                      sync.Mutex // local admission/terminal state only; no foreign I/O
	ioMu                    sync.Mutex
	uses                    sync.WaitGroup
	ctx                     context.Context
	parent                  context.Context
	cancel                  context.CancelFunc
	selection               *sourceSelection
	invocation              *process.Invocation
	scope                   *cgroup.SourceScope
	identity                *sourceRuntimeIdentity
	namespace               *sourceNamespace
	manager                 *systemd.SourceRuntime
	exchange                *completion.Exchange
	id                      [16]byte
	attempt                 sourceAttemptNumber
	denied, closed          bool
	terminal                error
	closeOnce               sync.Once
	closeErr                error
	closeDone               chan struct{}
	ready, reportAttempted  bool
	completionFrame         [160]byte
	completionDirectory     string
	completionDirectoryInfo os.FileInfo
}

// SourceRuntimeInputs contains detached bytes, never Startup or State authority.
// The sole runtime consumer clears its key copies after all State/TLS users join.
// No field belongs in a public receipt or automatic JSON serialization.
type SourceRuntimeInputs struct {
	Invocation  uint32 `json:"-"`
	Plan        []byte `json:"-"`
	Certificate []byte `json:"-"`
	PrivateKey  []byte `json:"-"`
	ClientCA    []byte `json:"-"`
}

func OpenSourceStartup(parent context.Context, binding string) (result *SourceStartup, err error) {
	if parent == nil || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() || runtime.GOARCH != "amd64" {
		return nil, ErrInput
	}
	ctx, cancel := context.WithCancel(parent)
	s := &sourceStartup{ctx: ctx, parent: parent, cancel: cancel, closeDone: make(chan struct{})}
	defer func() {
		if err != nil {
			s.record(err)
			err = errors.Join(err, s.close())
			result = nil
		}
	}()
	if err = sourceInstalledPlatform(ctx); err != nil {
		return nil, err
	}
	s.selection, err = openSourceSelection(ctx, binding)
	if err != nil {
		return nil, err
	}
	b := s.selection.binding
	if err = observeSourcePrincipal(ctx, b.UID, b.GID); err != nil {
		return nil, err
	}
	rawID, e := hex.DecodeString(os.Getenv("INVOCATION_ID"))
	if e != nil || len(rawID) != 16 || hex.EncodeToString(rawID) != os.Getenv("INVOCATION_ID") {
		return nil, ErrBinding
	}
	copy(s.id[:], rawID)
	if s.id == [16]byte{} {
		return nil, ErrBinding
	}
	program := s.selection.files[filepath.Join(s.selection.sealed, ProgramName)]
	s.invocation, err = process.RetainSourceSelf(ctx, process.SourceExpected{PID: uint32(os.Getpid()), UID: b.UID, GID: b.GID, Invocation: s.id, InstallationRoot: b.Request.InstallationRoot, Generation: b.Generation, ProgramIdentity: program.info})
	if err != nil {
		return nil, err
	}
	s.scope, err = cgroup.RetainSourceSelf(ctx)
	if err != nil {
		return nil, err
	}
	s.identity, err = openSourceRuntimeIdentity(ctx)
	if err != nil {
		return nil, err
	}
	s.manager, err = systemd.OpenSourceRuntime(ctx)
	if err != nil {
		return nil, err
	}
	declaration, err := s.manager.Observe()
	if err != nil {
		return nil, err
	}
	s.namespace, err = openSourceNamespace(ctx, declaration.UnitPaths)
	if err != nil {
		return nil, err
	}
	if err = s.observe(); err != nil {
		return nil, err
	}
	s.attempt, err = latestSourceAttempt(s.selection)
	if err != nil {
		return nil, err
	}
	directory := s.attempt.directory(b.Request.InstallationRoot)
	if err = s.selection.pinDirectory(directory, 0750, 0, b.GID); err != nil {
		return nil, err
	}
	frame, e := completion.Encode(s.id, b.Generation, digest(s.selection.files[binding].raw))
	if e != nil {
		return nil, e
	}
	s.completionFrame = frame
	s.completionDirectory = directory
	s.completionDirectoryInfo = s.selection.identities[directory]
	setup, finish := context.WithTimeout(ctx, 2*time.Minute)
	defer finish()
	s.exchange, err = completion.Connect(setup, directory, s.selection.identities[directory], b.GID, frame)
	if err != nil {
		return nil, err
	}
	if err = s.observe(); err != nil {
		return nil, err
	}
	if err = s.exchange.Wait(); err != nil {
		return nil, err
	}
	if err = s.exchange.Close(); err != nil {
		return nil, err
	}
	s.exchange = nil
	if err = s.observe(); err != nil {
		return nil, err
	}
	s.ready = true
	return &SourceStartup{original: s}, nil
}

func sourceInstalledPlatform(ctx context.Context) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}
	file, err := readSourceAccountFile("/usr/lib/os-release")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.file.Close(), ctx.Err()); clear(file.raw) }()
	fields := map[string]string{}
	for _, line := range strings.Split(string(file.raw), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if _, exists := fields[key]; exists {
			return ErrBinding
		}
		fields[key] = strings.Trim(value, "\"")
	}
	if fields["ID"] != "ubuntu" || fields["VERSION_ID"] != "24.04" {
		return ErrBinding
	}
	return checkSourceAccountFile(file)
}

func (s *sourceStartup) local() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.terminal != nil {
		return s.terminal
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if s.denied || s.closed {
		return ErrBinding
	}
	return nil
}

func (s *sourceStartup) begin() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.terminal != nil {
		return s.terminal
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if s.denied || s.closed {
		return ErrBinding
	}
	s.uses.Add(1)
	return nil
}

func (s *sourceStartup) record(err error) error {
	if err == nil {
		return nil
	}
	s.mu.Lock()
	if s.terminal == nil {
		s.terminal = err
	} else if s.terminal != err {
		s.terminal = errors.Join(s.terminal, err)
	}
	result := s.terminal
	s.denied = true
	s.mu.Unlock()
	s.cancel()
	return result
}

// observe is serialized by the original I/O mutex after publication. Both
// complete manager rounds surround independent original file/process/scope
// observations; none of this foreign I/O holds the local admission lock.
func (s *sourceStartup) observe() error {
	if err := s.local(); err != nil {
		return err
	}
	b := s.selection.binding
	base := func() error {
		if err := s.selection.observe(); err != nil {
			return err
		}
		if err := observeSourcePrincipal(s.ctx, b.UID, b.GID); err != nil {
			return err
		}
		if err := s.identity.check(); err != nil {
			return err
		}
		if err := s.namespace.check(); err != nil {
			return err
		}
		if err := s.invocation.Observe(s.ctx); err != nil {
			return err
		}
		return s.scope.Observe()
	}
	if err := base(); err != nil {
		return err
	}
	for round := 0; round < 2; round++ {
		facts, err := s.manager.Observe()
		if err != nil {
			return err
		}
		if err = verifySourceRunning(facts.Unit, facts.Service, b.Request, b.Generation, uint32(os.Getpid()), s.id); err != nil {
			return err
		}
		if err = base(); err != nil {
			return err
		}
	}
	return s.local()
}

func (s *SourceStartup) Check() (err error) {
	if s == nil || s.original == nil {
		return ErrInput
	}
	o := s.original
	if err = o.begin(); err != nil {
		return err
	}
	defer o.uses.Done()
	o.ioMu.Lock()
	defer o.ioMu.Unlock()
	err = o.observe()
	if err != nil {
		return o.record(err)
	}
	return o.local()
}

func (s *SourceStartup) Context() context.Context {
	if s == nil || s.original == nil {
		return nil
	}
	return s.original.ctx
}

// Stop synchronously denies new observations/work and interrupts original
// context-bound native I/O. It releases no input, key, process or scope pin.
// Composition still joins State and all check users before physical Close.
func (s *SourceStartup) Stop() {
	if s == nil || s.original == nil {
		return
	}
	o := s.original
	o.mu.Lock()
	o.denied = true
	o.mu.Unlock()
	o.cancel()
}

func (s *SourceStartup) Inputs() (result SourceRuntimeInputs, err error) {
	if s == nil || s.original == nil {
		return result, ErrInput
	}
	o := s.original
	if err = o.begin(); err != nil {
		return result, err
	}
	defer o.uses.Done()
	o.ioMu.Lock()
	defer o.ioMu.Unlock()
	if err = o.observe(); err != nil {
		return result, o.record(err)
	}
	get := func(path string) []byte { return bytes.Clone(o.selection.files[path].raw) }
	r := o.selection.binding.Request
	result = SourceRuntimeInputs{Invocation: uint32(o.attempt), Plan: get(filepath.Join(o.selection.sealed, "source-plan.json")), Certificate: get(filepath.Join(r.CredentialDirectory, "certificate.pem")), PrivateKey: get(filepath.Join(r.CredentialDirectory, "private-key.pem")), ClientCA: get(filepath.Join(r.CredentialDirectory, "client-ca.pem"))}
	if err = o.local(); err != nil {
		clear(result.Plan)
		clear(result.Certificate)
		clear(result.PrivateKey)
		clear(result.ClientCA)
		return SourceRuntimeInputs{}, err
	}
	return result, nil
}

func (s *SourceStartup) Close() error {
	if s == nil || s.original == nil {
		return nil
	}
	return s.original.close()
}

func (s *sourceStartup) close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.denied = true
		s.closed = true
		s.mu.Unlock()
		s.cancel()
		managerErr := s.manager.Close() // interrupt before joining held observers
		s.uses.Wait()
		s.closeErr = errors.Join(s.terminal, managerErr, s.exchange.Close(), s.selection.close(), s.scope.Close(), s.namespace.close(), s.identity.close(), s.invocation.Close(), s.parent.Err())
		if s.closeDone != nil {
			close(s.closeDone)
		}
	})
	return s.closeErr
}
