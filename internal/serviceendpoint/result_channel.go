package serviceendpoint

import (
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/applicationipc"
)

const resultAdmissionWindow = 50 * time.Millisecond

func optionalResultListener(applicationPath string, deadline time.Duration) (string, *net.UnixListener) {
	path, err := applicationipc.ResultPath(applicationPath)
	if err != nil {
		return "", nil
	}
	listener, err := listenLocal(path, deadline)
	if err != nil {
		return "", nil
	}
	return path, listener
}

// acceptOptionalResult preserves Stage 4 peers that connect only the raw data
// socket while allowing the maintained tracer to receive an early Result.
func acceptOptionalResult(listener *net.UnixListener) (net.Conn, error) {
	if err := listener.SetDeadline(time.Now().Add(resultAdmissionWindow)); err != nil {
		return nil, err
	}
	connection, err := listener.Accept()
	if err == nil {
		return connection, nil
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return nil, nil
	}
	return nil, err
}
