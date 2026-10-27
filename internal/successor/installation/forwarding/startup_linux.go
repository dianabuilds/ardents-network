package forwarding

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

// Startup retains independently acquired observations of this actual non-root
// Forwarding invocation and its original root completion. It grants no State,
// duty, Guard, Admission or Hosting right. Copies share one original lifetime.
type Startup struct{ original *startup }

type startup struct {
	mu, ioMu              sync.Mutex
	uses                  sync.WaitGroup
	once                  sync.Once
	ctx, parent           context.Context
	cancel                context.CancelFunc
	selection             *runtimeSelection
	identity              *runtimeIdentity
	invocation            process.ForwardingInvocation
	scope                 *cgroup.ForwardingScope
	manager               *systemd.ForwardingRuntime
	exchange              *completion.Exchange
	absent                []string
	id                    [16]byte
	attempt               attemptNumber
	frame                 [160]byte
	directory             string
	directoryInfo         os.FileInfo
	ready, denied, closed bool
	reportAttempted       bool
	terminal, closeErr    error
	closeDone             chan struct{}
}

// RuntimeInputs are detached copies. The direct canonical consumer clears every
// private copy after its real users join. No field may enter a public receipt.
type RuntimeInputs struct {
	Invocation uint32 `json:"-"`
	Plan       []byte `json:"-"`
}

// OpenStartup cannot accept a root observation, caller-provided process/manager
// proof or historical ACK. Exact kernel self admission and a fresh original
// root-peer exchange precede every returned handle and private input handoff.
func OpenStartup(parent context.Context, bindingPath string) (result *Startup, returned error) {
	if parent == nil || runtime.GOARCH != "amd64" || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() {
		return nil, ErrInput
	}
	ctx, cancel := context.WithCancel(parent)
	s := &startup{ctx: ctx, parent: parent, cancel: cancel, closeDone: make(chan struct{})}
	defer func() {
		if returned != nil {
			s.record(returned)
			returned = errors.Join(returned, s.close())
			result = nil
		}
	}()
	if err := installedPlatform(ctx); err != nil {
		return nil, err
	}
	var err error
	s.selection, err = readRuntimeSelection(ctx, bindingPath)
	if err != nil {
		return nil, err
	}
	b := s.selection.binding
	rawID, err := hex.DecodeString(os.Getenv("INVOCATION_ID"))
	if err != nil || len(rawID) != len(s.id) || hex.EncodeToString(rawID) != os.Getenv("INVOCATION_ID") {
		return nil, ErrBinding
	}
	copy(s.id[:], rawID)
	if s.id == [16]byte{} {
		return nil, ErrBinding
	}
	program := s.selection.files.files[filepath.Join(s.selection.files.sealed, programName)]
	s.invocation, err = process.RetainForwardingSelf(ctx, process.ForwardingExpected{PID: uint32(os.Getpid()), UID: b.UID, GID: b.GID,
		Invocation: s.id, InstallationRoot: b.Request.InstallationRoot, Generation: b.Generation, ProgramIdentity: program.info})
	if err != nil {
		return nil, err
	}
	s.scope, err = cgroup.RetainForwardingSelf(ctx)
	if err != nil {
		return nil, err
	}
	s.identity, err = openRuntimeIdentity(ctx)
	if err != nil {
		return nil, err
	}
	s.manager, err = systemd.OpenForwardingRuntime(ctx)
	if err != nil {
		return nil, err
	}
	facts, err := s.manager.Observe()
	if err != nil {
		return nil, err
	}
	s.absent, err = pinStartupNamespace(s.selection, facts.UnitPaths)
	if err != nil {
		return nil, err
	}
	if err = s.observe(); err != nil {
		return nil, err
	}
	s.attempt, err = latestAttempt(s.selection)
	if err != nil {
		return nil, err
	}
	s.directory = s.attempt.directory(b.Request.InstallationRoot)
	if err := s.selection.pinStartupDirectory(s.directory); err != nil {
		return nil, err
	}
	s.directoryInfo = s.selection.files.dirs[s.directory].info
	s.frame, err = completion.Encode(s.id, b.Generation, digest(s.selection.files.files[bindingPath].body))
	if err != nil {
		return nil, err
	}
	setup, finish := context.WithTimeout(ctx, 2*time.Minute)
	defer finish()
	s.exchange, err = completion.Connect(setup, s.directory, s.directoryInfo, b.GID, s.frame)
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
	return &Startup{original: s}, nil
}

