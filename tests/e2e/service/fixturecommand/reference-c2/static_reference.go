//go:build referencec2

package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

const (
	referenceDocument   = "<!doctype html><title>Reference</title><link rel=\"stylesheet\" href=\"site.css\"><img src=\"mark.svg\" alt=\"Reference\">"
	referenceStylesheet = "body{color:#252525}"
	referenceMark       = "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"1\" height=\"1\"/>"
)

// serveStatic is the Publisher fixture's finite Reference Site source. It
// accepts each declared resource only once, then records exact bounded proof.
func serveStatic(connection net.Conn, proofPath string) error {
	defer connection.Close()
	reader := bufio.NewReader(connection)
	resources := map[string]struct {
		contentType, body string
	}{
		"/":         {contentType: "text/html", body: referenceDocument},
		"/site.css": {contentType: "text/css", body: referenceStylesheet},
		"/mark.svg": {contentType: "image/svg+xml", body: referenceMark},
	}
	served := make(map[string]bool, len(resources))
	for len(served) != len(resources) {
		request, err := http.ReadRequest(reader)
		if err != nil || request.Method != http.MethodGet || request.Host != "reference" || request.URL.RawQuery != "" {
			return errors.New("publisher fixture static request is invalid or incomplete")
		}
		resource, found := resources[request.URL.Path]
		if !found || served[request.URL.Path] {
			return errors.New("publisher fixture static resource is not declared")
		}
		if _, err := fmt.Fprintf(connection, "HTTP/1.1 200 OK\r\nContent-Type: %s\r\nContent-Length: %d\r\nConnection: keep-alive\r\n\r\n%s", resource.contentType, len(resource.body), resource.body); err != nil {
			return err
		}
		served[request.URL.Path] = true
	}
	if err := os.WriteFile(proofPath, []byte("declared-resources\n"), 0o600); err != nil {
		return err
	}
	return nil
}

func waitForResourceProof(deadline time.Time, proofPath string) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if proof, err := os.ReadFile(proofPath); err == nil && string(proof) == "declared-resources\n" {
			return nil
		}
		if !time.Now().UTC().Before(deadline) {
			return errors.New("user C2 fixture did not load every declared Reference Site resource")
		}
		<-ticker.C
	}
}
