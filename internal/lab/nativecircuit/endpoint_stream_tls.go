package nativecircuit

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"
)

func runEndpointUserStream(ctx context.Context, transport net.Conn, trust endpointTrust, nonce handle, spec streamSpec, setupVerified func() error) (endpointObservation, error) {
	defer transport.Close()
	if trust.Roots == nil {
		return endpointObservation{}, errors.New("endpoint trust is outside the fixed contract")
	}
	secured := tls.Client(transport, userStreamTLSConfig(trust))
	if err := secured.HandshakeContext(ctx); err != nil {
		return endpointObservation{}, fmt.Errorf("authenticate exact Target/Instance: %w", err)
	}
	observation, err := observeEndpointTLS(secured.ConnectionState())
	if err != nil {
		return observation, err
	}
	_ = secured.SetDeadline(time.Now().Add(spec.Duration + setupDeadline))
	if err := exchangeUserCanary(secured, nonce); err != nil {
		return observation, err
	}
	if setupVerified != nil {
		if err := setupVerified(); err != nil {
			return observation, err
		}
	}
	if spec.Direction == streamUpload {
		result, err := sendTimedStream(ctx, secured, spec)
		if err != nil {
			return observation, err
		}
		if err := receiveStreamReceipt(secured, result); err != nil {
			return observation, err
		}
		applyStreamObservation(&observation, result)
		return observation, nil
	}
	result, err := receiveTimedStream(ctx, secured, spec)
	if err != nil {
		return observation, err
	}
	if err := sendStreamReceipt(secured, result); err != nil {
		return observation, err
	}
	applyStreamObservation(&observation, result)
	return observation, nil
}

func runEndpointServiceStream(ctx context.Context, transport net.Conn, certificate tls.Certificate, nonce handle, spec streamSpec) (endpointObservation, error) {
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
	_ = secured.SetDeadline(time.Now().Add(spec.Duration + setupDeadline))
	if err := exchangeServiceCanary(secured, nonce); err != nil {
		return observation, err
	}
	if spec.Direction == streamUpload {
		result, err := receiveTimedStream(ctx, secured, spec)
		if err != nil {
			return observation, err
		}
		if err := sendStreamReceipt(secured, result); err != nil {
			return observation, err
		}
		applyStreamObservation(&observation, result)
		return observation, nil
	}
	result, err := sendTimedStream(ctx, secured, spec)
	if err != nil {
		return observation, err
	}
	if err := receiveStreamReceipt(secured, result); err != nil {
		return observation, err
	}
	applyStreamObservation(&observation, result)
	return observation, nil
}

func userStreamTLSConfig(trust endpointTrust) *tls.Config {
	return &tls.Config{
		RootCAs: trust.Roots, ServerName: endpointServerName, MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519}, NextProtos: []string{endpointALPN},
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) != 2 {
				return errors.New("active Instance sent an unexpected certificate chain")
			}
			leaf := sha256.Sum256(state.PeerCertificates[0].Raw)
			if subtle.ConstantTimeCompare(leaf[:], trust.LeafSHA256[:]) != 1 {
				return errors.New("active Instance leaf does not match the preconfigured Target")
			}
			return nil
		},
	}
}

func exchangeUserCanary(connection net.Conn, nonce handle) error {
	if err := writeFrame(connection, frame{Type: frameProtectedData, Payload: nonce[:]}); err != nil {
		return err
	}
	acknowledgement, err := readFrame(connection)
	if err != nil {
		return candidatePeerReadFailure(err)
	}
	if acknowledgement.Type != frameProtectedData || subtle.ConstantTimeCompare(acknowledgement.Payload, nonce[:]) != 1 {
		return candidateContractFailure("endpoint handshake nonce was not authenticated")
	}
	return nil
}

func exchangeServiceCanary(connection net.Conn, expected handle) error {
	nonce, err := readFrame(connection)
	if err != nil {
		return candidatePeerReadFailure(err)
	}
	if nonce.Type != frameProtectedData || subtle.ConstantTimeCompare(nonce.Payload, expected[:]) != 1 {
		return candidateContractFailure("endpoint handshake nonce does not match the sealed invitation")
	}
	return writeFrame(connection, frame{Type: frameProtectedData, Payload: nonce.Payload})
}

func sendStreamReceipt(connection net.Conn, result streamResult) error {
	if err := writeFrame(connection, frame{Type: frameProtectedData, Payload: encodeStreamReceipt(result)}); err != nil {
		return err
	}
	return writeFrame(connection, frame{Type: frameClose})
}

func receiveStreamReceipt(connection net.Conn, expected streamResult) error {
	receipt, err := readFrame(connection)
	if err != nil || receipt.Type != frameProtectedData {
		return errors.New("timed Application stream receipt is missing")
	}
	if err := verifyStreamReceipt(receipt.Payload, expected); err != nil {
		return err
	}
	closed, err := readFrame(connection)
	if err != nil || closed.Type != frameClose {
		return errors.New("timed Application stream did not close explicitly")
	}
	return nil
}

func applyStreamObservation(observation *endpointObservation, result streamResult) {
	observation.ApplicationBytes = int(result.Bytes)
	observation.ApplicationBytesVerified = true
	observation.QueueHighWaterBytes = maximumApplicationPayload
	observation.StreamElapsedMilliseconds = result.Elapsed.Milliseconds()
}
