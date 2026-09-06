package connection

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// Client is the local Adapter side of the Connection Interface. Write and
// CloseInput are serialized so the zero-length input-close frame follows every
// accepted data frame. Close and context cancellation instead abort the owned
// transport independently, interrupt in-flight operations, join Client-owned
// work, and publish LocalCancellation only after the receiver had its chance to
// complete a verified remote outcome. A failed Write may report only its
// completed payload prefix and is never a clean terminal result.
type Client interface {
	Stream
	CloseInput() error
}

type clientTransport interface {
	io.ReadWriteCloser
	CloseWrite() error
}

type client struct {
	connection       clientTransport
	stream           *io.PipeReader
	sink             *io.PipeWriter
	writeMu          sync.Mutex
	stateMu          sync.Mutex
	outputOperations sync.WaitGroup
	closing          bool
	done             chan Outcome
	receiveDone      chan struct{}
	doneOnce         sync.Once
	closeOnce        sync.Once
	inputOnce        sync.Once
	inputClosed      bool
	inputErr         error
	closeErr         error
	stopContext      func() bool
}

// Dial requests one Target Link and returns no Target, State, Entry, Route,
// credential, or administration handle.
func Dial(ctx context.Context, path, targetLink string) (Client, error) {
	if ctx == nil || path == "" || targetLink == "" || len(targetLink) > maximumLink {
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
	request := make([]byte, len(localMagic)+2+len(targetLink))
	copy(request, localMagic)
	binary.BigEndian.PutUint16(request[len(localMagic):], uint16(len(targetLink)))
	copy(request[len(localMagic)+2:], targetLink)
	if _, err := connection.Write(request); err != nil {
		_ = connection.Close()
		return nil, err
	}
	var status [1]byte
	if _, err := io.ReadFull(connection, status[:]); err != nil {
		_ = connection.Close()
		return nil, errors.New("local Application Connection is unavailable")
	}
	if status[0] != 1 {
		outcome, refusalErr := readRefusal(connection)
		_ = connection.Close()
		if refusalErr != nil {
			return nil, errors.New("local Application Connection is unavailable")
		}
		return nil, errors.New(string(outcome.Class) + ": " + outcome.Reason)
	}
	_ = connection.SetDeadline(time.Time{})
	return newClient(ctx, connection), nil
}

func newClient(ctx context.Context, transport clientTransport) Client {
	stream, sink := io.Pipe()
	opened := &client{connection: transport, stream: stream, sink: sink,
		done: make(chan Outcome, 1), receiveDone: make(chan struct{})}
	opened.stateMu.Lock()
	opened.stopContext = context.AfterFunc(ctx, func() { _ = opened.Close() })
	opened.stateMu.Unlock()
	go opened.receive()
	return opened
}

func readRefusal(reader io.Reader) (Outcome, error) {
	var marker [4]byte
	if _, err := io.ReadFull(reader, marker[:]); err != nil || binary.BigEndian.Uint32(marker[:]) != terminalMarker {
		return Outcome{}, errors.New("local Application refusal is invalid")
	}
	return readTerminal(reader)
}

func (connection *client) Read(destination []byte) (int, error) {
	if connection == nil || connection.stream == nil {
		return 0, net.ErrClosed
	}
	return connection.stream.Read(destination)
}

func (connection *client) receive() {
	defer close(connection.receiveDone)
	reader := bufio.NewReader(connection.connection)
	var header [4]byte
	for {
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			connection.finishReceive(Outcome{Class: LocalFailure, Reason: "Endpoint Application attachment ended without a terminal outcome"}, err)
			return
		}
		length := binary.BigEndian.Uint32(header[:])
		if length == terminalMarker {
			outcome, err := readTerminal(reader)
			connection.finishReceive(outcome, err)
			return
		}
		if length == 0 || length > maximumFrame {
			connection.finishReceive(Outcome{Class: LocalFailure, Reason: "Endpoint Application response frame is invalid"}, errors.New("local Application response frame is invalid"))
			return
		}
		data := make([]byte, int(length))
		if _, err := io.ReadFull(reader, data); err != nil {
			connection.finishReceive(Outcome{Class: LocalFailure, Reason: "Endpoint Application response frame was incomplete"}, err)
			return
		}
		if _, err := connection.sink.Write(data); err != nil {
			return
		}
	}
}

