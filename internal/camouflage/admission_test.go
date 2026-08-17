package camouflage

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAdmissionListenerRefusesAboveDeclaredCapacityAndRecovers(t *testing.T) {
	base, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener := &admissionListener{Listener: base, maximum: 1}
	t.Cleanup(func() { _ = listener.Close() })

	firstAccepted := acceptConnection(t, listener)
	firstClient := dialConnection(t, base.Addr().String())
	first := <-firstAccepted
	t.Cleanup(func() { _ = first.Close(); _ = firstClient.Close() })

	blockedAccept := acceptConnection(t, listener)
	second := dialConnection(t, base.Addr().String())
	_ = second.SetReadDeadline(time.Now().Add(time.Second))
	buffer := make([]byte, 1)
	_, readErr := second.Read(buffer)
	if !errors.Is(readErr, io.EOF) {
		t.Fatalf("over-capacity connection was not closed within one second: %v", readErr)
	}
	_ = second.Close()

	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	third := dialConnection(t, base.Addr().String())
	recovered := <-blockedAccept
	_ = recovered.Close()
	_ = third.Close()
}

func TestAuthenticatedSessionAdmissionRefusesOverflowAndRecovers(t *testing.T) {
	started, release := make(chan struct{}, 1), make(chan struct{})
	next := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		started <- struct{}{}
		<-release
		response.WriteHeader(http.StatusNoContent)
	})
	gate := &sessionAdmission{maximum: 1}
	handler := authenticatedSessionHandler(gate, next)
	first := httptest.NewRecorder()
	firstDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "https://front.example/entry", nil))
		close(firstDone)
	}()
	<-started
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "https://front.example/entry", nil))
	if second.Code != http.StatusServiceUnavailable {
		t.Fatalf("overflow status=%d want=%d", second.Code, http.StatusServiceUnavailable)
	}
	if active, accepted, refused := gate.snapshot(); active != 1 || accepted != 1 || refused != 1 {
		t.Fatalf("admission counters=%d/%d/%d", active, accepted, refused)
	}
	close(release)
	<-firstDone
	if first.Code != http.StatusNoContent {
		t.Fatalf("established session status=%d", first.Code)
	}
	third := httptest.NewRecorder()
	handler.ServeHTTP(third, httptest.NewRequest(http.MethodGet, "https://front.example/entry", nil))
	if third.Code != http.StatusNoContent {
		t.Fatalf("recovered admission status=%d", third.Code)
	}
	if active, accepted, refused := gate.snapshot(); active != 0 || accepted != 2 || refused != 1 {
		t.Fatalf("recovered admission counters=%d/%d/%d", active, accepted, refused)
	}
}

func TestAdmissionListenerProtectRefusesWithoutConsumingCapacity(t *testing.T) {
	base, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener := &admissionListener{Listener: base, maximum: 1}
	t.Cleanup(func() { _ = listener.Close() })
	listener.protected.Store(true)
	accepted := acceptConnection(t, listener)
	refused := dialConnection(t, base.Addr().String())
	_ = refused.SetReadDeadline(time.Now().Add(time.Second))
	_, readErr := refused.Read(make([]byte, 1))
	if !errors.Is(readErr, io.EOF) {
		t.Fatalf("PROTECT connection was not closed: %v", readErr)
	}
	_ = refused.Close()
	listener.protected.Store(false)
	client := dialConnection(t, base.Addr().String())
	connection := <-accepted
	_ = connection.Close()
	_ = client.Close()
}

func acceptConnection(t *testing.T, listener net.Listener) <-chan net.Conn {
	t.Helper()
	result := make(chan net.Conn, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			result <- connection
		}
	}()
	return result
}

func dialConnection(t *testing.T, address string) net.Conn {
	t.Helper()
	connection, err := net.DialTimeout("tcp4", address, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return connection
}
