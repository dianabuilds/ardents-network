package route

import (
	"errors"
	"fmt"
	"io"
)

const (
	routeWireMagic   = "ardents-interactive-route-v2\x00"
	routeWireVersion = uint16(2)
	// Profile is the exact native Interactive Route v2 ALPN and wire profile.
	Profile         = "ardents-interactive-route-v2"
	maximumWireBody = 4096
)

type wireReader struct {
	raw []byte
	off int
}

func (reader *wireReader) take(length int) []byte {
	if length < 0 || length > len(reader.raw)-reader.off {
		reader.off = len(reader.raw) + 1
		return nil
	}
	value := reader.raw[reader.off : reader.off+length]
	reader.off += length
	return value
}

func (reader *wireReader) uint8() byte {
	value := reader.take(1)
	if len(value) != 1 {
		return 0
	}
	return value[0]
}

func (reader *wireReader) uint16() uint16 {
	value := reader.take(2)
	if len(value) != 2 {
		return 0
	}
	return uint16(value[0])<<8 | uint16(value[1])
}

func (reader *wireReader) uint64() uint64 {
	value := reader.take(8)
	if len(value) != 8 {
		return 0
	}
	var result uint64
	for _, item := range value {
		result = result<<8 | uint64(item)
	}
	return result
}

func routeEnvelope(body []byte) ([]byte, error) {
	if len(body) == 0 || len(body) > maximumWireBody {
		return nil, errors.New("route wire body is outside its bound")
	}
	result := make([]byte, 0, len(routeWireMagic)+2+len(body))
	result = append(result, routeWireMagic...)
	result = appendUint16(result, uint16(len(body)))
	return append(result, body...), nil
}

func routeBody(raw []byte, kind byte) (*wireReader, error) {
	if len(raw) < len(routeWireMagic)+2 || string(raw[:len(routeWireMagic)]) != routeWireMagic {
		return nil, errors.New("route wire magic is invalid")
	}
	reader := &wireReader{raw: raw[len(routeWireMagic):]}
	length := int(reader.uint16())
	body := reader.take(length)
	if reader.off != len(reader.raw) || len(body) != length || length == 0 || length > maximumWireBody {
		return nil, errors.New("route wire length is invalid")
	}
	reader = &wireReader{raw: body}
	if reader.uint16() != routeWireVersion || reader.uint8() != kind {
		return nil, errors.New("route wire kind or version is invalid")
	}
	profileLength := int(reader.uint8())
	profile := string(reader.take(profileLength))
	if !validRouteProfile(profile) {
		return nil, errors.New("route wire profile is invalid")
	}
	return reader, nil
}

func appendUint16(destination []byte, value uint16) []byte {
	return append(destination, byte(value>>8), byte(value))
}

func appendUint64(destination []byte, value uint64) []byte {
	for shift := uint(56); ; shift -= 8 {
		destination = append(destination, byte(value>>shift))
		if shift == 0 {
			return destination
		}
	}
}

func appendProfile(destination []byte) []byte {
	return append(append(destination, byte(len(Profile))), Profile...)
}

func validRouteProfile(value string) bool {
	if value != Profile || len(value) == 0 || len(value) > 63 {
		return false
	}
	for _, item := range []byte(value) {
		if item < 0x21 || item > 0x7e {
			return false
		}
	}
	return true
}

func wireIdentifier(reader *wireReader, name string) ([32]byte, error) {
	var result [32]byte
	copy(result[:], reader.take(len(result)))
	if result == [32]byte{} {
		return [32]byte{}, fmt.Errorf("route wire %s is missing", name)
	}
	return result, nil
}

func writeAll(writer io.Writer, value []byte) error {
	for len(value) != 0 {
		count, err := writer.Write(value)
		if err != nil {
			return err
		}
		if count <= 0 || count > len(value) {
			return io.ErrShortWrite
		}
		value = value[count:]
	}
	return nil
}
