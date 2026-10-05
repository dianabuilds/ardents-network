package transport

import "syscall"

func isPeerSocketRetirement(err error) bool {
	cause, ok := err.(syscall.Errno)
	// Go's portable errno values can occur in wrapped I/O failures; native
	// Windows TCP reset arrives as the distinct Winsock error instead.
	return ok && (cause == syscall.EPIPE || cause == syscall.ECONNRESET || cause == syscall.WSAECONNRESET)
}
