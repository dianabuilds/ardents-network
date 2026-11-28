package administration

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"time"
)

const (
	maximumRequest       = len("withdraw\n")
	requestReadRearmWait = 50 * time.Millisecond
)

type requestDeadlineReader interface {
	io.Reader
	SetReadDeadline(time.Time) error
}

type requestInput struct {
	connection requestDeadlineReader
	deadline   time.Time
}

// Read re-arms a request read if its early deadline expires before the
// connection's original deadline. Windows AF_UNIX can otherwise leave a read
// waiting for EOF after a concurrent peer CloseWrite.
func (input requestInput) Read(body []byte) (int, error) {
	for {
		readDeadline := time.Now().Add(requestReadRearmWait)
		if readDeadline.After(input.deadline) {
			readDeadline = input.deadline
		}
		if err := input.connection.SetReadDeadline(readDeadline); err != nil {
			return 0, err
		}
		n, err := input.connection.Read(body)
		if n == 0 && errors.Is(err, os.ErrDeadlineExceeded) && time.Now().Before(input.deadline) {
			continue
		}
		return n, err
	}
}

func (server *server) handle(connection *net.UnixConn, started time.Time) {
	deadline := started.Add(15 * time.Second)
	_ = connection.SetDeadline(deadline)
	input := requestInput{connection: connection, deadline: deadline}
	raw, err := io.ReadAll(io.LimitReader(input, int64(maximumRequest)))
	if err == nil && string(raw) == snapshotRequest {
		server.handleSnapshot(connection, input, started.Add(SnapshotTimeout))
		return
	}
	if err == nil && server.handleByteStream(connection, raw, started) {
		return
	}
	var trailing [1]byte
	n, tailErr := input.Read(trailing[:])
	if err != nil || n != 0 || tailErr != io.EOF {
		refuseMalformedRequest(connection, input, n, tailErr)
		return
	}
	if string(raw) == "link\n" {
		server.handlePublishedLink(connection)
		return
	}
	var operation func(context.Context) error
	var response string
	switch string(raw) {
	case "publish\n":
		operation, response = server.owner.Publish, "published\n"
	case "withdraw\n":
		operation, response = server.owner.Withdraw, "withdrawn\n"
	default:
		writeResponse(connection, "unavailable\n")
		return
	}
	if err := operation(server.ctx); err != nil {
		writeResponse(connection, "unavailable\n")
		return
	}
	writeResponse(connection, response)
}

// refuseMalformedRequest drains a bounded surplus after the first extra byte.
// Closing a Windows Unix socket with unread inbound bytes can reset the peer
// before it receives the refusal. Requests beyond this bound are closed.
func refuseMalformedRequest(connection *net.UnixConn, input io.Reader, surplusRead int, surplusErr error) {
	if drainMalformedTail(input, surplusRead, surplusErr) {
		writeResponse(connection, "unavailable\n")
	}
}

func drainMalformedTail(input io.Reader, surplusRead int, surplusErr error) bool {
	if surplusRead == 0 || surplusErr != nil {
		return true
	}
	_, err := io.CopyN(io.Discard, input, 4096)
	return errors.Is(err, io.EOF)
}

func writeResponse(connection *net.UnixConn, response string) {
	_, _ = io.WriteString(connection, response)
	_ = connection.CloseWrite()
}

// Close refuses new callers, cancels operations, and removes only this exact
// socket entry.
