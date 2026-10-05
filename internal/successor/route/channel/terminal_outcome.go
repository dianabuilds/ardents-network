package channel

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// TerminalFailureStage describes the complete retained error tree for command
// reporting. It does not turn any failure into success or change its outcome.
func TerminalFailureStage(err error) string {
	if err == nil {
		return "completed"
	}
	physical, peer := physicalFailureTree(err)
	var write *physicalWriteFailure
	hasWrite := errors.As(err, &write) && write.cause != nil
	physical = physical && closeOwnersHaveWrites(err)
	if physical && peer && hasWrite {
		return "peer-retired-write"
	}
	if physical && hasWrite {
		return "physical-write"
	}
	return "terminal"
}

func physicalFailureTree(err error) (physical, peer bool) {
	if err == nil {
		return false, false
	}
	if frame, ok := err.(*physicalWriteFailure); ok {
		return frame.cause != nil, transport.IsPeerRetirementCause(frame.cause)
	}
	if close, ok := err.(*physicalCloseFailure); ok {
		peer := close.owner != nil && transport.IsPeerRetirementCause(close.cause)
		return peer, peer
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false, false
		}
		physical, peer = true, true
		for _, child := range children {
			p, c := physicalFailureTree(child)
			physical = physical && p
			peer = peer && c
		}
		return physical, peer
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return physicalFailureTree(wrapped.Unwrap())
	}
	return false, false
}

func closeOwnersHaveWrites(err error) bool {
	writes := make(map[*Session]bool)
	var closes []*Session
	var visit func(error)
	visit = func(err error) {
		switch failure := err.(type) {
		case *physicalWriteFailure:
			if failure.owner != nil && failure.cause != nil {
				writes[failure.owner] = true
			}
		case *physicalCloseFailure:
			closes = append(closes, failure.owner)
		case interface{ Unwrap() []error }:
			for _, child := range failure.Unwrap() {
				visit(child)
			}
		case interface{ Unwrap() error }:
			visit(failure.Unwrap())
		}
	}
	visit(err)
	for _, owner := range closes {
		if owner == nil || !writes[owner] {
			return false
		}
	}
	return true
}
