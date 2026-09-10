//go:build linux

package main

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"sync/atomic"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
)

// This is only a local AAI3 peer. It supplies no State, Route, worker launch or
// qualified Endpoint. The tests assert the trusted client's wire and UI seam.
type textReadPeer struct {
	response     []byte
	class        connection.OutcomeClass
	terminal     <-chan struct{}
	responseSent chan struct{}
	opens        atomic.Int32
	requestBytes atomic.Int32
}

func textResponse(body []byte) []byte {
	response := make([]byte, 13, 13+len(body))
	copy(response, "ARDTXT01")
	binary.BigEndian.PutUint32(response[9:], uint32(len(body)))
	return append(response, body...)
}

func (peer *textReadPeer) Open(ctx context.Context, request connection.Request) (connection.Stream, error) {
	if request.Destination != connection.TargetLink || request.Value != "fixture-link" {
		return nil, errors.New("fixture refused destination")
	}
	peer.opens.Add(1)
	input, requestSink := io.Pipe()
	response, output := io.Pipe()
	bounded, cancel := context.WithCancel(ctx)
	stream := &textReadStream{request: requestSink, response: response, input: input, output: output,
		cancel: cancel, finished: make(chan struct{}), done: make(chan connection.Outcome, 1)}
	go func() {
		defer close(stream.finished)
		request, err := io.ReadAll(io.LimitReader(input, 513))
		peer.requestBytes.Add(int32(len(request)))
		valid := err == nil && len(request) == 512 && string(request[:9]) == "ARDTXT01\x01"
		if valid {
			for _, value := range request[9:] {
				valid = valid && value == 0
			}
		}
		if !valid {
			err = errors.New("fixture got invalid or repeated text request")
		} else {
			_, err = output.Write(peer.response)
		}
		_ = output.CloseWithError(err)
		if peer.responseSent != nil {
			close(peer.responseSent)
		}
		if peer.terminal != nil {
			select {
			case <-peer.terminal:
			case <-bounded.Done():
			}
		}
		class := peer.class
		if err != nil || bounded.Err() != nil {
			class = connection.IndeterminateFailure
		}
		stream.done <- connection.Outcome{Class: class}
		close(stream.done)
	}()
	return stream, nil
}

type textReadStream struct {
	request  *io.PipeWriter
	response *io.PipeReader
	input    *io.PipeReader
	output   *io.PipeWriter
	cancel   context.CancelFunc
	finished chan struct{}
	done     chan connection.Outcome
	once     sync.Once
}

func (stream *textReadStream) Read(body []byte) (int, error)   { return stream.response.Read(body) }
func (stream *textReadStream) Write(body []byte) (int, error)  { return stream.request.Write(body) }
func (stream *textReadStream) CloseInput() error               { return stream.request.Close() }
func (stream *textReadStream) Done() <-chan connection.Outcome { return stream.done }
func (stream *textReadStream) Close() error {
	stream.once.Do(func() {
		stream.cancel()
		_ = stream.request.Close()
		_ = stream.input.Close()
		_ = stream.response.Close()
		_ = stream.output.Close()
		<-stream.finished
	})
	return nil
}
