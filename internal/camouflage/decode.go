package camouflage

import "encoding/binary"

type decoder struct {
	raw    []byte
	offset int
	failed bool
}

func (reader *decoder) take(length int) []byte {
	if length < 0 || reader.offset > len(reader.raw)-length {
		reader.failed = true
		return nil
	}
	value := reader.raw[reader.offset : reader.offset+length]
	reader.offset += length
	return value
}

func (reader *decoder) byte() byte {
	value := reader.take(1)
	if value == nil {
		return 0
	}
	return value[0]
}

func (reader *decoder) uint16() uint16 {
	value := reader.take(2)
	if value == nil {
		return 0
	}
	return binary.BigEndian.Uint16(value)
}

func (reader *decoder) short(minimum, maximum int) []byte {
	length := int(reader.byte())
	if length < minimum || length > maximum {
		reader.failed = true
		return nil
	}
	return reader.take(length)
}

func (reader *decoder) medium(minimum, maximum int) []byte {
	length := int(reader.uint16())
	if length < minimum || length > maximum {
		reader.failed = true
		return nil
	}
	return reader.take(length)
}

func (reader *decoder) done() bool { return reader.offset == len(reader.raw) }
