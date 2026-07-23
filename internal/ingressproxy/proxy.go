// Package ingressproxy owns generation-bound forwarding for admitted hosted-service ingress.
// It does not own workload admission or readiness.
package ingressproxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"sync"
	"time"
)

const (
	maxPorts                       = 16
	DefaultMaxConnections          = 128
	DefaultMaxConnectionsPerPort   = 64
	DefaultMaxConnectionsPerSource = 16
	DefaultDialTimeout             = 5 * time.Second
	DefaultIdleTimeout             = 30 * time.Second
	DefaultWriteTimeout            = 10 * time.Second
)

var targetPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,127}$`)

type proxyEvent struct {
	err          error
	listenerDone bool
}

type Config struct {
	Target                  string
	Ports                   []uint16
	DialTimeout             time.Duration
	IdleTimeout             time.Duration
	WriteTimeout            time.Duration
	MaxConnections          int
	MaxConnectionsPerPort   int
	MaxConnectionsPerSource int
	Observe                 func(Event)
}

type EventType string

const (
	EventConnectionAccepted EventType = "connection_accepted"
	EventConnectionRejected EventType = "connection_rejected"
	EventConnectionClosed   EventType = "connection_closed"
)

type EventReason string

const (
	ReasonGlobalLimit       EventReason = "global_limit"
	ReasonPortLimit         EventReason = "port_limit"
	ReasonSourceLimit       EventReason = "source_limit"
	ReasonIdleOrIOTimeout   EventReason = "idle_or_io_timeout"
	ReasonConnectionIOError EventReason = "connection_io_error"
)

type Event struct {
	Type   EventType   `json:"type"`
	Reason EventReason `json:"reason,omitempty"`
	Port   uint16      `json:"port,omitempty"`
	Source string      `json:"source,omitempty"`
}

func DefaultConfig(target string, ports []uint16) Config {
	return Config{
		Target: target, Ports: append([]uint16(nil), ports...),
		DialTimeout: DefaultDialTimeout, IdleTimeout: DefaultIdleTimeout, WriteTimeout: DefaultWriteTimeout,
		MaxConnections: DefaultMaxConnections, MaxConnectionsPerPort: DefaultMaxConnectionsPerPort,
		MaxConnectionsPerSource: DefaultMaxConnectionsPerSource,
	}
}

type listenerBinding struct {
	listener   net.Listener
	targetPort uint16
}

func Run(ctx context.Context, target string, ports []uint16) error {
	config := DefaultConfig(target, ports)
	return RunConfig(ctx, config)
}

func RunConfig(ctx context.Context, config Config) error {
	if err := validate(config); err != nil {
		return err
	}
	bindings, err := openListeners(config.Ports)
	if err != nil {
		return err
	}
	return runWithListeners(ctx, config, bindings)
}

func runWithListeners(ctx context.Context, config Config, bindings []listenerBinding) (returnErr error) {
	if err := validate(config); err != nil {
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, closeBindings(bindings))
	}()
	events := make(chan proxyEvent, len(bindings)+1)
	go func() {
		<-ctx.Done()
		if err := closeBindings(bindings); err != nil {
			events <- proxyEvent{err: err}
		}
	}()
	admission := newAdmissionLimiter(config)
	for _, binding := range bindings {
		go accept(ctx, binding.listener, config, binding.targetPort, admission, events)
	}
	completed := 0
	for completed < len(bindings) {
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

func closeBindings(bindings []listenerBinding) error {
	var failures []error
	for _, binding := range bindings {
		if err := closeListener(binding.listener); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func validate(config Config) error {
	if !targetPattern.MatchString(config.Target) {
		return fmt.Errorf("ingress proxy target is invalid")
	}
	if len(config.Ports) == 0 || len(config.Ports) > maxPorts {
		return fmt.Errorf("ingress proxy port set is invalid")
	}
	if config.DialTimeout <= 0 || config.IdleTimeout <= 0 || config.WriteTimeout <= 0 {
		return fmt.Errorf("ingress proxy deadlines are invalid")
	}
	if config.MaxConnections <= 0 || config.MaxConnectionsPerPort <= 0 ||
		config.MaxConnectionsPerSource <= 0 ||
		config.MaxConnectionsPerPort > config.MaxConnections ||
		config.MaxConnectionsPerSource > config.MaxConnections {
		return fmt.Errorf("ingress proxy connection limits are invalid")
	}
	seen := map[uint16]struct{}{}
	for _, port := range config.Ports {
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

func openListeners(ports []uint16) ([]listenerBinding, error) {
	bindings := make([]listenerBinding, 0, len(ports))
	for _, port := range ports {
		listener, err := net.Listen("tcp", ":"+strconv.Itoa(int(port)))
		if err != nil {
			return nil, errors.Join(fmt.Errorf("open ingress listener: %w", err), closeBindings(bindings))
		}
		bindings = append(bindings, listenerBinding{listener: listener, targetPort: port})
	}
	return bindings, nil
}

func closeListener(listener net.Listener) error {
	if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("close ingress listener: %w", err)
	}
	return nil
}

func accept(
	ctx context.Context,
	listener net.Listener,
	config Config,
	port uint16,
	admission *admissionLimiter,
	events chan<- proxyEvent,
) {
	for {
		connection, err := listener.Accept()
		if err != nil {
			events <- proxyEvent{err: err, listenerDone: true}
			return
		}
		source := sourceHost(connection.RemoteAddr())
		release, reason := admission.acquire(port, source)
		if release == nil {
			observe(config, Event{Type: EventConnectionRejected, Reason: reason, Port: port, Source: source})
			_ = connection.Close()
			continue
		}
		observe(config, Event{Type: EventConnectionAccepted, Port: port, Source: source})
		go func() {
			err := proxyConnection(ctx, config, connection, port, release)
			if err != nil && ctx.Err() == nil {
				observe(config, Event{Type: EventConnectionClosed, Reason: classifyConnectionError(err), Port: port, Source: source})
			}
		}()
	}
}

func proxyConnection(ctx context.Context, config Config, source net.Conn, port uint16, release func()) (returnErr error) {
	defer release()
	defer func() {
		if err := source.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			returnErr = errors.Join(returnErr, fmt.Errorf("close ingress source: %w", err))
		}
	}()
	dialer := net.Dialer{Timeout: config.DialTimeout}
	destination, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(config.Target, strconv.Itoa(int(port))))
	if err != nil {
		return fmt.Errorf("dial ingress destination: %w", err)
	}
	defer func() {
		if err := destination.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			returnErr = errors.Join(returnErr, fmt.Errorf("close ingress destination: %w", err))
		}
	}()
	connectionDone := make(chan struct{})
	defer close(connectionDone)
	go func() {
		select {
		case <-ctx.Done():
			_ = source.Close()
			_ = destination.Close()
		case <-connectionDone:
		}
	}()
	results := make(chan error, 2)
	activity := newConnectionActivity()
	go func() { results <- copyStream(config, source, destination, activity) }()
	go func() { results <- copyStream(config, destination, source, activity) }()
	return errors.Join(<-results, <-results)
}

type connectionActivity struct {
	mu   sync.Mutex
	last time.Time
}

func newConnectionActivity() *connectionActivity {
	return &connectionActivity{last: time.Now()}
}

func (a *connectionActivity) touch() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.last = time.Now()
}

func (a *connectionActivity) deadline(idleTimeout time.Duration) time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.last.Add(idleTimeout)
}

func copyStream(config Config, destination, source net.Conn, activity *connectionActivity) error {
	buffer := make([]byte, 32*1024)
	for {
		if err := source.SetReadDeadline(activity.deadline(config.IdleTimeout)); err != nil {
			return fmt.Errorf("set ingress read deadline: %w", err)
		}
		count, readErr := source.Read(buffer)
		if count > 0 {
			activity.touch()
			if err := destination.SetWriteDeadline(time.Now().Add(config.WriteTimeout)); err != nil {
				return fmt.Errorf("set ingress write deadline: %w", err)
			}
			if _, err := io.CopyN(destination, bytes.NewReader(buffer[:count]), int64(count)); err != nil {
				return fmt.Errorf("copy ingress stream: %w", err)
			}
			activity.touch()
		}
		if readErr == nil {
			continue
		}
		var networkError net.Error
		if errors.As(readErr, &networkError) && networkError.Timeout() &&
			time.Now().Before(activity.deadline(config.IdleTimeout)) {
			continue
		}
		if !errors.Is(readErr, io.EOF) {
			return fmt.Errorf("read ingress stream: %w", readErr)
		}
		break
	}
	if tcp, ok := destination.(*net.TCPConn); ok {
		if err := tcp.CloseWrite(); err != nil {
			return fmt.Errorf("close ingress write stream: %w", err)
		}
	}
	return nil
}

func sourceHost(address net.Addr) string {
	host, _, err := net.SplitHostPort(address.String())
	if err != nil {
		return address.String()
	}
	return host
}

func observe(config Config, event Event) {
	if config.Observe != nil {
		config.Observe(event)
	}
}

func classifyConnectionError(err error) EventReason {
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return ReasonIdleOrIOTimeout
	}
	return ReasonConnectionIOError
}
