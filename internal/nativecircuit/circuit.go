package nativecircuit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"time"
)

const (
	nodeServerName = "node.carrier.invalid"
	nodeALPN       = "carrier-lab-node/1"
	setupDeadline  = 15 * time.Second
)

type circuitHop struct {
	Address           string
	CertificateSHA256 [32]byte
}

type routeExtension struct {
	NextAddress string `json:"next_address"`
}

type relayObservation struct {
	NextAddress string
	Err         error
}

func dialTelescopedCircuit(ctx context.Context, path []circuitHop) (net.Conn, error) {
	if len(path) == 0 {
		return nil, errors.New("native circuit path is empty")
	}
	connection, err := (&net.Dialer{Timeout: setupDeadline}).DialContext(ctx, "tcp", path[0].Address)
	if err != nil {
		return nil, fmt.Errorf("dial first native circuit hop: %w", err)
	}
	secured, err := authenticateNode(ctx, connection, path[0].CertificateSHA256)
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	var current net.Conn = secured
	for _, hop := range path[1:] {
		payload, err := json.Marshal(routeExtension{NextAddress: hop.Address})
		if err != nil {
			_ = current.Close()
			return nil, err
		}
		if err := writeFrame(current, frame{Type: frameRouteExtend, Payload: payload}); err != nil {
			_ = current.Close()
			return nil, fmt.Errorf("request route extension: %w", err)
		}
		result, err := readFrame(current)
		if err != nil || result.Type != frameRouteResult || string(result.Payload) != "ok" {
			_ = current.Close()
			return nil, errors.New("native circuit extension failed closed")
		}
		nextLayer, err := authenticateNode(ctx, current, hop.CertificateSHA256)
		if err != nil {
			_ = current.Close()
			return nil, err
		}
		current = nextLayer
	}
	return current, nil
}

func authenticateNode(ctx context.Context, transport net.Conn, expected [32]byte) (*tls.Conn, error) {
	secured := tls.Client(transport, &tls.Config{
		InsecureSkipVerify: true, // The per-run certificate digest is the trust root.
		ServerName:         nodeServerName, MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519}, NextProtos: []string{nodeALPN},
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) != 1 {
				return errors.New("native circuit Node sent an unexpected certificate chain")
			}
			observed := sha256.Sum256(state.PeerCertificates[0].Raw)
			if subtle.ConstantTimeCompare(observed[:], expected[:]) != 1 {
				return errors.New("native circuit Node certificate does not match the per-run identity")
			}
			return nil
		},
	})
	if err := secured.HandshakeContext(ctx); err != nil {
		return nil, fmt.Errorf("authenticate native circuit Node: %w", err)
	}
	state := secured.ConnectionState()
	if state.Version != tls.VersionTLS13 || state.CurveID != tls.X25519 || state.DidResume || state.NegotiatedProtocol != nodeALPN {
		return nil, errors.New("native circuit Node negotiated outside the fixed TLS contract")
	}
	return secured, nil
}

func serveOneRelay(ctx context.Context, listener net.Listener, certificate tls.Certificate, allowedNext []string) relayObservation {
	observation := relayObservation{}
	observation.Err = serveOneNode(ctx, listener, certificate, func(connection net.Conn) error {
		var err error
		observation.NextAddress, err = handleRelayConnection(ctx, connection, allowedNext)
		return err
	})
	return observation
}

func handleRelayConnection(ctx context.Context, connection net.Conn, allowedNext []string) (string, error) {
	request, err := readFrame(connection)
	if err != nil || request.Type != frameRouteExtend {
		return "", errors.New("relay expected one route-extension frame")
	}
	decoder := json.NewDecoder(bytes.NewReader(request.Payload))
	decoder.DisallowUnknownFields()
	var extension routeExtension
	if err := decoder.Decode(&extension); err != nil || extension.NextAddress == "" || len(extension.NextAddress) > 255 {
		return "", errors.New("relay received an invalid route extension")
	}
	if !slices.Contains(allowedNext, extension.NextAddress) {
		return "", errors.New("relay rejected a non-adjacent route extension")
	}
	next, err := (&net.Dialer{Timeout: setupDeadline}).DialContext(ctx, "tcp", extension.NextAddress)
	if err != nil {
		return extension.NextAddress, fmt.Errorf("relay dial adjacent role: %w", err)
	}
	defer next.Close()
	if err := writeFrame(connection, frame{Type: frameRouteResult, Payload: []byte("ok")}); err != nil {
		return extension.NextAddress, err
	}
	proxyOpaque(connection, next)
	return extension.NextAddress, nil
}

func serveOneNode(ctx context.Context, listener net.Listener, certificate tls.Certificate, handler func(net.Conn) error) error {
	defer listener.Close()
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stop()
	connection, err := listener.Accept()
	if err != nil {
		return err
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(setupDeadline))
	secured := tls.Server(connection, &tls.Config{
		Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519}, NextProtos: []string{nodeALPN}, SessionTicketsDisabled: true,
	})
	if err := secured.HandshakeContext(ctx); err != nil {
		return fmt.Errorf("native circuit Node TLS handshake: %w", err)
	}
	_ = secured.SetDeadline(time.Time{})
	return handler(secured)
}

func proxyOpaque(left, right net.Conn) {
	done := make(chan struct{}, 2)
	copyStream := func(destination, source net.Conn) {
		_, _ = io.Copy(destination, source)
		done <- struct{}{}
	}
	go copyStream(left, right)
	go copyStream(right, left)
	<-done
	_ = left.Close()
	_ = right.Close()
	<-done
}
