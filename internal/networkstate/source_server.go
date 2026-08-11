package networkstate

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

func (s *store) serveSource(ctx context.Context, ready chan<- error) error {
	listener, err := net.Listen("tcp", s.config.server.address)
	if err != nil {
		wrapped := fmt.Errorf("listen for source distribution: %w", err)
		ready <- wrapped
		return wrapped
	}
	ready <- nil
	tlsListener := tls.NewListener(listener, s.sourceTLSConfig())
	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = tlsListener.Close()
		case <-stop:
		}
	}()
	defer close(stop)
	defer tlsListener.Close()
	semaphore := make(chan struct{}, 8)
	var active sync.WaitGroup
	defer active.Wait()
	for {
		connection, acceptErr := tlsListener.Accept()
		if acceptErr != nil {
			if ctx.Err() != nil {
				return context.Canceled
			}
			return fmt.Errorf("accept source connection: %w", acceptErr)
		}
		select {
		case semaphore <- struct{}{}:
			active.Add(1)
			go func() {
				defer active.Done()
				defer func() { <-semaphore }()
				s.handleSourceConnection(connection)
			}()
		default:
			_ = connection.Close()
		}
	}
}

func (s *store) sourceTLSConfig() *tls.Config {
	server := s.config.server
	return &tls.Config{
		MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{server.certificate},
		ClientAuth:   tls.RequireAndVerifyClientCert, ClientCAs: server.clientRoots,
		SessionTicketsDisabled: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return errors.New("source client certificate is missing")
			}
			digest, err := transportKeyDigest(state.PeerCertificates[0].PublicKey)
			if err != nil || !server.clientDigests[digest] {
				return errors.New("source client leaf key pin is not authorized")
			}
			return nil
		},
	}
}

func (s *store) handleSourceConnection(connection net.Conn) {
	defer connection.Close()
	started := time.Now()
	if err := connection.SetDeadline(started.Add(s.config.server.headerTimeout)); err != nil {
		return
	}
	request, err := readSourceRequest(connection)
	if err != nil {
		_ = writeSourceResponse(connection, sourceResponse{status: sourceBad})
		return
	}
	if err := connection.SetDeadline(started.Add(5 * time.Second)); err != nil {
		return
	}
	if request.networkDigest != networkIdentityDigest(s.config.networkID) {
		_ = writeSourceResponse(connection, sourceResponse{status: sourceBad})
		return
	}
	s.mu.RLock()
	if s.closed || s.currentDecision == nil {
		s.mu.RUnlock()
		_ = writeSourceResponse(connection, sourceResponse{status: sourceBusy})
		return
	}
	decision := *s.currentDecision
	digest := decision.epoch.digest
	materialIndex := s.config.material
	s.mu.RUnlock()
	if request.opcode == sourceByDigest && request.objectDigest != digest {
		_ = writeSourceResponse(connection, sourceResponse{status: sourceNotFound})
		return
	}
	payload, err := encodeSourceBundle(decision, materialIndex)
	if err != nil {
		_ = writeSourceResponse(connection, sourceResponse{status: sourceInternal})
		return
	}
	_ = writeSourceResponse(connection, sourceResponse{
		status: sourceOK, objectDigest: digest, payload: payload,
	})
}
