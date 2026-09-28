package connection

import (
	"encoding/binary"
	"errors"
	"io"
)

func writeData(writer io.Writer, data []byte) error {
	if len(data) == 0 || len(data) > maximumFrame {
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

const (
	localMagic     = "AAI3"
	maximumFrame   = 16 << 10
	terminalMarker = ^uint32(0)
)
