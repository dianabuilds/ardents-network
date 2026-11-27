package client

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

func (connection *client) receive() {
	defer close(connection.receiveDone)
	reader := bufio.NewReader(connection.connection)
	var header [4]byte
	outputEnded := false
	for {
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			connection.finishReceive(endpointstream.Outcome{Class: endpointstream.LocalFailure, Reason: "Endpoint Application attachment ended without a terminal outcome"}, err)
			return
		}
		length := binary.BigEndian.Uint32(header[:])
		if length == endpointstream.TerminalMarker {
			outcome, err := endpointstream.ReadTerminal(reader)
			if err == nil && connection.directionalEOF && outcome.Class == endpointstream.CleanClose && !outputEnded {
				err = errors.New("clean byte stream omitted directional EOF")
				outcome = endpointstream.Outcome{}
			}
			connection.finishReceive(outcome, err)
			return
		}
		if length == 0 && connection.directionalEOF && !outputEnded {
			outputEnded = true
			_ = connection.sink.Close()
			continue
		}
		if outputEnded || length == 0 || length > endpointstream.MaximumFrame {
			connection.finishReceive(endpointstream.Outcome{Class: endpointstream.LocalFailure, Reason: "Endpoint Application response frame is invalid"}, errors.New("local Application response frame is invalid"))
			return
		}
		data := make([]byte, int(length))
		if _, err := io.ReadFull(reader, data); err != nil {
			connection.finishReceive(endpointstream.Outcome{Class: endpointstream.LocalFailure, Reason: "Endpoint Application response frame was incomplete"}, err)
			return
		}
		if _, err := connection.sink.Write(data); err != nil {
			return
		}
	}
}

func (connection *client) finishReceive(outcome endpointstream.Outcome, err error) {
	connection.stopContextWatch()
	if err != nil {
		if !connection.isClosing() {
			if outcome.Class == "" {
				outcome = endpointstream.Outcome{Class: endpointstream.LocalFailure, Reason: "Endpoint Application terminal outcome was invalid"}
			}
			connection.publishDone(outcome)
		}
		_ = connection.sink.CloseWithError(err)
		return
	}
	connection.publishDone(outcome)
	_ = connection.sink.Close()
}
