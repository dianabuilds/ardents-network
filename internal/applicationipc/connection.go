package applicationipc

import (
	"errors"
	"net"
	"strings"
	"sync"
)

type connection struct {
	net.Conn
	result     net.Conn
	resultMu   sync.Mutex
	resultSent bool
}

// NewConnection preserves the Stage 4 raw Application byte stream and binds
// its classified Result to an optional owner-only channel selected by the peer.
func NewConnection(raw, result net.Conn) *connection {
	return &connection{Conn: raw, result: result}
}

// ResultPath derives the bounded local result-channel address without adding
// another caller-authored endpoint.
func ResultPath(applicationPath string) (string, error) {
	if applicationPath == "" || strings.HasSuffix(applicationPath, ".result") {
		return "", errors.New("application socket path cannot derive a result channel")
	}
	return applicationPath + ".result", nil
}

// SendResult emits the sole terminal result after all Application Data.
func (connection *connection) SendResult(result Result) error {
	connection.resultMu.Lock()
	defer connection.resultMu.Unlock()
	if connection.resultSent {
		return errors.New("terminal connection result was already sent")
	}
	connection.resultSent = true
	return Write(connection.result, result)
}

// Result reads the terminal result after the caller consumes its expected
// Application Data. EOF without a classified result is never success.
func (connection *connection) Result() (Result, error) { return Read(connection.result) }

// Close releases both local channels.
func (connection *connection) Close() error {
	return errors.Join(connection.Conn.Close(), connection.result.Close())
}
