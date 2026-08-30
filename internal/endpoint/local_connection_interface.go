package endpoint

import (
	"bufio"
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

const (
	localApplicationMagic       = "AAI2"
	localApplicationMaximumLink = 512
	localApplicationFrame       = 16 << 10
	localApplicationTerminal    = ^uint32(0)
)

type localConnectionInterfaceConfig struct {
	Path string
	Open func(context.Context, string) (*ApplicationConnection, error)
}

// LocalConnectionInterface owns one private local attachment for the
// name-to-stream Connection surface. It never accepts Target, State, Entry,
// Route, issuer, or Service Administration input.
type LocalConnectionInterface struct {
	path     string
	listener *net.UnixListener
	ctx      context.Context
	cancel   context.CancelFunc
	work     sync.WaitGroup
	mu       sync.Mutex
	clients  map[*net.UnixConn]struct{}
	once     sync.Once
	err      error
	open     func(context.Context, string) (*ApplicationConnection, error)
}

// OpenLocalConnectionInterface exposes an Endpoint Connection Interface on one
// explicit local Unix socket. The caller retains the Interface owner and must
// close both as part of Endpoint shutdown.
func OpenLocalConnectionInterface(path string, owner *ConnectionInterface) (*LocalConnectionInterface, error) {
	if owner == nil {
		return nil, errors.New("local Application Connection owner is unavailable")
	}
	return openLocalConnectionInterface(localConnectionInterfaceConfig{Path: path, Open: owner.Open})
}

func openLocalConnectionInterface(config localConnectionInterfaceConfig) (*LocalConnectionInterface, error) {
	if config.Path == "" || !filepath.IsAbs(config.Path) || config.Open == nil {
		return nil, errors.New("local Application Connection configuration is invalid")
	}
	if _, err := os.Lstat(config.Path); err == nil || !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("local Application Connection attachment already exists")
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
	server := &LocalConnectionInterface{path: config.Path, listener: listener, ctx: ctx, cancel: cancel, open: config.Open,
		clients: make(map[*net.UnixConn]struct{})}
	server.work.Add(1)
	go server.serve()
	return server, nil
}

func (server *LocalConnectionInterface) serve() {
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

func (server *LocalConnectionInterface) handle(local *net.UnixConn) {
	defer local.Close()
	_ = local.SetDeadline(time.Now().Add(15 * time.Second))
	serviceLink, err := readLocalApplicationRequest(local)
	if err != nil {
		_ = writeLocalApplicationRefusal(local)
		return
	}
	application, err := server.open(server.ctx, serviceLink)
	if err != nil || application == nil {
		_ = writeLocalApplicationRefusal(local)
		return
	}
	defer application.Close()
	if _, err := local.Write([]byte{1}); err != nil {
		return
	}
	_ = local.SetDeadline(time.Time{})
	inputDone := make(chan struct{})
	go func() {
		defer close(inputDone)
		if readLocalApplicationFrames(local, application) != nil {
			_ = application.Close()
		}
	}()
	buffer := make([]byte, localApplicationFrame)
	for {
		read, readErr := application.Read(buffer)
		if read > 0 {
			if err := writeLocalApplicationData(local, buffer[:read]); err != nil {
				return
			}
		}
		if readErr != nil {
			break
		}
	}
	var outcome ApplicationOutcome
	select {
	case value, open := <-application.Done():
		if open {
			outcome = value
		}
	case <-server.ctx.Done():
		outcome = ApplicationOutcome{Class: "local timeout or cancellation", Reason: "Endpoint stopped"}
	}
	if outcome.Class == "" {
		outcome = ApplicationOutcome{Class: "indeterminate failure", Reason: "Application Connection ended without an outcome"}
	}
	_ = writeLocalApplicationTerminal(local, outcome)
	_ = application.Close()
	<-inputDone
}

// Close refuses new Adapter connections, stops active local streams, removes
// only its exact socket, and joins its workers.
func (server *LocalConnectionInterface) Close() error {
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

func readLocalApplicationRequest(reader io.Reader) (string, error) {
	header := make([]byte, len(localApplicationMagic)+2)
	if _, err := io.ReadFull(reader, header); err != nil || string(header[:len(localApplicationMagic)]) != localApplicationMagic {
		return "", errors.New("local Application request is invalid")
	}
	length := int(binary.BigEndian.Uint16(header[len(localApplicationMagic):]))
	if length == 0 || length > localApplicationMaximumLink {
		return "", errors.New("local Application Service Link is invalid")
	}
	raw := make([]byte, length)
	if _, err := io.ReadFull(reader, raw); err != nil {
		return "", err
	}
	return string(raw), nil
}

func writeLocalApplicationRefusal(writer io.Writer) error {
	_, err := writer.Write([]byte{0})
	return err
}

func readLocalApplicationFrames(reader io.Reader, application io.Writer) error {
	var header [4]byte
	for {
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			return err
		}
		length := binary.BigEndian.Uint32(header[:])
		if length == 0 {
			return nil
		}
		if length > localApplicationFrame {
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

func writeLocalApplicationData(writer io.Writer, data []byte) error {
	if len(data) == 0 || len(data) > localApplicationFrame {
		return errors.New("local Application data frame is invalid")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(data)))
	if _, err := writer.Write(header[:]); err != nil {
		return err
	}
	_, err := writer.Write(data)
	return err
}

func writeLocalApplicationTerminal(writer io.Writer, outcome ApplicationOutcome) error {
	if len(outcome.Class) == 0 || len(outcome.Class) > 128 || len(outcome.Reason) > 512 {
		return errors.New("local Application terminal outcome is invalid")
	}
	var header [8]byte
	binary.BigEndian.PutUint32(header[:4], localApplicationTerminal)
	binary.BigEndian.PutUint16(header[4:6], uint16(len(outcome.Class)))
	binary.BigEndian.PutUint16(header[6:8], uint16(len(outcome.Reason)))
	if _, err := writer.Write(header[:]); err != nil {
		return err
	}
	if _, err := io.WriteString(writer, outcome.Class); err != nil {
		return err
	}
	_, err := io.WriteString(writer, outcome.Reason)
	return err
}

// LocalApplicationConnection is the Adapter side of the local Connection
// Interface. Its stream framing is private to this local seam.
type LocalApplicationConnection struct {
	connection *net.UnixConn
	stream     *io.PipeReader
	sink       *io.PipeWriter
	writeMu    sync.Mutex
	done       chan ApplicationOutcome
	doneOnce   sync.Once
	closeOnce  sync.Once
	closed     chan struct{}
}

// DialLocalApplication requests one Service Link from a running Endpoint and
// returns no Target, State, Entry, Route, or administration handle.
func DialLocalApplication(ctx context.Context, path, serviceLink string) (*LocalApplicationConnection, error) {
	if ctx == nil || path == "" || serviceLink == "" || len(serviceLink) > localApplicationMaximumLink {
		return nil, errors.New("local Application dial input is invalid")
	}
	raw, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	connection, ok := raw.(*net.UnixConn)
	if !ok {
		_ = raw.Close()
		return nil, errors.New("local Application attachment is not a Unix connection")
	}
	if deadline, available := ctx.Deadline(); available {
		_ = connection.SetDeadline(deadline)
	}
	request := make([]byte, len(localApplicationMagic)+2+len(serviceLink))
	copy(request, localApplicationMagic)
	binary.BigEndian.PutUint16(request[len(localApplicationMagic):], uint16(len(serviceLink)))
	copy(request[len(localApplicationMagic)+2:], serviceLink)
	if _, err := connection.Write(request); err != nil {
		_ = connection.Close()
		return nil, err
	}
	var status [1]byte
	if _, err := io.ReadFull(connection, status[:]); err != nil || status[0] != 1 {
		_ = connection.Close()
		return nil, errors.New("local Application Connection is unavailable")
	}
	_ = connection.SetDeadline(time.Time{})
	stream, sink := io.Pipe()
	opened := &LocalApplicationConnection{connection: connection, stream: stream, sink: sink,
		done: make(chan ApplicationOutcome, 1), closed: make(chan struct{})}
	go opened.receive()
	return opened, nil
}

func (connection *LocalApplicationConnection) Read(destination []byte) (int, error) {
	if connection == nil || connection.stream == nil {
		return 0, net.ErrClosed
	}
	return connection.stream.Read(destination)
}

func (connection *LocalApplicationConnection) receive() {
	reader := bufio.NewReader(connection.connection)
	defer close(connection.closed)
	var header [4]byte
	for {
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			connection.finishReceive(ApplicationOutcome{Class: "local attachment failure", Reason: "Endpoint Application attachment ended without a terminal outcome"}, err)
			return
		}
		length := binary.BigEndian.Uint32(header[:])
		if length == localApplicationTerminal {
			outcome, err := readLocalApplicationTerminal(reader)
			connection.finishReceive(outcome, err)
			return
		}
		if length == 0 || length > localApplicationFrame {
			connection.finishReceive(ApplicationOutcome{Class: "local attachment failure", Reason: "Endpoint Application response frame is invalid"}, errors.New("local Application response frame is invalid"))
			return
		}
		data := make([]byte, int(length))
		if _, err := io.ReadFull(reader, data); err != nil {
			connection.finishReceive(ApplicationOutcome{Class: "local attachment failure", Reason: "Endpoint Application response frame was incomplete"}, err)
			return
		}
		if _, err := connection.sink.Write(data); err != nil {
			return
		}
	}
}

func readLocalApplicationTerminal(reader io.Reader) (ApplicationOutcome, error) {
	var lengths [4]byte
	if _, err := io.ReadFull(reader, lengths[:]); err != nil {
		return ApplicationOutcome{}, err
	}
	classLength, reasonLength := int(binary.BigEndian.Uint16(lengths[:2])), int(binary.BigEndian.Uint16(lengths[2:]))
	if classLength == 0 || classLength > 128 || reasonLength > 512 {
		return ApplicationOutcome{}, errors.New("local Application terminal frame is invalid")
	}
	raw := make([]byte, classLength+reasonLength)
	if _, err := io.ReadFull(reader, raw); err != nil {
		return ApplicationOutcome{}, err
	}
	return ApplicationOutcome{Class: string(raw[:classLength]), Reason: string(raw[classLength:])}, nil

}

func (connection *LocalApplicationConnection) finishReceive(outcome ApplicationOutcome, err error) {
	if err != nil && outcome.Class == "" {
		outcome = ApplicationOutcome{Class: "local attachment failure", Reason: "Endpoint Application terminal outcome was invalid"}
	}
	connection.publishDone(outcome)
	if err != nil {
		_ = connection.sink.CloseWithError(err)
		return
	}
	_ = connection.sink.Close()
}

func (connection *LocalApplicationConnection) Write(source []byte) (int, error) {
	connection.writeMu.Lock()
	defer connection.writeMu.Unlock()
	written := 0
	for len(source) > 0 {
		length := len(source)
		if length > localApplicationFrame {
			length = localApplicationFrame
		}
		if err := writeLocalApplicationData(connection.connection, source[:length]); err != nil {
			return written, err
		}
		written += length
		source = source[length:]
	}
	return written, nil
}

func (connection *LocalApplicationConnection) Done() <-chan ApplicationOutcome {
	if connection == nil {
		return nil
	}
	return connection.done
}

func (connection *LocalApplicationConnection) publishDone(outcome ApplicationOutcome) {
	connection.doneOnce.Do(func() {
		connection.done <- outcome
		close(connection.done)
	})
}

func (connection *LocalApplicationConnection) Close() error {
	if connection == nil {
		return nil
	}
	var result error
	connection.closeOnce.Do(func() {
		connection.writeMu.Lock()
		var frame [4]byte
		_, writeErr := connection.connection.Write(frame[:])
		result = errors.Join(writeErr, connection.connection.Close())
		connection.writeMu.Unlock()
		connection.publishDone(ApplicationOutcome{Class: "local cancellation", Reason: "Application Adapter closed the local connection"})
		_ = connection.stream.Close()
	})
	return result
}
