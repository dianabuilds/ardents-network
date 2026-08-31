package browserreference

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

type alphaForwardRoundTrip func(*http.Request) (*http.Response, error)

func (roundTrip alphaForwardRoundTrip) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

type alphaForwardOrigin struct{}

func (alphaForwardOrigin) alphaOriginAddress() string         { return "127.0.0.1:1" }
func (alphaForwardOrigin) alphaOriginHost(host string) string { return host }
func (alphaForwardOrigin) alphaOriginPath(path string) string { return path }

type alphaEOFOwnedBody struct {
	reader          *bytes.Reader
	sawEOF          bool
	closed          bool
	closedBeforeEOF bool
}

func (body *alphaEOFOwnedBody) Read(destination []byte) (int, error) {
	if body.closed {
		return 0, net.ErrClosed
	}
	read, err := body.reader.Read(destination)
	if errors.Is(err, io.EOF) {
		body.sawEOF = true
	}
	return read, err
}

func (body *alphaEOFOwnedBody) Close() error {
	body.closedBeforeEOF = !body.sawEOF
	body.closed = true
	return nil
}

func TestForwardAlphaRequestRetainsInboundBodyUntilTerminalEOF(t *testing.T) {
	payload := []byte("exact known-length body")
	body := &alphaEOFOwnedBody{reader: bytes.NewReader(payload)}
	request := newAlphaForwardTestRequest(t, payload, body)
	transport := alphaForwardRoundTrip(func(forward *http.Request) (*http.Response, error) {
		if forward.Body == request.Body {
			t.Fatal("transport received the server-owned inbound body directly")
		}
		if _, err := io.CopyN(io.Discard, forward.Body, forward.ContentLength); err != nil {
			return nil, err
		}
		if err := forward.Body.Close(); err != nil {
			return nil, err
		}
		return alphaForwardTestResponse(http.StatusOK), nil
	})
	response, closeInbound, err := forwardAlphaRequestWith(request, alphaForwardOrigin{}, transport)
	if err != nil {
		t.Fatal(err)
	}
	if closeInbound {
		t.Fatal("complete request unexpectedly requires the inbound connection to close")
	}
	_ = response.Body.Close()
	requireAlphaInboundBodyOwnership(t, body)
}

func TestForwardAlphaRequestToleratesAsynchronousTransportCloseAfterReturn(t *testing.T) {
	payload := []byte("asynchronous transport close")
	body := &alphaEOFOwnedBody{reader: bytes.NewReader(payload)}
	request := newAlphaForwardTestRequest(t, payload, body)
	releaseClose, transportClosed := make(chan struct{}), make(chan error, 1)
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseClose) }) }
	t.Cleanup(release)
	transport := alphaForwardRoundTrip(func(forward *http.Request) (*http.Response, error) {
		if _, err := io.CopyN(io.Discard, forward.Body, forward.ContentLength); err != nil {
			return nil, err
		}
		go func() {
			<-releaseClose
			transportClosed <- forward.Body.Close()
		}()
		return alphaForwardTestResponse(http.StatusOK), nil
	})
	response, closeInbound, err := forwardAlphaRequestWith(request, alphaForwardOrigin{}, transport)
	if err != nil {
		t.Fatal(err)
	}
	if closeInbound {
		t.Fatal("complete request unexpectedly requires the inbound connection to close")
	}
	_ = response.Body.Close()
	requireAlphaInboundBodyOwnership(t, body)
	release()
	select {
	case err := <-transportClosed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("asynchronous transport Body.Close did not finish")
	}
	if body.closed {
		t.Fatal("asynchronous transport Close reached the server-owned body")
	}
}

type alphaBlockingBody struct {
	readStarted chan struct{}
	closed      chan struct{}
	closeOnce   sync.Once
}

func (body *alphaBlockingBody) Read([]byte) (int, error) {
	select {
	case <-body.readStarted:
	default:
		close(body.readStarted)
	}
	<-body.closed
	return 0, net.ErrClosed
}

func (body *alphaBlockingBody) Close() error {
	body.closeOnce.Do(func() { close(body.closed) })
	return nil
}

