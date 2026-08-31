package endpoint

import (
	"net"
	"time"

	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/connection"
)

func listenResult(applicationPath string, deadline time.Duration) (string, *net.UnixListener, error) {
	path, err := applicationconnection.ResultPath(applicationPath)
	if err != nil {
		return "", nil, err
	}
	listener, err := listenLocal(path, deadline)
	if err != nil {
		return "", nil, err
	}
	return path, listener, nil
}
