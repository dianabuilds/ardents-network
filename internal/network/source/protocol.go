package source

import (
	"encoding/binary"
	"errors"
	"io"
)

const maximumPayloadBytes = 1 << 20

const (
	latestOpcode   = byte(1)
	byDigestOpcode = byte(2)
	okStatus       = byte(0)
	notFoundStatus = byte(1)
	busyStatus     = byte(2)
	badStatus      = byte(3)
	internalStatus = byte(4)
)

// Message is one exact acquisition request or bounded terminal response.
type Message struct {
	Operation     string
	NetworkDigest [32]byte
	ObjectDigest  [32]byte
	MaterialIndex uint32
	Status        string
	Payload       []byte
}

func readRequest(reader io.Reader) (Message, error) {
	var raw [77]byte
	if _, err := io.ReadFull(reader, raw[:]); err != nil {
		return Message{}, err
	}
	if string(raw[:8]) != "ARDH3Q1\x00" {
		return Message{}, errors.New("distribution request framing is invalid")
	}
	request := Message{Operation: operationName(raw[8])}
	copy(request.NetworkDigest[:], raw[9:41])
	copy(request.ObjectDigest[:], raw[41:73])
	request.MaterialIndex = binary.BigEndian.Uint32(raw[73:77])
	if request.Operation == "" || request.MaterialIndex >= 64 {
		return Message{}, errors.New("distribution request selector is invalid")
	}
	if request.Operation == "latest" && request.ObjectDigest != [32]byte{} {
		return Message{}, errors.New("latest request digest is not zero")
	}
	if request.Operation == "by-digest" && request.ObjectDigest == [32]byte{} {
		return Message{}, errors.New("by-digest request digest is zero")
	}
	return request, nil
}

func writeRequest(writer io.Writer, request Message) error {
	opcode := operationCode(request.Operation)
	if opcode == 0 || request.MaterialIndex >= 64 ||
		(request.Operation == "latest" && request.ObjectDigest != [32]byte{}) ||
		(request.Operation == "by-digest" && request.ObjectDigest == [32]byte{}) {
		return errors.New("distribution request is invalid")
	}
	var raw [77]byte
	copy(raw[:8], "ARDH3Q1\x00")
	raw[8] = opcode
	copy(raw[9:41], request.NetworkDigest[:])
	copy(raw[41:73], request.ObjectDigest[:])
	binary.BigEndian.PutUint32(raw[73:77], request.MaterialIndex)
	_, err := writer.Write(raw[:])
	return err
}

func readResponse(reader io.Reader) (Message, error) {
	var header [45]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return Message{}, err
	}
	status := statusName(header[8])
	if string(header[:8]) != "ARDH3S1\x00" || status == "" {
		return Message{}, errors.New("distribution response framing is invalid")
	}
	response := Message{Status: status}
	copy(response.ObjectDigest[:], header[9:41])
	length := binary.BigEndian.Uint32(header[41:45])
	if status != "ok" {
		if response.ObjectDigest != [32]byte{} || length != 0 {
			return Message{}, errors.New("non-OK distribution response carries an object")
		}
		return response, nil
	}
	if response.ObjectDigest == [32]byte{} || length == 0 || length > maximumPayloadBytes {
		return Message{}, errors.New("OK distribution response object is invalid")
	}
	response.Payload = make([]byte, length)
	if _, err := io.ReadFull(reader, response.Payload); err != nil {
		response.Payload = nil
		return response, err
	}
	return response, nil
}

func writeResponse(writer io.Writer, response Message) error {
	status := statusCode(response.Status)
	if status > internalStatus || statusName(status) == "" {
		return errors.New("distribution response status is invalid")
	}
	if response.Status != "ok" && (response.ObjectDigest != [32]byte{} || len(response.Payload) != 0) {
		return errors.New("non-OK distribution response carries an object")
	}
	var header [45]byte
	copy(header[:8], "ARDH3S1\x00")
	header[8] = status
	if response.Status == "ok" {
		if response.ObjectDigest == [32]byte{} || len(response.Payload) == 0 || len(response.Payload) > maximumPayloadBytes {
			return errors.New("OK distribution response object is invalid")
		}
		copy(header[9:41], response.ObjectDigest[:])
		binary.BigEndian.PutUint32(header[41:45], uint32(len(response.Payload)))
	}
	if _, err := writer.Write(header[:]); err != nil {
		return err
	}
	if response.Status == "ok" {
		_, err := writer.Write(response.Payload)
		return err
	}
	return nil
}

func operationCode(value string) byte {
	if value == "latest" {
		return latestOpcode
	}
	if value == "by-digest" {
		return byDigestOpcode
	}
	return 0
}

func operationName(value byte) string {
	if value == latestOpcode {
		return "latest"
	}
	if value == byDigestOpcode {
		return "by-digest"
	}
	return ""
}

func statusCode(value string) byte {
	switch value {
	case "ok":
		return okStatus
	case "not-found":
		return notFoundStatus
	case "busy":
		return busyStatus
	case "bad-request":
		return badStatus
	case "internal":
		return internalStatus
	default:
		return 255
	}
}

func statusName(value byte) string {
	switch value {
	case okStatus:
		return "ok"
	case notFoundStatus:
		return "not-found"
	case busyStatus:
		return "busy"
	case badStatus:
		return "bad-request"
	case internalStatus:
		return "internal"
	default:
		return ""
	}
}
