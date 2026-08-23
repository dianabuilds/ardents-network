package endpoint

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"time"

	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

const exporterLabel = "EXPORTER-ardents-service-connection-v1"

var errInstanceMismatch = errors.New("service Instance certificate does not match the current Credential")

type securedAttachment struct {
	connection         *tls.Conn
	transport          net.Conn
	generation         uint64
	context            [32]byte
	exporterCommitment [32]byte
}

func (attachment *securedAttachment) close() {
	if attachment == nil || attachment.transport == nil {
		return
	}
	_ = attachment.transport.Close()
}

func secureClient(ctx context.Context, raw net.Conn, credential Credential, connectionContext [32]byte,
	generation uint64) (*securedAttachment, [32]byte, error) {
	config := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		InsecureSkipVerify: true, SessionTicketsDisabled: true, VerifyConnection: verifyInstance(credential.InstancePublic)}
	connection := tls.Client(raw, config)
	if err := connection.HandshakeContext(ctx); err != nil {
		raw.Close()
		return nil, [32]byte{}, err
	}
	return exportedAttachment(connection, connectionContext, generation)
}

func securePublisher(ctx context.Context, raw net.Conn, credential Credential,
	signer crypto.Signer, connectionContext [32]byte, generation uint64) (*securedAttachment, [32]byte, error) {
	certificate, err := instanceCertificate(credential, signer)
	if err != nil {
		raw.Close()
		return nil, [32]byte{}, err
	}
	config := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{certificate}, SessionTicketsDisabled: true}
	connection := tls.Server(raw, config)
	if err := connection.HandshakeContext(ctx); err != nil {
		raw.Close()
		return nil, [32]byte{}, err
	}
	return exportedAttachment(connection, connectionContext, generation)
}

func exportedAttachment(connection *tls.Conn, connectionContext [32]byte, generation uint64) (*securedAttachment, [32]byte, error) {
	if connectionContext == [32]byte{} {
		_ = connection.Close()
		return nil, [32]byte{}, errors.New("native ConnectionContext is absent")
	}
	state := connection.ConnectionState()
	material, err := state.ExportKeyingMaterial(exporterLabel, connectionContext[:], 32)
	if err != nil {
		_ = connection.Close()
		return nil, [32]byte{}, err
	}
	key := hmac.New(sha256.New, material)
	_, _ = key.Write([]byte("ardents-service-connection-continuity-key-v1\x00"))
	continuityBytes := key.Sum(nil)
	var continuity [32]byte
	copy(continuity[:], continuityBytes)
	erase(material)
	erase(continuityBytes)
	exporterCommitment := sha256.Sum256(append([]byte("ardents-service-connection-exporter-v1\x00"), continuity[:]...))
	return &securedAttachment{connection: connection, generation: generation,
		context: connectionContext, transport: connection.NetConn(), exporterCommitment: exporterCommitment}, continuity, nil
}

func connectionContext(credential Credential, recovery Recovery, publicationDigest [32]byte) ([32]byte, error) {
	return nativeconnection.Context(nativeconnection.ContextInput{Network: credential.NetworkID, Target: credential.Target,
		InstancePublic: credential.InstancePublic, PublicationDigest: publicationDigest,
		InstanceGeneration: credential.Generation, CandidateView: recovery.CandidateView,
		IsolationContext: recovery.IsolationContext, DestinationBinding: recovery.DestinationBinding,
		WorkSafetyNotAfter: recovery.WorkSafetyNotAfter, WorkSafetyMaximum: recovery.WorkSafetyMaximum,
		NoNewRecoveryAfter: recovery.NoNewRecoveryAfter})
}

func verifyInstance(expected [32]byte) func(tls.ConnectionState) error {
	return func(state tls.ConnectionState) error {
		if len(state.PeerCertificates) != 1 {
			return errors.New("service Instance certificate is missing")
		}
		public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
		if !ok || len(public) != ed25519.PublicKeySize || subtle.ConstantTimeCompare(public, expected[:]) != 1 {
			return errInstanceMismatch
		}
		return nil
	}
}

func instanceCertificate(credential Credential, signer crypto.Signer) (tls.Certificate, error) {
	public, ok := signer.Public().(ed25519.PublicKey)
	if !ok || len(public) != ed25519.PublicKeySize || !bytes.Equal(public, credential.InstancePublic[:]) {
		return tls.Certificate{}, errors.New("service Instance key does not match the current Credential")
	}
	template := &x509.Certificate{SerialNumber: new(big.Int).SetUint64(credential.Generation),
		Subject:   pkix.Name{CommonName: "Ardents laboratory Service Instance"},
		NotBefore: time.Unix(credential.NotBefore, 0), NotAfter: time.Unix(credential.NotAfter, 0),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, signer)
	if err != nil {
		return tls.Certificate{}, err
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: signer, Leaf: parsed}, nil
}
