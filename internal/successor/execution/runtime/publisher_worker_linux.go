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
	return owner.launchInvocation(ctx, principal, execution.Administration, body, true)
}

// This sole reader belongs to the fixed Text protocol. No successful Service
// Connection is invented: its incoming channel has no producer until a real
// Connection owner is implemented. Unsolicited frames still retire the Job.
func (invocation *Invocation) observePublisherAttachment() {
	go func() {
		defer close(invocation.observationDone)
		incoming := make(chan connection.Stream)
		err := textdocument.ServeWorkerConnections(invocation.job.Context(), invocation.activation.Attachment, incoming)
		if invocation.job.Retire() {
			invocation.observationError = errors.Join(errUnexpectedWorkerEnd, err)
		}
	}()
}
