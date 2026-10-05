//go:build linux

package transport

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/successor/route/carrier"
)

// physicalWriteFailure is created only after a session selects and starts its
// original physical frame. Queue refusal and ordinary EOF cannot mint it.
type physicalWriteFailure struct {
	owner *session
	kind  uint8
	cause error
}

func (e *physicalWriteFailure) Error() string {
	return fmt.Sprintf("route frame %d physical write: %v", e.kind, e.cause)
}
func (e *physicalWriteFailure) Unwrap() error { return e.cause }

// physicalCloseFailure is minted only by the exact session's owned Conn.Close.
// An equal native error elsewhere is not evidence of this physical operation.
type physicalCloseFailure struct {
	owner *session
	cause error
}

func (e *physicalCloseFailure) Error() string {
	return fmt.Sprintf("route physical close: %v", e.cause)
}
func (e *physicalCloseFailure) Unwrap() error { return e.cause }

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
		return frame.cause != nil, carrier.IsPeerRetirementCause(frame.cause)
	}
	if close, ok := err.(*physicalCloseFailure); ok {
		peer := close.owner != nil && carrier.IsPeerRetirementCause(close.cause)
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
	writes := make(map[*session]bool)
	var closes []*session
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

// retiredConn retains one physical close result across independent cancellation
// and borrower cleanup. A later benign ErrClosed cannot mask the first failure.
type retiredConn struct {
	net.Conn
	once sync.Once
	err  error
}

func (c *retiredConn) Close() error {
	c.once.Do(func() {
		// Revocation interrupts physical I/O; closeNotify would be a new write.
		if secured, ok := c.Conn.(*tls.Conn); ok {
			c.err = secured.NetConn().Close()
		} else {
			c.err = c.Conn.Close()
		}
	})
	return c.err
}
func (r *Receiver) closeListener() error {
	r.listenerOnce.Do(func() { r.listenerErr = r.listener.Close() })
	return r.listenerErr
}
