package systemd

import (
	"bytes"
	"encoding/binary"
)

func referenceScalar(frame []byte, pos, end int, kind byte, order binary.ByteOrder) (string, int, error) {
	var size uint64
	if kind == 'g' {
		if pos >= end {
			return "", pos, ErrObservation
		}
		size = uint64(frame[pos])
		pos++
	} else {
		pos = (pos + 3) &^ 3
		if pos+4 > end {
			return "", pos, ErrObservation
		}
		size = uint64(order.Uint32(frame[pos : pos+4]))
		pos += 4
	}
	if size > uint64(end-pos) || size+1 > uint64(end-pos) {
		return "", pos, ErrObservation
	}
	finish := pos + int(size)
	if frame[finish] != 0 || bytes.IndexByte(frame[pos:finish], 0) >= 0 {
		return "", pos, ErrObservation
	}
	return string(frame[pos:finish]), finish + 1, nil
}
