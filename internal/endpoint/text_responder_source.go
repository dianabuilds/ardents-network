//go:build linux

package endpoint

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

// Only an already accepted Introduction reaches this preparation. Waiting for
// Source use neither owns nor waits for the publication's Descriptor ACK.
func (owner *textContext) prepareTextResponderSource(ctx context.Context, job *textJobIdentity) error {
	owner.mu.Lock()
	live := owner.liveTextServiceJobLocked(job, broker.Administration)
	owner.mu.Unlock()
	if !live {
		return errors.New("text Responder Source authority unavailable")
	}
	if err := owner.prepareTextSourceReady(ctx); err != nil {
		return err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if ctx.Err() != nil || !owner.liveTextServiceJobLocked(job, broker.Administration) {
		return errors.New("text Responder Source job retired")
	}
	return nil
}
