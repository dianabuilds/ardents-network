package source

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

func fetch(ctx context.Context, client client, request Message) (Message, error) {
	dialTimeout := time.Second
	handshakeTimeout := 2 * time.Second
	exchangeTimeout := 5 * time.Second
	totalContext, cancel := context.WithTimeout(ctx, exchangeTimeout)
	defer cancel()
	connection, err := (&net.Dialer{Timeout: dialTimeout}).DialContext(totalContext, "tcp", client.address)
	if err != nil {
		return Message{}, fmt.Errorf("distribution source unavailable: %w", err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(exchangeTimeout)); err != nil {
		return Message{}, err
	}
	tlsConnection := tls.Client(connection, clientTLSConfig(client))
	handshakeContext, stopHandshake := context.WithTimeout(totalContext, handshakeTimeout)
	err = tlsConnection.HandshakeContext(handshakeContext)
	stopHandshake()
	if err != nil {
		return Message{}, fmt.Errorf("distribution source authentication failed: %w", err)
	}
	if err := writeRequest(tlsConnection, request); err != nil {
		return Message{}, fmt.Errorf("write distribution request: %w", err)
	}
	response, err := readResponse(tlsConnection)
	if err != nil {
		return response, fmt.Errorf("read distribution response: %w", err)
	}
	var trailing [1]byte
	if count, trailingErr := tlsConnection.Read(trailing[:]); count != 0 ||
		(trailingErr != nil && !errors.Is(trailingErr, io.EOF)) {
		return Message{}, errors.New("distribution response has trailing bytes or an unclean close")
	}
	return response, nil
}

// Serve owns the configured bounded TLS listener until cancellation or
// terminal failure.
func (p *Plan) Serve(ctx context.Context, ready chan<- error, protected func() bool,
	active func(int), resolve func(context.Context, Message) Message) error {
	if p == nil || !p.details.Serving {
		return errors.New("source server is not configured")
	}
	return serve(ctx, p.server, ready, protected, active, resolve)
}

func serve(ctx context.Context, server server, ready chan<- error, protected func() bool,
	active func(int), resolve func(context.Context, Message) Message) error {
	listener, err := net.Listen("tcp", server.address)
	if err != nil {
		wrapped := fmt.Errorf("listen for Network State distribution: %w", err)
		ready <- wrapped
		return wrapped
	}
	tlsListener := tls.NewListener(listener, serverTLSConfig(server))
	ready <- nil
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
	credits := make(chan struct{}, 8)
	var connections sync.WaitGroup
	defer connections.Wait()
	for {
		credits <- struct{}{}
		connection, acceptErr := tlsListener.Accept()
		if acceptErr != nil {
			<-credits
			if ctx.Err() != nil {
				return context.Canceled
			}
			return fmt.Errorf("accept distribution connection: %w", acceptErr)
		}
		if protected != nil && protected() {
			<-credits
			_ = connection.Close()
			continue
		}
		connections.Add(1)
		go func() {
			defer connections.Done()
			defer func() { <-credits }()
			handleConnection(ctx, server, active, connection, resolve)
		}()
	}
}

func handleConnection(ctx context.Context, server server, active func(int), connection net.Conn,
	resolve func(context.Context, Message) Message) {
	if active != nil {
		active(1)
		defer active(-1)
	}
	defer connection.Close()
	started := time.Now()
	if err := connection.SetDeadline(started.Add(server.headerTimeout)); err != nil {
		return
	}
	request, err := readRequest(connection)
	if err != nil {
		_ = writeResponse(connection, Message{Status: "bad-request"})
		return
	}
	if err := connection.SetDeadline(started.Add(5 * time.Second)); err != nil {
		return
	}
	_ = writeResponse(connection, resolve(ctx, request))
}

func clientTLSConfig(client client) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		RootCAs: client.roots, ServerName: client.serverName,
		Certificates:       []tls.Certificate{client.certificate},
		ClientSessionCache: nil, SessionTicketsDisabled: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return errors.New("distribution source certificate is missing")
			}
			digest, err := keyDigest(state.PeerCertificates[0].PublicKey)
			if err != nil || digest != client.leafKeyDigest {
				return errors.New("distribution source leaf key pin does not match")
			}
			return nil
		},
	}
}

func serverTLSConfig(server server) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{server.certificate},
		ClientAuth:   tls.RequireAndVerifyClientCert, ClientCAs: server.clientRoots,
		SessionTicketsDisabled: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return errors.New("distribution client certificate is missing")
			}
			digest, err := keyDigest(state.PeerCertificates[0].PublicKey)
			if err != nil || !server.clientDigests[digest] {
				return errors.New("distribution client leaf key pin is not authorized")
			}
			return nil
		},
	}
}

func keyDigest(public any) ([32]byte, error) {
	key, ok := public.(ed25519.PublicKey)
	if !ok || len(key) != ed25519.PublicKeySize {
		return [32]byte{}, errors.New("distribution transport key is not Ed25519")
	}
	return sha256.Sum256(append([]byte("ardents-h3-source-transport-key-v1\x00"), key...)), nil
}
