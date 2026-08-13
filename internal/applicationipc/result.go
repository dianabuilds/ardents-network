package applicationipc

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
)

const maximumResult = 1024

// Result is the Route-opaque terminal outcome delivered to an authorized Application.
type Result struct {
	Class               string   `json:"class"`
	Reason              string   `json:"reason"`
	AuthenticatedTarget [32]byte `json:"authenticated_target"`
	AcceptedBytes       uint32   `json:"accepted_bytes"`
	ReceivedBytes       uint32   `json:"received_bytes"`
}

// ReadControl requires one complete half-closed local control frame before the operation deadline.
func ReadControl(ctx context.Context, connection net.Conn, maximum int) ([]byte, error) {
	if maximum < 1 || maximum > 4<<10 {
		return nil, errors.New("local control frame bound is invalid")
	}
	if deadline, ok := ctx.Deadline(); ok {
		if err := connection.SetDeadline(deadline); err != nil {
			return nil, err
		}
	}
	stop := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stop()
	raw, err := io.ReadAll(io.LimitReader(connection, int64(maximum+1)))
	if err != nil || len(raw) == 0 || len(raw) > maximum {
		return nil, errors.Join(err, errors.New("local control frame is empty, slow, or oversized"))
	}
	return raw, nil
}

// Write emits one complete bounded Connection Result frame.
func Write(output io.Writer, result Result) error {
	payload, err := json.Marshal(result)
	if err != nil || len(payload) == 0 || len(payload) > maximumResult || result.Class == "" {
		return errors.Join(err, errors.New("application result is invalid or exceeds its bound"))
	}
	frame := make([]byte, 7+len(payload))
	copy(frame[:4], "ASRS")
	frame[4] = 1
	binary.BigEndian.PutUint16(frame[5:7], uint16(len(payload)))
	copy(frame[7:], payload)
	_, err = io.Copy(output, bytes.NewReader(frame))
	return err
}

// Read requires one complete bounded Connection Result frame; EOF is not success.
func Read(input io.Reader) (Result, error) {
	var result Result
	header := make([]byte, 7)
	if _, err := io.ReadFull(input, header); err != nil {
		return result, errors.Join(err, errors.New("classified Connection Result is absent"))
	}
	length := int(binary.BigEndian.Uint16(header[5:7]))
	if string(header[:4]) != "ASRS" || header[4] != 1 || length == 0 || length > maximumResult {
		return result, errors.New("classified Connection Result frame is malformed")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(input, payload); err != nil {
		return result, err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil || decoder.Decode(&struct{}{}) != io.EOF || result.Class == "" {
		return Result{}, errors.New("classified Connection Result payload is invalid")
	}
	return result, nil
}
