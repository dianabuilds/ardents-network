package ingressproxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"sync"
	"time"
)

const (
	maxPorts       = 16
	maxConnections = 128
)

var targetPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,127}$`)

func Run(ctx context.Context, target string, ports []uint16) error {
	if err := validate(target, ports); err != nil {
		return err
	}
	listeners, err := openListeners(ports)
	if err != nil {
		return err
	}
	defer closeListeners(listeners)
	go func() {
		<-ctx.Done()
		closeListeners(listeners)
	}()
	semaphore := make(chan struct{}, maxConnections)
	errors := make(chan error, len(listeners))
	for index, listener := range listeners {
		go accept(ctx, listener, target, ports[index], semaphore, errors)
	}
	for range listeners {
		if err := <-errors; err != nil && ctx.Err() == nil {
			return err
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
			closeListeners(listeners)
			return nil, fmt.Errorf("open ingress listener: %w", err)
		}
		listeners = append(listeners, listener)
	}
	return listeners, nil
}

func closeListeners(listeners []net.Listener) {
	for _, listener := range listeners {
		_ = listener.Close()
	}
}

func accept(ctx context.Context, listener net.Listener, target string, port uint16, semaphore chan struct{}, errors chan<- error) {
	for {
		connection, err := listener.Accept()
		if err != nil {
			errors <- err
			return
		}
		select {
		case semaphore <- struct{}{}:
			go proxyConnection(ctx, connection, target, port, semaphore)
		default:
			_ = connection.Close()
		}
	}
}

func proxyConnection(ctx context.Context, source net.Conn, target string, port uint16, semaphore chan struct{}) {
	defer func() { <-semaphore }()
	defer source.Close()
	dialer := net.Dialer{Timeout: 5 * time.Second}
	destination, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(target, strconv.Itoa(int(port))))
	if err != nil {
		return
	}
	defer destination.Close()
	var wait sync.WaitGroup
	wait.Add(2)
	go copyStream(source, destination, &wait)
	go copyStream(destination, source, &wait)
	wait.Wait()
}

func copyStream(destination, source net.Conn, wait *sync.WaitGroup) {
	defer wait.Done()
	_, _ = io.Copy(destination, source)
	if tcp, ok := destination.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	}
}
