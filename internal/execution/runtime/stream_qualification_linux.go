//go:build linux

package runtime

import (
	"context"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/execution"
	"github.com/dianabuilds/ardents-network/internal/execution/worker"
)

// streamProtocol is the fixed Stream qualification inventory. It mirrors the
// ordinary Text launch path exactly (same gate, artifact, Job, Grant, lease and
// joined cleanup) but activates the stream worker, performs the fixed stream
// INIT/READY exchange and reobserves through the stream inventory. The trusted
// qualification composition selects it only through LaunchStreamQualification;
// ordinary Text jobs cannot reach it.
type streamProtocol struct {
	explicitRole string
	profile      streamqualification.Profile
	seed         [32]byte
	input        *streamQualificationInput
}

func (streamProtocol) activate(ctx context.Context, role string) (*worker.Activation, error) {
	return worker.ActivateStream(ctx, role)
}

func (protocol streamProtocol) initialize(ctx context.Context, attachment *worker.Attachment, nonce [32]byte, role string) error {
	protocol.input.init = streamqualification.Init{
		Role:    streamRole(role),
		Profile: protocol.profile,
		Nonce:   nonce,
		Seed:    protocol.seed,
	}
	return streamqualification.InitializeWorker(attachment, protocol.input.init)
}

func (streamProtocol) observe(ctx context.Context, name, role string) (worker.Instance, error) {
	return worker.ObserveInstanceStream(ctx, name, role)
}

func (protocol streamProtocol) role(execution.Surface) string { return protocol.explicitRole }

func (protocol streamProtocol) consumer() attachmentConsumer { return protocol.input }

func streamRole(role string) streamqualification.Role {
	if role == "publisher" {
		return streamqualification.PublisherRole
	}
	return streamqualification.ReaderRole
}

type streamQualificationRequest struct {
	ctx      context.Context
	supplied []streamqualification.BoundStream
	observe  func(context.Context, streamqualification.Report) error
}

type streamQualificationResult struct {
	report streamqualification.Report
	err    error
}

// streamQualificationInput owns the sole qualification attachment observation.
// The original Invocation useMu owns the one transfer attempt; the bridge runs
// inside this consumer goroutine and owns/closes the attachment and every
// supplied Service stream. No attachment is exported to the caller.
type streamQualificationInput struct {
	init      streamqualification.Init
	request   chan streamQualificationRequest
	result    chan streamQualificationResult
	attempted bool
}

func (input *streamQualificationInput) observe(invocation *Invocation) {
	go func() {
		defer close(invocation.observationDone)
		var result streamQualificationResult
		select {
		case request := <-input.request:
			borrow := &streamWorkerBorrow{invocation: invocation}
			report, err := streamqualification.RunConnections(request.ctx, borrow, input.init, request.supplied, request.observe)
			result = streamQualificationResult{report: report, err: err}
		case <-invocation.job.Context().Done():
			// The worker ended before any operation claimed the attachment.
			var byteBuffer [1]byte
			_, err := invocation.activation.Attachment.Read(byteBuffer[:])
			if invocation.job.Retire() {
				err = errors.Join(errUnexpectedWorkerEnd, err)
			}
			result = streamQualificationResult{err: err}
		}
		invocation.observationError = result.err
		input.result <- result
		invocation.job.Retire()
		close(input.result)
	}()
}

// streamWorkerBorrow adapts the retained attachment to the bridge while
// rechecking the exact original operation on every I/O. It mirrors the Text
// reader borrow: possession grants no authority, and Close captures final
// authority before physical retirement and retains the first cleanup failure.
type streamWorkerBorrow struct {
	invocation *Invocation
	once       sync.Once
	closeErr   error
}

func (borrow *streamWorkerBorrow) checkOperation() error {
	invocation := borrow.invocation
	invocation.useMu.Lock()
	operation := invocation.operation
	invocation.useMu.Unlock()
	if operation == nil || operation.invocation != invocation {
		return errors.New("original qualification operation is unavailable")
	}
	return operation.Check()
}

func (borrow *streamWorkerBorrow) Read(body []byte) (int, error) {
	if err := borrow.invocation.job.Check(); err != nil {
		return 0, err
	}
	n, err := borrow.invocation.activation.Attachment.Read(body)
	return n, errors.Join(err, borrow.checkOperation())
}

