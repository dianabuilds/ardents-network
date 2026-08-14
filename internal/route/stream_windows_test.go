//go:build windows

package route

import (
	"net"
	"syscall"
	"testing"
	"time"
)

func TestWindowsPeerResetAfterCompletedStreamIsBenign(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			accepted <- connection
		}
	}()
	client, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server := <-accepted
	tcpServer, ok := server.(*net.TCPConn)
	if !ok {
		t.Fatal("accepted connection is not TCP")
	}
	if err := tcpServer.SetLinger(0); err != nil {
		t.Fatal(err)
	}
	if err := tcpServer.Close(); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 1)
	_, resetErr := client.Read(buffer)
	if resetErr == nil {
		t.Fatal("forced peer close did not produce a reset")
	}
	if !benignStreamError(resetErr) {
		t.Fatalf("completed stream peer reset was not classified as benign: %v", resetErr)
	}
}

func TestWindowsLocalAbortAfterCompletedStreamIsBenign(t *testing.T) {
	err := &net.OpError{Op: "read", Net: "tcp", Err: syscall.WSAECONNABORTED}
	if !benignStreamError(err) {
		t.Fatalf("completed stream local abort was not classified as benign: %v", err)
	}
}
