package namedsite

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestApplicationInterfaceConnectsThenCarriesRawStream(t *testing.T) {
	t.Parallel()
	application, endpoint := net.Pipe()
	service, route := net.Pipe()
	t.Cleanup(func() { _ = application.Close(); _ = service.Close() })

	done := make(chan error, 1)
	go func() {
		done <- serveClientConnection(t.Context(), endpoint, func(context.Context, connectRequest) (connectionResult, io.ReadWriteCloser, error) {
			return connectionResult{Target: "target:gate-c:fixture", NameGeneration: 1, NameRevision: 1, InstanceGeneration: 2}, route, nil
		})
	}()

	writeTestFrame(t, application, `{"schema":"gatec-application-interface/v1","operation":"connect","destination":"ardents://site.reference"}`)
	response := readTestFrame(t, application)
	var result map[string]any
	if err := json.Unmarshal(response, &result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "connected" || result["target"] != "target:gate-c:fixture" || result["instance_generation"] != float64(2) {
		t.Fatalf("connect result = %#v", result)
	}
	for _, forbidden := range []string{"node", "relay", "gateway", "rendezvous", "topology", "address"} {
		if bytes.Contains(bytes.ToLower(response), []byte(forbidden)) {
			t.Fatalf("connect result exposes %q: %s", forbidden, response)
		}
	}

	payload := []byte("opaque-http-stream")
	go func() { _, _ = service.Write(payload) }()
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(application, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("stream = %q", got)
	}
	_ = application.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("connection did not terminate")
	}
}

func TestApplicationInterfaceRejectsInvalidControlFrames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		frame string
	}{
		{name: "unknown field", frame: `{"schema":"gatec-application-interface/v1","operation":"connect","destination":"ardents://site.reference","node":"forbidden"}`},
		{name: "wrong schema", frame: `{"schema":"gatec-application-interface/v2","operation":"connect","destination":"ardents://site.reference"}`},
		{name: "wrong operation", frame: `{"schema":"gatec-application-interface/v1","operation":"resolve","destination":"ardents://site.reference"}`},
		{name: "wrong destination", frame: `{"schema":"gatec-application-interface/v1","operation":"connect","destination":"ardents://other.reference"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			application, endpoint := net.Pipe()
			defer application.Close()
			go func() {
				_ = serveClientConnection(t.Context(), endpoint, func(context.Context, connectRequest) (connectionResult, io.ReadWriteCloser, error) {
					t.Error("connector called for invalid frame")
					return connectionResult{}, nil, nil
				})
			}()
			writeTestFrame(t, application, test.frame)
			response := string(readTestFrame(t, application))
			if !strings.Contains(response, `"status":"failed"`) || !strings.Contains(response, `"class":"invalid_request"`) {
				t.Fatalf("response = %s", response)
			}
		})
	}
}

func TestApplicationInterfaceRejectsOversizeAndPartialFrame(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		length uint32
		body   []byte
	}{
		{name: "oversize", length: 8193},
		{name: "partial", length: 16, body: []byte("short")},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
			defer cancel()
			application, endpoint := net.Pipe()
			defer application.Close()
			done := make(chan error, 1)
			go func() { done <- serveClientConnection(ctx, endpoint, nil) }()
			var prefix [4]byte
			binary.BigEndian.PutUint32(prefix[:], test.length)
			_, _ = application.Write(append(prefix[:], test.body...))
			if test.name == "partial" {
				<-ctx.Done()
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("invalid frame accepted")
				}
			case <-time.After(time.Second):
				t.Fatal("invalid frame did not terminate")
			}
		})
	}
}

func TestApplicationInterfaceRejectsTrailingAndContradictoryResponses(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		payload string
	}{
		{name: "trailing value", payload: `{"schema":"gatec-application-interface/v1","status":"connected","target":"target","name_generation":1,"name_revision":1,"instance_generation":1}{}`},
		{name: "connected failure class", payload: `{"schema":"gatec-application-interface/v1","status":"connected","target":"target","name_generation":1,"name_revision":1,"instance_generation":1,"class":"route_unavailable"}`},
		{name: "failed target", payload: `{"schema":"gatec-application-interface/v1","status":"failed","target":"target","class":"route_unavailable"}`},
		{name: "unknown status", payload: `{"schema":"gatec-application-interface/v1","status":"pending"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := readConnectResponse(bytes.NewReader(testControlFrame(test.payload))); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
}

func testControlFrame(payload string) []byte {
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(payload)))
	return append(prefix[:], payload...)
}

func writeTestFrame(t *testing.T, conn net.Conn, payload string) {
	t.Helper()
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(payload)))
	if _, err := conn.Write(append(prefix[:], payload...)); err != nil {
		t.Fatal(err)
	}
}

func readTestFrame(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	var prefix [4]byte
	if _, err := io.ReadFull(conn, prefix[:]); err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, binary.BigEndian.Uint32(prefix[:]))
	if _, err := io.ReadFull(conn, payload); err != nil {
		t.Fatal(err)
	}
	return payload
}
