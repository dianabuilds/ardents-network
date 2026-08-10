package tooling

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
)

func pcapWireBytes(path string) (uint64, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	header := make([]byte, 24)
	if _, err := io.ReadFull(file, header); err != nil {
		return 0, err
	}
	var order binary.ByteOrder
	switch string(header[:4]) {
	case "\xd4\xc3\xb2\xa1", "\x4d\x3c\xb2\xa1":
		order = binary.LittleEndian
	case "\xa1\xb2\xc3\xd4", "\xa1\xb2\x3c\x4d":
		order = binary.BigEndian
	default:
		return 0, errors.New("native capture has an unsupported pcap header")
	}
	var total uint64
	record := make([]byte, 16)
	for {
		_, err := io.ReadFull(file, record)
		if errors.Is(err, io.EOF) {
			return total, nil
		}
		if err != nil {
			return 0, errors.New("native capture has a truncated packet header")
		}
		captured := order.Uint32(record[8:12])
		original := order.Uint32(record[12:16])
		if captured > 1<<20 || original < captured {
			return 0, errors.New("native capture packet lengths are invalid")
		}
		total += uint64(original)
		if _, err := io.CopyN(io.Discard, file, int64(captured)); err != nil {
			return 0, errors.New("native capture has a truncated packet payload")
		}
	}
}
