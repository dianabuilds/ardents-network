package route

import (
	"bytes"
	"errors"
	"io"
)

func writeLegBinding(writer io.Writer, network, epoch, destination [32]byte) error {
	frame := make([]byte, 0, 101)
	frame = append(frame, "ARLG"...)
	frame = append(frame, 1)
	frame = append(frame, network[:]...)
	frame = append(frame, epoch[:]...)
	frame = append(frame, destination[:]...)
	return writeAll(writer, frame)
}

func acceptLegBinding(connection io.ReadWriter, network, epoch, destination [32]byte) error {
	frame := make([]byte, 101)
	if _, err := io.ReadFull(connection, frame); err != nil {
		return err
	}
	if string(frame[:4]) != "ARLG" || frame[4] != 1 || !bytes.Equal(frame[5:37], network[:]) ||
		!bytes.Equal(frame[37:69], epoch[:]) || !bytes.Equal(frame[69:101], destination[:]) {
		return errors.New("carrier leg does not match authenticated Network State duty")
	}
	return writeAll(connection, []byte("ARLA"))
}

func confirmLegBinding(connection io.ReadWriter, network, epoch, destination [32]byte) error {
	if err := writeLegBinding(connection, network, epoch, destination); err != nil {
		return err
	}
	acknowledgement := make([]byte, 4)
	if _, err := io.ReadFull(connection, acknowledgement); err != nil {
		return err
	}
	if string(acknowledgement) != "ARLA" {
		return errors.New("carrier leg binding was not accepted")
	}
	return nil
}
