//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// textConnection retains the participant's separately authorized Reader
// context. A local request supplies only a Target Link, never worker identity,
// qualification, permission material, or network selection.
type textConnection struct {
	context  *textContext
	mu       sync.Mutex
	closed   bool
	pending  chan struct{}
	cancel   context.CancelFunc
	once     sync.Once
	closeErr error
}

func (owner *textContext) openTextConnection() (*textConnection, error) {
	if owner == nil {
		return nil, errors.New("text Connection unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.liveLocked(owner.endpoint, broker.Connection) || owner.principal == [32]byte{} {
		return nil, errors.New("text Connection requires its own current authorization")
	}
	return &textConnection{context: owner}, nil
}

func (owner *textConnection) Open(ctx context.Context, request connection.Request) (_ connection.Stream, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || request.Destination != connection.TargetLink {
		return nil, errors.New("text destination unavailable")
	}
	destination, err := targetlink.Decode(request.Value)
	if err != nil || destination.Network != owner.context.endpoint.network {
		return nil, errors.New("text destination unavailable")
	}
	owner.mu.Lock()
	if owner.closed || owner.pending != nil {
		owner.mu.Unlock()
		return nil, errors.New("text read already owned or unavailable")
	}
	lifetime, cancel := context.WithCancel(owner.context.lease.Context())
	callerDone := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() { defer close(callerDone); cancel() })
	joinCaller := func() {
		if !stopCaller() {
			<-callerDone
		}
	}
	pending := make(chan struct{})
	owner.pending, owner.cancel = pending, cancel
	owner.mu.Unlock()
	transferred := false
	defer func() {
		if !transferred {
			joinCaller()
			cancel()
			owner.finish(pending)
		}
	}()
	endpoint := owner.context.endpoint
	capability, err := endpoint.Admit(owner.context.principal, broker.Connection)
	if err != nil {
		owner.context.reportTextOperationFailure("admission")
		return nil, err
	}
	lease, _, err := endpoint.admission.Activate(lifetime, capability, owner.context.principal, broker.Connection)
	if err != nil {
		owner.context.reportTextOperationFailure("activation")
		return nil, err
	}
	defer func() {
		if !transferred {
			lease.Release()
		}
	}()
	worker, err := owner.context.launchTextWorker(lease.Context(), nil)
	if err != nil {
		owner.context.reportTextOperationFailure("worker-launch")
		return nil, err
	}
	defer func() {
		if !transferred {
			outcome = errors.Join(outcome, worker.Close())
		}
	}()
	bounded, finish, err := worker.beginOperation(lease.Context(), broker.Connection)
	if err != nil {
		owner.context.reportTextOperationFailure("worker-operation")
		return nil, err
	}
	defer func() {
		if !transferred {
			finish()
		}
	}()
	until := endpoint.clock().UTC().Add(2 * time.Minute).Unix()
	attempt, err := owner.context.prepareTextIntroduction(bounded, worker.job, destination, [3]int64{until, until, until})
	if err != nil {
		owner.context.reportTextOperationFailure("introduction-preparation")
		return nil, err
	}
	service, err := owner.context.openTextJoinedService(bounded, worker.job, attempt)
	if err != nil {
		owner.context.reportTextOperationFailure("service-join")
		return nil, err
	}
	if err := lease.Context().Err(); err != nil {
		owner.context.reportTextOperationFailure("post-join-lifetime")
		return nil, errors.Join(err, service.Close())
	}
	// Open returns only after Service authentication. The fixed request and
	// confined worker exchange follow local ACCEPT, within this same lifetime.
	stream := newTextReadResult(owner, pending, lease, cancel, worker, bounded, finish, service, joinCaller, owner.context.reportTextOperationFailure)
	transferred = true
	return stream, nil
}

func (owner *textConnection) finish(pending chan struct{}) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.pending == pending {
		owner.pending = nil
		owner.cancel = nil
		close(pending)
	}
}

func (owner *textConnection) Close() error {
	if owner == nil {
		return nil
	}
	owner.once.Do(func() {
		owner.mu.Lock()
		owner.closed = true
		pending, cancel := owner.pending, owner.cancel
		owner.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		owner.closeErr = owner.context.Close()
		if pending != nil {
			<-pending
		}
	})
	return owner.closeErr
}

// textReadResult projects one validated worker RESULT through the fixed local
// document grammar. It never opens a second remote stream. Raw worker output
// and partial results cannot reach the trusted UI.
type textReadResult struct {
	input  *io.PipeWriter
	output *io.PipeReader
	cancel context.CancelFunc
	joined chan struct{}
	done   chan connection.Outcome
	err    error
}

func newTextReadResult(owner *textConnection, pending chan struct{}, lease *broker.ActiveSession, cancel context.CancelFunc, worker *qualifiedTextWorker, bounded context.Context, finish func(), service *textServiceStream, joinCaller func(), report func(string)) *textReadResult {
	request, input := io.Pipe()
	output, response := io.Pipe()
	result := &textReadResult{input: input, output: output, cancel: cancel, joined: make(chan struct{}), done: make(chan connection.Outcome, 1)}
	go func() {
		defer close(result.joined)
		defer owner.finish(pending)
		defer lease.Release()
		defer cancel()
		defer joinCaller()
		interrupted := make(chan struct{})
		stop := context.AfterFunc(lease.Context(), func() {
			defer close(interrupted)
			request.CloseWithError(lease.Context().Err())
			response.CloseWithError(lease.Context().Err())
		})
		// Respond reads at most the fixed 512 bytes plus its EOF probe.
		// Reuse its exact parser before admitting the worker's single request.
		var fixed bytes.Buffer
		empty, err := textdocument.NewSnapshot(nil)
		if err == nil {
			err = empty.Respond(io.TeeReader(request, &fixed), io.Discard)
		}
		if err != nil && lease.Context().Err() == nil && report != nil {
			report("service-result")
		}
		var body []byte
		if err == nil {
			body, err = worker.completeServiceRead(lease.Context(), bounded, finish, service, nil)
		} else {
			err = errors.Join(err, service.Close())
			finish()
			err = errors.Join(err, worker.Close())
		}
		if err == nil {
			var snapshot *textdocument.Snapshot
			snapshot, err = textdocument.NewSnapshot(body)
			clear(body)
			if err == nil {
				err = snapshot.Respond(bytes.NewReader(fixed.Bytes()), response)
			}
		}
		clear(body)
		if !stop() {
			<-interrupted
		}
		err = errors.Join(err, lease.Context().Err())
		result.err = err
		outcome := connection.Outcome{Class: connection.CleanClose}
		if err != nil {
			outcome = connection.Outcome{Class: connection.ServiceUnavailable, Reason: "text read did not complete"}
		}
		request.CloseWithError(err)
		response.CloseWithError(err)
		result.done <- outcome
		close(result.done)
	}()
	return result
}

func (stream *textReadResult) Read(body []byte) (int, error)   { return stream.output.Read(body) }
func (stream *textReadResult) Write(body []byte) (int, error)  { return stream.input.Write(body) }
func (stream *textReadResult) CloseInput() error               { return stream.input.Close() }
func (stream *textReadResult) Done() <-chan connection.Outcome { return stream.done }
func (stream *textReadResult) Close() error {
	stream.cancel()
	stream.input.CloseWithError(context.Canceled)
	stream.output.CloseWithError(context.Canceled)
	<-stream.joined
	return stream.err
}
