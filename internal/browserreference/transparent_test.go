package browserreference_test

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	reference "github.com/dianabuilds/ardents-network/internal/browserreference"
)

func TestTransparentAlphaRoutePreservesOneDynamicServiceHTTPFlow(t *testing.T) {
	bridgeSide, applicationSide := net.Pipe()
	defer applicationSide.Close()
	bridge, err := reference.OpenTransparent(reference.TransparentConfig{
		Target: [32]byte{1}, Hostname: "blog.alice.ard", Connection: bridgeSide,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.Close()
	proxy, err := reference.OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	route, err := proxy.RegisterTransparent("blog.alice.ard", bridge)
	if err != nil {
		t.Fatal(err)
	}
	defer route.Close()

	firstChunkWritten := make(chan struct{})
	finishStream := make(chan struct{})
	applicationDone := make(chan error, 1)
	go func() {
		applicationDone <- serveTransparentFixture(applicationSide, firstChunkWritten, finishStream)
	}()

	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableCompression: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	post, err := http.NewRequest(http.MethodPost, "http://blog.alice.ard/submit?draft=1", bytes.NewBufferString("title=alpha"))
	if err != nil {
		t.Fatal(err)
	}
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.Header.Set("X-Publisher-Feature", "save")
	response, err := client.Do(post)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusFound || response.Header.Get("Location") != "/next" || response.Header.Get("Set-Cookie") != "session=alpha; Path=/" {
		t.Fatalf("POST response = %d %#v", response.StatusCode, response.Header)
	}

	get, err := http.NewRequest(http.MethodGet, "http://blog.alice.ard/next", nil)
	if err != nil {
		t.Fatal(err)
	}
	get.Header.Set("Cookie", "session=alpha")
	streamResponse := make(chan struct {
		response *http.Response
		err      error
	}, 1)
	go func() {
		value, requestErr := client.Do(get)
		streamResponse <- struct {
			response *http.Response
			err      error
		}{response: value, err: requestErr}
	}()
	select {
	case <-firstChunkWritten:
	case <-time.After(time.Second):
		t.Fatal("Publisher did not begin its streamed response")
	}
	var streamed struct {
		response *http.Response
		err      error
	}
	select {
	case streamed = <-streamResponse:
	case <-time.After(time.Second):
		t.Fatal("bridge buffered a streamed response before exposing its headers")
	}
	if streamed.err != nil {
		t.Fatal(streamed.err)
	}
	defer streamed.response.Body.Close()
	first := make([]byte, len("first-"))
	if _, err := io.ReadFull(streamed.response.Body, first); err != nil || string(first) != "first-" {
		t.Fatalf("first streamed response chunk = %q, %v", first, err)
	}
	close(finishStream)
	rest, err := io.ReadAll(streamed.response.Body)
	if err != nil || string(rest) != "second" {
		t.Fatalf("remaining streamed response = %q, %v", rest, err)
	}
	if err := <-applicationDone; err != nil {
		t.Fatal(err)
	}
}

func TestTransparentAlphaRouteFailureNeverSelectsAnotherRegisteredTarget(t *testing.T) {
	failedBridge, failedApplication := net.Pipe()
	decoyBridge, decoyApplication := net.Pipe()
	failed, err := reference.OpenTransparent(reference.TransparentConfig{
		Target: [32]byte{1}, Hostname: "failed.ard", Connection: failedBridge,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer failed.Close()
	decoy, err := reference.OpenTransparent(reference.TransparentConfig{
		Target: [32]byte{2}, Hostname: "decoy.ard", Connection: decoyBridge,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer decoy.Close()
	proxy, err := reference.OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	failedRoute, err := proxy.RegisterTransparent("failed.ard", failed)
	if err != nil {
		t.Fatal(err)
	}
	defer failedRoute.Close()
	decoyRoute, err := proxy.RegisterTransparent("decoy.ard", decoy)
	if err != nil {
		t.Fatal(err)
	}
	defer decoyRoute.Close()

	go func() {
		_, _ = http.ReadRequest(bufio.NewReader(failedApplication))
		_ = failedApplication.Close()
	}()
	decoyRequest := make(chan *http.Request, 1)
	go func() {
		request, readErr := http.ReadRequest(bufio.NewReader(decoyApplication))
		if readErr == nil {
			decoyRequest <- request
		}
	}()
	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: time.Second}
	response, err := client.Get("http://failed.ard/")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("failed Target status = %d, want 502", response.StatusCode)
	}
	select {
	case request := <-decoyRequest:
		t.Fatalf("failed Target request fell back to registered decoy Target: %s", request.URL)
	case <-time.After(100 * time.Millisecond):
	}

	responseDone := make(chan error, 1)
	go func() {
		request := <-decoyRequest
		_ = request.Body.Close()
		_, writeErr := io.WriteString(decoyApplication, "HTTP/1.1 204 No Content\r\nContent-Length: 0\r\n\r\n")
		responseDone <- writeErr
	}()
	response, err = client.Get("http://decoy.ard/")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("explicit decoy Target status = %d, want 204", response.StatusCode)
	}
	if err := <-responseDone; err != nil {
		t.Fatal(err)
	}
	_ = decoyApplication.Close()
}

func TestTransparentAlphaRouteRejectsUpgradeBeforeTheSelectedService(t *testing.T) {
	bridgeSide, applicationSide := net.Pipe()
	defer applicationSide.Close()
	bridge, err := reference.OpenTransparent(reference.TransparentConfig{
		Target: [32]byte{1}, Hostname: "blog.alice.ard", Connection: bridgeSide,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.Close()
	proxy, err := reference.OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	route, err := proxy.RegisterTransparent("blog.alice.ard", bridge)
	if err != nil {
		t.Fatal(err)
	}
	defer route.Close()
	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodGet, "http://blog.alice.ard/", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	response, err := (&http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("transparent upgrade status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}
	if err := applicationSide.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	defer applicationSide.SetReadDeadline(time.Time{})
	buffer := make([]byte, 1)
	if _, err := applicationSide.Read(buffer); err == nil {
		t.Fatal("transparent upgrade reached the selected Publisher Service")
	}
}

func TestTransparentAlphaRouteRejectsOversizedKnownRequestBeforeSelectedService(t *testing.T) {
	bridgeSide, applicationSide := net.Pipe()
	defer applicationSide.Close()
	bridge, err := reference.OpenTransparent(reference.TransparentConfig{
		Target: [32]byte{1}, Hostname: "blog.alice.ard", Connection: bridgeSide,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.Close()
	proxy, err := reference.OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	route, err := proxy.RegisterTransparent("blog.alice.ard", bridge)
	if err != nil {
		t.Fatal(err)
	}
	defer route.Close()

	requestReachedApplication := make(chan struct{}, 1)
	go func() {
		if err := applicationSide.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
			return
		}
		defer applicationSide.SetReadDeadline(time.Time{})
		one := make([]byte, 1)
		if _, err := applicationSide.Read(one); err == nil {
			requestReachedApplication <- struct{}{}
		}
	}()
	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, "http://blog.alice.ard/upload", bytes.NewReader(make([]byte, (1<<20)+1)))
	if err != nil {
		t.Fatal(err)
	}
	response, err := (&http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableCompression: true}}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized request status = %d, want %d", response.StatusCode, http.StatusRequestEntityTooLarge)
	}
	select {
	case <-requestReachedApplication:
		t.Fatal("oversized request reached the selected Publisher Service")
	case <-time.After(250 * time.Millisecond):
	}
}

func TestTransparentAlphaRouteRejectsOversizedKnownResponseBeforeBrowserHeaders(t *testing.T) {
	bridgeSide, applicationSide := net.Pipe()
	defer applicationSide.Close()
	bridge, err := reference.OpenTransparent(reference.TransparentConfig{
		Target: [32]byte{1}, Hostname: "blog.alice.ard", Connection: bridgeSide,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer bridge.Close()
	proxy, err := reference.OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	route, err := proxy.RegisterTransparent("blog.alice.ard", bridge)
	if err != nil {
		t.Fatal(err)
	}
	defer route.Close()

	applicationDone := make(chan struct{})
	go func() {
		defer close(applicationDone)
		request, err := http.ReadRequest(bufio.NewReader(applicationSide))
		if err == nil {
			_ = request.Body.Close()
			_, _ = io.WriteString(applicationSide, "HTTP/1.1 200 OK\r\nContent-Length: 1048577\r\n\r\n")
		}
	}()
	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	response, err := (&http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableCompression: true}}).Get("http://blog.alice.ard/download")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("oversized response status = %d, want %d", response.StatusCode, http.StatusBadGateway)
	}
	<-applicationDone
	requireSelectedServiceConnectionClosed(t, applicationSide, "oversized response body")
}

func serveTransparentFixture(connection net.Conn, firstChunkWritten chan<- struct{}, finishStream <-chan struct{}) error {
	reader := bufio.NewReader(connection)
	post, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(post.Body)
	_ = post.Body.Close()
	if err != nil {
		return err
	}
	if post.Method != http.MethodPost || post.Host != "blog.alice.ard" || post.URL.String() != "/submit?draft=1" ||
		string(body) != "title=alpha" || post.Header.Get("Content-Type") != "application/x-www-form-urlencoded" ||
		post.Header.Get("X-Publisher-Feature") != "save" {
		return errors.New("bridge changed the Publisher request")
	}
	if _, err := io.WriteString(connection, "HTTP/1.1 302 Found\r\nLocation: /next\r\nSet-Cookie: session=alpha; Path=/\r\nContent-Length: 0\r\n\r\n"); err != nil {
		return err
	}
	get, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	_ = get.Body.Close()
	if get.Method != http.MethodGet || get.Host != "blog.alice.ard" || get.URL.String() != "/next" || get.Header.Get("Cookie") != "session=alpha" {
		return errors.New("bridge changed the follow-up Publisher request")
	}
	if _, err := io.WriteString(connection, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n\r\n6\r\nfirst-\r\n"); err != nil {
		return err
	}
	close(firstChunkWritten)
	<-finishStream
	_, err = io.WriteString(connection, "6\r\nsecond\r\n0\r\n\r\n")
	return err
}
