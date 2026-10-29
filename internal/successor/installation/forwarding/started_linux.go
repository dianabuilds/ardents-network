package forwarding

import (
	"context"
	"errors"
	"os"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

var errStartedClosed = errors.New("forwarding installation: original start observation closed")

// Started retains independently reopened complete invocation-chain bytes and native file
// identities. It grants no current process, domain join, Stop or restart right.
// Copies share original denial, actual uses and physically joined Close.
type Started struct{ original *started }

type started struct {
	*invocationChain
	ctx       context.Context
	verifier  *release.Verifier
	completed Completed
	mu        sync.Mutex
	uses      sync.WaitGroup
	once      sync.Once
	closed    bool
	failure   error
}

// ReadStarted is root-only and noncreating. It requires a complete ordered
// chain of published starts or never-started terminal entries, every runtime
// predecessor retirement and independently held initial/child
// journal leases. Expiry of a Release
// cannot prevent shutdown: this observation matches retained history identity,
// without granting fresh artifact admission. Restart separately requires that.
func ReadStarted(ctx context.Context, v *release.Verifier, bindingPath, history string) (result Started, returned error) {
	if ctx == nil || v == nil || os.Geteuid() != 0 || !validPath(bindingPath) || !validPath(history) {
		return result, ErrInput
	}
	s := &started{ctx: ctx, verifier: v}
	defer func() {
		if returned != nil {
			s.failure = returned
			returned = errors.Join(returned, s.close())
			result = Started{}
		}
	}()
	var err error
	s.completed, err = ReadCompleted(ctx, bindingPath)
	if err != nil {
		return result, err
	}
	s.invocationChain = s.completed.original.files.invocations
	if s.invocationChain == nil || s.head() == nil {
		return result, os.ErrNotExist
	}
	if err := s.completed.CheckHistoryPath(history); err != nil {
		return result, err
	}
	if err := v.CheckHistoryPath(ctx, history); err != nil {
		return result, err
	}
	result = Started{original: s}
	return result, result.Check()
}

func (s *started) begin() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return s.failure
	}
	if s.closed {
		return errStartedClosed
	}
	if err := s.ctx.Err(); err != nil {
		s.failure = err
		return err
	}
	s.uses.Add(1)
	return nil
}

func (s *started) local() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return s.failure
	}
	if s.closed {
		return errStartedClosed
	}
	return s.ctx.Err()
}

func (s *started) record(err error) {
	if err == nil || err == errStartedClosed {
		return
	}
	s.mu.Lock()
	if s.failure == nil {
		s.failure = err
	}
	s.mu.Unlock()
}

func (h Started) Check() error { return h.checkWithEffects(nil, nil) }

// Only this Installation owner supplies its own exclusive newborn storage.
// This permits no caller-supplied native success or borrowed live root.
func (h Started) checkWithEffects(start *startStorage, retirement *retirementStorage) (returned error) {
	s := h.original
	if s == nil {
		return ErrInput
	}
	if err := s.begin(); err != nil {
		return err
	}
	defer s.uses.Done()
	defer func() { s.record(returned) }()
	if s.invocationChain == nil || s.verifier == nil {
		return ErrInput
	}
	if err := s.verifier.CheckHistoryPath(s.ctx, s.completed.original.binding.History); err != nil {
		return err
	}
	if err := s.completed.Check(); err != nil {
		return err
	}
	s.ioMu.Lock()
	err := s.checkChain(start, retirement)
	s.ioMu.Unlock()
	if err != nil {
		return err
	}
	if err := s.completed.Check(); err != nil {
		return err
	}
	if err := s.verifier.CheckHistoryPath(s.ctx, s.completed.original.binding.History); err != nil {
		return err
	}
	return s.local()
}

func (s *started) close() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.uses.Wait()
		// Completed owns and joins every child before its original initial lease.
		physical := errors.Join(s.completed.Close(), s.ctx.Err())
		s.mu.Lock()
		s.failure = errors.Join(s.failure, physical)
		s.mu.Unlock()
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failure
}

func (h Started) Close() error {
	if h.original == nil {
		return nil
	}
	return h.original.close()
}
