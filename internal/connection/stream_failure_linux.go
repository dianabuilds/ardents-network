//go:build linux

package connection

import (
	"context"
	"errors"
	"io"
	"os"
)

// StreamFailureReason projects fixed Connection failure categories. It omits
// transport addresses, destinations, authority and detailed child errors.
func StreamFailureReason(err error) string {
	switch {
	case errors.Is(err, errRecoveryExhausted):
		return "Service Connection recovery bound reached"
	case errors.Is(err, errIntegrity):
		return "Service Connection rejected an integrity contradiction"
	case errors.Is(err, os.ErrDeadlineExceeded):
		return "Service Connection transport deadline reached"
	case errors.Is(err, context.DeadlineExceeded):
		return "Service Connection lifetime expired"
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "Service Connection was truncated"
	default:
		return ""
	}
}
