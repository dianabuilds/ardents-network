//go:build linux

package carrier

import (
	"syscall"

	"github.com/quic-go/quic-go"
)

// IsPeerRetirementCause classifies one physical I/O cause without discarding it.
// Every branch must describe a peer retirement; a joined unrelated failure is
// never classified from another branch's closed sentinel. Callers still retain
// the original failure and must join and release the original work.
func IsPeerRetirementCause(err error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !IsPeerRetirementCause(child) {
				return false
			}
		}
		return true
	}
	if peer, ok := err.(*quic.ApplicationError); ok {
		return peer.Remote && peer.ErrorCode == 0 && (peer.ErrorMessage == "carrier-close" || peer.ErrorMessage == "role-close")
	}
	if cause, ok := err.(syscall.Errno); ok {
		return cause == syscall.EPIPE || cause == syscall.ECONNRESET
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return IsPeerRetirementCause(wrapped.Unwrap())
	}
	return false
}
