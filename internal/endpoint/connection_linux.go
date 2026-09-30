//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	processdiag "github.com/dianabuilds/ardents-network/internal/diagnostics/process"
	"github.com/dianabuilds/ardents-network/internal/endpoint/service"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// connection retains the participant's separately authorized Reader
// context. A local request supplies only a Target Link, never worker identity,
// qualification, permission material, or network selection.
type connection struct {
	context  *dutyContext
	mu       sync.Mutex
	closed   bool
	pending  chan struct{}
	cancel   context.CancelFunc
	once     sync.Once
	closeErr error
}

func (owner *dutyContext) openConnection() (*connection, error) {
	if owner == nil {
		return nil, errors.New("text Connection unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if !owner.liveLocked(owner.endpoint, broker.Connection) || owner.principal == [32]byte{} {
		return nil, errors.New("text Connection requires its own current authorization")
	}
	return &connection{context: owner}, nil
}

func (owner *connection) Open(ctx context.Context, request applicationconnection.Request) (_ applicationconnection.Stream, outcome error) {
	if owner == nil || ctx == nil || ctx.Err() != nil || request.Destination != applicationconnection.TargetLink {
		return nil, errors.New("text destination unavailable")
	}
	target, err := owner.context.endpoint.TargetFromLink(request.Value)
	if errors.Is(err, ErrAlphaDestinationRetired) {
		return nil, applicationconnection.Refuse(applicationconnection.Outcome{Class: applicationconnection.ServiceUnavailable, Reason: err.Error()})
	}
	if err != nil {
		return nil, errors.New("text destination unavailable")
	}
	destination := targetlink.Link{Network: owner.context.endpoint.network, Target: target}
	owner.mu.Lock()
	if owner.closed || owner.pending != nil {
		owner.mu.Unlock()
		return nil, errors.New("text read already owned or unavailable")
	}
	trace := processdiag.ReaderTrace(owner.context.lease.Context())
	lifetime, cancel := context.WithCancel(trace.Bind(owner.context.lease.Context()))
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
			trace.Finish(outcome)
		}
	}()
	defer func() {
		if !transferred {
			joined := trace.Observe(processdiag.CallerJoin, lifetime)
			joinCaller()
			joined(nil)
			cancel()
			owner.finish(pending)
		}
	}()
	endpoint := owner.context.endpoint
	admitted := trace.Observe(processdiag.Admission, lifetime)
	capability, err := endpoint.Admit(owner.context.principal, broker.Connection)
	admitted(err)
	if err != nil {
		owner.context.reportOperationFailure("admission")
		return nil, err
	}
	activated := trace.Observe(processdiag.Activation, lifetime)
	lease, _, err := endpoint.admission.Activate(lifetime, capability, owner.context.principal, broker.Connection)
	activated(err)
	if err != nil {
		owner.context.reportOperationFailure("activation")
		return nil, err
	}
	defer func() {
		if !transferred {
			released := trace.Observe(processdiag.SessionRelease, lifetime)
			lease.Release()
			released(nil)
		}
	}()
	launched := trace.Observe(processdiag.WorkerLaunch, lease.Context())
	worker, err := owner.context.launchWorker(trace.Bind(lease.Context()), nil)
	launched(err)
	if err != nil {
		owner.context.reportOperationFailure("worker-launch")
		return nil, err
	}
	defer func() {
		if !transferred {
			closed := trace.Observe(processdiag.WorkerClose, lifetime)
			closeErr := worker.Close()
			closed(closeErr)
			outcome = errors.Join(outcome, closeErr)
		}
	}()
	begun := trace.Observe(processdiag.WorkerOperation, lease.Context())
	bounded, finish, err := worker.beginOperation(lease.Context(), broker.Connection)
	begun(err)
	if err != nil {
		owner.context.reportOperationFailure("worker-operation")
		return nil, err
	}
	bounded = trace.Bind(bounded)
	defer func() {
		if !transferred {
			finish()
		}
	}()
	until := endpoint.clock().UTC().Add(2 * time.Minute).Unix()
	prepared := trace.Observe(processdiag.Introduction, bounded)
	attempt, err := owner.context.prepareIntroduction(bounded, worker.job, destination, [3]int64{until, until, until})
	prepared(err)
	if err != nil {
		owner.context.reportOperationFailure("introduction-preparation")
		return nil, err
	}
	joined := trace.Observe(processdiag.ServiceJoin, bounded)
	service, err := owner.context.openJoinedService(bounded, worker.job, attempt)
	joined(err)
	if err != nil {
		owner.context.reportOperationFailure("service-join")
		return nil, err
	}
	if err := lease.Context().Err(); err != nil {
		owner.context.reportOperationFailure("post-join-lifetime")
		closed := trace.Observe(processdiag.ServiceClose, bounded)
		closeErr := service.Close()
		closed(closeErr)
		return nil, errors.Join(err, closeErr)
	}
	// Open returns only after Service authentication. The fixed request and
	// confined worker exchange follow local ACCEPT, within this same lifetime.
	stream := newReadResult(owner, pending, lease, cancel, worker, bounded, finish, service, joinCaller, owner.context.reportOperationFailure, trace)
	transferred = true
	return stream, nil
}

