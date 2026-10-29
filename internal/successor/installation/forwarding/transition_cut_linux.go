package forwarding

import (
	"context"
	"errors"
	"net"
	"sync"
	"syscall"
	"time"
)

// Only this short local fence decides whether a proven stopped prefix is still
// frozen or its next effect is outstanding. Native I/O happens after admission
// denial, outside the fence. Neither process exit nor an ACK attempt is a cut.
type transitionControl struct {
	mu       sync.Mutex
	scope    *transitionScope
	channel  *net.UnixConn
	denied   bool
	pending  *transitionPrefix
	cut      *transitionPrefix
	terminal error
}

func (c *transitionControl) freeze(ctx context.Context, prefix *transitionPrefix) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.denied || c.pending != nil || c.cut != nil || prefix == nil {
		return errors.Join(ErrInput, ctx.Err())
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.pending = prefix
	return nil
}

// ACK is outstanding BEFORE closing the independent frozen observer. A late
// Close failure or uncertain send cannot turn it back into a safe boundary.
func (c *transitionControl) acknowledge(ctx context.Context) (*transitionPrefix, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.denied || c.pending == nil {
		return nil, errors.Join(ErrInput, ctx.Err())
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	prefix := c.pending
	c.pending = nil
	return prefix, nil
}

func (c *transitionControl) deny() {
	c.mu.Lock()
	if !c.denied {
		c.denied = true
		c.cut, c.pending = c.pending, nil
	}
	c.mu.Unlock()
	err := errors.Join(c.scope.deny(), c.channel.SetDeadline(time.Now()))
	c.mu.Lock()
	c.terminal = errors.Join(c.terminal, err)
	c.mu.Unlock()
}

func (c *transitionControl) finish() (*transitionPrefix, error) {
	c.mu.Lock()
	c.denied = true
	cut, pending := c.cut, c.pending
	c.cut, c.pending = nil, nil
	err := c.terminal
	c.mu.Unlock()
	err = errors.Join(err, pending.close(), c.scope.join())
	if cut == nil {
		return nil, err
	}
	if err == nil && c.scope.status.Signaled() && c.scope.status.Signal() == syscall.SIGKILL && c.scope.reaped == 1 {
		err = syscall.Flock(int(cut.journal.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			cut.exclusive = true
			err = cut.check()
		}
	} else {
		err = errors.Join(err, ErrRepairRequired)
	}
	if err != nil {
		return nil, errors.Join(err, cut.close())
	}
	return cut, nil
}
