//go:build linux

package runtime

import (
	"bytes"
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	"github.com/dianabuilds/ardents-network/internal/domain/execution"
)

// LaunchPublisher admits Administration and initializes the exact fixed worker
// with one bounded selected snapshot. The document Application owns its protocol semantics;
// Execution retains artifact, Job, operation and original descendant cleanup.
func (owner *Owner) LaunchPublisher(ctx context.Context, principal [32]byte, snapshot []byte) (*Invocation, error) {
	if len(snapshot) > document.MaximumBytes {
		return nil, errors.New("document snapshot is invalid")
	}
	body := bytes.Clone(snapshot)
	defer clear(body)
	return owner.launchApplicationInvocation(ctx, principal, execution.Administration, documentProtocol{snapshot: body, start: (*Invocation).observePublisherAttachment}, true, false)
}

// This sole reader belongs to the fixed document protocol. Only handoffs through
// the same original qualified operation reach it; the incoming byte boundary
// neither authenticates Service nor supplies a replacement Job.
func (invocation *Invocation) observePublisherAttachment() {
	invocation.publisherInput = &publisherConnectionInput{incoming: make(chan endpointstream.Stream)}
	go func() {
		defer close(invocation.observationDone)
		err := document.ServeWorkerConnections(invocation.job.Context(), invocation.activation.Attachment, invocation.publisherInput.incoming)
		if invocation.job.Retire() {
			invocation.observationError = errors.Join(errUnexpectedWorkerEnd, err)
		}
	}()
}
