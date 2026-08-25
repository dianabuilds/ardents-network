package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// publisherApplicationListener is the test-only Publisher-local handoff
// listener. It admits one exact loopback application byte stream and carries
// no Route, Target, State, or browser-selection authority.
type publisherApplicationListener struct {
	listener net.Listener
	token    [32]byte
}

func openPublisherApplication(input config) (*publisherApplicationListener, error) {
	token, err := fixed(input.PublisherApplicationToken)
	if err != nil || !validPublisherApplicationAddress(input.PublisherApplicationAddress) {
		return nil, errors.New("publisher Application listener is invalid")
	}
	listener, err := net.Listen("tcp", input.PublisherApplicationAddress)
	if err != nil {
		return nil, err
	}
	return &publisherApplicationListener{listener: listener, token: token}, nil
}

func (listener *publisherApplicationListener) Accept(ctx context.Context) (net.Conn, error) {
	if listener == nil || listener.listener == nil || ctx == nil {
		return nil, errors.New("publisher Application listener is unavailable")
	}
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		return nil, errors.New("publisher Application handoff lacks a deadline")
	}
	if tcp, ok := listener.listener.(*net.TCPListener); ok {
		if err := tcp.SetDeadline(deadline); err != nil {
			return nil, err
		}
	}
	stop := context.AfterFunc(ctx, func() { _ = listener.listener.Close() })
	defer stop()
	connection, err := listener.listener.Accept()
	if err != nil {
		return nil, err
	}
	if connection.SetDeadline(deadline) != nil {
		_ = connection.Close()
		return nil, errors.New("publisher Application handoff deadline is unavailable")
	}
	remote, ok := connection.RemoteAddr().(*net.TCPAddr)
	if !ok || !remote.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		_ = connection.Close()
		return nil, errors.New("publisher Application handoff is not loopback")
	}
	var presented [32]byte
	if _, err := io.ReadFull(connection, presented[:]); err != nil || subtle.ConstantTimeCompare(presented[:], listener.token[:]) != 1 {
		_ = connection.Close()
		return nil, errors.New("publisher Application handoff token is invalid")
	}
	if err := connection.SetDeadline(time.Time{}); err != nil {
		_ = connection.Close()
		return nil, err
	}
	return connection, nil
}

func (listener *publisherApplicationListener) Close() error {
	if listener == nil || listener.listener == nil {
		return nil
	}
	return listener.listener.Close()
}

func runPublisherApplication(input config) error {
	deadline, _ := input.deadline()
	token, _ := fixed(input.PublisherApplicationToken)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", input.PublisherApplicationAddress)
	if err != nil {
		return err
	}
	if err := connection.SetDeadline(deadline); err != nil {
		_ = connection.Close()
		return err
	}
	if _, err := connection.Write(token[:]); err != nil {
		_ = connection.Close()
		return err
	}
	if err := connection.SetDeadline(time.Time{}); err != nil {
		_ = connection.Close()
		return err
	}
	serveStatic(connection, input.ResourceProofPath)
	return json.NewEncoder(os.Stdout).Encode(result{Schema: "ardents-e2e-reference-c2-result-v1", Role: "publisher-app", Class: "served", Passed: true})
}

func validPublisherApplicationAddress(value string) bool {
	host, port, err := net.SplitHostPort(value)
	if err != nil || host != "127.0.0.1" {
		return false
	}
	number, err := strconv.Atoi(port)
	return err == nil && number > 0 && number <= 65535
}

func writePublisherApplicationReady(path string) error {
	if path == "" || filepath.Base(path) == "." || filepath.Base(path) == string(filepath.Separator) {
		return errors.New("publisher Application readiness path is invalid")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.WriteString("ready\n"); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
