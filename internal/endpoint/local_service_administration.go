package endpoint

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

const localServiceAdministrationMaximumRequest = len("withdraw\n")

type localServiceAdministrationConfig struct {
	Path     string
	Publish  func(context.Context) error
	Withdraw func(context.Context) error
}

// LocalServiceAdministration owns one private local attachment for the
// Publisher start/withdraw surface. It accepts no Credential, key, Target,
// State, Route, peer, or transport input.
type LocalServiceAdministration struct {
	path     string
	listener *net.UnixListener
	ctx      context.Context
	cancel   context.CancelFunc
	work     sync.WaitGroup
	mu       sync.Mutex
	clients  map[*net.UnixConn]struct{}
	once     sync.Once
	err      error
	publish  func(context.Context) error
	withdraw func(context.Context) error
}

func openLocalServiceAdministration(config localServiceAdministrationConfig) (*LocalServiceAdministration, error) {
	if config.Path == "" || !filepath.IsAbs(config.Path) || config.Publish == nil || config.Withdraw == nil {
		return nil, errors.New("local Service Administration configuration is invalid")
	}
	if _, err := os.Lstat(config.Path); err == nil || !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("local Service Administration attachment already exists")
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: config.Path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	listener.SetUnlinkOnClose(false)
	if err := os.Chmod(config.Path, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(config.Path)
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := &LocalServiceAdministration{path: config.Path, listener: listener, ctx: ctx, cancel: cancel,
		clients: make(map[*net.UnixConn]struct{}), publish: config.Publish, withdraw: config.Withdraw}
	server.work.Add(1)
	go server.serve()
	return server, nil
}

func (server *LocalServiceAdministration) serve() {
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

func (server *LocalServiceAdministration) handle(connection *net.UnixConn) {
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(15 * time.Second))
	raw, err := io.ReadAll(io.LimitReader(connection, int64(localServiceAdministrationMaximumRequest+1)))
	if err != nil || len(raw) > localServiceAdministrationMaximumRequest {
		writeLocalServiceAdministrationResponse(connection, "unavailable\n")
		return
	}
	var operation func(context.Context) error
	var response string
	switch string(raw) {
	case "publish\n":
		operation, response = server.publish, "published\n"
	case "withdraw\n":
		operation, response = server.withdraw, "withdrawn\n"
	default:
		writeLocalServiceAdministrationResponse(connection, "unavailable\n")
		return
	}
	if err := operation(server.ctx); err != nil {
		writeLocalServiceAdministrationResponse(connection, "unavailable\n")
		return
	}
	writeLocalServiceAdministrationResponse(connection, response)
}

func writeLocalServiceAdministrationResponse(connection *net.UnixConn, response string) {
	_, _ = io.WriteString(connection, response)
	_ = connection.CloseWrite()
}

// Close refuses new callers, cancels active operations, removes only its
// exact socket, and joins all workers.
func (server *LocalServiceAdministration) Close() error {
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
