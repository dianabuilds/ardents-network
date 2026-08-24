package reference

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestReferenceServerServesOnlyScopedStaticSite(t *testing.T) {
	running, err := Open(Config{Target: [32]byte{1}, Document: Resource{ContentType: "text/html; charset=utf-8", Body: []byte("<h1>Reference</h1>")},
		Resources: map[string]Resource{"site.css": {ContentType: "text/css", Body: []byte("body{}")}}})
	if err != nil {
		t.Fatal(err)
	}
	defer running.Close()
	response, err := http.Get(running.URL())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || string(body) != "<h1>Reference</h1>" ||
		response.Header.Get("Content-Security-Policy") != contentPolicy || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("document response = status=%d headers=%v body=%q", response.StatusCode, response.Header, body)
	}
	resource, err := http.Get(running.URL() + "site.css")
	if err != nil {
		t.Fatal(err)
	}
	resource.Body.Close()
	if resource.StatusCode != http.StatusOK || resource.Header.Get("Content-Type") != "text/css" {
		t.Fatalf("resource response = status=%d headers=%v", resource.StatusCode, resource.Header)
	}
	for _, suffix := range []string{"missing", "?next=https://example.com"} {
		response, err := http.Get(running.URL() + suffix)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("suffix %q status = %d", suffix, response.StatusCode)
		}
	}
	connection, err := net.Dial("tcp", running.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(connection, "GET http://example.com/ HTTP/1.1\r\nHost: example.com\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	response, err = http.ReadResponse(bufio.NewReader(connection), nil)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	connection.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("proxy-form request status = %d", response.StatusCode)
	}
}

func TestReferenceServerOriginIsFreshAndCloseWithdrawsIt(t *testing.T) {
	first, err := Open(Config{Target: [32]byte{1}, Document: Resource{ContentType: "text/html", Body: []byte("one")}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Open(Config{Target: [32]byte{1}, Document: Resource{ContentType: "text/html", Body: []byte("two")}})
	if err != nil {
		first.Close()
		t.Fatal(err)
	}
	if first.URL() == second.URL() || !strings.Contains(first.URL(), "/site/") {
		first.Close()
		second.Close()
		t.Fatalf("Reference origins are not fresh: %q and %q", first.URL(), second.URL())
	}
	url := first.URL()
	if err := first.Close(); err != nil {
		second.Close()
		t.Fatal(err)
	}
	if _, err := http.Get(url); err == nil {
		second.Close()
		t.Fatal("withdrawn Reference origin remained reachable")
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReferenceServerRejectsInvalidStaticConfiguration(t *testing.T) {
	for _, config := range []Config{
		{},
		{Target: [32]byte{1}, Document: Resource{ContentType: "text/html"}},
		{Target: [32]byte{1}, Document: Resource{ContentType: "text/html", Body: []byte("page")}, Resources: map[string]Resource{"../x": {ContentType: "text/css", Body: []byte("x")}}},
	} {
		if server, err := Open(config); err == nil || server != nil {
			t.Fatalf("invalid Reference configuration result = (%v, %v)", server, err)
		}
	}
}
