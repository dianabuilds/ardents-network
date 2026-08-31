package browserreference

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func transparentExactHead(t *testing.T, opening string, bytes int) string {
	t.Helper()
	const field = "X-Alpha-Limit: "
	const ending = "\r\n\r\n"
	padding := bytes - len(opening) - len(field) - len(ending)
	if padding < 0 {
		t.Fatal("transparent head fixture is larger than its declared boundary")
	}
	return opening + field + strings.Repeat("x", padding) + ending
}

func TestTransparentServerAcceptsExactRequestHeadLimit(t *testing.T) {
	server, applicationSide := openTransparentTimeoutServer(t)
	applicationDone := make(chan error, 1)
	go func() {
		request, err := http.ReadRequest(bufio.NewReader(applicationSide))
		if err == nil {
			_ = request.Body.Close()
			_, err = io.WriteString(applicationSide, "HTTP/1.1 204 No Content\r\nContent-Length: 0\r\n\r\n")
		}
		applicationDone <- err
	}()
	browser, err := net.Dial("tcp", server.alphaOriginAddress())
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	requestHead := transparentExactHead(t, "GET / HTTP/1.1\r\nHost: blog.alice.ard\r\n", transparentMaximumHeaderBytes)
	if _, err := io.WriteString(browser, requestHead); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(browser), nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("exact request head status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	if err := <-applicationDone; err != nil {
		t.Fatal(err)
	}
}

func TestTransparentServerRejectsRequestHeadAboveExactLimit(t *testing.T) {
	server, applicationSide := openTransparentTimeoutServer(t)
	browser, err := net.Dial("tcp", server.alphaOriginAddress())
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	requestHead := transparentExactHead(t, "GET / HTTP/1.1\r\nHost: blog.alice.ard\r\n", transparentMaximumHeaderBytes+1)
	if _, err := io.WriteString(browser, requestHead); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(browser), nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("oversized request head status = %d, want %d", response.StatusCode, http.StatusRequestHeaderFieldsTooLarge)
	}
	if err := applicationSide.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	defer applicationSide.SetReadDeadline(time.Time{})
	if _, err := applicationSide.Read(make([]byte, 1)); err == nil {
		t.Fatal("oversized request head reached the selected Publisher Service")
	}
}

func TestTransparentServerAcceptsExactResponseHeadLimit(t *testing.T) {
	server, applicationSide := openTransparentTimeoutServer(t)
	applicationDone := make(chan error, 1)
	go func() {
		request, err := http.ReadRequest(bufio.NewReader(applicationSide))
		if err == nil {
			_ = request.Body.Close()
			_, err = io.WriteString(applicationSide, transparentExactHead(t, "HTTP/1.1 204 No Content\r\n", transparentMaximumHeaderBytes))
		}
		applicationDone <- err
	}()
	browser, err := net.Dial("tcp", server.alphaOriginAddress())
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	if _, err := io.WriteString(browser, "GET / HTTP/1.1\r\nHost: blog.alice.ard\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(browser), nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("exact response head status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	if err := <-applicationDone; err != nil {
		t.Fatal(err)
	}
}

func TestTransparentServerRejectsResponseHeadAboveExactLimit(t *testing.T) {
	server, applicationSide := openTransparentTimeoutServer(t)
	applicationDone := make(chan struct{})
	go func() {
		defer close(applicationDone)
		request, err := http.ReadRequest(bufio.NewReader(applicationSide))
		if err != nil {
			return
		}
		_ = request.Body.Close()
		_, _ = io.WriteString(applicationSide, transparentExactHead(t, "HTTP/1.1 204 No Content\r\n", transparentMaximumHeaderBytes+1))
	}()
	browser, err := net.Dial("tcp", server.alphaOriginAddress())
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	if _, err := io.WriteString(browser, "GET / HTTP/1.1\r\nHost: blog.alice.ard\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(browser), nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("oversized response head status = %d, want %d", response.StatusCode, http.StatusBadGateway)
	}
	select {
	case <-applicationDone:
	case <-time.After(time.Second):
		t.Fatal("oversized response head did not stop the selected Publisher Service")
	}
}
