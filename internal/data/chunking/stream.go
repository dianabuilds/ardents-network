package chunking

import (
	"fmt"
	"io"
)

const PlaintextChunkSize = 64 * 1024

func Stream(reader io.Reader, emit func(int, []byte) error) (int, int64, error) {
	if reader == nil || emit == nil {
		return 0, 0, fmt.Errorf("chunk stream dependencies are unavailable")
	}
	count := 0
	var total int64
	for {
		chunk := make([]byte, PlaintextChunkSize)
		read, err := io.ReadFull(reader, chunk)
		if err == io.EOF && read == 0 {
			if count == 0 {
				return 0, 0, fmt.Errorf("chunk stream payload is empty")
			}
			return count, total, nil
		}
		if err != nil && err != io.ErrUnexpectedEOF {
			return count, total, err
		}
		chunk = chunk[:read]
		if err := emit(count, chunk); err != nil {
			return count, total, err
		}
		count++
		total += int64(read)
		if err == io.ErrUnexpectedEOF {
			return count, total, nil
		}
	}
}
