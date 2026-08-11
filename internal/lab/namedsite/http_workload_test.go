package namedsite

import (
	"bytes"
	"io"
	"net"
	"testing"
)

func TestHTTPWorkloadVerifiesExactRequestResponseAndEOF(t *testing.T) {
	t.Parallel()
	nonce := bytes.Repeat([]byte{0x7c}, 32)
	request, err := buildHTTPRequest(nonce)
	if err != nil {
		t.Fatal(err)
	}
	if len(request) != 512 {
		t.Fatalf("request length = %d", len(request))
	}
	client, service := net.Pipe()
	defer client.Close()
	serverDone := make(chan error, 1)
	go func() { serverDone <- serveHTTPApplication(service, nonce) }()
	result, err := executeHTTPWorkload(client, nonce)
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != 200 || result.ResponseBytes != 64*1024 || !result.EOF {
		t.Fatalf("result = %#v", result)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}

func TestHTTPWorkloadRejectsWrongBodyAndRedirect(t *testing.T) {
	t.Parallel()
	nonce := bytes.Repeat([]byte{0x6d}, 32)
	tests := []struct {
		name     string
		response []byte
	}{
		{name: "redirect", response: []byte("HTTP/1.1 302 Found\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")},
		{name: "wrong body", response: append([]byte("HTTP/1.1 200 OK\r\nContent-Length: 65536\r\nContent-Type: application/octet-stream\r\nConnection: close\r\nCache-Control: no-store\r\n\r\n"), bytes.Repeat([]byte{0}, 64*1024)...)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, service := net.Pipe()
			defer client.Close()
			go func() {
				defer service.Close()
				request := make([]byte, 512)
				_, _ = io.ReadFull(service, request)
				_, _ = service.Write(test.response)
			}()
			if _, err := executeHTTPWorkload(client, nonce); err == nil {
				t.Fatal("invalid HTTP result accepted")
			}
		})
	}
}
