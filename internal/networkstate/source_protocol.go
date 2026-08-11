package networkstate

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	sourceLatest   = byte(1)
	sourceByDigest = byte(2)
	sourceOK       = byte(0)
	sourceNotFound = byte(1)
	sourceBusy     = byte(2)
	sourceBad      = byte(3)
	sourceInternal = byte(4)
)

type sourceRequest struct {
	opcode        byte
	networkDigest [32]byte
	objectDigest  [32]byte
	materialIndex uint32
}

type sourceResponse struct {
	status       byte
	objectDigest [32]byte
	payload      []byte
}

func readSourceRequest(reader io.Reader) (sourceRequest, error) {
	var raw [77]byte
	if _, err := io.ReadFull(reader, raw[:]); err != nil {
		return sourceRequest{}, err
	}
	if string(raw[:8]) != "ARDH3Q1\x00" || (raw[8] != sourceLatest && raw[8] != sourceByDigest) {
		return sourceRequest{}, errors.New("source request framing is invalid")
	}
	var request sourceRequest
	request.opcode = raw[8]
	copy(request.networkDigest[:], raw[9:41])
	copy(request.objectDigest[:], raw[41:73])
	request.materialIndex = binary.BigEndian.Uint32(raw[73:77])
	if request.materialIndex >= 64 {
		return sourceRequest{}, errors.New("source materialization index is invalid")
	}
	if request.opcode == sourceLatest && !isZero32(request.objectDigest) {
		return sourceRequest{}, errors.New("latest request digest is not zero")
	}
	if request.opcode == sourceByDigest && isZero32(request.objectDigest) {
		return sourceRequest{}, errors.New("by-digest request digest is zero")
	}
	return request, nil
}

func writeSourceRequest(writer io.Writer, request sourceRequest) error {
	var raw [77]byte
	copy(raw[:8], "ARDH3Q1\x00")
	raw[8] = request.opcode
	copy(raw[9:41], request.networkDigest[:])
	copy(raw[41:], request.objectDigest[:])
	binary.BigEndian.PutUint32(raw[73:77], request.materialIndex)
	_, err := writer.Write(raw[:])
	return err
}

func readSourceResponse(reader io.Reader) (sourceResponse, error) {
	var header [45]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return sourceResponse{}, err
	}
	if string(header[:8]) != "ARDH3S1\x00" || header[8] > sourceInternal {
		return sourceResponse{}, errors.New("source response framing is invalid")
	}
	var response sourceResponse
	response.status = header[8]
	copy(response.objectDigest[:], header[9:41])
	length := binary.BigEndian.Uint32(header[41:45])
	if response.status != sourceOK {
		if !isZero32(response.objectDigest) || length != 0 {
			return sourceResponse{}, errors.New("non-OK source response carries an object")
		}
		return response, nil
	}
	if isZero32(response.objectDigest) || length == 0 || length > maximumSourceBundleBytes {
		return sourceResponse{}, errors.New("OK source response object is invalid")
	}
	response.payload = make([]byte, length)
	if _, err := io.ReadFull(reader, response.payload); err != nil {
		response.payload = nil
		return response, err
	}
	return response, nil
}

func writeSourceResponse(writer io.Writer, response sourceResponse) error {
	if response.status > sourceInternal {
		return errors.New("source response status is invalid")
	}
	if response.status != sourceOK && (!isZero32(response.objectDigest) || len(response.payload) != 0) {
		return errors.New("non-OK source response carries an object")
	}
	var header [45]byte
	copy(header[:8], "ARDH3S1\x00")
	header[8] = response.status
	if response.status == sourceOK {
		if isZero32(response.objectDigest) || len(response.payload) == 0 || len(response.payload) > maximumSourceBundleBytes {
			return errors.New("OK source response object is invalid")
		}
		copy(header[9:41], response.objectDigest[:])
		binary.BigEndian.PutUint32(header[41:45], uint32(len(response.payload)))
	}
	if _, err := writer.Write(header[:]); err != nil {
		return err
	}
	if response.status == sourceOK {
		_, err := writer.Write(response.payload)
		return err
	}
	return nil
}
