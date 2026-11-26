//go:build linux || darwin

package reader

import (
	"context"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/connection"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	executionruntime "github.com/dianabuilds/ardents-network/internal/execution/runtime"
)

// readerOperation retains one qualified Reader worker and its sole stream contract.
type readerOperation struct {
	caller    context.Context
	worker    *executionruntime.Invocation
	operation *executionruntime.Operation
	contract  executionruntime.StreamContract
}

// beginRead qualifies the original worker before any Source or Application effect.
func beginRead(ctx context.Context, local *executionruntime.Owner, principal [32]byte, profile application.Application) (*readerOperation, error) {
	if ctx == nil || local == nil {
		return nil, errors.New("original Reader runtime is unavailable")
	}
	var worker *executionruntime.Invocation
	var err error
	send, receive := uint64(512), uint64(document.MaximumBytes+13)
	switch profile {
	case application.Document:
		worker, err = local.LaunchReader(ctx, principal)
	case application.ByteStream:
		worker, err = local.LaunchByteStreamReader(ctx, principal)
		send, receive = application.ByteStreamDirectionalLimit, application.ByteStreamDirectionalLimit
	default:
		return nil, errors.New("Reader Application profile is unavailable")
	}
	if err != nil {
		return nil, err
	}
	operation, err := worker.BeginOperation(ctx)
	if err != nil {
		return nil, errors.Join(err, worker.Close())
	}
	contract, err := operation.BindStreamContract(send, receive)
	if err != nil {
		operation.Close()
		return nil, errors.Join(err, worker.Close())
	}
	return &readerOperation{caller: ctx, worker: worker, operation: operation, contract: contract}, nil
}

// check rechecks the original live Execution operation; it lends no new grant.
func (read *readerOperation) check() error { return read.operation.CheckReader() }

// close retires the original operation/worker after all Source borrowers join.
func (read *readerOperation) close() error {
	if read == nil {
		return nil
	}
	read.operation.Close()
	return read.worker.Close()
}

// preparedRead owns the Source and its exact operation/worker retirement. It is
// published only after setup; concurrent result cancellation then shares Close.
type preparedRead struct {
	read        *readerOperation
	service     *connection.Stream
	closeSource func() error
	once        sync.Once
	result      error
}

func (read *readerOperation) prepare(plan funding.Plan, authority funding.Authority, holder *stock.Owner, target string, shared *funding.Parents) (_ *preparedRead, result error) {
	pending := &preparedRead{read: read}
	defer func() {
		if result != nil {
			result = errors.Join(result, pending.close())
		}
	}()
	source, closeSource, err := openSource(read.operation, plan, authority, holder, target, shared)
	if err != nil {
		return nil, err
	}
	pending.closeSource = closeSource
	pending.service, err = source.OpenStream(read.contract)
	if err != nil {
		return nil, err
	}
	return pending, nil
}

func (pending *preparedRead) complete(check func() error) ([]byte, error) {
	read := pending.read
	transferred, response, err := read.operation.DeliverReaderStream(read.contract, pending.service)
	if !transferred {
		_ = pending.service.Close()
		read.operation.RetainCleanup(pending.service.JoinedCleanup())
	}
	if err != nil || response == nil {
		return nil, errors.Join(err, errors.New("original Reader result is unavailable"), pending.close())
	}
	defer response.Close()
	if err := errors.Join(pending.close(), read.caller.Err(), check()); err != nil {
		return nil, err
	}
	return response.TakeBytes()
}

func (pending *preparedRead) close() error {
	pending.once.Do(func() {
		if pending.closeSource != nil {
			pending.result = pending.closeSource()
		} else if pending.service != nil {
			pending.result = pending.service.Close()
			pending.read.operation.RetainCleanup(pending.service.JoinedCleanup())
		}
		pending.result = errors.Join(pending.result, pending.read.close())
	})
	return pending.result
}
