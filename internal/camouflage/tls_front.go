package camouflage

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type tlsFront struct {
	listener net.Listener
	admit    *admissionListener
	server   *http.Server
	session  *sessionAdmission
	done     chan error
	mu       sync.Mutex
	closed   bool
	closeErr error
}

type admissionListener struct {
	net.Listener
	protected atomic.Bool
	active    atomic.Uint32
	maximum   uint16
}

type sessionAdmission struct {
	active   atomic.Uint32
	accepted atomic.Uint64
	refused  atomic.Uint64
	maximum  uint16
}

func (listener *admissionListener) Accept() (net.Conn, error) {
	for {
		connection, err := listener.Listener.Accept()
		if err != nil {
			return nil, err
		}
		if !listener.protected.Load() && listener.reserve() {
			return &admittedConnection{Conn: connection, release: func() { listener.active.Add(^uint32(0)) }}, nil
		}
		_ = connection.Close()
	}
}

func (listener *admissionListener) reserve() bool {
	for {
		active := listener.active.Load()
		if active >= uint32(listener.maximum) {
			return false
		}
		if listener.active.CompareAndSwap(active, active+1) {
			return true
		}
	}
}

type admittedConnection struct {
	net.Conn
	release func()
	once    sync.Once
}

func (connection *admittedConnection) Close() error {
	err := connection.Conn.Close()
	connection.once.Do(connection.release)
	return err
}

func startTLSFront(config Config, certificate tls.Certificate, backend string, capacity bridgeCapacity) (*tlsFront, error) {
	target, err := url.Parse("http://" + backend)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorLog = nil
	gate := &sessionAdmission{maximum: capacity.sessions}
	proxyHandler := authenticatedSessionHandler(gate, proxy)
	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != config.path || request.URL.RawQuery != "" || request.Host != config.serverName {
			http.NotFound(response, request)
			return
		}
		proxyHandler.ServeHTTP(response, request)
	})
	address := net.JoinHostPort(net.IP(config.address[:]).String(), intString(config.port))
	listener, err := net.Listen("tcp4", address)
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13}
	admit := &admissionListener{Listener: tls.NewListener(listener, tlsConfig), maximum: capacity.rawSockets}
	front := &tlsFront{listener: admit, admit: admit, session: gate, done: make(chan error, 1)}
	front.server = &http.Server{
		Handler: handler, ReadHeaderTimeout: 2 * time.Second, IdleTimeout: 5 * time.Second,
		MaxHeaderBytes: 16 << 10, ErrorLog: log.New(io.Discard, "", 0),
	}
	go func() { front.done <- front.server.Serve(front.listener) }()
	return front, nil
}

func authenticatedSessionHandler(gate *sessionAdmission, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !gate.reserve() {
			http.Error(response, "bridge capacity unavailable", http.StatusServiceUnavailable)
			return
		}
		defer gate.active.Add(^uint32(0))
		next.ServeHTTP(response, request)
	})
}

func (gate *sessionAdmission) reserve() bool {
	for {
		active := gate.active.Load()
		if active >= uint32(gate.maximum) {
			gate.refused.Add(1)
			return false
		}
		if gate.active.CompareAndSwap(active, active+1) {
			gate.accepted.Add(1)
			return true
		}
	}
}

func (gate *sessionAdmission) snapshot() (uint64, uint64, uint64) {
	return uint64(gate.active.Load()), gate.accepted.Load(), gate.refused.Load()
}

func (front *tlsFront) protect(enabled bool) { front.admit.protected.Store(enabled) }

func (front *tlsFront) closeBefore(deadline time.Time) error {
	front.mu.Lock()
	defer front.mu.Unlock()
	if front.closed {
		return front.closeErr
	}
	front.closed = true
	shutdownDeadline := deadline
	if maximum := time.Now().Add(2 * time.Second); maximum.Before(shutdownDeadline) {
		shutdownDeadline = maximum
	}
	ctx, cancel := context.WithDeadline(context.Background(), shutdownDeadline)
	defer cancel()
	shutdownErr := front.server.Shutdown(ctx)
	if shutdownErr != nil {
		shutdownErr = errors.Join(shutdownErr, front.server.Close())
	}
	listenerErr := front.listener.Close()
	if errors.Is(listenerErr, net.ErrClosed) {
		listenerErr = nil
	}
	var serveErr error
	select {
	case serveErr = <-front.done:
		if errors.Is(serveErr, http.ErrServerClosed) || errors.Is(serveErr, net.ErrClosed) {
			serveErr = nil
		}
	case <-ctx.Done():
		serveErr = ctx.Err()
	}
	front.closeErr = errors.Join(shutdownErr, listenerErr, serveErr)
	if !time.Now().Before(deadline) {
		front.closeErr = errors.Join(front.closeErr, errors.New("adapter front missed cleanup deadline"))
	}
	return front.closeErr
}
