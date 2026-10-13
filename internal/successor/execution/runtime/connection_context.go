package runtime

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

// CheckReader requires the original qualified ordinary Reader invocation.
// A setup-only Connection worker or Publisher cannot become a Reader.
func (operation *Operation) CheckReader() error {
	if operation == nil || operation.surface != execution.Connection || operation.publisher || operation.invocation == nil {
		return errors.New("qualified Reader operation is absent")
	}
	invocation := operation.invocation
	invocation.useMu.Lock()
	valid := !invocation.closing && invocation.operation == operation && invocation.readerInput != nil && invocation.readerCaller != nil
	caller := invocation.readerCaller
	invocation.useMu.Unlock()
	if !valid {
		return errors.New("original ordinary Reader invocation is absent")
	}
	return errors.Join(operation.Check(), caller.Err())
}

// ConnectionContext is local to the original Endpoint operation. Only its
// salted opaque commitment enters Service transcripts; this value is never
// a remote identity, public permission or replacement Job.
func (operation *Operation) ConnectionContext() ([32]byte, error) {
	if err := operation.CheckReader(); err != nil {
		return [32]byte{}, err
	}
	if operation.connectionContext == [32]byte{} {
		return [32]byte{}, errors.New("original Connection context is absent")
	}
	return operation.connectionContext, nil
}

// ClaimConnection consumes the sole initial logical Connection construction.
// Failure after this handoff cannot create another Application request under
// the same invocation. Recovery retains its original binding instead.
func (operation *Operation) ClaimConnection(expected [32]byte) error {
	if err := operation.CheckReader(); err != nil {
		return err
	}
	operation.contractMu.Lock()
	defer operation.contractMu.Unlock()
	if expected == [32]byte{} || operation.connectionContext != expected || operation.connectionClaimed {
		return errors.New("original Connection construction is absent or consumed")
	}
	if err := operation.CheckReader(); err != nil {
		return err
	}
	operation.connectionClaimed = true
	return nil
}
