package administration

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
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

// Server owns the lifecycle of one private local Administration transport.
type Server interface {
	Close() error
}

type server struct {
	path      string
	listener  *net.UnixListener
	ctx       context.Context
	cancel    context.CancelFunc
	work      sync.WaitGroup
	mu        sync.Mutex
	clients   map[*net.UnixConn]struct{}
	once      sync.Once
	err       error
	owner     Interface
	snapshots chan struct{}
}

// Listen exposes owner on one explicit absolute Unix-socket path.
func Listen(path string, owner Interface) (Server, error) {
	if path == "" || !filepath.IsAbs(path) || owner == nil {
		return nil, errors.New("local Service Administration configuration is invalid")
	}
	if _, err := os.Lstat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("local Service Administration attachment already exists")
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	listener.SetUnlinkOnClose(false)
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := &server{path: path, listener: listener, ctx: ctx, cancel: cancel, owner: owner,
		clients: make(map[*net.UnixConn]struct{}), snapshots: make(chan struct{}, 1)}
	server.work.Add(1)
	go server.serve()
	return server, nil
}

func (server *server) serve() {
	defer server.work.Done()
	for {
		connection, err := server.listener.AcceptUnix()
		if err != nil {
			return
		}
		server.mu.Lock()
		if server.ctx.Err() != nil {
			server.err = errors.Join(server.err, connection.Close())
			server.mu.Unlock()
			return
		}
		server.clients[connection] = struct{}{}
		server.mu.Unlock()
		server.work.Add(1)
		go func() {
			defer server.work.Done()
			defer func() {
				server.mu.Lock()
				if _, owned := server.clients[connection]; owned {
					server.err = errors.Join(server.err, connection.Close())
					delete(server.clients, connection)
				}
				server.mu.Unlock()
			}()
			server.handle(connection)
		}()
	}
}

func (server *server) handle(connection *net.UnixConn) {
	deadline := time.Now().Add(15 * time.Second)
	_ = connection.SetDeadline(deadline)
	input := requestInput{connection: connection, deadline: deadline}
	raw, err := io.ReadAll(io.LimitReader(input, int64(maximumRequest)))
	if err == nil && string(raw) == snapshotRequest {
		server.handleSnapshot(connection, input)
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
	if surplusRead != 0 && surplusErr == nil {
		if _, err := io.CopyN(io.Discard, input, 4096); !errors.Is(err, io.EOF) {
			return
		}
	}
	writeResponse(connection, "unavailable\n")
}

func writeResponse(connection *net.UnixConn, response string) {
	_, _ = io.WriteString(connection, response)
	_ = connection.CloseWrite()
}

// Close refuses new callers, cancels operations, and removes only this exact
// socket path.
func (server *server) Close() error {
	if server == nil {
		return nil
	}
	server.once.Do(func() {
		server.cancel()
		listenerErr := server.listener.Close()
		server.mu.Lock()
		server.err = errors.Join(server.err, listenerErr)
		for client := range server.clients {
			server.err = errors.Join(server.err, client.Close())
			delete(server.clients, client)
		}
		server.mu.Unlock()
		server.work.Wait()
		server.err = errors.Join(server.err, os.Remove(server.path))
	})
	return server.err
}
