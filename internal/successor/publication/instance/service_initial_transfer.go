package instance

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
)

// SendInitial writes the Connection-verified coalesced response on this exact
// TLS object. Only a complete actual write and final live checks release the
// setup-only bound into the unchanged original Work Safety lifetime.
func (service *ServiceAttachment) SendInitial(ctx context.Context, response *wire.InitialResponse) error {
	if ctx == nil {
		return ErrUnavailable
	}
	if err := service.enter(); err != nil {
		return err
	}
	defer service.users.Done()
	root := service.lifetime.binding.root
	root.mu.Lock()
	if service.generation != 1 || service.initialComplete || service.responseStarted || !service.proofIssued || service.evidence == nil || ctx.Err() != nil {
		root.mu.Unlock()
		return ErrUnavailable
	}
	body, err := response.Consume(service.evidence, service.initialProof)
	if err == nil {
		service.responseStarted = true
	}
	root.mu.Unlock()
	if err != nil {
		return err
	}
	defer clear(body)
	n, err := service.connection.Write(body)
	if n != len(body) {
		err = errors.Join(err, io.ErrShortWrite)
	}
	err = errors.Join(err, ctx.Err(), service.recheck())
	if err != nil {
		return err
	}
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := errors.Join(ctx.Err(), service.checkLocked()); err != nil {
		return err
	}
	// Joined has only a synchronous lane-local deadline setter; it performs no
	// network observation, physical I/O or callback under the Instance lock.
	if err := service.physical.SetDeadline(time.Unix(service.identity.WorkSafetyNotAfter, 0)); err != nil {
		return err
	}
	if err := errors.Join(ctx.Err(), service.checkLocked()); err != nil {
		return err
	}
	key, _ := service.evidence.Secrets()
	defer clear(key[:])
	if key == [32]byte{} || service.lease.continuityCommitment != [32]byte{} {
		return ErrUnavailable
	}
	service.lease.continuityCommitment = wire.ContinuityCommitment(key)
	service.initialComplete = true
	return nil
}
