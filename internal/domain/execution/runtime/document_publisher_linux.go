//go:build linux

package runtime

import (
	"bytes"
	"context"
	"errors"
	"io"

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

// LaunchSnapshotPublisher uses the qualified Publisher attachment to serve
// bounded reads from the original source. The caller retains it through Close.
func (owner *Owner) LaunchSnapshotPublisher(ctx context.Context, principal [32]byte, source io.ReaderAt, size int64) (*Invocation, error) {
	if source == nil || size < 0 {
		return nil, errors.New("document snapshot source is invalid")
	}
	return owner.launchApplicationInvocation(ctx, principal, execution.Administration, documentProtocol{
		sourced: true,
		start: func(invocation *Invocation) {
			invocation.servePublisherAttachment(source, size)
		},
	}, true, false)
}

// This sole reader belongs to the fixed document protocol. Only handoffs through
// the same original qualified operation reach it; the incoming byte boundary
// neither authenticates Service nor supplies a replacement Job.
func (invocation *Invocation) observePublisherAttachment() {
	invocation.servePublisherAttachment(nil, 0)
}

func (invocation *Invocation) servePublisherAttachment(source io.ReaderAt, size int64) {
	invocation.publisherInput = &publisherConnectionInput{incoming: make(chan endpointstream.Stream)}
	go func() {
		defer close(invocation.observationDone)
		var err error
		if source != nil {
			err = document.ServeWorkerSnapshot(invocation.job.Context(), invocation.activation.Attachment, invocation.publisherInput.incoming, source, size)
		} else {
			err = document.ServeWorkerConnections(invocation.job.Context(), invocation.activation.Attachment, invocation.publisherInput.incoming)
		}
		if invocation.job.Retire() {
			invocation.observationError = errors.Join(errUnexpectedWorkerEnd, err)
		}
	}()
}
