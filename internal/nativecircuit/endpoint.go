package nativecircuit

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"time"
)

const (
	endpointServerName = "carrier.invalid"
	endpointALPN       = "carrier-lab-route/1"
	maximumQueueBytes  = 256 * 1024
)

type endpointTrust struct {
	Roots      *x509.CertPool
	LeafSHA256 [32]byte
}

type endpointObservation struct {
	TLSVersion                string
	Curve                     string
	CipherSuite               string
	SessionResumed            bool
	ApplicationBytesVerified  bool
	ApplicationBytes          int
	QueueHighWaterBytes       int
	StreamElapsedMilliseconds int64
}

func runEndpointUser(ctx context.Context, transport net.Conn, trust endpointTrust, nonce handle, payload []byte) (endpointObservation, error) {
	return runEndpointUserWithProgress(ctx, transport, trust, nonce, payload, nil)
}

func runEndpointUserWithProgress(ctx context.Context, transport net.Conn, trust endpointTrust, nonce handle, payload []byte, firstChunkVerified func() error) (endpointObservation, error) {
	return runEndpointUserWithCallbacks(ctx, transport, trust, nonce, payload, nil, firstChunkVerified)
}

func runEndpointUserWithCallbacks(ctx context.Context, transport net.Conn, trust endpointTrust, nonce handle, payload []byte, setupVerified, firstChunkVerified func() error) (endpointObservation, error) {
	defer transport.Close()
	if trust.Roots == nil || len(payload) > maximumQueueBytes {
		return endpointObservation{}, errors.New("endpoint trust or Application payload is outside the fixed contract")
	}
	secured := tls.Client(transport, &tls.Config{
		RootCAs: trust.Roots, ServerName: endpointServerName, MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519}, NextProtos: []string{endpointALPN},
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) != 2 {
				return errors.New("active Instance sent an unexpected certificate chain")
			}
			observed := sha256.Sum256(state.PeerCertificates[0].Raw)
			if subtle.ConstantTimeCompare(observed[:], trust.LeafSHA256[:]) != 1 {
				return errors.New("active Instance leaf does not match the preconfigured Target")
			}
			return nil
		},
	})
	if err := secured.HandshakeContext(ctx); err != nil {
		return endpointObservation{}, fmt.Errorf("authenticate exact Target/Instance: %w", err)
	}
	observation, err := observeEndpointTLS(secured.ConnectionState())
	if err != nil {
		return observation, err
	}
	_ = secured.SetDeadline(time.Now().Add(setupDeadline))
	if err := writeFrame(secured, frame{Type: frameProtectedData, Payload: nonce[:]}); err != nil {
		return observation, err
	}
	acknowledgement, err := readFrame(secured)
	if err != nil || acknowledgement.Type != frameProtectedData || subtle.ConstantTimeCompare(acknowledgement.Payload, nonce[:]) != 1 {
		return observation, errors.New("endpoint handshake nonce was not authenticated")
	}
	if setupVerified != nil {
		if err := setupVerified(); err != nil {
			return observation, err
		}
	}
	for offset := 0; offset < len(payload); {
		end := min(offset+maximumApplicationPayload, len(payload))
		chunk := payload[offset:end]
		observation.QueueHighWaterBytes = max(observation.QueueHighWaterBytes, len(chunk))
		if err := writeFrame(secured, frame{Type: frameProtectedData, Payload: chunk}); err != nil {
			return observation, err
		}
		returned, err := readFrame(secured)
		if err != nil || returned.Type != frameProtectedData || subtle.ConstantTimeCompare(returned.Payload, chunk) != 1 {
			return observation, errors.New("application bytes were not verified in order")
		}
		observation.ApplicationBytes += len(chunk)
		if observation.ApplicationBytes == len(chunk) && firstChunkVerified != nil {
			if err := firstChunkVerified(); err != nil {
				return observation, err
			}
		}
		offset = end
	}
	if err := writeFrame(secured, frame{Type: frameClose}); err != nil {
		return observation, err
	}
	closed, err := readFrame(secured)
	if err != nil || closed.Type != frameClose {
		return observation, errors.New("joined Route did not close explicitly")
	}
	observation.ApplicationBytesVerified = true
	return observation, nil
}

func runEndpointService(ctx context.Context, transport net.Conn, certificate tls.Certificate, expectedNonce handle) (endpointObservation, error) {
	defer transport.Close()
	secured := tls.Server(transport, &tls.Config{
		Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519}, NextProtos: []string{endpointALPN}, SessionTicketsDisabled: true,
	})
	if err := secured.HandshakeContext(ctx); err != nil {
		return endpointObservation{}, fmt.Errorf("active Instance TLS handshake: %w", err)
	}
	observation, err := observeEndpointTLS(secured.ConnectionState())
	if err != nil {
		return observation, err
	}
	_ = secured.SetDeadline(time.Now().Add(setupDeadline))
	nonce, err := readFrame(secured)
	if err != nil || nonce.Type != frameProtectedData || subtle.ConstantTimeCompare(nonce.Payload, expectedNonce[:]) != 1 {
		return observation, errors.New("endpoint handshake nonce does not match the sealed invitation")
	}
	if err := writeFrame(secured, frame{Type: frameProtectedData, Payload: nonce.Payload}); err != nil {
		return observation, err
	}
	for {
		value, err := readFrame(secured)
		if err != nil {
			return observation, err
		}
		switch value.Type {
		case frameProtectedData:
			observation.QueueHighWaterBytes = max(observation.QueueHighWaterBytes, len(value.Payload))
			observation.ApplicationBytes += len(value.Payload)
			if observation.ApplicationBytes > maximumQueueBytes {
				return observation, errors.New("logical Application queue exceeded 256 KiB")
			}
			if err := writeFrame(secured, value); err != nil {
				return observation, err
			}
		case frameClose:
			if err := writeFrame(secured, frame{Type: frameClose}); err != nil {
				return observation, err
			}
			observation.ApplicationBytesVerified = true
			return observation, nil
		default:
			return observation, errors.New("endpoint stream received an invalid state transition")
		}
	}
}

func observeEndpointTLS(state tls.ConnectionState) (endpointObservation, error) {
	observation := endpointObservation{
		TLSVersion: "TLS1.3", Curve: state.CurveID.String(), CipherSuite: tls.CipherSuiteName(state.CipherSuite), SessionResumed: state.DidResume,
	}
	if state.Version != tls.VersionTLS13 || state.CurveID != tls.X25519 || state.DidResume || state.NegotiatedProtocol != endpointALPN {
		return observation, errors.New("endpoint stream negotiated outside the fixed TLS 1.3/X25519 contract")
	}
	return observation, nil
}
