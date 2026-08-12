package source

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

var errTestResponseWrite = errors.New("response write failed")

type responseWriteFailure struct {
	request bytes.Reader
}

func (connection *responseWriteFailure) Read(raw []byte) (int, error) {
	return connection.request.Read(raw)
}
func (*responseWriteFailure) Write([]byte) (int, error)        { return 0, errTestResponseWrite }
func (*responseWriteFailure) Close() error                     { return nil }
func (*responseWriteFailure) LocalAddr() net.Addr              { return testAddress("local") }
func (*responseWriteFailure) RemoteAddr() net.Addr             { return testAddress("remote") }
func (*responseWriteFailure) SetDeadline(time.Time) error      { return nil }
func (*responseWriteFailure) SetReadDeadline(time.Time) error  { return nil }
func (*responseWriteFailure) SetWriteDeadline(time.Time) error { return nil }

type testAddress string

func (address testAddress) Network() string { return "test" }
func (address testAddress) String() string  { return string(address) }

func TestHandleConnectionReturnsFinalResponseWriteError(t *testing.T) {
	var request bytes.Buffer
	if err := writeRequest(&request, Message{Operation: "latest"}); err != nil {
		t.Fatal(err)
	}
	connection := &responseWriteFailure{request: *bytes.NewReader(request.Bytes())}
	err := handleConnection(context.Background(), server{headerTimeout: time.Second}, nil, connection,
		func(context.Context, Message) Message { return Message{Status: "not-found"} })
	if !errors.Is(err, errTestResponseWrite) || !strings.Contains(err.Error(), "response") {
		t.Fatalf("connection error = %v, want final response write failure", err)
	}
}

func TestServePreservesParentDeadlineIdentity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	ready := make(chan error, 1)
	served := make(chan error, 1)
	go func() {
		served <- serve(ctx, server{address: "127.0.0.1:0", headerTimeout: time.Second}, ready, nil, nil,
			func(context.Context, Message) Message { return Message{Status: "not-found"} })
	}()
	if err := <-ready; err != nil {
		t.Fatal(err)
	}
	if err := <-served; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("serve error = %v, want deadline identity", err)
	}
}
