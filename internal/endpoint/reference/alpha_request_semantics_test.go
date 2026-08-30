package reference

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestAlphaProxyDoesNotReadExpectContinueBodyBeforeOriginDecision(t *testing.T) {
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
		if _, readErr := http.ReadRequest(bufio.NewReader(connection)); readErr != nil {
			originDone <- readErr
			return
		}
		_, writeErr := io.WriteString(connection, "HTTP/1.1 413 Payload Too Large\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
		originDone <- writeErr
	}()

	proxy := openAlphaForwardTestProxy(t, originListener.Addr().String())
	browser, err := net.Dial("tcp", proxy.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	_ = browser.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := fmt.Fprint(browser, "POST http://blog.alice.ard/upload HTTP/1.1\r\nHost: blog.alice.ard\r\nContent-Length: 8\r\nExpect: 100-continue\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(browser), &http.Request{Method: http.MethodPost})
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("first browser response = %d, want origin's final %d without interim 100", response.StatusCode, http.StatusRequestEntityTooLarge)
	}
	select {
	case err := <-originDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("origin decision did not finish")
	}
}

func TestAlphaProxyForwardsChunkedRequestTrailerValues(t *testing.T) {
	originListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = originListener.Close() })
	type originResult struct {
		body    []byte
		trailer string
		err     error
	}
	originDone := make(chan originResult, 1)
	go func() {
		connection, acceptErr := originListener.Accept()
		if acceptErr != nil {
			originDone <- originResult{err: acceptErr}
			return
		}
		defer connection.Close()
		_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
		request, readErr := http.ReadRequest(bufio.NewReader(connection))
		if readErr != nil {
			originDone <- originResult{err: readErr}
			return
		}
		body, bodyErr := io.ReadAll(request.Body)
		_ = request.Body.Close()
		if bodyErr == nil {
			_, bodyErr = io.WriteString(connection, "HTTP/1.1 204 No Content\r\nContent-Length: 0\r\n\r\n")
		}
		originDone <- originResult{body: body, trailer: request.Trailer.Get("X-Ardents-Proof"), err: bodyErr}
	}()

	proxy := openAlphaForwardTestProxy(t, originListener.Addr().String())
	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableCompression: true}
	t.Cleanup(transport.CloseIdleConnections)
	payload := []byte("trailer-bearing request")
	request, err := http.NewRequest(http.MethodPost, "http://blog.alice.ard/upload", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.ContentLength = -1
	request.Trailer = http.Header{"X-Ardents-Proof": {"exact-proof"}}
	response, err := (&http.Client{Transport: transport}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("trailer request status = %d", response.StatusCode)
	}
	select {
	case result := <-originDone:
		if result.err != nil {
			t.Fatal(result.err)
		}
		if !bytes.Equal(result.body, payload) || result.trailer != "exact-proof" {
			t.Fatalf("origin request = body %q, trailer %q", result.body, result.trailer)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("origin did not receive the chunked request trailer")
	}
}

func openAlphaForwardTestProxy(t *testing.T, originAddress string) *AlphaProxy {
	t.Helper()
	proxy, err := OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = proxy.Close() })
	route, err := proxy.register("blog.alice.ard", alphaEarlyResponseOrigin{address: originAddress})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = route.Close() })
	return proxy
}
