//go:build referencec2

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

// serveDynamic is an unmodified-in-shape HTTP application workload for the
// H4-3B tracer. It is deliberately unaware of Ardents: it expects its normal
// Host, request body, cookie, redirect, and streamed response semantics.
func serveDynamic(connection net.Conn, proofPath string) error {
	defer connection.Close()
	reader := bufio.NewReader(connection)
	if err := acceptDynamicPublishAndTimeline(connection, reader); err != nil {
		return err
	}
	if _, err := fmt.Fprint(connection, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n\r\n6\r\nfirst-\r\n"); err != nil {
		return err
	}
	time.Sleep(25 * time.Millisecond)
	if _, err := fmt.Fprint(connection, "6\r\nsecond\r\n0\r\n\r\n"); err != nil {
		return err
	}
	closeRequest, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	_ = closeRequest.Body.Close()
	if closeRequest.Method != http.MethodGet || closeRequest.Host != "reference.ard" || closeRequest.URL.String() != "/close" {
		return errors.New("dynamic Publisher close request was changed or incomplete")
	}
	if _, err := fmt.Fprint(connection, "HTTP/1.1 204 No Content\r\nContent-Length: 0\r\n\r\n"); err != nil {
		return err
	}
	return os.WriteFile(proofPath, []byte("dynamic-http\n"), 0o600)
}

// serveDynamicApplicationCrash preserves one ordinary request and redirect,
// then resets the Publisher-local socket after committing a partial streamed
// response. The reset is the fixture's explicit observable for an abrupt local
// Application failure; an orderly EOF remains the normal dynamic scenario.
func serveDynamicApplicationCrash(connection net.Conn, proofPath string) error {
	reader := bufio.NewReader(connection)
	if err := acceptDynamicPublishAndTimeline(connection, reader); err != nil {
		return err
	}
	if _, err := fmt.Fprint(connection, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n\r\n6\r\nfirst-\r\n"); err != nil {
		return err
	}
	if err := os.WriteFile(proofPath, []byte("dynamic-application-crash\n"), 0o600); err != nil {
		return err
	}
	if tcp, ok := connection.(*net.TCPConn); ok {
		_ = tcp.SetLinger(0)
	}
	_ = connection.Close()
	return errors.New("simulated Publisher Application crash after partial response")
}

// serveDynamicUntilPublisherEndpointCrash completes the ordinary dynamic
// workload, then holds one request while the process harness hard-stops the
// Publisher Endpoint. The local Application learns only that its admitted
// handoff disappeared; it receives no Route or crash-control authority.
func serveDynamicUntilPublisherEndpointCrash(connection net.Conn, proofPath, crashReadyPath string) error {
	defer connection.Close()
	reader := bufio.NewReader(connection)
	if err := acceptDynamicPublishAndTimeline(connection, reader); err != nil {
		return err
	}
	if _, err := fmt.Fprint(connection, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nTransfer-Encoding: chunked\r\n\r\n6\r\nfirst-\r\n6\r\nsecond\r\n0\r\n\r\n"); err != nil {
		return err
	}
	if err := os.WriteFile(proofPath, []byte("dynamic-endpoint-crash\n"), 0o600); err != nil {
		return err
	}
	crashRequest, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	_ = crashRequest.Body.Close()
	if crashRequest.Method != http.MethodGet || crashRequest.Host != "reference.ard" || crashRequest.URL.String() != "/crash" {
		return errors.New("dynamic Publisher crash request was changed or incomplete")
	}
	if err := os.WriteFile(crashReadyPath, []byte("ready\n"), 0o600); err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, reader)
	return errors.New("simulated Publisher Endpoint crash closed the local Application handoff")
}

func acceptDynamicPublishAndTimeline(connection net.Conn, reader *bufio.Reader) error {
	post, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	body, err := io.ReadAll(post.Body)
	_ = post.Body.Close()
	if err != nil || post.Method != http.MethodPost || post.Host != "reference.ard" || post.URL.String() != "/publish?draft=1" ||
		string(body) != "title=ardents" || post.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || post.Header.Get("Cookie") != "before=bridge" {
		return errors.New("dynamic Publisher request was changed or incomplete")
	}
	if _, err := fmt.Fprint(connection, "HTTP/1.1 302 Found\r\nLocation: /timeline\r\nSet-Cookie: session=dynamic; Path=/\r\nContent-Length: 0\r\n\r\n"); err != nil {
		return err
	}
	get, err := http.ReadRequest(reader)
	if err != nil {
		return err
	}
	_ = get.Body.Close()
	if get.Method != http.MethodGet || get.Host != "reference.ard" || get.URL.String() != "/timeline" || get.Header.Get("Cookie") != "session=dynamic" {
		return errors.New("dynamic Publisher follow-up request was changed or incomplete")
	}
	return nil
}

func waitForDynamicProof(deadline time.Time, proofPath, expected string) error {
	if expected == "" {
		return errors.New("dynamic Publisher proof expectation is unavailable")
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if proof, err := os.ReadFile(proofPath); err == nil && string(proof) == expected {
			return nil
		}
		if !time.Now().UTC().Before(deadline) {
			return errors.New("user C2 fixture did not complete the dynamic Publisher HTTP flow")
		}
		<-ticker.C
	}
}

func dynamicProofForPublisherTerminal(terminal publisherTerminal, fault transitFault) string {
	if fault != "" {
		return "dynamic-" + string(fault) + "\n"
	}
	switch terminal {
	case publisherTerminalApplicationReset:
		return "dynamic-application-crash\n"
	case publisherTerminalEndpointStop:
		return "dynamic-endpoint-crash\n"
	default:
		return "dynamic-http\n"
	}
}
