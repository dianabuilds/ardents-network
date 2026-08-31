package browserreference

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func openTransparentTimeoutServer(t *testing.T) (*TransparentServer, net.Conn) {
	t.Helper()
	bridgeSide, applicationSide := net.Pipe()
	t.Cleanup(func() { _ = applicationSide.Close() })
	server, err := OpenTransparent(TransparentConfig{
		Target: [32]byte{1}, Hostname: "blog.alice.ard", Connection: bridgeSide,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	return server, applicationSide
}

func requireBrowserConnectionClosed(t *testing.T, connection net.Conn, subject string) {
	t.Helper()
	closed := make(chan error, 1)
	go func() {
		buffer := make([]byte, 1)
		for {
			_, err := connection.Read(buffer)
			if err != nil {
				closed <- err
				return
			}
		}
	}()
	select {
	case err := <-closed:
		if isTimeout(err) {
			t.Fatalf("%s remained open: %v", subject, err)
		}
	case <-time.After(time.Second):
		t.Fatalf("%s did not close", subject)
	}
}

func TestTransparentServerStopsIncompleteBrowserRequestAtHeaderTimeout(t *testing.T) {
	server, applicationSide := openTransparentTimeoutServer(t)
	browser, err := net.Dial("tcp", server.alphaOriginAddress())
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	if _, err := io.WriteString(browser, "GET / HTTP/1.1\r\nHost: blog.alice.ard\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := browser.SetReadDeadline(time.Now().Add(transparentReadHeaderTimeout / 2)); err != nil {
		t.Fatal(err)
	}
	if _, err := browser.Read(make([]byte, 1)); !isTimeout(err) {
		t.Fatalf("incomplete request closed before the declared header timeout: %v", err)
	}
	if err := browser.SetReadDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(transparentReadHeaderTimeout/2 + 100*time.Millisecond)
	requireBrowserConnectionClosed(t, browser, "incomplete browser request after the header timeout")
	if err := applicationSide.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	defer applicationSide.SetReadDeadline(time.Time{})
	if _, err := applicationSide.Read(make([]byte, 1)); err == nil {
		t.Fatal("incomplete browser request reached the selected Publisher Service")
	}
}

func TestTransparentServerClosesIdleBrowserKeepAliveBeforeSecondServiceRequest(t *testing.T) {
	server, applicationSide := openTransparentTimeoutServer(t)
	firstRequest := make(chan error, 1)
	go func() {
		request, err := http.ReadRequest(bufio.NewReader(applicationSide))
		if err == nil {
			_ = request.Body.Close()
			_, err = io.WriteString(applicationSide, "HTTP/1.1 204 No Content\r\nContent-Length: 0\r\n\r\n")
		}
		firstRequest <- err
	}()
	browser, err := net.Dial("tcp", server.alphaOriginAddress())
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	if _, err := io.WriteString(browser, "GET /first HTTP/1.1\r\nHost: blog.alice.ard\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(browser), nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("first browser response status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	if err := <-firstRequest; err != nil {
		t.Fatal(err)
	}
	if err := browser.SetReadDeadline(time.Now().Add(transparentIdleTimeout / 2)); err != nil {
		t.Fatal(err)
	}
	if _, err := browser.Read(make([]byte, 1)); !isTimeout(err) {
		t.Fatalf("browser keep-alive closed before the declared idle timeout: %v", err)
	}
	if err := browser.SetReadDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(transparentIdleTimeout/2 + 100*time.Millisecond)
	requireBrowserConnectionClosed(t, browser, "idle browser keep-alive after the idle timeout")
}

func isTimeout(err error) bool {
	timeout, ok := err.(net.Error)
	return ok && timeout.Timeout()
}
