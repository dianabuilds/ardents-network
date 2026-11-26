package connection

import (
	"io"
	"net"
	"sync"
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
	directionalEOF   bool
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

func newClientWithStop(transport clientTransport, stopContext func() bool) *client {
	stream, sink := io.Pipe()
	opened := &client{connection: transport, stream: stream, sink: sink,
		done: make(chan Outcome, 1), receiveDone: make(chan struct{}), stopContext: stopContext}
	return opened
}

func (connection *client) Read(destination []byte) (int, error) {
	if connection == nil || connection.stream == nil {
		return 0, net.ErrClosed
	}
	return connection.stream.Read(destination)
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
		length := min(len(source), maximumFrame)
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
