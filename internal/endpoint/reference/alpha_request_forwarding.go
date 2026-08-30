package reference

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var errAlphaForwardRequestBodyAborted = errors.New("alpha Reference request body forwarding stopped")

func forwardAlphaRequest(request *http.Request, origin alphaRouteOrigin) (*http.Response, bool, error) {
	transport := &http.Transport{Proxy: nil, DisableCompression: true, ExpectContinueTimeout: time.Second, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", origin.alphaOriginAddress())
	}}
	defer transport.CloseIdleConnections()
	return forwardAlphaRequestWith(request, origin, transport)
}

// alphaForwardRequestBody streams a server-owned request Body through a pipe
// owned by the downstream transport. Closing the pipe satisfies Transport's
// concurrent Read/Close contract without closing the inbound Body. On an early
// response the handler closes the browser connection after its reply; net/http
// then interrupts the source read and remains its sole Body owner.
type alphaForwardRequestBody struct {
	reader         *io.PipeReader
	writer         *io.PipeWriter
	source         io.Reader
	sourceTrailer  http.Header
	forwardTrailer http.Header
	declared       int64
	done           chan struct{}

	mu      sync.Mutex
	started bool
	read    int64
	eof     bool
	closed  bool
	aborted bool
	pumpErr error
}

func newAlphaForwardRequestBody(source io.Reader, declared int64, sourceTrailer, forwardTrailer http.Header) *alphaForwardRequestBody {
	reader, writer := io.Pipe()
	return &alphaForwardRequestBody{reader: reader, writer: writer, source: source, sourceTrailer: sourceTrailer,
		forwardTrailer: forwardTrailer, declared: declared, done: make(chan struct{})}
}

func (body *alphaForwardRequestBody) startLocked() {
	if body.started {
		return
	}
	body.started = true
	go body.pump()
}

func (body *alphaForwardRequestBody) pump() {
	var err error
	if body.declared >= 0 {
		_, err = io.CopyN(body.writer, body.source, body.declared)
		if err == nil {
			var probe [1]byte
			read, probeErr := body.source.Read(probe[:])
			if read != 0 || !errors.Is(probeErr, io.EOF) {
				err = errors.New("alpha Reference request body does not end at its declared length")
			}
		}
	} else {
		_, err = io.Copy(body.writer, body.source)
	}
	if err == nil {
		copyAlphaForwardTrailers(body.forwardTrailer, body.sourceTrailer)
	}
	if err != nil {
		_ = body.writer.CloseWithError(err)
	} else {
		_ = body.writer.Close()
	}
	body.mu.Lock()
	body.pumpErr = err
	close(body.done)
	body.mu.Unlock()
}

func (body *alphaForwardRequestBody) Read(destination []byte) (int, error) {
	body.mu.Lock()
	if !body.closed {
		body.startLocked()
	}
	body.mu.Unlock()
	read, err := body.reader.Read(destination)
	body.mu.Lock()
	body.read += int64(read)
	if errors.Is(err, io.EOF) {
		body.eof = true
	}
	body.mu.Unlock()
	return read, err
}

func (body *alphaForwardRequestBody) Close() error {
	body.mu.Lock()
	if body.closed {
		body.mu.Unlock()
		return nil
	}
	body.closed = true
	if body.declared >= 0 {
		body.aborted = body.read != body.declared
	} else {
		body.aborted = !body.eof
	}
	if !body.aborted && body.declared == 0 {
		body.startLocked()
	}
	aborted := body.aborted
	body.mu.Unlock()
	if aborted {
		return body.reader.CloseWithError(errAlphaForwardRequestBodyAborted)
	}
	return body.reader.Close()
}

func (body *alphaForwardRequestBody) finish() (bool, error) {
	if err := body.Close(); err != nil {
		return true, fmt.Errorf("close alpha Reference transport body: %w", err)
	}
	body.mu.Lock()
	aborted := body.aborted
	body.mu.Unlock()
	if aborted {
		return true, nil
	}
	<-body.done
	body.mu.Lock()
	err := body.pumpErr
	body.mu.Unlock()
	if err != nil {
		return true, fmt.Errorf("read alpha Reference request body: %w", err)
	}
	return false, nil
}

func forwardAlphaRequestWith(request *http.Request, origin alphaRouteOrigin, transport http.RoundTripper) (*http.Response, bool, error) {
	if origin == nil || origin.alphaOriginAddress() == "" || origin.alphaOriginHost(request.URL.Hostname()) == "" {
		return nil, false, errors.New("alpha Reference origin is unavailable")
	}
	if transport == nil {
		return nil, false, errors.New("alpha Reference transport is unavailable")
	}
	forward := request.Clone(request.Context())
	forward.RequestURI = ""
	forward.URL = &url.URL{Scheme: "http", Host: origin.alphaOriginAddress(),
		Path: origin.alphaOriginPath(request.URL.Path), RawQuery: request.URL.RawQuery}
	forward.Host = origin.alphaOriginHost(request.URL.Hostname())
	forward.Header = cloneForwardHeaders(request.Header)
	var ownedBody *alphaForwardRequestBody
	if request.Body != nil && request.Body != http.NoBody {
		ownedBody = newAlphaForwardRequestBody(request.Body, request.ContentLength, request.Trailer, forward.Trailer)
		forward.Body = ownedBody
		forward.GetBody = nil
	}
	response, err := transport.RoundTrip(forward)
	closeInbound := false
	if ownedBody != nil {
		aborted, finishErr := ownedBody.finish()
		closeInbound = aborted
		if finishErr != nil {
			closeAlphaForwardResponse(response)
			return nil, true, finishErr
		}
		if aborted && err == nil && response != nil && response.StatusCode < http.StatusBadRequest {
			closeAlphaForwardResponse(response)
			return nil, true, errors.New("alpha Reference origin returned before consuming the request body")
		}
	}
	if err != nil {
		closeAlphaForwardResponse(response)
		return nil, closeInbound, err
	}
	if response == nil {
		return nil, closeInbound, errors.New("alpha Reference transport returned no response")
	}
	return response, closeInbound, nil
}

func copyAlphaForwardTrailers(destination, source http.Header) {
	for key, values := range source {
		destination[key] = append([]string(nil), values...)
	}
}

func closeAlphaForwardResponse(response *http.Response) {
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
}
