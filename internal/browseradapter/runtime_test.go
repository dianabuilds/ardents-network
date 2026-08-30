package browseradapter

import (
	"bufio"
	"context"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/browserentry"
	"github.com/dianabuilds/ardents-network/internal/endpoint"
)

type fixtureApplication struct {
	net.Conn
	done chan endpoint.ApplicationOutcome
}

func (application *fixtureApplication) Done() <-chan endpoint.ApplicationOutcome {
	return application.done
}

func TestRuntimePresentsOnlyNameThroughLocalApplicationInterface(t *testing.T) {
	t.Parallel()
	requested := make(chan string, 1)
	runtime, err := open(t.Context(), Config{ApplicationSocket: "endpoint.sock",
		BrowserEntryStatePath: filepath.Join(t.TempDir(), "browser-entry.json")},
		func(_ context.Context, path, serviceLink string) (applicationStream, error) {
			if path != "endpoint.sock" {
				t.Fatalf("Browser Adapter used Application socket %q", path)
			}
			requested <- serviceLink
			adapter, service := net.Pipe()
			done := make(chan endpoint.ApplicationOutcome, 1)
			go func() {
				request, readErr := http.ReadRequest(bufio.NewReader(service))
				if readErr == nil && request.Host == "reference.ard" {
					_, _ = io.WriteString(service, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
				}
				_ = service.Close()
				done <- endpoint.ApplicationOutcome{Class: "clean service connection close", Reason: "fixture complete"}
				close(done)
			}()
			return &fixtureApplication{Conn: adapter, done: done}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Close() })
	proxyURL, err := url.Parse(runtime.proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	credential := runtime.entry.ProxyCredential()
	proxyURL.User = url.UserPassword(browserentry.ProxyUsername, hex.EncodeToString(credential[:]))
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	response, err := client.Get("http://reference.ard/")
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("Browser Adapter response = %d %q %v", response.StatusCode, body, readErr)
	}
	select {
	case link := <-requested:
		if link != "ardents-alpha://reference" {
			t.Fatalf("Browser Adapter requested %q", link)
		}
	case <-time.After(time.Second):
		t.Fatal("Browser Adapter did not request the typed Service Name")
	}
}