func readTerminal(reader io.Reader) (Outcome, error) {
	var lengths [4]byte
	if _, err := io.ReadFull(reader, lengths[:]); err != nil {
		return Outcome{}, err
	}
	classLength, reasonLength := int(binary.BigEndian.Uint16(lengths[:2])), int(binary.BigEndian.Uint16(lengths[2:]))
	if classLength == 0 || classLength > maximumOutcomeClassBytes || reasonLength > maximumOutcomeReasonBytes {
		return Outcome{}, errors.New("local Application terminal frame is invalid")
	}
	raw := make([]byte, classLength+reasonLength)
	if _, err := io.ReadFull(reader, raw); err != nil {
		return Outcome{}, err
	}
	outcome := Outcome{Class: OutcomeClass(raw[:classLength]), Reason: string(raw[classLength:])}
	if err := validOutcome(outcome); err != nil {
		return Outcome{}, err
	}
	return outcome, nil
}

func (connection *client) finishReceive(outcome Outcome, err error) {
	connection.stopContextWatch()
	if err != nil {
		if !connection.isClosing() {
			if outcome.Class == "" {
				outcome = Outcome{Class: LocalFailure, Reason: "Endpoint Application terminal outcome was invalid"}
			}
			connection.publishDone(outcome)
		}
		_ = connection.sink.CloseWithError(err)
		return
	}
	connection.publishDone(outcome)
	_ = connection.sink.Close()
}

func (connection *client) Write(source []byte) (int, error) {
	if connection == nil || connection.connection == nil {
		return 0, net.ErrClosed
	}
	if !connection.beginOperation() {
		return 0, net.ErrClosed
	}
	defer connection.outputOperations.Done()
	connection.writeMu.Lock()
	defer connection.writeMu.Unlock()
	if connection.inputClosed {
		return 0, net.ErrClosed
	}
	written := 0
	for len(source) > 0 {
		length := len(source)
		if length > maximumFrame {
			length = maximumFrame
		}
		if err := writeData(connection.connection, source[:length]); err != nil {
			return written, err
		}
		written += length
		source = source[length:]
	}
	return written, nil
}

// Done returns the one bounded terminal outcome.
func (connection *client) Done() <-chan Outcome {
	if connection == nil {
		return nil
	}
	return connection.done
}

// CloseInput completes only the Adapter-to-Service byte direction.
func (connection *client) CloseInput() error {
	if connection == nil {
		return nil
	}
	connection.inputOnce.Do(func() {
		if !connection.beginOperation() {
			connection.inputErr = net.ErrClosed
			return
		}
		defer connection.outputOperations.Done()
		connection.writeMu.Lock()
		defer connection.writeMu.Unlock()
		connection.inputClosed = true
		var frame [4]byte
		if _, err := connection.connection.Write(frame[:]); err != nil {
			connection.inputErr = err
			return
		}
		connection.inputErr = connection.connection.CloseWrite()
	})
	return connection.inputErr
}

func (connection *client) beginOperation() bool {
	connection.stateMu.Lock()
	defer connection.stateMu.Unlock()
	if connection.closing {
		return false
	}
	connection.outputOperations.Add(1)
	return true
}

func (connection *client) isClosing() bool {
	connection.stateMu.Lock()
	defer connection.stateMu.Unlock()
	return connection.closing
}

func (connection *client) stopContextWatch() {
	connection.stateMu.Lock()
	stopContext := connection.stopContext
	connection.stateMu.Unlock()
	if stopContext != nil {
		stopContext()
	}
}

func (connection *client) publishDone(outcome Outcome) {
	connection.doneOnce.Do(func() {
		connection.done <- outcome
		close(connection.done)
	})
}

// Close withdraws only this local Adapter attachment.
func (connection *client) Close() error {
	if connection == nil {
		return nil
	}
	connection.closeOnce.Do(func() {
		connection.stateMu.Lock()
		connection.closing = true
		stopContext := connection.stopContext
		connection.stateMu.Unlock()
		if stopContext != nil {
			stopContext()
		}
		connection.closeErr = connection.connection.Close()
		_ = connection.stream.Close()
		connection.outputOperations.Wait()
		<-connection.receiveDone
		connection.publishDone(Outcome{Class: LocalCancellation, Reason: "Application Adapter closed the local connection"})
	})
	return connection.closeErr
}
