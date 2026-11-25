package connection

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
)

// StreamFailureReason projects fixed Connection failure categories. It omits
// transport addresses, destinations, authority and detailed child errors.
func StreamFailureReason(err error) string {
	switch {
	case errors.Is(err, errRecoveryExhausted):
		const exhausted = "Service Connection recovery bound reached"
		if lost, ok := errors.AsType[originalAttachmentFailure](err); ok {
			return exhausted + " after " + attachmentFailureReason(lost.cause)
		}
		return exhausted
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

func attachmentFailureReason(err error) string {
	switch {
	case errors.Is(err, os.ErrDeadlineExceeded), errors.Is(err, context.DeadlineExceeded):
		return "the original attachment timed out"
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "the original attachment was truncated"
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrClosedPipe), errors.Is(err, net.ErrClosed):
		return "the original attachment closed"
	default:
		return "the original attachment failed"
	}
}
