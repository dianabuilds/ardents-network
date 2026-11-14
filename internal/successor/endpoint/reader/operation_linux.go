//go:build linux

package reader

import (
	"context"
	"errors"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/connection"
	"github.com/dianabuilds/ardents-network/internal/successor/endpoint/funding"
	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
)

// Operation retains one qualified Reader worker and its sole stream contract.
// The adapter may bind its original Network/Stock after Begin, using Check at
// their effects. Read joins its Source before closing the operation and worker.
// A caller closes Operation after its Read or local result stream has joined.
type Operation struct {
	caller    context.Context
	worker    *executionruntime.Invocation
	operation *executionruntime.Operation
	contract  executionruntime.StreamContract
}

// Begin qualifies the original worker before any Source or Application effect.
func Begin(ctx context.Context, local *executionruntime.Owner, principal [32]byte) (*Operation, error) {
	if ctx == nil || local == nil {
		return nil, errors.New("original Reader runtime is unavailable")
	}
	worker, err := local.LaunchReader(ctx, principal)
	if err != nil {
		return nil, err
	}
	operation, err := worker.BeginOperation(ctx)
	if err != nil {
		return nil, errors.Join(err, worker.Close())
	}
	contract, err := operation.BindStreamContract(512, uint64(textdocument.MaximumBytes+13))
	if err != nil {
		operation.Close()
		return nil, errors.Join(err, worker.Close())
	}
	return &Operation{caller: ctx, worker: worker, operation: operation, contract: contract}, nil
}

// Check rechecks the original live Execution operation; it lends no new grant.
func (read *Operation) Check() error { return read.operation.CheckReader() }

// Close retires the original operation/worker after all Source borrowers join.
func (read *Operation) Close() error {
	if read == nil {
		return nil
	}
	read.operation.Close()
	return read.worker.Close()
}

// Read completes one ordinary text read. Only bounded, joined, original-current
// result bytes leave this owner; the caller erases its returned copy after use.
func (read *Operation) Read(plan funding.Plan, authority funding.Authority, holder *stock.Owner, target string) ([]byte, error) {
	pending, err := read.prepare(plan, authority, holder, target)
	if err != nil {
		return nil, err
	}
	return pending.complete(nil)
}

// preparedRead owns the Source and its exact operation/worker retirement. It is
// published only after setup; concurrent result cancellation then shares Close.
type preparedRead struct {
	read        *Operation
	service     *connection.Stream
	closeSource func() error
	once        sync.Once
	result      error
}

func (read *Operation) prepare(plan funding.Plan, authority funding.Authority, holder *stock.Owner, target string, shared ...*funding.Parents) (_ *preparedRead, result error) {
	pending := &preparedRead{read: read}
	defer func() {
		if result != nil {
			result = errors.Join(result, pending.close())
		}
	}()
	source, closeSource, err := openSource(read.operation, plan, authority, holder, target, shared...)
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
	err = errors.Join(pending.close(), read.caller.Err())
	if check != nil {
		err = errors.Join(err, check())
	}
	if err != nil {
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
		pending.result = errors.Join(pending.result, pending.read.Close())
	})
	return pending.result
}
