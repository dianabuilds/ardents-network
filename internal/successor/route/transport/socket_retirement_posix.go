//go:build !windows

package transport

import "syscall"

func isPeerSocketRetirement(err error) bool {
	cause, ok := err.(syscall.Errno)
	return ok && (cause == syscall.EPIPE || cause == syscall.ECONNRESET)
}
