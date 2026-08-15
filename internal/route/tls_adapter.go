package route

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"time"
)

func serverTLS(certificate tls.Certificate, expected [32]byte) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{certificate}, ClientAuth: tls.RequireAnyClientCert,
		SessionTicketsDisabled: true, VerifyConnection: exactPeer(expected)}
}

func clientTLS(certificate tls.Certificate, expected [32]byte) *tls.Config {
	config := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		InsecureSkipVerify: true, SessionTicketsDisabled: true, VerifyConnection: exactPeer(expected)}
	if len(certificate.Certificate) > 0 {
		config.Certificates = []tls.Certificate{certificate}
	}
	return config
}

func exactPeer(expected [32]byte) func(tls.ConnectionState) error {
	return func(state tls.ConnectionState) error {
		if expected == [32]byte{} || len(state.PeerCertificates) != 1 {
			return errors.New("carrier peer identity is missing")
		}
		public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
		if !ok || len(public) != ed25519.PublicKeySize || !bytes.Equal(public, expected[:]) {
			return errors.New("carrier peer identity does not match authenticated state")
		}
		return nil
	}
}

func validateEndpoint(value string) error {
	host, port, err := net.SplitHostPort(value)
	number, portErr := strconv.Atoi(port)
	if err != nil || net.ParseIP(host) == nil || portErr != nil || number < 1 || number > 65535 {
		return errors.New("carrier endpoint must be a literal IP and port")
	}
	return nil
}

func validateDeadline(value time.Duration) error {
	if value <= 0 || value > 15*time.Second {
		return errors.New("carrier deadline is outside the frozen bound")
	}
	return nil
}

func validateLifetime(deadline, lifetime time.Duration) error {
	if lifetime < deadline || lifetime > 30*time.Minute {
		return errors.New("carrier lifetime is outside the frozen development bound")
	}
	return nil
}

func validateCertificate(value tls.Certificate) error {
	if len(value.Certificate) != 1 || value.PrivateKey == nil {
		return errors.New("carrier certificate is invalid")
	}
	return nil
}

func dialTLS(ctx context.Context, address string, certificate tls.Certificate, pin [32]byte, deadline time.Duration) (*tls.Conn, error) {
	end := time.Now().Add(deadline)
	var raw net.Conn
	var err error
	for {
		remaining := time.Until(end)
		if remaining <= 0 {
			return nil, err
		}
		attempt := min(remaining, 250*time.Millisecond)
		raw, err = (&net.Dialer{Timeout: attempt}).DialContext(ctx, "tcp", address)
		if err == nil {
			if err = configureCarrierLiveness(raw); err != nil {
				raw.Close()
				return nil, err
			}
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(min(remaining, 20*time.Millisecond)):
		}
	}
	secured := tls.Client(raw, clientTLS(certificate, pin))
	_ = raw.SetDeadline(time.Now().Add(deadline))
	if err := secured.HandshakeContext(ctx); err != nil {
		raw.Close()
		return nil, err
	}
	return secured, nil
}
