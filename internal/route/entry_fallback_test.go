package route

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"testing"
	"time"
)

func TestEntryFailureDoesNotDialOrdinaryInitiator(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	input := Actor{Deadline: time.Second}
	input.OpenEntry = func(context.Context,
		func(context.Context, net.Conn) (*tls.Conn, error),
	) (*tls.Conn, func() error, error) {
		return nil, nil, errors.New("bridge-attempt-exhausted")
	}
	first := Position{Endpoint: listener.Addr().String(), PublicKey: [32]byte{1}}
	if connection, _, err := openInitiator(context.Background(), input, first); err == nil || connection != nil {
		t.Fatalf("exhausted Bridge entry returned connection=%v error=%v", connection, err)
	}
	_ = listener.(*net.TCPListener).SetDeadline(time.Now().Add(100 * time.Millisecond))
	if connection, err := listener.Accept(); err == nil {
		_ = connection.Close()
		t.Fatal("Bridge exhaustion fell back to the ordinary Initiator endpoint")
	}
}
