//go:build linux

package connection

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Server owns the lifecycle of one private local Connection transport.
type Server interface {
	Close() error
}

type server struct {
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
func Listen(path string, owner Interface) (Server, error) {
	if path == "" || !filepath.IsAbs(path) || owner == nil {
		return nil, errors.New("local Application Connection configuration is invalid")
	}
	if _, err := os.Lstat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("local Application Connection attachment already exists")
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
		server.mu.Lock()
		if server.ctx.Err() != nil || len(server.clients) >= 64 {
			server.mu.Unlock()
			_ = connection.SetDeadline(time.Now().Add(time.Second))
			_ = writeRefusal(connection, Refuse(Outcome{Class: "insufficient local or network capacity", Reason: "local Application capacity exhausted"}))
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
			server.handle(connection)
		}()
	}
}

func (server *server) handle(local *net.UnixConn) {
	defer local.Close()
	_ = local.SetDeadline(time.Now().Add(15 * time.Second))
	targetLink, err := readRequest(local)
	if err != nil {
		_ = writeRefusal(local, err)
		return
	}
	application, cancel, err := server.openAuthorizedAttachment(local, targetLink)
	if err != nil || application == nil {
		_ = writeRefusal(local, err)
		return
	}
	defer cancel()
	stop := context.AfterFunc(server.ctx, func() { _ = application.Close() })
	defer stop()
	defer func() {
		err := application.Close()
		server.mu.Lock()
		if server.err == nil && err != nil {
			server.err = err
		}
		server.mu.Unlock()
	}()
	if _, err := local.Write([]byte{1}); err != nil {
		return
	}
	_ = local.SetDeadline(time.Time{})
	inputDone := make(chan struct{})
	go func() {
		defer close(inputDone)
		if readFrames(local, application) != nil {
			_ = application.Close()
		}
	}()
	defer func() { _ = application.Close(); _ = local.Close(); <-inputDone }()
	buffer := make([]byte, maximumFrame)
	for {
		read, readErr := application.Read(buffer)
		if read > 0 && writeData(local, buffer[:read]) != nil {
			return
		}
		if readErr != nil {
			break
		}
	}
	var outcome Outcome
	select {
	case value, open := <-application.Done():
		if open {
			outcome = value
		}
	case <-server.ctx.Done():
		outcome = Outcome{Class: LocalTimeout, Reason: "Endpoint stopped"}
	}
	if outcome.Class == "" {
		outcome = Outcome{Class: IndeterminateFailure, Reason: "Application Connection ended without an outcome"}
	}
	if err := application.Close(); err != nil {
		outcome = Outcome{Class: IndeterminateFailure, Reason: "Application cleanup did not complete"}
	}
	_ = writeTerminal(local, outcome)
}

// Close refuses new clients, cancels active streams, and removes only this
// server's exact socket path.
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
		server.err = errors.Join(server.err, os.Remove(server.path))
	})
	return server.err
}

func readRequest(reader io.Reader) (Request, error) {
	var header [7]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil || string(header[:4]) != localMagic {
		return Request{}, errors.New("local Application request is invalid")
	}
	length := int(binary.BigEndian.Uint16(header[5:]))
	if length == 0 || length > maximumTarget {
		return Request{}, errors.New("local Application destination is invalid")
	}
	raw := make([]byte, len(header)+length)
	copy(raw, header[:])
	if _, err := io.ReadFull(reader, raw[len(header):]); err != nil {
		return Request{}, err
	}
	request, err := DecodeRequest(raw)
	if err != nil {
		return Request{}, err
	}
	if request.Destination == Name {
		return Request{}, Refuse(Outcome{Class: "not-selected", Reason: "canonical Names are not selected"})
	}
	return request, nil
}
func writeRefusal(writer io.Writer, cause error) error {
	if _, err := writer.Write([]byte{0}); err != nil {
		return err
	}
	return writeTerminal(writer, refusal(cause))
}

func readFrames(reader io.Reader, application interface {
	io.Writer
	CloseInput() error
}) error {
	var header [4]byte
	for {
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			return err
		}
		length := binary.BigEndian.Uint32(header[:])
		if length == 0 {
			return application.CloseInput()
		}
		if length > maximumFrame {
			return errors.New("local Application frame exceeds its bound")
		}
		data := make([]byte, int(length))
		if _, err := io.ReadFull(reader, data); err != nil {
			return err
		}
		if _, err := application.Write(data); err != nil {
			return err
		}
	}
}

func writeTerminal(writer io.Writer, outcome Outcome) error {
	if err := validOutcome(outcome); err != nil {
		return err
	}
	var header [8]byte
	binary.BigEndian.PutUint32(header[:4], terminalMarker)
	binary.BigEndian.PutUint16(header[4:6], uint16(len(outcome.Class)))
	binary.BigEndian.PutUint16(header[6:8], uint16(len(outcome.Reason)))
	if _, err := writer.Write(header[:]); err != nil {
		return err
	}
	if _, err := io.WriteString(writer, string(outcome.Class)); err != nil {
		return err
	}
	_, err := io.WriteString(writer, outcome.Reason)
	return err
}
