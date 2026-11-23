//go:build linux

package artifacts

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrObservedInput = errors.New("original observed input unavailable")

// Copies share one original lifetime. Native observations and physical joins
// never hold mu; cancellation can seal admission while an observation is held.
// The retaining operation joins its original borrowers before it calls Close.
type ObservedInputs struct{ *observedInputLifetime }

type observedInputLifetime struct {
	mu        sync.Mutex
	ioMu      sync.Mutex
	uses      sync.WaitGroup
	ctx       context.Context
	deny      context.CancelFunc
	stop      func() bool
	callback  chan struct{}
	watchDone chan struct{}
	files     []*ObservedInput
	observe   func(string, *os.File)
	failed    error
	closing   bool
	done      chan struct{}
	result    error
}

type ObservedInput struct {
	parent  *DirectoryChain
	file    *os.File
	info    os.FileInfo
	raw     []byte
	observe func(string, *os.File)
}

func NewObservedInputs(ctx context.Context, observe func(string, *os.File)) ObservedInputs {
	admission, deny := context.WithCancel(ctx)
	l := &observedInputLifetime{ctx: admission, deny: deny, callback: make(chan struct{}), done: make(chan struct{}), observe: observe}
	l.stop = context.AfterFunc(ctx, func() { l.refuse(ctx.Err()); close(l.callback) })
	return ObservedInputs{l}
}

// Context interrupts operations when an original input becomes unavailable.
func (inputs ObservedInputs) Context() context.Context {
	if inputs.observedInputLifetime == nil {
		return nil
	}
	return inputs.ctx
}

// Deny stops new observations while retaining the original files.
func (inputs ObservedInputs) Deny() { inputs.deny() }

// Watch starts periodic reobservation after synchronous acquisition is complete.
// The creator calls it once, before lending the inputs.
func (inputs ObservedInputs) Watch() {
	l := inputs.observedInputLifetime
	l.watchDone = make(chan struct{})
	go func() {
		defer close(l.watchDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-l.ctx.Done():
				return
			case <-ticker.C:
				if inputs.Check() != nil {
					return
				}
			}
		}
	}()
}

// Bytes lends the exact admitted bytes until the original input lifetime joins.
func (f *ObservedInput) Bytes() []byte { return f.raw }

// Descriptor is the held file for an original checked observation.
func (f *ObservedInput) Descriptor() *os.File { return f.file }

// Files returns the admitted handles in acquisition order. The caller borrows
// their content while it retains the original inputs.
func (inputs ObservedInputs) Files() []*ObservedInput {
	return append([]*ObservedInput(nil), inputs.files...)
}

// Info returns the original native file observation.
func (f *ObservedInput) Info() os.FileInfo { return f.info }

// Acquisition is creator-owned and synchronous, before publishing the handle.
// It borrows only the unchanged direct-open mechanism, not offline signing's
// root-only/ext4/0700 policy. Runtime keys must belong to the invoking user and
// exclude other users; public inputs retain their admitted access exactly.
func (l *observedInputLifetime) Acquire(path string, maximum int64, private bool) (*ObservedInput, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || l.ctx.Err() != nil {
		return nil, l.refuse(ErrObservedInput)
	}
	uid := uint32(os.Geteuid())
	p, err := OpenDirectoryChain(l.ctx, filepath.Dir(path), nil)
	if err != nil {
		return nil, l.refuse(errors.Join(ErrObservedInput, err))
	}
	f := &ObservedInput{parent: p, observe: l.observe}
	l.files = append(l.files, f)
	f.file, err = p.OpenFile(filepath.Base(path))
	if err != nil {
		return nil, l.refuse(errors.Join(ErrObservedInput, err))
	}
	var afterRead func(*os.File)
	if l.observe != nil {
		afterRead = func(file *os.File) { l.observe("read", file) }
	}
	f.info, f.raw, err = ReadRetainedFile(f.file, maximum, func(mode os.FileMode, owner uint32) bool {
		return !private || owner == uid && mode.Perm()&0077 == 0
	}, afterRead)
	if err != nil {
		return nil, l.refuse(observedInputError(err))
	}
	if err := errors.Join(f.check(), l.ctx.Err()); err != nil {
		return nil, l.refuse(err)
	}
	return f, nil
}

func (l *observedInputLifetime) refuse(cause error) error {
	l.mu.Lock()
	if l.failed == nil {
		l.failed = cause
		if callerErr := l.ctx.Err(); callerErr != nil && callerErr != cause {
			l.failed = errors.Join(callerErr, cause)
		}
	} else if cause != context.Canceled && cause != context.DeadlineExceeded && !errors.Is(l.failed, cause) {
		// An already admitted native observation can fail after caller denial.
		// Keep its independent result without replacing the original cause.
		l.failed = errors.Join(l.failed, cause)
	}
	err := l.failed
	l.mu.Unlock()
	l.deny()
	return err
}

func (inputs ObservedInputs) Check() error {
	l := inputs.observedInputLifetime
	if l == nil {
		return ErrObservedInput
	}
	l.mu.Lock()
	if l.failed != nil || l.closing || l.ctx.Err() != nil {
		err := l.failed
		if err == nil {
			err = l.ctx.Err()
		}
		if err == nil {
			err = ErrObservedInput
		}
		l.mu.Unlock()
		return l.refuse(err)
	}
	l.uses.Add(1)
	l.mu.Unlock()
	defer l.uses.Done()
	l.ioMu.Lock()
	defer l.ioMu.Unlock()
	for _, f := range l.files {
		if l.ctx.Err() != nil {
			break
		}
		if err := f.check(); err != nil {
			return l.refuse(err)
		}
	}
	l.mu.Lock()
	err := l.failed
	if err == nil && (l.closing || l.ctx.Err() != nil) {
		err = l.ctx.Err()
		if err == nil {
			err = ErrObservedInput
		}
	}
	l.mu.Unlock()
	if err != nil {
		return l.refuse(err)
	}
	return nil
}

func (f *ObservedInput) check() error {
	var observe func(*os.File)
	if f.observe != nil {
		observe = func(file *os.File) { f.observe("check", file) }
	}
	return observedInputError(f.parent.CheckRetainedFile(f.Descriptor(), f.info, f.raw, observe))
}

func observedInputError(err error) error {
	if err == ErrInput {
		return ErrObservedInput
	}
	if err != nil {
		return errors.Join(ErrObservedInput, err)
	}
	return nil
}

func (inputs ObservedInputs) Close() error {
	l := inputs.observedInputLifetime
	if l == nil {
		return nil
	}
	l.mu.Lock()
	if l.closing {
		l.mu.Unlock()
		<-l.done
		return l.result
	}
	l.closing = true
	l.mu.Unlock()
	l.deny()
	if !l.stop() {
		<-l.callback
	}
	if l.watchDone != nil {
		<-l.watchDone
	}
	l.uses.Wait()
	var result error
	for _, f := range l.files {
		if f.file != nil {
			if l.observe != nil {
				l.observe("close", f.file)
			}
			result = errors.Join(result, f.file.Close())
		}
		result = errors.Join(result, f.parent.Close())
		clear(f.raw)
	}
	l.result = result
	close(l.done)
	return result
}
