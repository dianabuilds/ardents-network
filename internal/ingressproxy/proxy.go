// Package ingressproxy owns generation-bound forwarding for admitted hosted-service ingress.
// It does not own workload admission or readiness.
package ingressproxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"time"
)

const (
	maxPorts       = 16
	maxConnections = 128
)

var targetPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,127}$`)

type proxyEvent struct {
	err          error
	listenerDone bool
}

func Run(ctx context.Context, target string, ports []uint16) (returnErr error) {
	if err := validate(target, ports); err != nil {
		return err
	}
	listeners, err := openListeners(ports)
	if err != nil {
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, closeListeners(listeners))
	}()
	events := make(chan proxyEvent, maxConnections+len(listeners))
	go func() {
		<-ctx.Done()
		if err := closeListeners(listeners); err != nil {
			events <- proxyEvent{err: err}
		}
	}()
	semaphore := make(chan struct{}, maxConnections)
	for index, listener := range listeners {
		go accept(ctx, listener, target, ports[index], semaphore, events)
	}
	completed := 0
	for completed < len(listeners) {
		event := <-events
		if event.listenerDone {
			completed++
		}
		if event.err != nil && ctx.Err() == nil {
			return event.err
		}
	}
	return ctx.Err()
}

func validate(target string, ports []uint16) error {
	if !targetPattern.MatchString(target) {
		return fmt.Errorf("ingress proxy target is invalid")
	}
	if len(ports) == 0 || len(ports) > maxPorts {
		return fmt.Errorf("ingress proxy port set is invalid")
	}
	seen := map[uint16]struct{}{}
	for _, port := range ports {
		if port < 1024 {
			return fmt.Errorf("ingress proxy port is privileged")
		}
		if _, duplicate := seen[port]; duplicate {
			return fmt.Errorf("ingress proxy port is duplicated")
		}
		seen[port] = struct{}{}
	}
	return nil
}

func openListeners(ports []uint16) ([]net.Listener, error) {
	listeners := make([]net.Listener, 0, len(ports))
	for _, port := range ports {
		listener, err := net.Listen("tcp", ":"+strconv.Itoa(int(port)))
		if err != nil {
			return nil, errors.Join(fmt.Errorf("open ingress listener: %w", err), closeListeners(listeners))
		}
		listeners = append(listeners, listener)
	}
	return listeners, nil
}

func closeListeners(listeners []net.Listener) error {
	var failures []error
	for _, listener := range listeners {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			failures = append(failures, fmt.Errorf("close ingress listener: %w", err))
		}
	}
	return errors.Join(failures...)
}

func accept(ctx context.Context, listener net.Listener, target string, port uint16, semaphore chan struct{}, events chan<- proxyEvent) {
	for {
		connection, err := listener.Accept()
		if err != nil {
			events <- proxyEvent{err: err, listenerDone: true}
			return
		}
		select {
		case semaphore <- struct{}{}:
			go func() {
				if err := proxyConnection(ctx, connection, target, port, semaphore); err != nil {
					events <- proxyEvent{err: err}
				}
			}()
		default:
			if err := connection.Close(); err != nil {
				events <- proxyEvent{err: fmt.Errorf("reject excess ingress connection: %w", err)}
			}
		}
	}
}

func proxyConnection(ctx context.Context, source net.Conn, target string, port uint16, semaphore chan struct{}) (returnErr error) {
	defer func() { <-semaphore }()
	defer func() {
		if err := source.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			returnErr = errors.Join(returnErr, fmt.Errorf("close ingress source: %w", err))
		}
	}()
	dialer := net.Dialer{Timeout: 5 * time.Second}
	destination, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(target, strconv.Itoa(int(port))))
	if err != nil {
		return fmt.Errorf("dial ingress destination: %w", err)
	}
	defer func() {
		if err := destination.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			returnErr = errors.Join(returnErr, fmt.Errorf("close ingress destination: %w", err))
		}
	}()
	results := make(chan error, 2)
	go func() { results <- copyStream(source, destination) }()
	go func() { results <- copyStream(destination, source) }()
	return errors.Join(<-results, <-results)
}

func copyStream(destination, source net.Conn) error {
	if _, err := io.Copy(destination, source); err != nil {
		return fmt.Errorf("copy ingress stream: %w", err)
	}
	if tcp, ok := destination.(*net.TCPConn); ok {
		if err := tcp.CloseWrite(); err != nil {
			return fmt.Errorf("close ingress write stream: %w", err)
		}
	}
	return nil
}
