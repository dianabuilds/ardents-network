//go:build linux

package endpoint

import (
	"context"
	"crypto"
	"crypto/tls"
	"net"
)

// The protected Service profile fixes both supported key-exchange groups.
// The preceding runtime keeps its own compatibility TLS configuration.
func secureTextClient(ctx context.Context, raw net.Conn, credential publicationCredential, exporterContext [32]byte) (*securedAttachment, [32]byte, error) {
	config := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		CurvePreferences:   []tls.CurveID{tls.X25519MLKEM768, tls.X25519},
		InsecureSkipVerify: true, SessionTicketsDisabled: true, VerifyConnection: verifyInstance(credential.InstancePublic)}
	connection := tls.Client(raw, config)
	if err := connection.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, [32]byte{}, err
	}
	return exportedAttachment(connection, exporterContext, 1)
}

func secureTextPublisher(ctx context.Context, raw net.Conn, credential publicationCredential, signer crypto.Signer, exporterContext [32]byte) (*securedAttachment, [32]byte, error) {
	certificate, err := instanceCertificate(credential, signer)
	if err != nil {
		_ = raw.Close()
		return nil, [32]byte{}, err
	}
	config := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519},
		Certificates:     []tls.Certificate{certificate}, SessionTicketsDisabled: true}
	connection := tls.Server(raw, config)
	if err := connection.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, [32]byte{}, err
	}
	return exportedAttachment(connection, exporterContext, 1)
}
