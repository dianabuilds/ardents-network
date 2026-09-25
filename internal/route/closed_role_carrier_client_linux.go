//go:build linux

package route

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/quic-go/quic-go"
)

// closedRoleOpenFailure retains the exact opening error while recording the
// bounded local operation that observed it. It never carries peer or payload
// data outside the original error chain.
type closedRoleOpenFailure struct {
	stage string
	cause error
}

func (failure *closedRoleOpenFailure) Error() string { return failure.cause.Error() }

func (failure *closedRoleOpenFailure) Unwrap() error { return failure.cause }

func closedRoleOpenFailureAt(stage string, cause error) error {
	return &closedRoleOpenFailure{stage: stage, cause: cause}
}

func closedRoleOpenFailureDetail(cause error) string {
	var stages []string
	for {
		failure, ok := cause.(*closedRoleOpenFailure)
		if !ok || failure.stage == "" {
			break
		}
		stages = append(stages, failure.stage)
		cause = failure.cause
	}
	if len(stages) == 0 {
		return "unknown"
	}
	switch {
	case errors.Is(cause, errCarrierPeerMissing):
		stages = append(stages, "peer-missing")
	case errors.Is(cause, errCarrierPeerMismatch):
		stages = append(stages, "peer-mismatch")
	case closedRoleQUICCryptoHandshakeError(cause):
		if len(stages) == 1 && stages[0] == "quic-dial" {
			stages[0] = "quic-handshake"
		} else {
			stages = append(stages, "quic-handshake")
		}
	case errors.Is(cause, context.Canceled):
		stages = append(stages, "canceled")
	case errors.Is(cause, context.DeadlineExceeded):
		stages = append(stages, "deadline")
	case errors.Is(cause, io.EOF):
		stages = append(stages, "eof")
	case errors.Is(cause, net.ErrClosed):
		stages = append(stages, "closed")
	case errors.Is(cause, syscall.ECONNREFUSED):
		stages = append(stages, "refused")
	case errors.Is(cause, syscall.ENETUNREACH):
		stages = append(stages, "network-unreachable")
	case errors.Is(cause, syscall.EHOSTUNREACH):
		stages = append(stages, "host-unreachable")
	case errors.Is(cause, syscall.EADDRNOTAVAIL):
		stages = append(stages, "address-unavailable")
	case errors.Is(cause, syscall.ECONNRESET):
		stages = append(stages, "reset")
	case errors.Is(cause, syscall.EACCES) || errors.Is(cause, syscall.EPERM):
		stages = append(stages, "permission")
	case errors.Is(cause, syscall.EMFILE) || errors.Is(cause, syscall.ENFILE) || errors.Is(cause, syscall.ENOBUFS):
		stages = append(stages, "resource")
	case errors.Is(cause, syscall.EINVAL):
		stages = append(stages, "invalid")
	default:
		var networkError net.Error
		if errors.As(cause, &networkError) && networkError.Timeout() {
			stages = append(stages, "timeout")
		} else {
			stages = append(stages, "other")
		}
	}
	return strings.Join(stages, "-")
}

func closedRoleQUICCryptoHandshakeError(cause error) bool {
	var transport *quic.TransportError
	return errors.As(cause, &transport) && transport.ErrorCode.IsCryptoError()
}

// OpenClosedRoleCarrier opens exactly one State-selected TCP/TLS or QUIC role
// channel. QUIC's TLS handshake is the role TLS; it is not wrapped in a
// second TLS stream.
func OpenClosedRoleCarrier(ctx context.Context, input ClosedRoleCarrierRequest) (net.Conn, error) {
	if ctx == nil || !literalEndpoint(input.Endpoint) || input.ExpectedServer == [32]byte{} || input.Deadline.IsZero() || !time.Now().Before(input.Deadline) {
		return nil, errors.New("closed role carrier request is invalid")
	}
	attempt, cancel := context.WithDeadline(ctx, input.Deadline)
	defer cancel()
	switch input.CarrierProfile {
	case ClosedCarrierTCP:
		raw, err := (&net.Dialer{}).DialContext(attempt, "tcp", input.Endpoint)
		if err != nil {
			return nil, closedRoleOpenFailureAt("tcp-dial", err)
		}
		secured, err := OpenClosedRoleTLS(attempt, raw, input.ExpectedServer, input.Deadline)
		if err != nil {
			_ = raw.Close()
			return nil, closedRoleOpenFailureAt("tcp-tls", err)
		}
		return secured, nil
	case ClosedCarrierQUIC:
		connection, err := quic.DialAddr(attempt, input.Endpoint, closedRoleClientTLS(input.ExpectedServer), closedRoleQUICConfig())
		if err != nil {
			return nil, closedRoleOpenFailureAt("quic-dial", err)
		}
		if err := validClosedRoleTLSState(connection.ConnectionState().TLS, input.ExpectedServer, false); err != nil {
			_ = connection.CloseWithError(1, "role-peer-invalid")
			return nil, closedRoleOpenFailureAt("quic-state", err)
		}
		stream, err := connection.OpenStreamSync(attempt)
		if err != nil {
			_ = connection.CloseWithError(1, "role-open-failed")
			return nil, closedRoleOpenFailureAt("quic-stream", err)
		}
		carrier := &closedRoleQUICCarrier{stream: stream, connection: connection}
		if err := carrier.SetDeadline(input.Deadline); err != nil {
			_ = carrier.Close()
			return nil, closedRoleOpenFailureAt("quic-deadline-set", err)
		}
		if err := carrier.SetDeadline(time.Time{}); err != nil {
			_ = carrier.Close()
			return nil, closedRoleOpenFailureAt("quic-deadline-clear", err)
		}
		return carrier, nil
	default:
		return nil, errors.New("closed role carrier profile is unsupported")
	}
}
