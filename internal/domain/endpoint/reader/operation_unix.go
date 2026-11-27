//go:build linux || darwin

package reader

import (
	"context"
	"errors"
	"sync"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/connection"
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/funding"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
)

// readerOperation retains one qualified Reader worker and its sole stream contract.
type readerOperation struct {
	worker    *executionruntime.Invocation
	operation *executionruntime.Operation
	contract  executionruntime.StreamContract
}

// beginRead qualifies the original worker before any Source or Application effect.
func beginRead(ctx context.Context, local *executionruntime.Owner, principal [32]byte) (*readerOperation, error) {
	if ctx == nil || local == nil {
		return nil, errors.New("original Reader runtime is unavailable")
	}
	var worker *executionruntime.Invocation
	var err error
	worker, err = local.LaunchByteStreamReader(ctx, principal)
	send, receive := application.ByteStreamDirectionalLimit, application.ByteStreamDirectionalLimit
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
	return &readerOperation{worker: worker, operation: operation, contract: contract}, nil
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
