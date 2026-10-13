//go:build linux

package runtime

import (
	"bytes"
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

// LaunchPublisher admits Administration and initializes the exact fixed worker
// with one bounded selected snapshot. Text owns document/protocol semantics;
// Execution retains artifact, Job, operation and original descendant cleanup.
func (owner *Owner) LaunchPublisher(ctx context.Context, principal [32]byte, snapshot []byte) (*Invocation, error) {
	if len(snapshot) > textdocument.MaximumBytes {
		return nil, errors.New("text snapshot is invalid")
	}
	body := bytes.Clone(snapshot)
	defer clear(body)
	if _, err := textdocument.NewSnapshot(body); err != nil {
		return nil, err
	}
	return owner.launchInvocation(ctx, principal, execution.Administration, body, true, false)
}

// This sole reader belongs to the fixed Text protocol. Only handoffs through
// the same original qualified operation reach it; the incoming byte boundary
// neither authenticates Service nor supplies a replacement Job.
func (invocation *Invocation) observePublisherAttachment() {
	invocation.publisherInput = &publisherConnectionInput{incoming: make(chan connection.Stream)}
	go func() {
		defer close(invocation.observationDone)
		err := textdocument.ServeWorkerConnections(invocation.job.Context(), invocation.activation.Attachment, invocation.publisherInput.incoming)
		if invocation.job.Retire() {
			invocation.observationError = errors.Join(errUnexpectedWorkerEnd, err)
		}
	}()
}