func installedPlatform(ctx context.Context) (returned error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := readNodeAccountFile("/usr/lib/os-release")
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, f.file.Close(), ctx.Err()); clear(f.raw) }()
	fields := map[string]string{}
	for _, line := range strings.Split(string(f.raw), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if _, duplicate := fields[key]; duplicate {
			return ErrBinding
		}
		fields[key] = strings.Trim(value, "\"")
	}
	if fields["ID"] != "ubuntu" || fields["VERSION_ID"] != "24.04" {
		return ErrBinding
	}
	return checkNodeAccountFile(f)
}

func (s *startup) local() error {
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

func (s *startup) begin() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.terminal != nil {
		return s.terminal
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if s.denied || s.closed || !s.ready {
		return ErrBinding
	}
	s.uses.Add(1)
	return nil
}

func (s *startup) record(err error) error {
	if err == nil {
		return nil
	}
	s.mu.Lock()
	if s.terminal == nil {
		s.terminal = err
	} else if s.terminal != err {
		s.terminal = errors.Join(s.terminal, err)
	}
	s.denied = true
	result := s.terminal
	s.mu.Unlock()
	s.cancel()
	return result
}

// Every foreign read occurs outside the short local admission mutex. The
// original physical I/O mutex serializes both complete running rounds with
// original file, namespace, account, kernel process/scope and boot observations.
func (s *startup) observe() error {
	if err := s.local(); err != nil {
		return err
	}
	base := func() error {
		if err := checkStartupNamespace(s.selection, s.absent); err != nil {
			return err
		}
		if err := s.identity.check(); err != nil {
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
	b := s.selection.binding
	for round := 0; round < 2; round++ {
		facts, err := s.manager.Observe()
		if err != nil {
			return err
		}
		if err := verifyForwardingRunning(facts.Unit, facts.Service, b.Request, b.Generation, uint32(os.Getpid()), s.id); err != nil {
			return err
		}
		if err := base(); err != nil {
			return err
		}
	}
	return s.local()
}

func (h *Startup) Check() error {
	if h == nil || h.original == nil {
		return ErrInput
	}
	s := h.original
	if err := s.begin(); err != nil {
		return err
	}
	defer s.uses.Done()
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if err := s.observe(); err != nil {
		return s.record(err)
	}
	return s.local()
}

func (h *Startup) Context() context.Context {
	if h == nil || h.original == nil {
		return nil
	}
	return h.original.ctx
}

// Stop denies synchronously and interrupts context-bound native I/O. It releases
// no observations, key, domain roots, reservations or original kernel scope.
func (h *Startup) Stop() {
	if h == nil || h.original == nil {
		return
	}
	s := h.original
	s.mu.Lock()
	s.denied = true
	s.mu.Unlock()
	s.cancel()
}

func (h *Startup) Inputs() (result RuntimeInputs, returned error) {
	if h == nil || h.original == nil {
		return result, ErrInput
	}
	s := h.original
	if err := s.begin(); err != nil {
		return result, err
	}
	defer s.uses.Done()
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if err := s.observe(); err != nil {
		return result, s.record(err)
	}
	f := s.selection.files
	get := func(path string) []byte { return bytes.Clone(f.files[path].body) }
	result = RuntimeInputs{Invocation: uint32(s.attempt), Plan: get(filepath.Join(f.sealed, "forwarding-plan.json"))}
	if err := s.local(); err != nil {
		clear(result.Plan)
		return RuntimeInputs{}, err
	}
	return result, nil
}

func (h *Startup) Close() error {
	if h == nil || h.original == nil {
		return nil
	}
	return h.original.close()
}

func (s *startup) close() error {
	s.once.Do(func() {
		s.mu.Lock()
		s.denied, s.closed = true, true
		s.mu.Unlock()
		s.cancel()
		managerErr := s.manager.Close()
		s.uses.Wait()
		s.closeErr = errors.Join(s.terminal, managerErr, s.exchange.Close(), s.selection.close(), s.scope.Close(), s.identity.close(), s.invocation.Close(), s.parent.Err())
		close(s.closeDone)
	})
	return s.closeErr
}
