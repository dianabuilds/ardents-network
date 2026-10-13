package runtime

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication/instance"
	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
)

type registrationPair struct {
	ctx            context.Context
	cancel         context.CancelFunc
	stopCaller     func() bool
	callerCallback chan struct{}
	callerOnce     sync.Once
	registration   *introduction.HolderRegistration
	receipt        introduction.Receipt
	recipient      *instance.Recipient
	descriptor     []byte
	firstACK       time.Time
	cutoff         time.Time
	closeOnce      sync.Once
	closeErr       error
	cleanupErr     error
}

func (pair *registrationPair) check(p *Publisher, ctx context.Context) error {
	if pair.registration == nil || pair.recipient == nil || pair.ctx.Err() != nil {
		return errors.New("Publisher pair unavailable")
	}
	if err := p.check(ctx); err != nil {
		return err
	}
	if err := pair.registration.CheckReceipt(pair.receipt); err != nil {
		return err
	}
	raw, err := pair.recipient.Descriptor(ctx)
	if err != nil || !bytes.Equal(raw, pair.descriptor) {
		return errors.Join(errors.New("Publisher exact Descriptor unavailable"), err)
	}
	value := p.credential.Delegation()
	proof, err := reachability.Verify(raw, value.Target, value.Network, pair.receipt.Facts().Profile, time.Now())
	if err != nil {
		return err
	}
	return proof.Current(time.Now())
}

// joinCaller relinquishes setup-only cancellation before an accepting handoff.
// A callback already in flight must finish; it cannot cancel the pair afterward.
// The pair's original Publisher parent remains its independent lifetime owner.
func (pair *registrationPair) joinCaller() {
	pair.callerOnce.Do(func() {
		if !pair.stopCaller() {
			<-pair.callerCallback
		}
	})
}

func (pair *registrationPair) close() error {
	pair.closeOnce.Do(func() {
		if pair.registration != nil {
			pair.registration.Stop()
		}
		pair.cancel()
		if pair.registration != nil {
			pair.closeErr = pair.registration.Close()
			cleanup, complete := pair.registration.CleanupResult()
			if !complete {
				cleanup = errors.Join(cleanup, errors.New("original Publisher registration cleanup did not join"))
			}
			pair.cleanupErr = cleanup
		}
		if pair.recipient != nil {
			cleanup := pair.recipient.Close()
			pair.closeErr = errors.Join(pair.closeErr, cleanup)
			pair.cleanupErr = errors.Join(pair.cleanupErr, cleanup)
		}
		pair.joinCaller()
	})
	return pair.closeErr
}
