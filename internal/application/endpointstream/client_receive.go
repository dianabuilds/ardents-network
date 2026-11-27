package endpointstream

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
)

func (connection *client) receive() {
	defer close(connection.receiveDone)
	reader := bufio.NewReader(connection.connection)
	var header [4]byte
	outputEnded := false
	for {
		if _, err := io.ReadFull(reader, header[:]); err != nil {
			connection.finishReceive(Outcome{Class: LocalFailure, Reason: "Endpoint Application attachment ended without a terminal outcome"}, err)
			return
		}
		length := binary.BigEndian.Uint32(header[:])
		if length == terminalMarker {
			outcome, err := readTerminal(reader)
			if err == nil && connection.directionalEOF && outcome.Class == CleanClose && !outputEnded {
				err = errors.New("clean byte stream omitted directional EOF")
				outcome = Outcome{}
			}
			connection.finishReceive(outcome, err)
			return
		}
		if length == 0 && connection.directionalEOF && !outputEnded {
			outputEnded = true
			_ = connection.sink.Close()
			continue
		}
		if outputEnded || length == 0 || length > maximumFrame {
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
