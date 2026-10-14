//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"time"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/connection"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

// installedReader uses the Endpoint's original qualified local runtime and
// already imported Stock. The retained startup/State observer belongs to the
// constructing installed participant; it is never supplied by an Application.
type installedReader struct {
	local     *executionruntime.Owner
	principal [32]byte
	plan      routePrefixPlan
	authority admissionAuthority
	holder    *stock.Owner
	check     func() error
	route     *installedRouteOwners
}

func (owner *installedReader) Open(ctx context.Context, request application.Request) (_ application.Stream, result error) {
	if ctx == nil || owner == nil || owner.local == nil || owner.holder == nil || owner.check == nil || owner.authority.current == nil || request.Destination != application.TargetLink {
		return nil, application.Refuse(application.Outcome{Class: application.ServiceUnavailable})
	}
	if err := errors.Join(ctx.Err(), owner.check()); err != nil {
		return nil, err
	}
	view, err := owner.authority.current()
	if err != nil {
		return nil, err
	}
	plan := owner.plan
	plan.Deadline = time.Now().Add(admission.ForwardClass.Lifetime()).UTC().Truncate(time.Second)
	if end := view.Profile().NotAfter; end.Before(plan.Deadline) {
		plan.Deadline = end
	}
	if end, bounded := ctx.Deadline(); bounded && end.Before(plan.Deadline) {
		plan.Deadline = end.UTC().Truncate(time.Second)
	}
	worker, err := owner.local.LaunchReader(ctx, owner.principal)
	if err != nil {
		return nil, err
	}
	operation, err := worker.BeginOperation(ctx)
	if err != nil {
		return nil, errors.Join(err, worker.Close())
	}
	authority := owner.authority
	originalCurrent := authority.current
	authority.current = func() (network.RuntimeView, error) {
		// The installed authority reobserves the original Startup before and
		// after CurrentRuntime. Retain this operation's rights around that
		// observation without repeating the same full native file/manager I/O.
		if err := errors.Join(ctx.Err(), operation.CheckReader()); err != nil {
			return network.RuntimeView{}, err
		}
		view, err := originalCurrent()
		return view, errors.Join(err, ctx.Err(), operation.CheckReader())
	}
	var service *connection.Stream
	var closeSource func() error
	var retireOnce sync.Once
	var retired error
	retire := func() error {
		retireOnce.Do(func() {
			if closeSource != nil {
				retired = closeSource()
			} else if service != nil {
				retired = service.Close()
				operation.RetainCleanup(service.JoinedCleanup())
			}
			operation.Close()
			retired = errors.Join(retired, worker.Close())
		})
		return retired
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, retire())
		}
	}()
	contract, err := operation.BindStreamContract(512, uint64(textdocument.MaximumBytes+13))
	if err != nil {
		return nil, err
	}
	var source *connection.InitiatorSource
	source, closeSource, err = openTextReaderSource(operation, plan, authority, owner.holder, request.Value, owner.route)
	if err != nil {
		return nil, err
	}
	service, err = source.OpenStream(contract)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(ctx.Err(), owner.check(), operation.CheckReader()); err != nil {
		return nil, err
	}
	complete := func() ([]byte, error) {
		transferred, response, err := operation.DeliverReaderStream(contract, service)
		if !transferred {
			_ = service.Close()
			operation.RetainCleanup(service.JoinedCleanup())
		}
		if err != nil || response == nil {
			return nil, errors.Join(err, errors.New("original Reader result is unavailable"), retire())
		}
		defer response.Close()
		if err := errors.Join(retire(), ctx.Err(), owner.check()); err != nil {
			return nil, err
		}
		return response.TakeBytes()
	}
	// The local parser admits the fixed request before the sole confined
	// worker transfer. The caller receives no raw Service transport.
	return newInstalledReaderResult(ctx, owner.check, complete, retire), nil
}

// installedReaderResult owns only local request/result projection. Execution
// owns the worker and sealed result; Source owns authenticated network cleanup.
type installedReaderResult struct {
	input  *io.PipeWriter
	output *io.PipeReader
	cancel context.CancelFunc
	joined chan struct{}
	done   chan application.Outcome
	err    error
}

func newInstalledReaderResult(original context.Context, check func() error, complete func() ([]byte, error), retire func() error) *installedReaderResult {
	ctx, cancel := context.WithCancel(original)
	request, input := io.Pipe()
	output, response := io.Pipe()
	stream := &installedReaderResult{input: input, output: output, cancel: cancel, joined: make(chan struct{}), done: make(chan application.Outcome, 1)}
	go func() {
		defer close(stream.joined)
		interrupted := make(chan struct{})
		stop := context.AfterFunc(ctx, func() {
			defer close(interrupted)
			_ = request.CloseWithError(ctx.Err())
			_ = response.CloseWithError(ctx.Err())
			_ = retire()
		})
		var fixed bytes.Buffer
		empty, err := textdocument.NewSnapshot(nil)
		if err == nil {
			err = errors.Join(ctx.Err(), check())
		}
		if err == nil {
			err = empty.Respond(io.TeeReader(request, &fixed), io.Discard)
		}
		var body []byte
		if err == nil {
			err = errors.Join(ctx.Err(), check())
		}
		if err == nil {
			body, err = complete()
		}
		if err == nil {
			err = errors.Join(ctx.Err(), check())
		}
		if err == nil {
			var snapshot *textdocument.Snapshot
			snapshot, err = textdocument.NewSnapshot(body)
			clear(body)
			if err == nil {
				err = snapshot.Respond(bytes.NewReader(fixed.Bytes()), installedReaderOutput{ctx: ctx, check: check, output: response})
			}
		}
		clear(body)
		err = errors.Join(err, retire(), ctx.Err(), check())
		if !stop() {
			<-interrupted
		}
		stream.err = err
		outcome := application.Outcome{Class: application.CleanClose}
		if err != nil {
			outcome = application.Outcome{Class: application.IndeterminateFailure, Reason: "text read did not complete"}
		}
		_ = request.CloseWithError(err)
		_ = response.CloseWithError(err)
		stream.done <- outcome
		close(stream.done)
		cancel()
	}()
	return stream
}

type installedReaderOutput struct {
	ctx    context.Context
	check  func() error
	output io.Writer
}

func (output installedReaderOutput) Write(body []byte) (int, error) {
	if err := errors.Join(output.ctx.Err(), output.check()); err != nil {
		return 0, err
	}
	n, err := output.output.Write(body)
	return n, errors.Join(err, output.ctx.Err(), output.check())
}

func (stream *installedReaderResult) Read(body []byte) (int, error)    { return stream.output.Read(body) }
func (stream *installedReaderResult) Write(body []byte) (int, error)   { return stream.input.Write(body) }
func (stream *installedReaderResult) CloseInput() error                { return stream.input.Close() }
func (stream *installedReaderResult) Done() <-chan application.Outcome { return stream.done }
func (stream *installedReaderResult) Close() error {
	select {
	case <-stream.joined:
		return stream.err
	default:
		stream.cancel()
		_ = stream.input.CloseWithError(context.Canceled)
		_ = stream.output.CloseWithError(context.Canceled)
		<-stream.joined
		return stream.err
	}
}
