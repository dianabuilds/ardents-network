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

const maximumRequest = len("withdraw\n")

// Server owns one private local transport for Service Administration.
type Server struct {
	path     string
	listener *net.UnixListener
	ctx      context.Context
	cancel   context.CancelFunc
	work     sync.WaitGroup
	mu       sync.Mutex
	clients  map[*net.UnixConn]struct{}
	once     sync.Once
	err      error
	owner    Interface
}

// Listen exposes owner on one explicit absolute Unix-socket path.
func Listen(path string, owner Interface) (*Server, error) {
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
	server := &Server{path: path, listener: listener, ctx: ctx, cancel: cancel, owner: owner,
		clients: make(map[*net.UnixConn]struct{})}
	server.work.Add(1)
	go server.serve()
	return server, nil
}

func (server *Server) serve() {
	defer server.work.Done()
	for {
		connection, err := server.listener.AcceptUnix()
		if err != nil {
			return
		}
		server.mu.Lock()
		server.clients[connection] = struct{}{}
		server.mu.Unlock()
		server.work.Add(1)
		go func() {
			defer server.work.Done()
			defer func() {
				server.mu.Lock()
				delete(server.clients, connection)
				server.mu.Unlock()
			}()
			server.handle(connection)
		}()
	}
}

func (server *Server) handle(connection *net.UnixConn) {
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(15 * time.Second))
	raw, err := io.ReadAll(io.LimitReader(connection, int64(maximumRequest+1)))
	if err != nil || len(raw) > maximumRequest {
		writeResponse(connection, "unavailable\n")
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

func writeResponse(connection *net.UnixConn, response string) {
	_, _ = io.WriteString(connection, response)
	_ = connection.CloseWrite()
}

// Close refuses new callers, cancels operations, and removes only this exact
// socket path.
func (server *Server) Close() error {
	if server == nil {
		return nil
	}
	server.once.Do(func() {
		server.cancel()
		server.err = server.listener.Close()
		server.mu.Lock()
		for client := range server.clients {
			server.err = errors.Join(server.err, client.Close())
		}
		server.mu.Unlock()
		server.work.Wait()
		server.err = errors.Join(server.err, os.Remove(server.path))
	})
	return server.err
}