func (owner *connection) finish(pending chan struct{}) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.pending == pending {
		owner.pending = nil
		owner.cancel = nil
		close(pending)
	}
}

func (owner *connection) Close() error {
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

// readResult projects one validated worker RESULT through the fixed local
// document grammar. It never opens a second remote stream. Raw worker output
// and partial results cannot reach the trusted UI.
type readResult struct {
	input  *io.PipeWriter
	output *io.PipeReader
	cancel context.CancelFunc
	joined chan struct{}
	done   chan applicationconnection.Outcome
	err    error
}

func newReadResult(owner *connection, pending chan struct{}, lease *broker.ActiveSession, cancel context.CancelFunc, worker *qualifiedWorker, bounded context.Context, finish func(), serviceStream *service.Stream, joinCaller func(), report func(string), trace *processdiag.ConnectionTrace) *readResult {
	request, input := io.Pipe()
	output, response := io.Pipe()
	result := &readResult{input: input, output: output, cancel: cancel, joined: make(chan struct{}), done: make(chan applicationconnection.Outcome, 1)}
	go func() {
		defer close(result.joined)
		defer func() { trace.Finish(result.err) }()
		defer owner.finish(pending)
		defer func() { released := trace.Observe(processdiag.SessionRelease, bounded); lease.Release(); released(nil) }()
		defer cancel()
		defer func() { joined := trace.Observe(processdiag.CallerJoin, bounded); joinCaller(); joined(nil) }()
		interrupted := make(chan struct{})
		stop := context.AfterFunc(lease.Context(), func() {
			defer close(interrupted)
			request.CloseWithError(lease.Context().Err())
			response.CloseWithError(lease.Context().Err())
		})
		// Respond reads at most the fixed 512 bytes plus its EOF probe.
		// Reuse its exact parser before admitting the worker's single request.
		requested := trace.Observe(processdiag.LocalRequest, bounded)
		var fixed bytes.Buffer
		empty, err := textdocument.NewSnapshot(nil)
		if err == nil {
			err = empty.Respond(io.TeeReader(request, &fixed), io.Discard)
		}
		requested(err)
		if err != nil && lease.Context().Err() == nil && report != nil {
			report("service-result")
		}
		var body []byte
		if err == nil {
			body, err = worker.completeServiceRead(lease.Context(), bounded, finish, serviceStream, nil)
		} else {
			lifetimeErr := lease.Context().Err()
			closed := trace.Observe(processdiag.ServiceClose, bounded)
			serviceErr := serviceStream.Close()
			closed(serviceErr)
			finish()
			workerClosed := trace.Observe(processdiag.WorkerClose, bounded)
			workerErr := worker.Close()
			workerClosed(workerErr)
			cleanupErr := errors.Join(serviceErr, workerErr)
			if lifetimeErr != nil && canceledBeforeRequestCleanupOnly(cleanupErr) {
				// No local request was admitted. A native active-protocol abort is
				// therefore a consequence of our cancellation, not peer evidence.
				err = lifetimeErr
			} else {
				err = errors.Join(err, cleanupErr)
			}
		}
		if err == nil {
			var snapshot *textdocument.Snapshot
			responded := trace.Observe(processdiag.ApplicationResponse, bounded)
			snapshot, err = textdocument.NewSnapshot(body)
			clear(body)
			if err == nil {
				err = snapshot.Respond(bytes.NewReader(fixed.Bytes()), response)
			}
			responded(err)
		}
		clear(body)
		if !stop() {
			<-interrupted
		}
		err = errors.Join(err, lease.Context().Err())
		result.err = err
		outcome := applicationconnection.Outcome{Class: applicationconnection.CleanClose}
		if err != nil {
			outcome = applicationconnection.Outcome{Class: applicationconnection.ServiceUnavailable, Reason: "text read did not complete"}
		}
		request.CloseWithError(err)
		response.CloseWithError(err)
		result.done <- outcome
		close(result.done)
	}()
	return result
}

func canceledBeforeRequestCleanupOnly(err error) bool {
	if err == nil || err == context.Canceled || err == os.ErrDeadlineExceeded || err == nativeconnection.ErrActiveViolation {
		return true
	}
	if err == client.ErrClosedJoinPeerCleanupDeadline {
		return true
	}
	if timeout, ok := err.(interface{ Timeout() bool }); ok && timeout.Timeout() {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		if len(causes) > 1 {
			switch causes[0].Error() {
			case "text Service cleanup failed", "text Service transport retirement failed":
				causes = causes[1:]
			}
		}
		for _, cause := range causes {
			if !canceledBeforeRequestCleanupOnly(cause) {
				return false
			}
		}
		return true
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		return canceledBeforeRequestCleanupOnly(wrapped)
	}
	return false
}

func (stream *readResult) Read(body []byte) (int, error)              { return stream.output.Read(body) }
func (stream *readResult) Write(body []byte) (int, error)             { return stream.input.Write(body) }
func (stream *readResult) CloseInput() error                          { return stream.input.Close() }
func (stream *readResult) Done() <-chan applicationconnection.Outcome { return stream.done }
func (stream *readResult) Close() error {
	stream.cancel()
	stream.input.CloseWithError(context.Canceled)
	stream.output.CloseWithError(context.Canceled)
	<-stream.joined
	return stream.err
}
