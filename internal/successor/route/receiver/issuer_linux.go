//go:build linux

package receiver

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

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
