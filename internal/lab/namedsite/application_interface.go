package namedsite

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	applicationInterfaceSchema = "gatec-application-interface/v1"
	gateCServiceLink           = "ardents://site.reference"
	maximumControlFrame        = 8 * 1024
)

var errInvalidRequest = errors.New("invalid application request")

type connectRequest struct {
	Schema      string `json:"schema"`
	Operation   string `json:"operation"`
	Destination string `json:"destination"`
}

type connectionResult struct {
	Target             string
	NameGeneration     uint64
	NameRevision       uint64
	InstanceGeneration uint64
}

type connectResponse struct {
	Schema             string `json:"schema"`
	Status             string `json:"status"`
	Target             string `json:"target,omitempty"`
	NameGeneration     uint64 `json:"name_generation,omitempty"`
	NameRevision       uint64 `json:"name_revision,omitempty"`
	InstanceGeneration uint64 `json:"instance_generation,omitempty"`
	Class              string `json:"class,omitempty"`
}

type connectionFailure struct{ class string }

func (failure connectionFailure) Error() string { return failure.class }

func serveClientConnection(ctx context.Context, connection net.Conn, connect func(context.Context, connectRequest) (connectionResult, io.ReadWriteCloser, error)) error {
	if ctx == nil || connection == nil {
		return errors.New("application connection and context are required")
	}
	defer connection.Close()
	applyContextDeadline(ctx, connection)
	request, err := readConnectRequest(connection)
	if err != nil {
		if errors.Is(err, errInvalidRequest) {
			_ = writeControlFrame(connection, connectResponse{Schema: applicationInterfaceSchema, Status: "failed", Class: "invalid_request"})
		}
		return err
	}
	if err := validateConnectRequest(request); err != nil {
		_ = writeControlFrame(connection, connectResponse{Schema: applicationInterfaceSchema, Status: "failed", Class: "invalid_request"})
		return err
	}
	if connect == nil {
		return errors.New("application connector is required")
	}
	result, stream, err := connect(ctx, request)
	if err != nil {
		failureClass := "indeterminate"
		var classified connectionFailure
		if errors.As(err, &classified) && validFailureClass(classified.class) {
			failureClass = classified.class
		}
		if writeErr := writeControlFrame(connection, connectResponse{Schema: applicationInterfaceSchema, Status: "failed", Class: failureClass}); writeErr != nil {
			return errors.Join(err, writeErr)
		}
		return err
	}
	if stream == nil || result.Target == "" || result.NameGeneration == 0 || result.NameRevision == 0 || result.InstanceGeneration == 0 {
		return errors.New("connector returned an incomplete authenticated result")
	}
	defer stream.Close()
	if err := writeControlFrame(connection, connectResponse{
		Schema: applicationInterfaceSchema, Status: "connected", Target: result.Target,
		NameGeneration: result.NameGeneration, NameRevision: result.NameRevision,
		InstanceGeneration: result.InstanceGeneration,
	}); err != nil {
		return err
	}
	return proxyOpaqueStream(ctx, connection, stream)
}

func readConnectRequest(reader io.Reader) (connectRequest, error) {
	payload, err := readControlFrame(reader)
	if err != nil {
		return connectRequest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var request connectRequest
	if err := decoder.Decode(&request); err != nil {
		return connectRequest{}, fmt.Errorf("%w: control frame has invalid encoding", errInvalidRequest)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return connectRequest{}, fmt.Errorf("%w: control frame has trailing data", errInvalidRequest)
	}
	return request, nil
}

func validateConnectRequest(request connectRequest) error {
	if request.Schema != applicationInterfaceSchema || request.Operation != "connect" || request.Destination != gateCServiceLink {
		return errors.New("application connect request is outside the Gate C contract")
	}
	return nil
}

func readControlFrame(reader io.Reader) ([]byte, error) {
	var prefix [4]byte
	if _, err := io.ReadFull(reader, prefix[:]); err != nil {
		return nil, fmt.Errorf("read application frame length: %w", err)
	}
	length := binary.BigEndian.Uint32(prefix[:])
	if length == 0 || length > maximumControlFrame {
		return nil, errors.New("application control frame length is invalid")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, fmt.Errorf("read application control frame: %w", err)
	}
	return payload, nil
}

func writeControlFrame(writer io.Writer, value connectResponse) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(payload) > maximumControlFrame {
		return errors.New("application response exceeds the control-frame bound")
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(payload)))
	if _, err := writer.Write(prefix[:]); err != nil {
		return err
	}
	_, err = writer.Write(payload)
	return err
}

func writeConnectRequest(writer io.Writer) error {
	payload, err := json.Marshal(connectRequest{Schema: applicationInterfaceSchema, Operation: "connect", Destination: gateCServiceLink})
	if err != nil {
		return err
	}
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(payload)))
	if _, err := writer.Write(prefix[:]); err != nil {
		return err
	}
	_, err = writer.Write(payload)
	return err
}

func readConnectResponse(reader io.Reader) (connectResponse, error) {
	payload, err := readControlFrame(reader)
	if err != nil {
		return connectResponse{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var response connectResponse
	if err := decoder.Decode(&response); err != nil {
		return connectResponse{}, errors.New("application response is invalid")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return connectResponse{}, errors.New("application response has trailing data")
	}
	if err := validateConnectResponse(response); err != nil {
		return connectResponse{}, err
	}
	return response, nil
}

func validateConnectResponse(response connectResponse) error {
	if response.Schema != applicationInterfaceSchema {
		return errors.New("application response schema is invalid")
	}
	switch response.Status {
	case "connected":
		if response.Target == "" || response.NameGeneration == 0 || response.NameRevision == 0 || response.InstanceGeneration == 0 || response.Class != "" {
			return errors.New("connected application response is incomplete or contradictory")
		}
	case "failed":
		validClass := validFailureClass(response.Class) || response.Class == "invalid_request"
		if !validClass || response.Target != "" || response.NameGeneration != 0 || response.NameRevision != 0 || response.InstanceGeneration != 0 {
			return errors.New("failed application response is incomplete or contradictory")
		}
	default:
		return errors.New("application response status is invalid")
	}
	return nil
}

func proxyOpaqueStream(ctx context.Context, left, right io.ReadWriteCloser) error {
	errorsFound := make(chan error, 2)
	copyOne := func(destination io.Writer, source io.Reader) {
		_, err := io.Copy(destination, source)
		errorsFound <- err
	}
	go copyOne(left, right)
	go copyOne(right, left)
	select {
	case <-ctx.Done():
		_ = left.Close()
		_ = right.Close()
		return ctx.Err()
	case err := <-errorsFound:
		_ = left.Close()
		_ = right.Close()
		if err == nil || errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	}
}

func applyContextDeadline(ctx context.Context, connection net.Conn) {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(15 * time.Second)
	}
	_ = connection.SetDeadline(deadline)
}

func validFailureClass(value string) bool {
	switch value {
	case "name_not_found", "authentication_failed", "service_offline", "route_unavailable", "indeterminate":
		return true
	default:
		return false
	}
}
