//go:build linux

package recoverysmoke

import (
	"encoding/binary"
	"errors"
	"net"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCarrierDiagRequestAndMalformedResponse(t *testing.T) {
	request := makeCarrierDiagRequest(unix.SOCK_DIAG_BY_FAMILY, unix.NLM_F_REQUEST|unix.NLM_F_DUMP, nil)
	if len(request) != 72 || binary.NativeEndian.Uint16(request[4:6]) != unix.SOCK_DIAG_BY_FAMILY ||
		binary.NativeEndian.Uint32(request[20:24]) != 1<<carrierTCPEstablished {
		t.Fatalf("malformed observation request: %x", request)
	}
	malformed := make([]byte, 16)
	binary.NativeEndian.PutUint32(malformed[:4], 15)
	if _, _, err := parseCarrierDiagDatagram(malformed); err == nil {
		t.Fatal("malformed response passed")
	}
}

func TestCarrierFaultObservesSocketAndControlsExactInterface(t *testing.T) {
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
	observation, err := observeCarrierSocket(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_, raw, err := decodeCarrierSocketID(observation.SocketID)
	if err != nil {
		t.Fatal(err)
	}
	present, err := platformCarrierSocketPresent(raw)
	if err != nil || !present {
		t.Fatalf("observed socket present=%v err=%v", present, err)
	}
	if err := platformSetCarrierInterface(observation.InterfaceName, false); errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES) {
		t.Skip("interface fault requires NET_ADMIN")
	} else if err != nil {
		t.Fatal(err)
	}
	defer platformSetCarrierInterface(observation.InterfaceName, true)
	down, err := net.InterfaceByName(observation.InterfaceName)
	if err != nil || down.Flags&net.FlagUp != 0 {
		t.Fatalf("Carrier interface remained up: %+v err=%v", down, err)
	}
	if err := platformSetCarrierInterface(observation.InterfaceName, true); err != nil {
		t.Fatal(err)
	}
	up, err := net.InterfaceByName(observation.InterfaceName)
	if err != nil || up.Flags&net.FlagUp == 0 {
		t.Fatalf("Carrier interface did not recover: %+v err=%v", up, err)
	}
}
