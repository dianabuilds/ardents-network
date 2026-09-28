//go:build linux

package endpoint

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

// Only an already accepted Introduction reaches this preparation. Waiting for
// Source use neither owns nor waits for the publication's Descriptor ACK.
func (owner *dutyContext) prepareResponderSource(ctx context.Context, job *jobIdentity) error {
	owner.mu.Lock()
	live := owner.liveServiceJobLocked(job, broker.Administration)
	owner.mu.Unlock()
	if !live {
		return errors.New("text Responder Source authority unavailable")
	}
	if err := owner.prepareSourceReady(ctx); err != nil {
		return err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if ctx.Err() != nil || !owner.liveServiceJobLocked(job, broker.Administration) {
		return errors.New("text Responder Source job retired")
	}
	return nil
}
