package reference

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
)

const maximumServiceResponse = 1 << 20

// HTTPFetcher serializes the closed Reference Site HTTP profile over one
// already-authenticated opaque Service Connection. It never dials a network
// address, forwards browser headers, follows redirects, or exposes cookies.
type HTTPFetcher struct {
	connection io.ReadWriteCloser
	reader     *bufio.Reader
	mu         sync.Mutex
	closed     bool
	done       chan struct{}
}

// NewHTTPFetcher wraps an already-authenticated Service Connection. The caller
// owns its Endpoint lifecycle and must Close the fetcher when that connection
// closes; closing one also closes the other.
func NewHTTPFetcher(connection io.ReadWriteCloser) (*HTTPFetcher, error) {
	if connection == nil {
		return nil, errors.New("reference Site Service Connection is unavailable")
	}
	return &HTTPFetcher{connection: connection, reader: bufio.NewReader(connection), done: make(chan struct{})}, nil
}

// Fetch sends one normalized GET or HEAD request and admits only an exact
// bounded 200 response with a Content-Length and content type. Cancellation
// closes the Service Connection rather than leaving an ambiguous request.
func (fetcher *HTTPFetcher) Fetch(ctx context.Context, input Request) (Response, error) {
	if fetcher == nil || ctx == nil || (input.Method != http.MethodGet && input.Method != http.MethodHead) || !validRemotePath(input.Path) {
		return Response{}, errors.New("reference Site request is invalid")
	}
	fetcher.mu.Lock()
	defer fetcher.mu.Unlock()
	if fetcher.closed {
		return Response{}, errors.New("reference Site Service Connection is closed")
	}
	stop := context.AfterFunc(ctx, func() { _ = fetcher.connection.Close() })
	defer stop()
	request, err := http.NewRequestWithContext(ctx, input.Method, "http://reference.invalid"+input.Path, nil)
	if err != nil {
		return Response{}, err
	}
	request.Host = "reference"
	request.Header = make(http.Header)
	request.Header.Set("Connection", "keep-alive")
	if err := request.Write(fetcher.connection); err != nil {
		_ = fetcher.closeLocked()
		return Response{}, err
	}
	response, err := http.ReadResponse(fetcher.reader, request)
	if err != nil {
		_ = fetcher.closeLocked()
		return Response{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength < 0 || response.ContentLength > maximumServiceResponse ||
		len(response.TransferEncoding) != 0 || response.Header.Get("Content-Type") == "" || response.Header.Get("Location") != "" ||
		len(response.Header.Values("Set-Cookie")) != 0 {
		_ = fetcher.closeLocked()
		return Response{}, errors.New("reference Site response is outside the static profile")
	}
	if input.Method == http.MethodHead {
		return Response{ContentType: response.Header.Get("Content-Type"), Body: []byte{}}, nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maximumServiceResponse+1))
	if err != nil || int64(len(body)) != response.ContentLength || len(body) == 0 || len(body) > maximumServiceResponse {
		_ = fetcher.closeLocked()
		return Response{}, errors.Join(err, errors.New("reference Site response body is invalid"))
	}
	return Response{ContentType: response.Header.Get("Content-Type"), Body: body}, nil
}

// Close closes the underlying Service Connection exactly once.
func (fetcher *HTTPFetcher) Close() error {
	if fetcher == nil {
		return nil
	}
	fetcher.mu.Lock()
	defer fetcher.mu.Unlock()
	if fetcher.closed {
		return nil
	}
	return fetcher.closeLocked()
}

// Done closes when this Service Connection is no longer usable for Reference
// Site requests. A loopback presentation that observes it must withdraw.
func (fetcher *HTTPFetcher) Done() <-chan struct{} {
	if fetcher == nil {
		return nil
	}
	return fetcher.done
}

func (fetcher *HTTPFetcher) closeLocked() error {
	if fetcher.closed {
		return nil
	}
	fetcher.closed = true
	close(fetcher.done)
	return fetcher.connection.Close()
}