func (borrow *streamWorkerBorrow) Write(body []byte) (int, error) {
	if err := borrow.checkOperation(); err != nil {
		return 0, err
	}
	n, err := borrow.invocation.activation.Attachment.Write(body)
	return n, errors.Join(err, borrow.checkOperation())
}

func (borrow *streamWorkerBorrow) Close() error {
	borrow.once.Do(func() {
		authority := borrow.checkOperation()
		physical := borrow.invocation.activation.Attachment.Close()
		borrow.closeErr = errors.Join(authority, physical)
		borrow.invocation.useMu.Lock()
		operation := borrow.invocation.operation
		borrow.invocation.useMu.Unlock()
		if operation != nil {
			operation.RetainCleanup(physical)
		}
	})
	return borrow.closeErr
}

// LaunchStreamQualification admits one exact local Connection session and
// launches the fixed installed Stream qualification worker with the fixed
// INIT/READY exchange (fresh invocation nonce, supplied workload seed and
// predeclared profile). It returns only the privately bound original invocation
// after installed READY reobservation. The caller supplies authenticated Service
// streams later through Operation.RunStreamConnections; no attachment, Service
// authentication, Route acquisition or destination is exported or granted here.
func (owner *Owner) LaunchStreamQualification(ctx context.Context, principal [32]byte, role string, profile streamqualification.Profile, seed [32]byte) (_ *Invocation, result error) {
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return nil, errors.New("stream qualification launch is unavailable")
	}
	if role != "reader" && role != "publisher" {
		return nil, errors.New("stream qualification role is invalid")
	}
	if seed == [32]byte{} {
		return nil, errors.New("stream qualification seed is unavailable")
	}
	if _, err := profile.Definition(streamRole(role)); err != nil {
		return nil, err
	}
	capability, err := owner.authority.Admit(principal, execution.Connection)
	if err != nil {
		return nil, err
	}
	session, err := owner.supervisor.Activate(ctx, capability, principal, execution.Connection)
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, session.Close())
		}
	}()
	job, err := session.BeginJob()
	if err != nil {
		return nil, err
	}
	protocol := streamProtocol{
		explicitRole: role,
		profile:      profile,
		seed:         seed,
		input: &streamQualificationInput{
			request: make(chan streamQualificationRequest),
			result:  make(chan streamQualificationResult, 1),
		},
	}
	return owner.launchInitializedJob(job, execution.Connection, protocol, false, nil)
}

// RunStreamConnections gives the sole qualification operation its authenticated
// Service stream set and runs the fixed bridge over the retained attachment
// inside the original invocation. The bridge owns and joins the attachment and
// every supplied stream; per-stream credit/EOF/terminal checks and the original
// 64MiB directional contract remain. One transfer attempt per invocation; the
// caller closes its Operation and Invocation after Source joins. The returned
// Report is local evidence, never a worker authority or admission receipt.
func (operation *Operation) RunStreamConnections(ctx context.Context, contract StreamContract, supplied []streamqualification.BoundStream, observe func(context.Context, streamqualification.Report) error) (streamqualification.Report, error) {
	if ctx == nil {
		return streamqualification.Report{}, errors.New("qualification stream context is unavailable")
	}
	if err := operation.Check(); err != nil {
		return streamqualification.Report{}, err
	}
	if _, _, err := contract.Limits(operation); err != nil {
		return streamqualification.Report{}, err
	}
	invocation := operation.invocation
	if invocation == nil {
		return streamqualification.Report{}, errors.New("original qualification invocation is absent")
	}
	invocation.useMu.Lock()
	input, ok := invocation.consumer.(*streamQualificationInput)
	if !ok || invocation.closing || invocation.operation != operation || input == nil || input.attempted {
		invocation.useMu.Unlock()
		return streamqualification.Report{}, errors.New("original qualification stream handoff is unavailable or consumed")
	}
	input.attempted = true
	invocation.useMu.Unlock()
	request := streamQualificationRequest{ctx: operation.Context(), supplied: supplied, observe: observe}
	select {
	case <-operation.Context().Done():
		return streamqualification.Report{}, operation.Context().Err()
	case <-ctx.Done():
		return streamqualification.Report{}, ctx.Err()
	case input.request <- request:
	}
	result, ok := <-input.result
	if !ok {
		return streamqualification.Report{}, errors.New("original qualification stream result is unavailable")
	}
	return result.report, result.err
}
