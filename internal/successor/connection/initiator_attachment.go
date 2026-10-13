package connection

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	"github.com/dianabuilds/ardents-network/internal/successor/route/join"
)

// The Reader retains its original JOIN until all TLS users have actually
// returned. TLS's cancellation Close interrupts only this borrowing adapter.
type initiatorAttachment struct {
	self            *initiatorAttachment
	binding         *InitiatorBinding
	physical        *join.Joined
	connection      *tls.Conn
	evidence        *wire.TLSBinding
	ctx             context.Context
	cancel          context.CancelFunc
	mu              sync.Mutex
	users           sync.WaitGroup
	initialComplete bool
	closed          bool
	watcherDone     chan struct{}
	once            sync.Once
	closeErr        error
	cleanupErr      error
}

// A physical cancellation interrupts only its TLS users. The original logical
// deadline remains unchanged through a gap; final retirement cancels it too.
func initiatorLifetimes(original context.Context, end time.Time) (context.Context, context.CancelFunc, context.Context, context.CancelFunc) {
	logical, cancelLogical := context.WithDeadline(original, end)
	physical, cancelPhysical := context.WithCancel(logical)
	return logical, cancelLogical, physical, cancelPhysical
}

func (attachment *initiatorAttachment) check() error {
	if attachment == nil || attachment.self != attachment || attachment.binding == nil || attachment.physical == nil {
		return errIntegrity
	}
	attachment.mu.Lock()
	if err := attachment.checkLocked(); err != nil {
		attachment.mu.Unlock()
		return err
	}
	initialComplete := attachment.initialComplete
	attachment.mu.Unlock()
	// Instance proof and original Route/Network observations can perform
	// native I/O. The attachment seal must remain independently accessible
	// to interruption and Close while those owners are being observed.
	err := errors.Join(attachment.binding.check(!initialComplete), attachment.physical.CheckOriginal())
	attachment.mu.Lock()
	defer attachment.mu.Unlock()
	if attachment.initialComplete != initialComplete {
		return errors.Join(errIntegrity, err)
	}
	return errors.Join(err, attachment.checkLocked())
}

func (attachment *initiatorAttachment) checkLocked() error {
	if attachment.binding == nil || attachment.binding.operation == nil || attachment.binding.setup == nil || attachment.ctx == nil {
		return errIntegrity
	}
	if attachment.ctx.Err() != nil {
		return context.Cause(attachment.ctx)
	}
	if attachment.closed {
		return net.ErrClosed
	}
	if err := attachment.binding.operation.CheckReader(); err != nil {
		return err
	}
	now := time.Now()
	proof := attachment.binding.proof.Delegation()
	if now.Before(proof.NotBefore) || !now.Before(proof.NotAfter) || !now.Before(time.Unix(attachment.binding.identity.WorkSafetyNotAfter, 0)) {
		return errIntegrity
	}
	if !attachment.initialComplete && (attachment.binding.setup.Err() != nil || !now.Before(attachment.binding.expires) ||
		!attachment.binding.recipientEnd.IsZero() && !now.Before(attachment.binding.recipientEnd)) {
		return errors.Join(errIntegrity, attachment.binding.setup.Err())
	}
	return nil
}

func (attachment *initiatorAttachment) enter() error {
	if err := attachment.check(); err != nil {
		return err
	}
	attachment.mu.Lock()
	defer attachment.mu.Unlock()
	if err := attachment.checkLocked(); err != nil {
		return err
	}
	attachment.users.Add(1)
	return nil
}

func (attachment *initiatorAttachment) Read(body []byte) (int, error) {
	if err := attachment.enter(); err != nil {
		return 0, err
	}
	defer attachment.users.Done()
	n, err := attachment.connection.Read(body)
	return n, errors.Join(err, attachment.check())
}

func (attachment *initiatorAttachment) Write(body []byte) (int, error) {
	if err := attachment.enter(); err != nil {
		return 0, err
	}
	defer attachment.users.Done()
	n, err := attachment.connection.Write(body)
	return n, errors.Join(err, attachment.check())
}

func (attachment *initiatorAttachment) watch() {
	defer close(attachment.watcherDone)
	<-attachment.ctx.Done()
	_ = attachment.physical.InterruptIO()
}

func (attachment *initiatorAttachment) Close() error {
	return attachment.close(false)
}

func (attachment *initiatorAttachment) closeFailed() error {
	return attachment.close(true)
}

func (attachment *initiatorAttachment) close(failed bool) error {
	if attachment == nil {
		return nil
	}
	if attachment.self != attachment {
		return errIntegrity
	}
	attachment.once.Do(func() {
		var disposition error
		if failed {
			disposition = attachment.physical.MarkFailedRetirement()
		}
		attachment.mu.Lock()
		attachment.closed = true
		attachment.cancel()
		attachment.mu.Unlock()
		<-attachment.watcherDone
		attachment.users.Wait()
		attachment.closeErr = errors.Join(disposition, attachment.physical.Close())
		var joined bool
		attachment.cleanupErr, joined = attachment.physical.CleanupResult()
		if !joined {
			attachment.cleanupErr = errors.Join(attachment.cleanupErr, errIntegrity)
		}
		if attachment.evidence != nil {
			attachment.evidence.Erase()
		}
	})
	return attachment.closeErr
}

type initiatorTLSBorrow struct{ attachment *initiatorAttachment }

func (borrow initiatorTLSBorrow) Read(body []byte) (int, error) {
	if err := borrow.attachment.check(); err != nil {
		return 0, err
	}
	n, err := borrow.attachment.physical.Read(body)
	return n, errors.Join(err, borrow.attachment.check())
}

func (borrow initiatorTLSBorrow) Write(body []byte) (int, error) {
	if err := borrow.attachment.check(); err != nil {
		return 0, err
	}
	n, err := borrow.attachment.physical.Write(body)
	return n, errors.Join(err, borrow.attachment.check())
}

func (borrow initiatorTLSBorrow) Close() error {
	borrow.attachment.cancel()
	return borrow.attachment.physical.InterruptIO()
}

func (borrow initiatorTLSBorrow) LocalAddr() net.Addr { return borrow.attachment.physical.LocalAddr() }
func (borrow initiatorTLSBorrow) RemoteAddr() net.Addr {
	return borrow.attachment.physical.RemoteAddr()
}
func (borrow initiatorTLSBorrow) SetDeadline(end time.Time) error {
	return borrow.attachment.physical.SetDeadline(end)
}
func (borrow initiatorTLSBorrow) SetReadDeadline(end time.Time) error {
	return borrow.attachment.physical.SetReadDeadline(end)
}
func (borrow initiatorTLSBorrow) SetWriteDeadline(end time.Time) error {
	return borrow.attachment.physical.SetWriteDeadline(end)
}
