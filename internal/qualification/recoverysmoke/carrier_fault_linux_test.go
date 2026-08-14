//go:build linux

package recoverysmoke

import (
	"encoding/binary"
	"net"
	"os"
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

func TestCarrierFaultDeletesExactIsolatedInterface(t *testing.T) {
	if os.Getenv("ARDENTS_TEST_DELETE_INTERFACE") != "1" {
		t.Skip("requires an isolated disposable network namespace")
	}
	if _, err := os.Stat("/.dockerenv"); err != nil {
		t.Fatal("destructive interface test requires a disposable Docker namespace")
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	var disposable []net.Interface
	for _, candidate := range interfaces {
		if candidate.Flags&net.FlagLoopback != 0 {
			continue
		}
		disposable = append(disposable, candidate)
	}
	if len(disposable) != 1 || disposable[0].Name != "eth0" || disposable[0].Flags&net.FlagUp == 0 {
		t.Fatalf("unexpected disposable namespace topology: %+v", disposable)
	}
	if err := platformDeleteCarrierInterface("eth0"); err != nil {
		t.Fatal(err)
	}
	if _, err := net.InterfaceByName("eth0"); err == nil {
		t.Fatal("deleted Carrier interface remained present")
	}
}

func TestCarrierFaultObservesExactSocket(t *testing.T) {
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
}
