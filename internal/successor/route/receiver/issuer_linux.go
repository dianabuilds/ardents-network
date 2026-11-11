//go:build linux

package receiver

import (
	"context"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
)

// relayIssuer keeps the reverse terminal behind the exact Issuer peer CLOSE.
// The original handler owns the upstream CLOSE after both copies have joined.
func relayIssuer(ctx context.Context, source, target *framing.Lane) error {
	interrupt := func() { _ = target.CloseRefused(); _ = source.CloseRefused() }
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); interrupt() })
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	results := make(chan error, 2)
	go func() {
		_, err := io.Copy(target, source)
		if err == nil {
			err = target.CloseWrite()
		}
		results <- err
	}()
	go func() {
		_, err := io.Copy(source, target)
		if err == nil {
			err = target.WaitPeerRetirement(ctx)
		}
		if err == nil {
			err = source.CloseWrite()
		}
		results <- err
	}()
	first := <-results
	if first != nil {
		interrupt()
	}
	return errors.Join(first, <-results, target.Close(), ctx.Err())
}

// holdIssuer admits one operation against the issuer's already exclusive durable
// owners. It neither waits on a root nor creates an unbounded signing queue.
// The caller retains this slot and all transport copies until physical join.
func (r *Receiver) holdIssuer(bootstrap bool) (func(), error) {
	if r.config.Issue == nil || r.issuerWork == nil {
		return nil, errors.New("route issuer composition absent")
	}
	select {
	case r.issuerWork <- struct{}{}:
	default:
		return nil, errors.New("route issuer operation busy")
	}
	queues := r.queues
	if bootstrap {
		queues = r.bootstrap.Queues()
	}
	// The request, Admission response, RESULT and encoded output are distinct
	// bounded transport buffers. The shared duty budget counts all four copies.
	release, err := queues.HoldChild(4*(ardp.HeaderSize+ardp.IssuerBodySize) + admission.MaximumBatchTokens*admission.BlindSignatureSize)
	if err != nil {
		<-r.issuerWork
		return nil, err
	}
	return func() { release(); <-r.issuerWork }, nil
}
