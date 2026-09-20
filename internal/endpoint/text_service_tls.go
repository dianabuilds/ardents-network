//go:build linux

package endpoint

import (
	"context"
	"crypto"
	"crypto/tls"
	"errors"
	"io"
	"net"

	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

// authenticatedRetirementCarrier preserves an exact admitted Route child's
// authenticated CLOSE(0) after TLS maps that lower EOF to truncation. The
// native terminal tail may accept only that witness after its own complete
// Terminal exchange; every other read failure keeps the recovery contract.
type authenticatedRetirementCarrier struct{ *tls.Conn }

func (carrier *authenticatedRetirementCarrier) Read(value []byte) (int, error) {
	read, err := carrier.Conn.Read(value)
	witness, witnessed := carrier.Conn.NetConn().(interface{ AuthenticatedPeerRetired() bool })
	truncated := errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
	if truncated && witnessed && witness.AuthenticatedPeerRetired() {
		err = nativeconnection.ErrAttachmentRetired
	}
	return read, err
}

func nativeTextAttachment(attachment *securedAttachment) (*nativeconnection.Attachment, error) {
	return nativeconnection.NewAttachment(&authenticatedRetirementCarrier{Conn: attachment.connection},
		attachment.generation, attachment.context, attachment.exporterCommitment, attachment.close)
}

// The protected Service profile fixes both supported key-exchange groups.
// The preceding runtime keeps its own compatibility TLS configuration.
func secureTextClient(ctx context.Context, raw net.Conn, credential publicationCredential, exporterContext [32]byte,
	generation uint64) (*securedAttachment, [32]byte, error) {
	config := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		CurvePreferences:   []tls.CurveID{tls.X25519MLKEM768, tls.X25519},
		InsecureSkipVerify: true, SessionTicketsDisabled: true, VerifyConnection: verifyInstance(credential.InstancePublic)}
	connection := tls.Client(raw, config)
	if err := connection.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, [32]byte{}, err
	}
	return exportedAttachment(connection, exporterContext, generation)
}

func secureTextPublisher(ctx context.Context, raw net.Conn, credential publicationCredential, signer crypto.Signer,
	exporterContext [32]byte, generation uint64) (*securedAttachment, [32]byte, error) {
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
	return exportedAttachment(connection, exporterContext, generation)
}
