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
	registration   *introduction.HolderRegistration
	receipt        introduction.Receipt
	recipient      *instance.Recipient
	descriptor     []byte
	firstACK       time.Time
	cutoff         time.Time
	closeOnce      sync.Once
	closeErr       error
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

func (pair *registrationPair) close() error {
	pair.closeOnce.Do(func() {
		if pair.registration != nil {
			pair.registration.Stop()
		}
		pair.cancel()
		if pair.registration != nil {
			pair.closeErr = pair.registration.Close()
		}
		if pair.recipient != nil {
			pair.closeErr = errors.Join(pair.closeErr, pair.recipient.Close())
		}
		if !pair.stopCaller() {
			<-pair.callerCallback
		}
	})
	return pair.closeErr
}
