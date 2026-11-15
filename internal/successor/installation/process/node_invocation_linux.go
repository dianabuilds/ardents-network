package process

import (
	"context"
	"errors"
	"os"
	"runtime"
	"sync"
	"syscall"
)

// nodeInvocation retains one original physical process and joins its own uses.
// Fixed-role wrappers select their own process and signal admission.
type nodeInvocation struct {
	mu          sync.Mutex
	ioMu        sync.Mutex
	uses        sync.WaitGroup
	once        sync.Once
	pin         *Invocation
	pidfd       *os.File
	closing     bool
	interrupted bool
	failure     error
	closeErr    error
}

// retainNode acquires one original process, its namespace observations and the
// original root pidfd. The caller supplies only its fixed role policy.
func retainNode(ctx context.Context, expected Expected, policy invocationPolicy, pidfdLabel string) (*nodeInvocation, error) {
	if runtime.GOARCH != "amd64" || expected.UID < 100 || expected.UID > 999 || expected.GID != expected.UID {
		return nil, errInput
	}
	pin, err := retainFixed(ctx, expected, policy)
	if err != nil {
		return nil, err
	}
	o := &nodeInvocation{pin: pin}
	if err = pin.retainNamespaces(ctx); err == nil {
		err = pin.Observe(ctx)
	}
	if err == nil && os.Geteuid() == 0 {
		const pidfdOpenAMD64 = 434
		fd, _, failure := syscall.Syscall(pidfdOpenAMD64, uintptr(expected.PID), 0, 0)
		if failure != 0 {
			err = failure
		} else {
			o.pidfd = os.NewFile(fd, pidfdLabel)
			err = pin.Observe(ctx)
		}
	}
	if err != nil {
		o.failure = err
		return nil, errors.Join(err, o.close())
	}
	return o, nil
}

// matchArchived retains the existing original process observation on success;
// a mismatch joins the exact acquired original before returning.
func (o *nodeInvocation) matchArchived(device, inode uint64) error {
	native, ok := o.pin.identity.Sys().(*syscall.Stat_t)
	if !ok || uint64(native.Dev) != device || native.Ino != inode {
		return errors.Join(errBinding, o.close())
	}
	return nil
}

func (o *nodeInvocation) begin(ctx context.Context) error {
	if ctx == nil {
		return errInput
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closing || o.failure != nil {
		return errors.Join(os.ErrClosed, o.failure)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	o.uses.Add(1)
	return nil
}

func (o *nodeInvocation) retainFailure(err error) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.failure == nil {
		o.failure = err
	}
	return errors.Join(o.failure, err)
}

func (o *nodeInvocation) observe(ctx context.Context) error {
	if o == nil {
		return errInput
	}
	if err := o.begin(ctx); err != nil {
		return err
	}
	defer o.uses.Done()
	o.ioMu.Lock()
	defer o.ioMu.Unlock()
	if err := o.pin.Observe(ctx); err != nil {
		return o.retainFailure(err)
	}
	// A Close or failing earlier queued observer may have denied this use
	// while its physical reads completed. No late successful observation escapes.
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closing || o.failure != nil {
		return errors.Join(os.ErrClosed, o.failure)
	}
	return ctx.Err()
}

func (o *nodeInvocation) interrupt(ctx context.Context) (bool, error) {
	if o == nil || os.Geteuid() != 0 || runtime.GOARCH != "amd64" {
		return false, errInput
	}
	if err := o.begin(ctx); err != nil {
		return false, err
	}
	defer o.uses.Done()
	o.ioMu.Lock()
	defer o.ioMu.Unlock()
	if o.pidfd == nil || o.interrupted {
		return false, errInput
	}
	if err := o.pin.Observe(ctx); err != nil {
		return false, o.retainFailure(err)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closing || o.failure != nil || ctx.Err() != nil {
		return false, errors.Join(os.ErrClosed, o.failure, ctx.Err())
	}
	o.interrupted = true
	const pidfdSendSignalAMD64 = 424
	_, _, failure := syscall.Syscall6(pidfdSendSignalAMD64, o.pidfd.Fd(), uintptr(syscall.SIGTERM), 0, 0, 0, 0)
	if failure != 0 {
		if o.failure == nil {
			o.failure = failure
		}
		return true, errors.Join(o.failure, failure)
	}
	return true, ctx.Err()
}

func (o *nodeInvocation) close() error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	o.closing = true
	o.mu.Unlock()
	o.once.Do(func() {
		o.uses.Wait()
		physical := o.pin.Close()
		if o.pidfd != nil {
			physical = errors.Join(physical, o.pidfd.Close())
		}
		o.mu.Lock()
		o.closeErr = errors.Join(o.failure, physical)
		o.mu.Unlock()
	})
	return o.closeErr
}
