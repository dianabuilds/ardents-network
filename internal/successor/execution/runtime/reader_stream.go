package runtime

import (
	"bytes"
	"errors"
	"sync"

	application "github.com/dianabuilds/ardents-network/internal/application/connection"
)

type readerCompletion struct {
	body []byte
	err  error
}

type readerConnectionInput struct {
	incoming  chan application.Stream
	result    chan readerCompletion
	finish    chan struct{}
	completed bool // Invocation.useMu owns the retained RESULT barrier.
	attempted bool // Original Invocation useMu owns the one transfer attempt.
}

// ReaderResult retains bounded Text bytes privately until original Service I/O,
// operation and worker descendants have joined. It is never live permission.
type ReaderResult struct {
	self       *ReaderResult
	operation  *Operation
	invocation *Invocation
	mu         sync.Mutex
	body       []byte
	used       bool
}

// DeliverReaderStream gives the sole initialized Text reader one authenticated
// stream. A true transfer remains receiver-owned even if completion fails.
// The caller closes its Operation and Invocation before consuming the result.
func (operation *Operation) DeliverReaderStream(contract StreamContract, stream application.Stream) (transferred bool, result *ReaderResult, failure error) {
	if stream == nil {
		return false, nil, errors.New("reader stream is absent")
	}
	if _, _, err := contract.Limits(operation); err != nil {
		return false, nil, err
	}
	if err := operation.CheckReader(); err != nil {
		return false, nil, err
	}
	invocation := operation.invocation
	if invocation == nil {
		return false, nil, errors.New("original Reader invocation is absent")
	}
	invocation.useMu.Lock()
	input := invocation.readerInput
	if invocation.closing || invocation.operation != operation || input == nil || input.attempted {
		invocation.useMu.Unlock()
		return false, nil, errors.New("original Reader stream handoff is unavailable or consumed")
	}
	input.attempted = true
	invocation.useMu.Unlock()
	select {
	case <-operation.Context().Done():
		return false, nil, operation.Context().Err()
	case input.incoming <- stream:
		transferred = true
	}
	// Receiver checks the exact operation at worker effects and stream join.
	// A successful result remains private while the original worker waits at
	// its RESULT barrier; caller-owned Source cleanup precedes Operation.Close
	// and final worker/Job retirement. It grants no second stream handoff.
	completion := <-input.result
	if completion.err != nil {
		clear(completion.body)
		return true, nil, completion.err
	}
	result = &ReaderResult{operation: operation, invocation: invocation, body: completion.body}
	result.self = result
	return true, result, nil
}

// TakeBytes checks joined original-current completion, never an unjoined READY
// or replaced Job. The result is consumed once; its private copy is erased.
func (result *ReaderResult) TakeBytes() ([]byte, error) {
	if result == nil || result.self != result || result.operation == nil || result.invocation == nil || result.operation.caller == nil || result.invocation.readerCaller == nil {
		return nil, errors.New("reader result is absent")
	}
	result.mu.Lock()
	defer result.mu.Unlock()
	if result.used {
		return nil, errors.New("reader result is consumed")
	}
	invocation := result.invocation
	invocation.useMu.Lock()
	original := invocation.operation == result.operation && result.operation.invocation == invocation
	invocation.useMu.Unlock()
	if !original || !invocation.CompletedCurrent() || result.operation.caller.Err() != nil || invocation.readerCaller.Err() != nil {
		return nil, errors.New("original Reader completion is unavailable")
	}
	body := bytes.Clone(result.body)
	if !invocation.CompletedCurrent() || result.operation.caller.Err() != nil || invocation.readerCaller.Err() != nil {
		clear(body)
		return nil, errors.New("original Reader completion ended during result handoff")
	}
	clear(result.body)
	result.body = nil
	result.used = true
	return body, nil
}

// Close discards private pending output; it grants no completion or permission.
func (result *ReaderResult) Close() {
	if result == nil || result.self != result {
		return
	}
	result.mu.Lock()
	defer result.mu.Unlock()
	clear(result.body)
	result.body = nil
	result.used = true
}
