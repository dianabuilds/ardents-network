//go:build linux

package route

import (
	"net"
	"syscall"
	"testing"
)

func TestCarrierLivenessBindsKeepaliveAndUserTimeout(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
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
	client, err := net.Dial("tcp4", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server := <-accepted
	defer server.Close()
	if err := configureCarrierLiveness(client); err != nil {
		t.Fatal(err)
	}
	raw, err := client.(*net.TCPConn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var keepalive, timeout int
	var optionErr error
	if err := raw.Control(func(fd uintptr) {
		keepalive, optionErr = syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_KEEPALIVE)
		if optionErr == nil {
			timeout, optionErr = syscall.GetsockoptInt(int(fd), syscall.IPPROTO_TCP, 18)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if optionErr != nil || keepalive != 1 || timeout != carrierUserTimeoutMillis {
		t.Fatalf("Carrier liveness keepalive=%d timeout=%d err=%v", keepalive, timeout, optionErr)
	}
}
