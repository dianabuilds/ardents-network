package administration

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/internal/socketfile"
)

// Server owns the lifecycle of one private local Administration transport.
type Server interface {
	Close() error
}

type server struct {
	entry     *socketfile.Entry
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
	listener, entry, err := socketfile.Listen(path)
	if errors.Is(err, socketfile.ErrOccupied) {
		return nil, errors.New("local Service Administration attachment already exists")
	}
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := &server{entry: entry, listener: listener, ctx: ctx, cancel: cancel, owner: owner,
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
		accepted := time.Now()
		server.mu.Lock()
		if server.ctx.Err() != nil {
			server.err = errors.Join(server.err, connection.Close())
			server.mu.Unlock()
			return
		}
		server.clients[connection] = struct{}{}
		server.mu.Unlock()
		server.work.Go(func() {
			defer func() {
				server.mu.Lock()
				if _, owned := server.clients[connection]; owned {
					if err := connection.Close(); !errors.Is(err, net.ErrClosed) {
						server.err = errors.Join(server.err, err)
					}
					delete(server.clients, connection)
				}
				server.mu.Unlock()
			}()
			server.handle(connection, accepted)
		})
	}
}

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
		server.err = errors.Join(server.err, server.entry.Retire())
	})
	return server.err
}
