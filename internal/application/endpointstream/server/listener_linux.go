package server

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	"github.com/dianabuilds/ardents-network/internal/application/internal/socketfile"
)

// Server owns one private local Connection transport. Copies of this handle
// retain the same original listener and joined Close operation.
type Server interface {
	Close() error
}

type server struct {
	entry    *socketfile.Entry
	listener *net.UnixListener
	ctx      context.Context
	cancel   context.CancelFunc
	work     sync.WaitGroup
	mu       sync.Mutex
	clients  map[*net.UnixConn]struct{}
	once     sync.Once
	err      error
	owner    endpointstream.Interface
}

// Listen exposes owner on one explicit absolute Unix-socket path.
func Listen(path string, owner endpointstream.Interface) (Server, error) {
	if path == "" || !filepath.IsAbs(path) || owner == nil {
		return nil, errors.New("local Application Connection configuration is invalid")
	}
	listener, entry, err := socketfile.Listen(path)
	if errors.Is(err, socketfile.ErrOccupied) {
		return nil, errors.New("local Application Connection attachment already exists")
	}
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := &server{entry: entry, listener: listener, ctx: ctx, cancel: cancel, owner: owner,
		clients: make(map[*net.UnixConn]struct{})}
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
		acceptedAt := time.Now()
		server.mu.Lock()
		if server.ctx.Err() != nil || len(server.clients) >= 64 {
			server.mu.Unlock()
			_ = connection.SetDeadline(time.Now().Add(time.Second))
			_ = WriteSetupRefusal(connection, endpointstream.Refuse(endpointstream.Outcome{Class: endpointstream.CapacityUnavailable, Reason: "local Application capacity exhausted"}))
			_ = connection.Close()
			continue
		}
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
			server.handle(connection, acceptedAt)
		}()
	}
}

func (server *server) handle(local *net.UnixConn, acceptedAt time.Time) {
	defer local.Close()
	deadline := acceptedAt.Add(endpointstream.ReaderTimeout)
	if err := local.SetDeadline(acceptedAt.Add(15 * time.Second)); err != nil {
		return
	}
	targetLink, err := endpointstream.ReadRequest(local)
	if err != nil {
		_ = WriteSetupRefusal(local, err)
		return
	}
	if err := local.SetDeadline(deadline); err != nil {
		return
	}
	application, cancel, err := server.openAuthorizedAttachment(local, targetLink, deadline)
	if err != nil || application == nil {
		_ = WriteSetupRefusal(local, err)
		return
	}
	defer cancel()
	if err := local.SetWriteDeadline(time.Time{}); err != nil {
		server.retainCleanupFailure(application.Close())
		return
	}
	server.retainCleanupFailure(ServeStream(server.ctx, local, application, true))
}

// retainCleanupFailure keeps one local failure across attachment turnover.
func (server *server) retainCleanupFailure(err error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.err == nil && err != nil {
		server.err = err
	}
}

// Close refuses new clients, cancels active streams, and removes only this
// server's original socket entry.
func (server *server) Close() error {
	if server == nil {
		return nil
	}
	server.once.Do(func() {
		server.cancel()
		listenerErr := server.listener.Close()
		server.mu.Lock()
		if !errors.Is(listenerErr, net.ErrClosed) {
			server.err = errors.Join(server.err, listenerErr)
		}
		for client := range server.clients {
			if err := client.Close(); !errors.Is(err, net.ErrClosed) {
				if server.err == nil && err != nil {
					server.err = err
				}
			}
		}
		server.mu.Unlock()
		server.work.Wait()
		server.err = errors.Join(server.err, server.entry.Retire())
	})
	return server.err
}
