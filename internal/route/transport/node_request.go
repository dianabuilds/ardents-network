package transport

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"time"
)

// ClosedNodeCarrierRequest is one exact Node-to-Node Carrier attempt. It has
// no Endpoint credential, retry or peer-choice input. The selected physical
// adapter must match CarrierProfile; this contract authorizes no fallback.
type ClosedNodeCarrierRequest struct {
	CarrierProfile  CarrierProfile
	Endpoint        string
	Certificate     tls.Certificate
	ExpectedPeerKey [32]byte
	Deadline        time.Time
}

// ValidateNodeRequest checks the shared pre-output constraints of both physical
// adapters. Exact key provenance remains with the caller's Network owner.
func ValidateNodeRequest(ctx context.Context, input ClosedNodeCarrierRequest) error {
	if ctx == nil || !LiteralEndpoint(input.Endpoint) || input.Certificate.PrivateKey == nil || input.ExpectedPeerKey == [32]byte{} ||
		input.Deadline.IsZero() || !time.Now().Before(input.Deadline) {
		return errors.New("closed Node Carrier request is invalid")
	}
	if input.CarrierProfile != ClosedCarrierTCP && input.CarrierProfile != ClosedCarrierQUIC {
		return errors.New("closed Node Carrier profile is unsupported")
	}
	return nil
}

// NodeClientTLS supplies identical mutual Node identity requirements to TCP/TLS
// and QUIC; it is distinct from a role client's certificate-free identity.
func NodeClientTLS(certificate tls.Certificate, expected [32]byte) *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		InsecureSkipVerify: true, SessionTicketsDisabled: true, ClientSessionCache: nil, NextProtos: []string{ClosedRouteProfile},
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519}, VerifyConnection: ExactPeer(expected)}
}

// ValidateNodeTLS checks one negotiated Node state. A zero expected key is only
// a syntax check and never establishes Network authority for the certificate.
func ValidateNodeTLS(state tls.ConnectionState, expected [32]byte) error {
	if state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != ClosedRouteProfile || len(state.PeerCertificates) != 1 {
		return errors.New("closed Node Carrier TLS state is invalid")
	}
	if expected != [32]byte{} {
		return ExactPeer(expected)(state)
	}
	public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize {
		return errors.New("closed Node Carrier peer key is invalid")
	}
	return nil
}
