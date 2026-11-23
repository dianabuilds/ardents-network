//go:build linux

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

var errSourceInput = errors.New("original Source input unavailable")

// Copies share one original lifetime. Native observations and physical joins
// never hold mu; cancellation can seal admission while an observation is held.
// Close belongs to command composition, after State.Close and Wait have joined.
type sourceInputs struct{ *sourceInputLifetime }

type sourceInputLifetime struct {
	mu        sync.Mutex
	ioMu      sync.Mutex
	uses      sync.WaitGroup
	ctx       context.Context
	deny      context.CancelFunc
	stop      func() bool
	callback  chan struct{}
	watchDone chan struct{}
	files     []*sourceInputFile
	observe   func(string, *os.File)
	failed    error
	closing   bool
	done      chan struct{}
	result    error
}

type sourceInputFile struct {
	parent  *preparationPath
	file    *os.File
	info    os.FileInfo
	raw     []byte
	observe func(string, *os.File)
}

type sourceInputAccess uint8

const (
	sourcePublicInput sourceInputAccess = iota
	sourcePrivateInput
	sourceStateInput
)

func newSourceInputs(ctx context.Context, observe func(string, *os.File)) sourceInputs {
	admission, deny := context.WithCancel(ctx)
	l := &sourceInputLifetime{ctx: admission, deny: deny, callback: make(chan struct{}), done: make(chan struct{}), observe: observe}
	l.stop = context.AfterFunc(ctx, func() { l.refuse(ctx.Err()); close(l.callback) })
	return sourceInputs{l}
}

func acquireSourceInputs(ctx context.Context, path string) (_ sourceInputs, plan servingdeclaration.ServingDeclaration, result error) {
	return acquireObservedSourceInputs(ctx, path, nil)
}

func acquireObservedSourceInputs(ctx context.Context, path string, observe func(string, *os.File)) (_ sourceInputs, plan servingdeclaration.ServingDeclaration, result error) {
	if ctx == nil || ctx.Err() != nil {
		return sourceInputs{}, plan, errSourceInput
	}
	inputs := newSourceInputs(ctx, observe)
	l := inputs.sourceInputLifetime
	defer func() {
		if result != nil {
			result = errors.Join(result, inputs.Close())
		}
	}()
	first, err := l.acquire(path, 16<<10, false)
	if err != nil {
		return sourceInputs{}, plan, err
	}
	plan, err = servingdeclaration.DecodeServingDeclaration(first.raw)
	if err != nil || !validSourceServePlan(plan) {
		return sourceInputs{}, plan, errSourceInput
	}
	for i, path := range []string{plan.Certificate, plan.PrivateKey, plan.ClientRoot} {
		bound := int64(64 << 10)
		if i == 1 {
			bound = 16 << 10
		}
		if _, err = l.acquire(path, bound, i == 1); err != nil {
			return sourceInputs{}, plan, err
		}
	}
	if err := inputs.Check(); err != nil {
		return sourceInputs{}, plan, err
	}
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
	return inputs, plan, nil
}

// Acquisition is creator-owned and synchronous, before publishing the handle.
// It borrows only the unchanged direct-open mechanism, not offline signing's
// root-only/ext4/0700 policy. Runtime keys must belong to the invoking user and
// exclude other users; public inputs retain their admitted access exactly.
func (l *sourceInputLifetime) acquire(path string, maximum int64, private bool) (*sourceInputFile, error) {
	if private {
		return l.acquireWithAccess(path, maximum, sourcePrivateInput)
	}
	return l.acquireWithAccess(path, maximum, sourcePublicInput)
}

func (l *sourceInputLifetime) acquireWithAccess(path string, maximum int64, access sourceInputAccess) (*sourceInputFile, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || l.ctx.Err() != nil {
		return nil, l.refuse(errSourceInput)
	}
	uid := uint32(os.Geteuid())
	var owner *uint32
	if access == sourceStateInput {
		owner = &uid
	}
	p, err := openPreparationDirectories(l.ctx, filepath.Dir(path), owner)
	if err != nil {
		return nil, l.refuse(errors.Join(errSourceInput, err))
	}
	f := &sourceInputFile{parent: p, observe: l.observe}
	l.files = append(l.files, f)
	f.file, err = p.OpenFile(filepath.Base(path))
	if err != nil {
		return nil, l.refuse(errors.Join(errSourceInput, err))
	}
	f.info, err = f.file.Stat()
	if err != nil || !f.info.Mode().IsRegular() || f.info.Size() < 1 || f.info.Size() > maximum {
		return nil, l.refuse(errors.Join(errSourceInput, err))
	}
	s, ok := f.info.Sys().(*syscall.Stat_t)
	if !ok || s.Nlink != 1 {
		return nil, l.refuse(errSourceInput)
	}
	switch access {
	case sourcePrivateInput:
		if s.Uid != uid || f.info.Mode().Perm()&0077 != 0 {
			return nil, l.refuse(errSourceInput)
		}
	case sourceStateInput:
		if s.Uid != 0 && s.Uid != uid || f.info.Mode().Perm()&0022 != 0 {
			return nil, l.refuse(errSourceInput)
		}
		if err := checkSourceStatePath(p); err != nil {
			return nil, l.refuse(err)
		}
	}
	f.raw, err = io.ReadAll(io.LimitReader(f.file, maximum+1))
	if l.observe != nil {
		l.observe("read", f.file)
	}
	if err != nil || int64(len(f.raw)) != f.info.Size() {
		return nil, l.refuse(errors.Join(errSourceInput, err))
	}
	if err := errors.Join(f.check(), l.ctx.Err()); err != nil {
		return nil, l.refuse(err)
	}
	return f, nil
}

func (l *sourceInputLifetime) refuse(cause error) error {
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

func (inputs sourceInputs) Check() error {
	l := inputs.sourceInputLifetime
	if l == nil {
		return errSourceInput
	}
	l.mu.Lock()
	if l.failed != nil || l.closing || l.ctx.Err() != nil {
		err := l.failed
		if err == nil {
			err = l.ctx.Err()
		}
		if err == nil {
			err = errSourceInput
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
			err = errSourceInput
		}
	}
	l.mu.Unlock()
	if err != nil {
		return l.refuse(err)
	}
	return nil
}

func (f *sourceInputFile) check() error {
	var observe func(*os.File)
	if f.observe != nil {
		observe = func(file *os.File) { f.observe("check", file) }
	}
	return sourceInputCheckError(f.parent.CheckRetainedFile(f.file, f.info, f.raw, observe))
}

func (f *sourceInputFile) checkLink() error {
	return sourceInputCheckError(f.parent.CheckRetainedFileLink(f.file.Name(), f.info))
}

func (f *sourceInputFile) checkAncestors() error {
	if f.parent.CheckOriginalAccess() != nil {
		return errSourceInput
	}
	return nil
}

func sourceInputCheckError(err error) error {
	if err == artifacts.ErrInput {
		return errSourceInput
	}
	if err != nil {
		return errors.Join(errSourceInput, err)
	}
	return nil
}

func (inputs sourceInputs) Close() error {
	l := inputs.sourceInputLifetime
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
