package reference

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

type alphaEarlyResponseOrigin struct{ address string }

func (origin alphaEarlyResponseOrigin) alphaOriginAddress() string  { return origin.address }
func (alphaEarlyResponseOrigin) alphaOriginHost(host string) string { return host }
func (alphaEarlyResponseOrigin) alphaOriginPath(path string) string { return path }

func TestAlphaProxyEarlyOriginErrorStopsSlowBrowserUpload(t *testing.T) {
	originListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = originListener.Close() })
	originDone := make(chan error, 1)
	go func() {
		connection, acceptErr := originListener.Accept()
		if acceptErr != nil {
			originDone <- acceptErr
			return
		}
		defer connection.Close()
		_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
		request, readErr := http.ReadRequest(bufio.NewReader(connection))
		if readErr != nil {
			originDone <- readErr
			return
		}
		_ = request
		_, writeErr := io.WriteString(connection, "HTTP/1.1 413 Payload Too Large\r\nContent-Length: 0\r\n\r\n")
		originDone <- writeErr
	}()

	proxy, err := OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = proxy.Close() })
	route, err := proxy.register("blog.alice.ard", alphaEarlyResponseOrigin{address: originListener.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = route.Close() })

	browser, err := net.Dial("tcp", proxy.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	_ = browser.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := fmt.Fprint(browser, "POST http://blog.alice.ard/upload HTTP/1.1\r\nHost: blog.alice.ard\r\nTransfer-Encoding: chunked\r\n\r\n1\r\nx\r\n"); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(browser)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodPost})
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("early origin response status = %d, want %d", response.StatusCode, http.StatusRequestEntityTooLarge)
	}
	if !response.Close && response.Header.Get("Connection") != "close" {
		t.Fatal("early origin response left the incomplete browser upload connection reusable")
	}
	if _, err := reader.ReadByte(); err == nil {
		t.Fatal("browser connection remained open after the early origin response")
	}
	select {
	case err := <-originDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("early origin did not finish")
	}
}
