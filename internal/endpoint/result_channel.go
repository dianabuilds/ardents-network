package endpoint

import (
	"bytes"
	"errors"
	"io"
	"net"
	"time"
)

func listenResult(applicationPath string, deadline time.Duration) (string, *net.UnixListener, error) {
	path, err := ResultPath(applicationPath)
	if err != nil {
		return "", nil, err
	}
	listener, err := listenLocal(path, deadline)
	if err != nil {
		return "", nil, err
	}
	return path, listener, nil
}

func acceptApplication(raw net.Conn, deadline time.Time) error {
	if raw == nil || deadline.IsZero() {
		return errors.New("application contract is incomplete")
	}
	if err := raw.SetDeadline(deadline); err != nil {
		return err
	}
	hello := make([]byte, len(applicationHello))
	if _, err := io.ReadFull(raw, hello); err != nil {
		return errors.Join(err, errors.New("application contract handshake is absent"))
	}
	if !bytes.Equal(hello, applicationHello) {
		return errors.New("application contract is unsupported")
	}
	return raw.SetDeadline(time.Time{})
}

func acceptResult(listener *net.UnixListener, deadline time.Time) (net.Conn, error) {
	if listener == nil || deadline.IsZero() {
		return nil, errors.New("application result listener is incomplete")
	}
	if err := listener.SetDeadline(deadline); err != nil {
		return nil, err
	}
	return listener.Accept()
}
