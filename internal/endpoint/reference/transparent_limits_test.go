package reference_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/reference"
)

const transparentAlphaBodyLimit = 1 << 20

const transparentAlphaHeaderLimit = 16 << 10

type unknownLengthBody struct{ reader io.Reader }

func (body unknownLengthBody) Read(destination []byte) (int, error) {
	return body.reader.Read(destination)
}

func (unknownLengthBody) Close() error { return nil }

func openTransparentLimitRoute(t *testing.T) (net.Conn, *reference.AlphaProxy) {
	t.Helper()
	bridgeSide, applicationSide := net.Pipe()
	t.Cleanup(func() { _ = applicationSide.Close() })
	bridge, err := reference.OpenTransparent(reference.TransparentConfig{
		Target: [32]byte{1}, Hostname: "blog.alice.ard", Connection: bridgeSide,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bridge.Close() })
	proxy, err := reference.OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = proxy.Close() })
	route, err := proxy.RegisterTransparent("blog.alice.ard", bridge)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = route.Close() })
	return applicationSide, proxy
}

func requireSelectedServiceConnectionClosed(t *testing.T, connection net.Conn, subject string) {
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
		timeout, isNetworkError := err.(net.Error)
		if isNetworkError && timeout.Timeout() {
			t.Fatalf("%s left the selected Publisher Service connection open: %v", subject, err)
		}
	case <-time.After(time.Second):
		t.Fatalf("%s did not close the selected Publisher Service connection", subject)
	}
}

func TestTransparentAlphaRouteAcceptsExactKnownBodyLimits(t *testing.T) {
	applicationSide, proxy := openTransparentLimitRoute(t)
	payload := bytes.Repeat([]byte{'x'}, transparentAlphaBodyLimit)
	applicationDone := make(chan error, 1)
	go func() {
		request, err := http.ReadRequest(bufio.NewReader(applicationSide))
		if err == nil {
			body, readErr := io.ReadAll(request.Body)
			_ = request.Body.Close()
			if readErr != nil || !bytes.Equal(body, payload) {
				if readErr != nil {
					err = readErr
				} else {
					err = fmt.Errorf("Publisher received %d exact request bytes, want %d", len(body), len(payload))
				}
			}
		}
		if err == nil {
			_, err = fmt.Fprintf(applicationSide, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\n\r\n", len(payload))
		}
		if err == nil {
			_, err = applicationSide.Write(payload)
		}
		applicationDone <- err
	}()
	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, "http://blog.alice.ard/exact-limit", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	response, err := (&http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableCompression: true}}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || readErr != nil || !bytes.Equal(body, payload) {
		t.Fatalf("exact body-limit response = %d, %d bytes, %v", response.StatusCode, len(body), readErr)
	}
	if err := <-applicationDone; err != nil {
		t.Fatal(err)
	}
}

func TestTransparentAlphaRouteStopsChunkedRequestAtBodyLimit(t *testing.T) {
	applicationSide, proxy := openTransparentLimitRoute(t)

	applicationBytes := make(chan int, 1)
	go func() {
		request, err := http.ReadRequest(bufio.NewReader(applicationSide))
		if err != nil {
			return
		}
		body, bodyErr := io.ReadAll(request.Body)
		_ = request.Body.Close()
		if bodyErr == nil {
			applicationBytes <- -len(body)
			return
		}
		applicationBytes <- len(body)
	}()
	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, "http://blog.alice.ard/upload", unknownLengthBody{reader: bytes.NewReader(make([]byte, transparentAlphaBodyLimit+1))})
	if err != nil {
		t.Fatal(err)
	}
	response, err := (&http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableCompression: true}}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("chunked oversized request status = %d, want %d", response.StatusCode, http.StatusRequestEntityTooLarge)
	}
	select {
	case bytes := <-applicationBytes:
		if bytes != transparentAlphaBodyLimit {
			t.Fatalf("Publisher received %d chunked request bytes, want %d", bytes, transparentAlphaBodyLimit)
		}
	case <-time.After(time.Second):
		t.Fatal("chunked oversized request did not close the selected Publisher Service")
	}
}

func TestTransparentAlphaRouteStopsChunkedResponseAtBodyLimit(t *testing.T) {
	applicationSide, proxy := openTransparentLimitRoute(t)

	applicationDone := make(chan struct{})
	go func() {
		defer close(applicationDone)
		request, err := http.ReadRequest(bufio.NewReader(applicationSide))
		if err != nil {
			return
		}
		_ = request.Body.Close()
		_, _ = fmt.Fprintf(applicationSide, "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n%x\r\n", transparentAlphaBodyLimit+1)
		_, _ = applicationSide.Write(bytes.Repeat([]byte{'x'}, transparentAlphaBodyLimit+1))
		_, _ = io.WriteString(applicationSide, "\r\n0\r\n\r\n")
	}()
	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableCompression: true}}
	response, err := client.Get("http://blog.alice.ard/download")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("chunked oversized response status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if len(body) != transparentAlphaBodyLimit {
		t.Fatalf("chunked oversized response bytes = %d, want %d", len(body), transparentAlphaBodyLimit)
	}
	second, err := client.Get("http://blog.alice.ard/after-limit")
	if err != nil {
		t.Fatal(err)
	}
	_ = second.Body.Close()
	if second.StatusCode != http.StatusBadGateway {
		t.Fatalf("request after chunked response limit status = %d, want %d", second.StatusCode, http.StatusBadGateway)
	}
	select {
	case <-applicationDone:
	case <-time.After(time.Second):
		t.Fatal("oversized Publisher response did not stop after the bridge closed its selected connection")
	}
}

func TestTransparentAlphaRouteRejectsOversizedRequestHeadersBeforeSelectedService(t *testing.T) {
	applicationSide, proxy := openTransparentLimitRoute(t)
	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodGet, "http://blog.alice.ard/header", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Alpha-Limit", strings.Repeat("x", transparentAlphaHeaderLimit+1))
	response, err := (&http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableCompression: true}}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("oversized request header status = %d, want %d", response.StatusCode, http.StatusRequestHeaderFieldsTooLarge)
	}
	if err := applicationSide.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	defer applicationSide.SetReadDeadline(time.Time{})
	if _, err := applicationSide.Read(make([]byte, 1)); err == nil {
		t.Fatal("oversized request headers reached the selected Publisher Service")
	}
}

func TestTransparentAlphaRouteRejectsOversizedResponseHeadersBeforeBrowserHeaders(t *testing.T) {
	applicationSide, proxy := openTransparentLimitRoute(t)
	applicationDone := make(chan struct{})
	go func() {
		defer close(applicationDone)
		request, err := http.ReadRequest(bufio.NewReader(applicationSide))
		if err == nil {
			_ = request.Body.Close()
			_, _ = fmt.Fprintf(applicationSide, "HTTP/1.1 200 OK\r\nX-Alpha-Limit: %s\r\nContent-Length: 0\r\n\r\n", strings.Repeat("x", transparentAlphaHeaderLimit+1))
		}
	}()
	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	response, err := (&http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableCompression: true}}).Get("http://blog.alice.ard/header")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("oversized response header status = %d, want %d", response.StatusCode, http.StatusBadGateway)
	}
	<-applicationDone
	requireSelectedServiceConnectionClosed(t, applicationSide, "oversized response headers")
}
