package infrastructure

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"syscall"
	"time"
)

// The current boundary has exactly one owner. A denial either takes custody
// while frozen or observes that ACK is already outstanding. It never infers
// quiescence from a packet, child exit, or an uncertain send.
type sourceTransitionControl struct {
	mu       sync.Mutex
	scope    *sourceTransitionScope
	channel  *net.UnixConn
	denied   bool
	pending  *sourceTransitionCut
	cut      *sourceTransitionCut
	terminal error
}

type sourceTransitionCut struct {
	message sourceTransitionMessage
	prefix  *sourceRecoveryPrefix
	// Failure-only seam after actual proof Sync; never replacement I/O.
	afterSync func(*os.File) error
}

func observeSourceTransitionCut(ctx context.Context, message sourceTransitionMessage) (*sourceTransitionCut, error) {
	// This independent physical borrower has no timer that could begin Close
	// while the original is frozen. The controller owns cancellation, native
	// interruption and joining; all reads are regular/ext4 and byte bounded.
	prefix, err := openSourceRecoveryPrefixCustody(context.WithoutCancel(ctx), message.Journal, message.Operation, false)
	if err != nil {
		return nil, err
	}
	cut := &sourceTransitionCut{message: message, prefix: prefix}
	if err = cut.check(); err != nil {
		return nil, errors.Join(err, cut.close())
	}
	if err = ctx.Err(); err != nil {
		return nil, errors.Join(err, cut.close())
	}
	return cut, nil
}

func (c *sourceTransitionCut) check() error {
	if c == nil || c.prefix == nil || c.prefix.journal == nil {
		return ErrInput
	}
	p := c.prefix
	if err := p.check(); err != nil {
		return err
	}
	native := p.journal.info.Sys().(*syscall.Stat_t)
	if c.message.Journal != p.journal.path || c.message.Device != uint64(native.Dev) || c.message.Inode != native.Ino || c.message.PrefixSHA256 != digest(p.journal.raw) || c.message.Records != len(p.records) {
		return ErrBinding
	}
	return nil
}

func (c *sourceTransitionCut) close() error {
	if c == nil {
		return nil
	}
	return c.prefix.close()
}

func (c *sourceTransitionControl) freeze(ctx context.Context, cut *sourceTransitionCut) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.denied || c.pending != nil || c.cut != nil {
		return errors.Join(ErrInput, ctx.Err())
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.pending = cut
	return nil
}

// Mark outstanding BEFORE even closing the frozen observer, not just before
// writing the ACK. A Close error also has an unknown outcome and must not be
// relabelled as a successful frozen interruption.
func (c *sourceTransitionControl) acknowledge(ctx context.Context) (*sourceTransitionCut, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.denied || c.pending == nil {
		return nil, errors.Join(ErrInput, ctx.Err())
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cut := c.pending
	c.pending = nil
	return cut, nil
}

func (c *sourceTransitionControl) deny() {
	c.mu.Lock()
	if !c.denied {
		c.denied = true
		c.cut, c.pending = c.pending, nil
	}
	c.mu.Unlock()
	// No foreign/native I/O under the short decision mutex.
	err := errors.Join(c.scope.deny(), c.channel.SetDeadline(time.Now()))
	c.mu.Lock()
	c.terminal = errors.Join(c.terminal, err)
	c.mu.Unlock()
}

func (c *sourceTransitionControl) finish(ctx context.Context) (cut *sourceTransitionCut, err error) {
	if ctx.Err() != nil {
		c.deny()
	}
	c.mu.Lock()
	c.denied = true
	cut, pending := c.cut, c.pending
	c.cut, c.pending = nil, nil
	err = c.terminal
	c.mu.Unlock()
	err = errors.Join(err, pending.close(), c.scope.join())
	if cut == nil {
		return nil, err
	}
	if err == nil && c.scope.status.Signaled() && c.scope.status.Signal() == syscall.SIGKILL && c.scope.reaped == 1 {
		// Original physical join precedes a new, independently held EX flock.
		err = syscall.Flock(int(cut.prefix.journal.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			cut.prefix.exclusive = true
			err = cut.check()
		}
	} else {
		err = errors.Join(err, ErrSourceRepairRequired)
	}
	if err != nil {
		return nil, errors.Join(err, cut.close())
	}
	return cut, nil
}
