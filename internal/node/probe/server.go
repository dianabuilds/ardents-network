package probe

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type service struct {
	plan        *Plan
	duty        Duty
	listener    net.Listener
	active      chan struct{}
	open        chan struct{}
	mu          sync.Mutex
	connections map[net.Conn]struct{}
	nonces      [512][32]byte
	nonceCount  int
	nonceNext   int
	stop        chan struct{}
	stopOnce    sync.Once
	protected   atomic.Bool
	terminal    chan error
	work        sync.WaitGroup
}

// Start binds the plan to one authenticated duty.
func (p *Plan) Start(duty Duty) (*Server, error) {
	if duty.Capacity == 0 {
		return nil, errors.New("role-probe duty has no capacity")
	}
	listener, err := net.Listen("tcp", p.config.ListenAddress)
	if err != nil {
		return nil, err
	}
	running := &service{plan: p, duty: duty, listener: tls.NewListener(listener, probeTLSConfig(p.config)),
		active: make(chan struct{}, min(4, int(duty.Capacity))), open: make(chan struct{}, 16), connections: make(map[net.Conn]struct{}),
		stop: make(chan struct{}), terminal: make(chan error, 1)}
	running.work.Add(1)
	go running.accept()
	return &Server{Done: running.terminal, Protect: running.protect, Usage: running.usage,
		Stop: running.stopAdmission, Drain: running.drain}, nil
}

func probeTLSConfig(config Config) *tls.Config {
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(config.ClientRootPEM)
	pins := make(map[[32]byte]bool, len(config.ClientKeyPins))
	for _, pin := range config.ClientKeyPins {
		pins[pin] = true
	}
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{config.Certificate}, ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs: roots, SessionTicketsDisabled: true, VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return errors.New("role-probe client certificate is missing")
			}
			raw, err := x509.MarshalPKIXPublicKey(state.PeerCertificates[0].PublicKey)
			if err != nil || !pins[sha256.Sum256(raw)] {
				return errors.New("role-probe client leaf key pin is not authorized")
			}
			return nil
		}}
}

func (s *service) accept() {
	defer s.work.Done()
	for {
		select {
		case s.open <- struct{}{}:
		case <-s.stop:
			s.terminal <- nil
			return
		}
		connection, err := s.listener.Accept()
		if err != nil {
			<-s.open
			select {
			case <-s.stop:
				s.terminal <- nil
			default:
				s.terminal <- err
			}
			return
		}
		if s.protected.Load() {
			<-s.open
			_ = connection.Close()
			continue
		}
		s.track(connection, true)
		s.work.Add(1)
		go s.handle(connection)
	}
}

func (s *service) protect(value bool) { s.protected.Store(value) }

func (s *service) usage() (uint64, uint64, uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	open := uint64(len(s.connections))
	return open + 1, uint64(len(s.active)), 0
}

func (s *service) handle(connection net.Conn) {
	defer s.work.Done()
	defer func() { <-s.open }()
	defer s.track(connection, false)
	defer connection.Close()
	now := s.plan.now()
	deadline := now.Add(s.plan.config.MaximumDuty)
	if now.Before(s.duty.EpochValidFrom) || now.Before(s.duty.RecordValidFrom) ||
		!deadline.Before(s.duty.EpochValidUntil) || !deadline.Before(s.duty.RecordValidUntil) {
		return
	}
	if err := connection.SetDeadline(deadline); err != nil {
		return
	}
	select {
	case s.active <- struct{}{}:
		defer func() { <-s.active }()
	default:
		return
	}
	request, err := readProbeRequest(connection)
	if err == nil && requestMatches(request, s.duty) && s.acceptNonce(request.nonce) {
		_ = writeProbeResponse(connection, request)
	}
}

func (s *service) acceptNonce(nonce [32]byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := 0; index < s.nonceCount; index++ {
		if s.nonces[index] == nonce {
			return false
		}
	}
	s.nonces[s.nonceNext] = nonce
	if s.nonceCount < len(s.nonces) {
		s.nonceCount++
	}
	s.nonceNext = (s.nonceNext + 1) % len(s.nonces)
	return true
}

func (s *service) track(connection net.Conn, add bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if add {
		s.connections[connection] = struct{}{}
	} else {
		delete(s.connections, connection)
	}
}

func (s *service) drain(ctx context.Context) {
	s.stopAdmission()
	done := make(chan struct{})
	go func() { s.work.Wait(); close(done) }()
	timer := time.NewTimer(s.plan.config.DrainTimeout)
	defer timer.Stop()
	select {
	case <-done:
		return
	case <-ctx.Done():
	case <-timer.C:
	}
	s.mu.Lock()
	for connection := range s.connections {
		_ = connection.Close()
	}
	s.mu.Unlock()
	<-done
}

func (s *service) stopAdmission() {
	s.stopOnce.Do(func() {
		close(s.stop)
		_ = s.listener.Close()
	})
}
