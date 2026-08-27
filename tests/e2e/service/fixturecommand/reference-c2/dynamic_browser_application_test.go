package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeBrowserDynamicCarriesAnOrdinaryBrowserFlow(t *testing.T) {
	server, client := net.Pipe()
	proof := filepath.Join(t.TempDir(), "browser-dynamic-proof")
	finished := make(chan error, 1)
	go func() { finished <- serveBrowserDynamic(server, proof) }()
	defer client.Close()

	reader := bufio.NewReader(client)
	request := func(method, path, cookie, body string) *http.Request {
		header := ""
		if cookie != "" {
			header += "Cookie: " + cookie + "\r\n"
		}
		if body != "" {
			header += "Content-Type: application/x-www-form-urlencoded\r\nContent-Length: " + fmt.Sprint(len(body)) + "\r\n"
		}
		if _, err := fmt.Fprintf(client, "%s %s HTTP/1.1\r\nHost: reference.ard\r\n%s\r\n%s", method, path, header, body); err != nil {
			t.Fatal(err)
		}
		return &http.Request{Method: method}
	}
	first := request(http.MethodGet, "/", "", "")
	response, err := http.ReadResponse(reader, first)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || response.StatusCode != http.StatusOK || response.Header.Get("Set-Cookie") != "before=bridge; Path=/" ||
		!strings.Contains(string(body), "publish?draft=1") {
		t.Fatalf("browser document = status %d headers %#v body %q error %v", response.StatusCode, response.Header, body, err)
	}
	post := request(http.MethodPost, "/publish?draft=1", "before=bridge", "title=ardents")
	response, err = http.ReadResponse(reader, post)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusFound || response.Header.Get("Location") != "/timeline" || response.Header.Get("Set-Cookie") != "session=dynamic; Path=/" {
		t.Fatalf("browser redirect = status %d headers %#v", response.StatusCode, response.Header)
	}
	timeline := request(http.MethodGet, "/timeline", "before=bridge; session=dynamic", "")
	response, err = http.ReadResponse(reader, timeline)
	if err != nil {
		t.Fatal(err)
	}
	body, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || response.StatusCode != http.StatusOK || !strings.Contains(string(body), "location.replace('/close')") {
		t.Fatalf("browser timeline = status %d body %q error %v", response.StatusCode, body, err)
	}
	closeRequest := request(http.MethodGet, "/close", "before=bridge; session=dynamic", "")
	response, err = http.ReadResponse(reader, closeRequest)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("browser close status = %d", response.StatusCode)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if observed, err := os.ReadFile(proof); err != nil || string(observed) != "browser-dynamic-http\n" {
		t.Fatalf("browser proof = %q, %v", observed, err)
	}
}
