package reference

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestLiveReferenceServerUsesOnlyDeclaredRoutes(t *testing.T) {
	fetcher := &recordingFetcher{responses: map[string]Response{
		"/":         {ContentType: "text/html", Body: []byte("<h1>Reference</h1>")},
		"/site.css": {ContentType: "text/css", Body: []byte("body{}")},
	}}
	running, err := OpenLive(LiveConfig{Target: [32]byte{1}, Fetcher: fetcher, Routes: map[string]string{"": "/", "site.css": "/site.css"}})
	if err != nil {
		t.Fatal(err)
	}
	defer running.Close()
	for _, suffix := range []string{"", "site.css"} {
		response, err := http.Get(running.URL() + suffix)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil || response.StatusCode != http.StatusOK || len(body) == 0 || response.Header.Get("Content-Security-Policy") != contentPolicy {
			t.Fatalf("live response %q = %d %q %v", suffix, response.StatusCode, body, readErr)
		}
	}
	response, err := http.Get(running.URL() + "missing")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound || fetcher.count() != 2 {
		t.Fatalf("undeclared resource result = %d, fetches=%d", response.StatusCode, fetcher.count())
	}
	response, err = http.Get(running.URL() + "?x=1")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound || fetcher.count() != 2 {
		t.Fatalf("query resource result = %d, fetches=%d", response.StatusCode, fetcher.count())
	}
}

func TestHTTPFetcherForwardsOnlyNormalizedStaticRequest(t *testing.T) {
	client, service := net.Pipe()
	defer service.Close()
	fetcher, err := NewHTTPFetcher(client)
	if err != nil {
		t.Fatal(err)
	}
	defer fetcher.Close()
	requestSeen := make(chan *http.Request, 1)
	go func() {
		request, readErr := http.ReadRequest(bufio.NewReader(service))
		if readErr != nil {
			requestSeen <- nil
			return
		}
		requestSeen <- request
		_, _ = io.WriteString(service, "HTTP/1.1 200 OK\r\nContent-Type: text/css\r\nContent-Length: 6\r\n\r\nbody{}")
	}()
	response, err := fetcher.Fetch(context.Background(), Request{Method: http.MethodGet, Path: "/site.css"})
	if err != nil || response.ContentType != "text/css" || string(response.Body) != "body{}" {
		t.Fatalf("Fetch = %+v, %v", response, err)
	}
	request := <-requestSeen
	if request == nil || request.Method != http.MethodGet || request.URL.Path != "/site.css" || request.Host != "reference" ||
		request.Header.Get("Cookie") != "" || request.Header.Get("Referer") != "" {
		t.Fatalf("service request = %#v", request)
	}
}

func TestLiveReferenceServerServesRepeatedResourceOverOneServiceConnection(t *testing.T) {
	client, service := net.Pipe()
	fetcher, err := NewHTTPFetcher(client)
	if err != nil {
		t.Fatal(err)
	}
	defer fetcher.Close()
	running, err := OpenLive(LiveConfig{Target: [32]byte{1}, Fetcher: fetcher,
		Routes: map[string]string{"": "/", "visual.svg": "/visual.svg"}})
	if err != nil {
		t.Fatal(err)
	}
	defer running.Close()
	paths := make(chan []string, 1)
	go func() {
		reader := bufio.NewReader(service)
		var received []string
		for range 3 {
			request, readErr := http.ReadRequest(reader)
			if readErr != nil {
				paths <- nil
				return
			}
			received = append(received, request.URL.Path)
			body, contentType := "<h1>Reference</h1>", "text/html"
			if request.URL.Path == "/visual.svg" {
				body, contentType = "<svg></svg>", "image/svg+xml"
			}
			if _, writeErr := io.WriteString(service, "HTTP/1.1 200 OK\r\nContent-Type: "+contentType+"\r\nContent-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"+body); writeErr != nil {
				paths <- nil
				return
			}
		}
		paths <- received
	}()
	httpClient := &http.Client{Transport: &http.Transport{Proxy: nil}}
	for _, suffix := range []string{"", "visual.svg", "visual.svg"} {
		response, requestErr := httpClient.Get(running.URL() + suffix)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("Reference resource %q status = %d", suffix, response.StatusCode)
		}
	}
	if got := <-paths; !slices.Equal(got, []string{"/", "/visual.svg", "/visual.svg"}) {
		t.Fatalf("Service requests = %q", got)
	}
}

func TestHTTPFetcherRejectsRedirectCookieAndUnknownLength(t *testing.T) {
	for _, raw := range []string{
		"HTTP/1.1 302 Found\r\nContent-Type: text/html\r\nContent-Length: 1\r\nLocation: /other\r\n\r\nx",
		"HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: 1\r\nSet-Cookie: x=y\r\n\r\nx",
		"HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nTransfer-Encoding: chunked\r\n\r\n1\r\nx\r\n0\r\n\r\n",
	} {
		client, service := net.Pipe()
		fetcher, err := NewHTTPFetcher(client)
		if err != nil {
			t.Fatal(err)
		}
		go func(response string) {
			_, _ = http.ReadRequest(bufio.NewReader(service))
			_, _ = io.WriteString(service, response)
			_ = service.Close()
		}(raw)
		if response, err := fetcher.Fetch(context.Background(), Request{Method: http.MethodGet, Path: "/"}); err == nil || response.ContentType != "" || len(response.Body) != 0 {
			t.Fatalf("forbidden response accepted: %+v, %v", response, err)
		}
		_ = fetcher.Close()
	}
}

func TestLiveReferenceOriginWithdrawsWhenServiceConnectionCloses(t *testing.T) {
	client, service := net.Pipe()
	defer service.Close()
	fetcher, err := NewHTTPFetcher(client)
	if err != nil {
		t.Fatal(err)
	}
	running, err := OpenLive(LiveConfig{Target: [32]byte{1}, Fetcher: fetcher, Routes: map[string]string{"": "/"}})
	if err != nil {
		_ = fetcher.Close()
		t.Fatal(err)
	}
	url := running.URL()
	if err := fetcher.Close(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := http.Get(url); err != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = running.Close()
	t.Fatal("Reference origin remained reachable after its Service Connection closed")
}

type recordingFetcher struct {
	mu        sync.Mutex
	responses map[string]Response
	requests  []Request
}

func (fetcher *recordingFetcher) Fetch(_ context.Context, request Request) (Response, error) {
	fetcher.mu.Lock()
	defer fetcher.mu.Unlock()
	fetcher.requests = append(fetcher.requests, request)
	response, found := fetcher.responses[request.Path]
	if !found {
		return Response{}, errors.New("unexpected Reference Site request")
	}
	return response, nil
}

func (fetcher *recordingFetcher) count() int {
	fetcher.mu.Lock()
	defer fetcher.mu.Unlock()
	return len(fetcher.requests)
}
