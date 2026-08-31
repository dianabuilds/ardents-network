package browserentry

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"path/filepath"
	"testing"
)

func TestServeNativeHostReturnsOnlyAnActiveProbedLoopbackPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	path := filepath.Join(t.TempDir(), "browser-entry.json")
	publisher, err := OpenPublisher(path)
	if err != nil {
		t.Fatal(err)
	}
	defer publisher.Close()
	port := uint16(listener.Addr().(*net.TCPAddr).Port)
	if err := publisher.Publish(port); err != nil {
		t.Fatal(err)
	}
	capability := publisher.Capability()
	accepted := make(chan struct{})
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		buffer := make([]byte, 512)
		_, _ = connection.Read(buffer)
		_, _ = connection.Write([]byte("HTTP/1.1 204 No Content\r\n" + ProbeHeader + ": " + hex.EncodeToString(capability[:]) + "\r\nContent-Length: 0\r\n\r\n"))
		close(accepted)
	}()
	request, err := nativeFrame(map[string]string{"operation": OperationLoopbackProxyPort})
	if err != nil {
		t.Fatal(err)
	}
	var response bytes.Buffer
	if err := ServeNativeHost(bytes.NewReader(request), &response, path); err != nil {
		t.Fatal(err)
	}
	<-accepted
	message, err := readNativeFrame(&response)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Port uint16 `json:"port"`
	}
	if err := json.Unmarshal(message, &result); err != nil || result.Port != port {
		t.Fatalf("native host result = %+v / %v, want port %d", result, err, port)
	}
}

func TestServeNativeHostReturnsAuthenticationOnlyAfterAFreshProof(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	path := filepath.Join(t.TempDir(), "browser-entry.json")
	publisher, err := OpenPublisher(path)
	if err != nil {
		t.Fatal(err)
	}
	defer publisher.Close()
	port := uint16(listener.Addr().(*net.TCPAddr).Port)
	if err := publisher.Publish(port); err != nil {
		t.Fatal(err)
	}
	capability := publisher.Capability()
	accepted := make(chan struct{})
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		buffer := make([]byte, 512)
		_, _ = connection.Read(buffer)
		_, _ = connection.Write([]byte("HTTP/1.1 204 No Content\r\n" + ProbeHeader + ": " + hex.EncodeToString(capability[:]) + "\r\nContent-Length: 0\r\n\r\n"))
		close(accepted)
	}()
	request, err := nativeFrame(map[string]string{"operation": OperationLoopbackProxyAuthentication})
	if err != nil {
		t.Fatal(err)
	}
	var response bytes.Buffer
	if err := ServeNativeHost(bytes.NewReader(request), &response, path); err != nil {
		t.Fatal(err)
	}
	<-accepted
	message, err := readNativeFrame(&response)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Port     uint16 `json:"port"`
		Password string `json:"password"`
	}
	wantCredential := publisher.ProxyCredential()
	if err := json.Unmarshal(message, &result); err != nil || result.Port != port || result.Password != hex.EncodeToString(wantCredential[:]) {
		t.Fatalf("native host authentication result = %+v / %v, want current port and credential", result, err)
	}
}

func TestServeNativeHostRejectsLoopbackPortThatDoesNotProveCapability(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	path := filepath.Join(t.TempDir(), "browser-entry.json")
	publisher, err := OpenPublisher(path)
	if err != nil {
		t.Fatal(err)
	}
	defer publisher.Close()
	port := uint16(listener.Addr().(*net.TCPAddr).Port)
	if err := publisher.Publish(port); err != nil {
		t.Fatal(err)
	}
	accepted := make(chan struct{})
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		buffer := make([]byte, 512)
		_, _ = connection.Read(buffer)
		_, _ = connection.Write([]byte("HTTP/1.1 204 No Content\r\nContent-Length: 0\r\n\r\n"))
		close(accepted)
	}()
	request, err := nativeFrame(map[string]string{"operation": OperationLoopbackProxyPort})
	if err != nil {
		t.Fatal(err)
	}
	var response bytes.Buffer
	if err := ServeNativeHost(bytes.NewReader(request), &response, path); err == nil || err.Error() != "browser Entry loopback proxy is unavailable" {
		t.Fatalf("native host error = %v, want unavailable loopback proxy", err)
	}
	<-accepted
	if response.Len() != 0 {
		t.Fatalf("native host returned a port after an unproven probe: %q", response.String())
	}
}

func nativeFrame(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	frame := make([]byte, 4+len(body))
	binary.LittleEndian.PutUint32(frame, uint32(len(body)))
	copy(frame[4:], body)
	return frame, nil
}

func readNativeFrame(source io.Reader) ([]byte, error) {
	var length [4]byte
	if _, err := io.ReadFull(source, length[:]); err != nil {
		return nil, err
	}
	body := make([]byte, binary.LittleEndian.Uint32(length[:]))
	if _, err := io.ReadFull(source, body); err != nil {
		return nil, err
	}
	return body, nil
}
