package bridge

import "encoding/binary"

type binaryDecoder struct {
	raw    []byte
	offset int
	failed bool
}

func (reader *binaryDecoder) take(length int) []byte {
	if length < 0 || reader.offset > len(reader.raw)-length {
		reader.failed = true
		return nil
	}
	value := reader.raw[reader.offset : reader.offset+length]
	reader.offset += length
	return value
}

func (reader *binaryDecoder) byte() byte {
	value := reader.take(1)
	if value == nil {
		return 0
	}
	return value[0]
}

func (reader *binaryDecoder) uint16() uint16 {
	value := reader.take(2)
	if value == nil {
		return 0
	}
	return binary.BigEndian.Uint16(value)
}

func (reader *binaryDecoder) uint64() uint64 {
	value := reader.take(8)
	if value == nil {
		return 0
	}
	return binary.BigEndian.Uint64(value)
}

func (reader *binaryDecoder) int64() int64 { return int64(reader.uint64()) }

func (reader *binaryDecoder) short(minimum, maximum int) []byte {
	length := int(reader.byte())
	if length < minimum || length > maximum {
		reader.failed = true
		return nil
	}
	return reader.take(length)
}

func (reader *binaryDecoder) medium(minimum, maximum int) []byte {
	length := int(reader.uint16())
	if length < minimum || length > maximum {
		reader.failed = true
		return nil
	}
	return reader.take(length)
}

func (reader *binaryDecoder) done() bool { return reader.offset == len(reader.raw) }