func TestForwardAlphaRequestEarlySuccessAbortsBlockedUpload(t *testing.T) {
	body := &alphaBlockingBody{readStarted: make(chan struct{}), closed: make(chan struct{})}
	t.Cleanup(func() { _ = body.Close() })
	request, err := http.NewRequest(http.MethodPost, "http://blog.alice.ard/early", body)
	if err != nil {
		t.Fatal(err)
	}
	request.ContentLength = 8
	transportReadDone := make(chan error, 1)
	responseBody := &alphaCloseObservedBody{Reader: bytes.NewReader(nil)}
	transport := alphaForwardRoundTrip(func(forward *http.Request) (*http.Response, error) {
		go func() {
			_, readErr := forward.Body.Read(make([]byte, 1))
			transportReadDone <- readErr
		}()
		<-body.readStarted
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: responseBody}, nil
	})
	type forwardResult struct {
		closeInbound bool
		err          error
	}
	done := make(chan forwardResult, 1)
	go func() {
		_, closeInbound, forwardErr := forwardAlphaRequestWith(request, alphaForwardOrigin{}, transport)
		done <- forwardResult{closeInbound: closeInbound, err: forwardErr}
	}()
	select {
	case result := <-done:
		if result.err == nil {
			t.Fatal("early successful response accepted an incomplete upload")
		}
		if !result.closeInbound {
			t.Fatal("early successful response did not require the inbound connection to close")
		}
	case <-time.After(time.Second):
		t.Fatal("early successful response deadlocked with a blocked upload")
	}
	select {
	case err := <-transportReadDone:
		if err == nil || errors.Is(err, io.EOF) {
			t.Fatalf("aborted transport read error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("request Body.Close did not unblock the transport Read")
	}
	if !responseBody.closed {
		t.Fatal("rejected early response body was not closed")
	}
	select {
	case <-body.closed:
		t.Fatal("forwarder closed the server-owned inbound body")
	default:
	}
}

func TestForwardAlphaRequestEarlyErrorAbortsUploadAndReturnsErrorResponse(t *testing.T) {
	body := &alphaBlockingBody{readStarted: make(chan struct{}), closed: make(chan struct{})}
	t.Cleanup(func() { _ = body.Close() })
	request, err := http.NewRequest(http.MethodPost, "http://blog.alice.ard/reject", body)
	if err != nil {
		t.Fatal(err)
	}
	request.ContentLength = transparentMaximumBodyBytes + 1
	transport := alphaForwardRoundTrip(func(forward *http.Request) (*http.Response, error) {
		go func() { _, _ = forward.Body.Read(make([]byte, 1)) }()
		<-body.readStarted
		return alphaForwardTestResponse(http.StatusRequestEntityTooLarge), nil
	})
	response, closeInbound, err := forwardAlphaRequestWith(request, alphaForwardOrigin{}, transport)
	if err != nil {
		t.Fatal(err)
	}
	if !closeInbound {
		t.Fatal("early error did not require the inbound connection to close")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("early error status = %d", response.StatusCode)
	}
	select {
	case <-body.closed:
		t.Fatal("forwarder closed the server-owned inbound body")
	default:
	}
}

type alphaCloseObservedBody struct {
	io.Reader
	closed bool
}

func (body *alphaCloseObservedBody) Close() error {
	body.closed = true
	return nil
}

func newAlphaForwardTestRequest(t *testing.T, payload []byte, body io.ReadCloser) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, "http://blog.alice.ard/exact", body)
	if err != nil {
		t.Fatal(err)
	}
	request.ContentLength = int64(len(payload))
	return request
}

func alphaForwardTestResponse(status int) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader([]byte("ok")))}
}

func requireAlphaInboundBodyOwnership(t *testing.T, body *alphaEOFOwnedBody) {
	t.Helper()
	if !body.sawEOF || body.closed || body.closedBeforeEOF {
		t.Fatalf("inbound body ownership = saw EOF %t, closed %t, closed before EOF %t", body.sawEOF, body.closed, body.closedBeforeEOF)
	}
}
