package camouflage

import (
	"context"
	"crypto/tls"
	"errors"
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
	done     chan error
	mu       sync.Mutex
	closed   bool
	closeErr error
}

type admissionListener struct {
	net.Listener
	protected atomic.Bool
}

func (listener *admissionListener) Accept() (net.Conn, error) {
	for {
		connection, err := listener.Listener.Accept()
		if err != nil {
			return nil, err
		}
		if !listener.protected.Load() {
			return connection, nil
		}
		_ = connection.Close()
	}
}

func startTLSFront(config Config, certificate tls.Certificate, backend string) (*tlsFront, error) {
	target, err := url.Parse("http://" + backend)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorLog = nil
	handler := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != config.path || request.URL.RawQuery != "" || request.Host != config.serverName {
			http.NotFound(response, request)
			return
		}
		proxy.ServeHTTP(response, request)
	})
	address := net.JoinHostPort(net.IP(config.address[:]).String(), intString(config.port))
	listener, err := net.Listen("tcp4", address)
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13}
	admit := &admissionListener{Listener: tls.NewListener(listener, tlsConfig)}
	front := &tlsFront{listener: admit, admit: admit, done: make(chan error, 1)}
	front.server = &http.Server{
		Handler: handler, ReadHeaderTimeout: 2 * time.Second, IdleTimeout: 5 * time.Second,
		MaxHeaderBytes: 16 << 10,
	}
	go func() { front.done <- front.server.Serve(front.listener) }()
	return front, nil
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
