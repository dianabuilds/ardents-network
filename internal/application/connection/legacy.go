package connection

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

const maximumResult = 1024

var legacyHello = []byte{'A', 'S', 'A', 'P', 1, 1}

// Result is the Route-opaque terminal record for the retained direct local
// Application transport.
type Result struct {
	Class               string   `json:"class"`
	Reason              string   `json:"reason"`
	AuthenticatedTarget [32]byte `json:"authenticated_target"`
	AcceptedBytes       uint32   `json:"accepted_bytes"`
	ReceivedBytes       uint32   `json:"received_bytes"`
}

// LegacyStream joins the retained opaque data channel to its derived terminal
// result sideband without giving the caller either wire grammar.
type LegacyStream struct {
	net.Conn
	result     net.Conn
	resultMu   sync.Mutex
	resultSent bool
}

// OpenApplication selects the retained direct local Application contract.
func OpenApplication(raw, result net.Conn) (*LegacyStream, error) {
	if raw == nil || result == nil {
		return nil, errors.New("application connections are incomplete")
	}
	if _, err := raw.Write(legacyHello); err != nil {
		return nil, err
	}
	return &LegacyStream{Conn: raw, result: result}, nil
}

// ResultPath derives the sole result-channel path.
func ResultPath(applicationPath string) (string, error) {
	if applicationPath == "" || strings.HasSuffix(applicationPath, ".result") {
		return "", errors.New("application socket path cannot derive a result channel")
	}
	return applicationPath + ".result", nil
}

// SendResult emits at most one terminal result.
func (connection *LegacyStream) SendResult(result Result) error {
	connection.resultMu.Lock()
	defer connection.resultMu.Unlock()
	if connection.resultSent {
		return errors.New("terminal connection result was already sent")
	}
	connection.resultSent = true
	return WriteResult(connection.result, result)
}

// Result reads the exact terminal result after Application data.
func (connection *LegacyStream) Result() (Result, error) { return ReadResult(connection.result) }

// Close releases both retained local channels.
func (connection *LegacyStream) Close() error {
	return errors.Join(connection.Conn.Close(), connection.result.Close())
}

// ReadControl requires one complete half-closed local control frame.
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

// WriteResult emits one complete bounded Connection Result frame.
func WriteResult(output io.Writer, result Result) error {
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

// ReadResult requires one complete bounded Connection Result frame.
func ReadResult(input io.Reader) (Result, error) {
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

// AcceptApplication verifies the direct local data-channel handshake.
func AcceptApplication(raw net.Conn, deadline time.Time) error {
	if raw == nil || deadline.IsZero() {
		return errors.New("application contract is incomplete")
	}
	if err := raw.SetDeadline(deadline); err != nil {
		return err
	}
	hello := make([]byte, len(legacyHello))
	if _, err := io.ReadFull(raw, hello); err != nil {
		return errors.Join(err, errors.New("application contract handshake is absent"))
	}
	if !bytes.Equal(hello, legacyHello) {
		return errors.New("application contract is unsupported")
	}
	return raw.SetDeadline(time.Time{})
}

// AcceptResult accepts one exact result sideband before the deadline.
func AcceptResult(listener *net.UnixListener, deadline time.Time) (net.Conn, error) {
	if listener == nil || deadline.IsZero() {
		return nil, errors.New("application result listener is incomplete")
	}
	if err := listener.SetDeadline(deadline); err != nil {
		return nil, err
	}
	return listener.Accept()
}
