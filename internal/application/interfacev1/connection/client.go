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

// Client is the local Adapter side of the Connection Interface. It exposes
// only the byte stream, its terminal outcome, and an orderly input half-close.
type Client interface {
	Stream
	CloseInput() error
}

type client struct {
	connection  *net.UnixConn
	stream      *io.PipeReader
	sink        *io.PipeWriter
	writeMu     sync.Mutex
	done        chan Outcome
	doneOnce    sync.Once
	closeOnce   sync.Once
	inputOnce   sync.Once
	inputErr    error
	stopContext func() bool
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
	stream, sink := io.Pipe()
	opened := &client{connection: connection, stream: stream, sink: sink,
		done: make(chan Outcome, 1)}
	opened.stopContext = context.AfterFunc(ctx, func() { _ = opened.Close() })
	go opened.receive()
	return opened, nil
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
	if connection.stopContext != nil {
		connection.stopContext()
	}
	if err != nil && outcome.Class == "" {
		outcome = Outcome{Class: LocalFailure, Reason: "Endpoint Application terminal outcome was invalid"}
	}
	connection.publishDone(outcome)
	if err != nil {
		_ = connection.sink.CloseWithError(err)
		return
	}
	_ = connection.sink.Close()
}

func (connection *client) Write(source []byte) (int, error) {
	connection.writeMu.Lock()
	defer connection.writeMu.Unlock()
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
		connection.writeMu.Lock()
		defer connection.writeMu.Unlock()
		var frame [4]byte
		if _, err := connection.connection.Write(frame[:]); err != nil {
			connection.inputErr = err
			return
		}
		connection.inputErr = connection.connection.CloseWrite()
	})
	return connection.inputErr
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
	var result error
	connection.closeOnce.Do(func() {
		if connection.stopContext != nil {
			connection.stopContext()
		}
		connection.writeMu.Lock()
		result = connection.connection.Close()
		connection.writeMu.Unlock()
		connection.publishDone(Outcome{Class: LocalCancellation, Reason: "Application Adapter closed the local connection"})
		_ = connection.stream.Close()
	})
	return result
}
