//go:build windows

package route

import (
	"errors"
	"syscall"
)

func platformBenignStreamError(err error) bool {
	return errors.Is(err, syscall.WSAECONNRESET) || errors.Is(err, syscall.WSAECONNABORTED)
}
