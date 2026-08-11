package nodelifecycle

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/networkstate"
)

type probeServer struct {
	config      runtimeConfig
	snapshot    networkstate.Snapshot
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
	terminal    chan error
	work        sync.WaitGroup
}

func startProbeServer(config runtimeConfig, snapshot networkstate.Snapshot) (*probeServer, error) {
	listener, err := net.Listen("tcp", config.ListenAddress)
	if err != nil {
		return nil, err
	}
	server := &probeServer{config: config, snapshot: snapshot, listener: tls.NewListener(listener, probeTLSConfig(config)),
		active: make(chan struct{}, min(4, int(snapshot.ProbeCapacity))), open: make(chan struct{}, 16), connections: make(map[net.Conn]struct{}),
		stop: make(chan struct{}), terminal: make(chan error, 1)}
	server.work.Add(1)
	go server.accept()
	return server, nil
}

func probeTLSConfig(config runtimeConfig) *tls.Config {
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

func (s *probeServer) accept() {
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
		s.track(connection, true)
		s.work.Add(1)
		go s.handle(connection)
	}
}

func (s *probeServer) handle(connection net.Conn) {
	defer s.work.Done()
	defer func() { <-s.open }()
	defer s.track(connection, false)
	defer connection.Close()
	now := s.config.now()
	deadline := now.Add(s.config.MaximumDuty)
	if now.Before(s.snapshot.EpochValidFrom) || now.Before(s.snapshot.RecordValidFrom) ||
		!deadline.Before(s.snapshot.ValidUntil) || !deadline.Before(s.snapshot.RecordValidUntil) {
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
	if err == nil && requestMatches(request, s.snapshot) && s.acceptNonce(request.nonce) {
		_ = writeProbeResponse(connection, request)
	}
}

func (s *probeServer) acceptNonce(nonce [32]byte) bool {
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

func (s *probeServer) track(connection net.Conn, add bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if add {
		s.connections[connection] = struct{}{}
	} else {
		delete(s.connections, connection)
	}
}

func (s *probeServer) drain(ctx context.Context) {
	s.stopAdmission()
	done := make(chan struct{})
	go func() { s.work.Wait(); close(done) }()
	timer := time.NewTimer(s.config.DrainTimeout)
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

func (s *probeServer) stopAdmission() {
	s.stopOnce.Do(func() {
		close(s.stop)
		_ = s.listener.Close()
	})
}
