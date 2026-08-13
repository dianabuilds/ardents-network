package serviceconn

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"errors"
	"math/big"
	"net"
	"time"
)

const exporterLabel = "EXPORTER-ardents-h3-service-continuity-v1"

var errInstanceMismatch = errors.New("service Instance certificate does not match the current Credential")

type securedAttachment struct {
	connection         *tls.Conn
	transport          net.Conn
	generation         uint64
	exporterCommitment [32]byte
}

func (attachment *securedAttachment) close() {
	if attachment == nil || attachment.transport == nil {
		return
	}
	_ = attachment.transport.Close()
}

func secureClient(ctx context.Context, raw net.Conn, credential Credential, binding Recovery,
	generation uint64) (*securedAttachment, [32]byte, error) {
	config := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		InsecureSkipVerify: true, SessionTicketsDisabled: true, VerifyConnection: verifyInstance(credential.InstancePublic)}
	connection := tls.Client(raw, config)
	if err := connection.HandshakeContext(ctx); err != nil {
		raw.Close()
		return nil, [32]byte{}, err
	}
	return exportedAttachment(connection, credential, binding, generation)
}

func securePublisher(ctx context.Context, raw net.Conn, credential Credential,
	private ed25519.PrivateKey, binding Recovery, generation uint64) (*securedAttachment, [32]byte, error) {
	certificate, err := instanceCertificate(credential, private)
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
	return exportedAttachment(connection, credential, binding, generation)
}

func exportedAttachment(connection *tls.Conn, credential Credential, recovery Recovery,
	generation uint64) (*securedAttachment, [32]byte, error) {
	binding := connectionBinding(credential, recovery)
	state := connection.ConnectionState()
	material, err := state.ExportKeyingMaterial(exporterLabel, binding[:], 32)
	if err != nil {
		_ = connection.Close()
		return nil, [32]byte{}, err
	}
	key := hmac.New(sha256.New, material)
	_, _ = key.Write([]byte("ardents-h3-continuity-key-v1\x00"))
	continuityBytes := key.Sum(nil)
	var continuity [32]byte
	copy(continuity[:], continuityBytes)
	erase(material)
	erase(continuityBytes)
	exporterCommitment := sha256.Sum256(append([]byte("ardents-h3-exporter-commitment-v1\x00"), continuity[:]...))
	return &securedAttachment{connection: connection, generation: generation,
		transport: connection.NetConn(), exporterCommitment: exporterCommitment}, continuity, nil
}

func connectionBinding(credential Credential, recovery Recovery) [32]byte {
	value := make([]byte, 0, 512)
	value = append(value, "ardents-h3-connection-binding-v1\x00"...)
	value = append(value, credential.Target[:]...)
	value = append(value, credential.InstancePublic[:]...)
	value = append(value, credential.NetworkID[:]...)
	value = append(value, credentialBody(credential)...)
	value = append(value, recovery.CandidateView[:]...)
	value = append(value, recovery.IsolationContext[:]...)
	value = append(value, recovery.DestinationBinding[:]...)
	value = append(value, byte(len(recovery.RouteProfile)))
	value = append(value, recovery.RouteProfile...)
	for _, bound := range []int64{recovery.WorkSafetyNotAfter, recovery.WorkSafetyMaximum, recovery.NoNewRecoveryAfter} {
		encoded := make([]byte, 8)
		binary.BigEndian.PutUint64(encoded, uint64(bound))
		value = append(value, encoded...)
	}
	return sha256.Sum256(value)
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

func instanceCertificate(credential Credential, private ed25519.PrivateKey) (tls.Certificate, error) {
	public, ok := private.Public().(ed25519.PublicKey)
	if !ok || len(private) != ed25519.PrivateKeySize || !bytes.Equal(public, credential.InstancePublic[:]) {
		return tls.Certificate{}, errors.New("service Instance key does not match the current Credential")
	}
	template := &x509.Certificate{SerialNumber: new(big.Int).SetUint64(credential.Generation),
		Subject:   pkix.Name{CommonName: "Ardents laboratory Service Instance"},
		NotBefore: time.Unix(credential.NotBefore, 0), NotAfter: time.Unix(credential.NotAfter, 0),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		return tls.Certificate{}, err
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private, Leaf: parsed}, nil
}
