package browserreference_test

import (
	"bufio"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/browser/entry"
	reference "github.com/dianabuilds/ardents-network/internal/browser/reference"
)

func TestAlphaProxyForwardsOnlyItsRegisteredHTTPName(t *testing.T) {
	origin, err := reference.Open(reference.Config{Target: [32]byte{1}, Document: reference.Resource{
		ContentType: "text/html", Body: []byte("alpha reference")}})
	if err != nil {
		t.Fatal(err)
	}
	defer origin.Close()
	proxy, err := reference.OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	route, err := proxy.Register("blog.alice.ard", origin)
	if err != nil {
		t.Fatal(err)
	}
	defer route.Close()
	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	response, err := client.Get("http://blog.alice.ard/")
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || string(body) != "alpha reference" {
		t.Fatalf("registered alpha response = %d %q %v", response.StatusCode, body, readErr)
	}

	response, err = client.Get("http://unregistered.ard/")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unregistered alpha status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
	response, err = client.Get("http://ordinary.invalid/")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("ordinary URL proxy status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}

	if err := route.Close(); err != nil {
		t.Fatal(err)
	}
	response, err = client.Get("http://blog.alice.ard/")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("withdrawn alpha status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}
}

func TestAlphaProxyDemandOpensOneExactNameOnce(t *testing.T) {
	proxy, err := reference.OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	var (
		mu       sync.Mutex
		openings int
		origin   *reference.Server
		route    *reference.AlphaRoute
	)
	if err := proxy.SetRouteOpener(func(_ context.Context, hostname string) error {
		if hostname != "blog.alice.ard" {
			return errors.New("unexpected alpha name")
		}
		mu.Lock()
		defer mu.Unlock()
		openings++
		var openErr error
		origin, openErr = reference.Open(reference.Config{Target: [32]byte{1}, Document: reference.Resource{
			ContentType: "text/plain", Body: []byte("opened on demand")}})
		if openErr != nil {
			return openErr
		}
		route, openErr = proxy.Register(hostname, origin)
		if openErr != nil {
			_ = origin.Close()
		}
		return openErr
	}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		mu.Lock()
		defer mu.Unlock()
		if route != nil {
			_ = route.Close()
		}
		if origin != nil {
			_ = origin.Close()
		}
	}()
	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	var requests sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		requests.Add(1)
		go func() {
			defer requests.Done()
			response, requestErr := client.Get("http://blog.alice.ard/")
			if requestErr != nil {
				errs <- requestErr
				return
			}
			body, readErr := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if readErr != nil || response.StatusCode != http.StatusOK || string(body) != "opened on demand" {
				errs <- fmt.Errorf("demand alpha response = %d %q %v", response.StatusCode, body, readErr)
			}
		}()
	}
	requests.Wait()
	close(errs)
	for requestErr := range errs {
		t.Error(requestErr)
	}
	mu.Lock()
	gotOpenings := openings
	mu.Unlock()
	if gotOpenings != 1 {
		t.Fatalf("demand alpha openings = %d, want 1", gotOpenings)
	}
	response, err := client.Get("http://ordinary.invalid/")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("ordinary browser URL status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}
	mu.Lock()
	gotOpenings = openings
	mu.Unlock()
	if gotOpenings != 1 {
		t.Fatalf("ordinary browser URL opened a target: %d", gotOpenings)
	}
}

func TestAlphaProxyRejectsInvalidOrAmbiguousRoutes(t *testing.T) {
	origin, err := reference.Open(reference.Config{Target: [32]byte{1}, Document: reference.Resource{
		ContentType: "text/html", Body: []byte("alpha reference")}})
	if err != nil {
		t.Fatal(err)
	}
	defer origin.Close()
	proxy, err := reference.OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	for _, hostname := range []string{"", "blog.alice.ard:80", "blog.alice.ard.localhost", "Blog.alice.ard", "-blog.ard"} {
		if route, routeErr := proxy.Register(hostname, origin); routeErr == nil || route != nil {
			t.Fatalf("invalid alpha hostname %q registered as (%v, %v)", hostname, route, routeErr)
		}
	}
	first, err := proxy.Register("blog.alice.ard", origin)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if second, secondErr := proxy.Register("blog.alice.ard", origin); secondErr == nil || second != nil {
		t.Fatalf("duplicate alpha hostname registered as (%v, %v)", second, secondErr)
	}
}

func TestAlphaProxyRefusesHTTPSConnectWithoutDialingAService(t *testing.T) {
	proxy, err := reference.OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	connection, err := net.Dial("tcp", proxyURL.Host)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := fmt.Fprintf(connection, "CONNECT blog.alice.ard:443 HTTP/1.1\r\nHost: blog.alice.ard:443\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("HTTPS CONNECT status = %d, want %d", response.StatusCode, http.StatusBadGateway)
	}
}

func TestAlphaProxyBrowserEntryProbeRequiresItsCurrentCapability(t *testing.T) {
	capability := [32]byte{1, 2, 3}
	credential := [32]byte{4, 5, 6}
	proxy, err := reference.OpenAlphaProxyForBrowserEntry(capability, credential)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	request, err := http.NewRequest(http.MethodGet, proxy.URL()+browserentry.ProbePath, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set(browserentry.ProbeHeader, hex.EncodeToString(capability[:]))
	response, err := (&http.Client{Transport: &http.Transport{Proxy: nil}}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("valid Browser Entry probe status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	if response.Header.Get(browserentry.ProbeHeader) != hex.EncodeToString(capability[:]) {
		t.Fatalf("valid Browser Entry response proof = %q, want current capability", response.Header.Get(browserentry.ProbeHeader))
	}
	request.Header.Set(browserentry.ProbeHeader, "wrong")
	response, err = (&http.Client{Transport: &http.Transport{Proxy: nil}}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid Browser Entry probe status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}
}

func TestAlphaProxyForBrowserEntryRequiresItsCurrentCredential(t *testing.T) {
	origin, err := reference.Open(reference.Config{Target: [32]byte{1}, Document: reference.Resource{
		ContentType: "text/html", Body: []byte("alpha reference")}})
	if err != nil {
		t.Fatal(err)
	}
	defer origin.Close()
	credential := [32]byte{4, 5, 6}
	proxy, err := reference.OpenAlphaProxyForBrowserEntry([32]byte{1, 2, 3}, credential)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	route, err := proxy.Register("blog.alice.ard", origin)
	if err != nil {
		t.Fatal(err)
	}
	defer route.Close()

	proxyURL, err := url.Parse(proxy.URL())
	if err != nil {
		t.Fatal(err)
	}
	unauthenticated := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	response, err := unauthenticated.Get("http://blog.alice.ard/")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusProxyAuthRequired || response.Header.Get("Proxy-Authenticate") != `Basic realm="Ardents Browser Entry"` {
		t.Fatalf("unauthenticated alpha response = %d %q, want Basic 407", response.StatusCode, response.Header.Get("Proxy-Authenticate"))
	}

	proxyURL.User = url.UserPassword(browserentry.ProxyUsername, "wrong")
	wrongCredential := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	response, err = wrongCredential.Get("http://blog.alice.ard/")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusProxyAuthRequired {
		t.Fatalf("wrong Browser Entry credential response = %d, want %d", response.StatusCode, http.StatusProxyAuthRequired)
	}

	proxyURL.User = url.UserPassword(browserentry.ProxyUsername, hex.EncodeToString(credential[:]))
	authenticated := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	response, err = authenticated.Get("http://blog.alice.ard/")
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || string(body) != "alpha reference" {
		t.Fatalf("authenticated alpha response = %d %q %v", response.StatusCode, body, readErr)
	}
}

func TestAlphaProxyCloseBoundsAnIncompleteBrowserRequest(t *testing.T) {
	proxy, err := reference.OpenAlphaProxy()
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	connection, err := net.Dial("tcp", proxy.URL()[len("http://"):])
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := fmt.Fprint(connection, "GET http://reference.ard/ HTTP/1.1\r\nHost: reference.ard\r\n"); err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- proxy.Close() }()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("alpha proxy close waited indefinitely for an incomplete browser request")
	}
}
