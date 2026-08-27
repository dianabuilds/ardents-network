//go:build ignore

// R-094 is a disposable TCP/TLS and QUIC v1 carrier-baseline experiment.
package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"time"

	"github.com/quic-go/quic-go"
)

const (
	alpn       = "ardents-r094-carrier"
	label      = "ardents-r094-leg-binding"
	transcript = "r094-request"
	response   = "r094-response"
)

type pki struct {
	server tls.Certificate
	client tls.Certificate
	wrong  tls.Certificate
	roots  *x509.CertPool
}

type laneResult struct {
	state         tls.ConnectionState
	exporter      []byte
	authenticated bool
	err           error
}

func main() {
	run("tcp_tls", false, runTCP)
	run("quic_v1", false, runQUIC)
	run("tcp_tls_wrong_peer", true, runTCP)
	run("quic_v1_wrong_peer", true, runQUIC)
}

func run(name string, wrongPeer bool, attempt func(context.Context, bool) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := attempt(ctx, wrongPeer); err != nil {
		if errors.Is(err, errRejectedPeer) {
			fmt.Printf("%s=rejected\n", name)
			return
		}
		fmt.Printf("%s=failure:%v\n", name, err)
		return
	}
	fmt.Printf("%s=peer-auth:ok,exporter:ok,transcript:ok\n", name)
}

var errRejectedPeer = errors.New("peer rejected")

func runTCP(ctx context.Context, wrongPeer bool) error {
	material, err := newPKI()
	if err != nil {
		return err
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", serverTLS(material))
	if err != nil {
		return err
	}
	defer listener.Close()
	server := make(chan laneResult, 1)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			server <- laneResult{err: err}
			return
		}
		defer connection.Close()
		tlsConnection := connection.(*tls.Conn)
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			server <- laneResult{err: err}
			return
		}
		server <- serveLane(tlsConnection, tlsConnection.ConnectionState())
	}()

	certificate := material.client
	if wrongPeer {
		certificate = material.wrong
	}
	dialer := tls.Dialer{Config: clientTLS(material, certificate)}
	connection, err := dialer.DialContext(ctx, "tcp", listener.Addr().String())
	if wrongPeer {
		if err == nil {
			connection.Close()
		}
		return rejectedServerResult(ctx, server)
	}
	if err != nil {
		return err
	}
	defer connection.Close()
	tlsConnection := connection.(*tls.Conn)
	if err := clientLane(tlsConnection, tlsConnection.ConnectionState(), server); err != nil {
		return err
	}
	return nil
}

func runQUIC(ctx context.Context, wrongPeer bool) error {
	material, err := newPKI()
	if err != nil {
		return err
	}
	listener, err := quic.ListenAddr("127.0.0.1:0", serverTLS(material), quicConfig())
	if err != nil {
		return err
	}
	defer listener.Close()
	server := make(chan laneResult, 1)
	go func() {
		connection, err := listener.Accept(ctx)
		if err != nil {
			server <- laneResult{err: err}
			return
		}
		stream, err := connection.AcceptStream(ctx)
		if err != nil {
			server <- laneResult{err: err}
			return
		}
		defer stream.Close()
		server <- serveLane(stream, connection.ConnectionState().TLS)
	}()

	certificate := material.client
	if wrongPeer {
		certificate = material.wrong
	}
	connection, err := quic.DialAddr(ctx, listener.Addr().String(), clientTLS(material, certificate), quicConfig())
	if wrongPeer {
		if err != nil {
			return errRejectedPeer
		}
		defer connection.CloseWithError(0, "wrong-peer-probe-complete")
		stream, openErr := connection.OpenStreamSync(ctx)
		if openErr != nil {
			return errRejectedPeer
		}
		defer stream.Close()
		if _, err := stream.Write([]byte(transcript)); err != nil {
			return errRejectedPeer
		}
		if err := stream.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			return err
		}
		received := make([]byte, len(response))
		if _, err := io.ReadFull(stream, received); err != nil {
			return errRejectedPeer
		}
		return errors.New("wrong QUIC client certificate received a response")
	}
	if err != nil {
		return err
	}
	defer connection.CloseWithError(0, "done")
	stream, err := connection.OpenStreamSync(ctx)
	if err != nil {
		return err
	}
	defer stream.Close()
	if err := clientLane(stream, connection.ConnectionState().TLS, server); err != nil {
		return err
	}
	return nil
}

func clientLane(connection io.ReadWriteCloser, state tls.ConnectionState, server <-chan laneResult) error {
	if err := verifyPeer(state, "r094-server"); err != nil {
		return err
	}
	clientExporter, err := state.ExportKeyingMaterial(label, nil, 32)
	if err != nil {
		return err
	}
	if _, err := connection.Write([]byte(transcript)); err != nil {
		return err
	}
	received := make([]byte, len(response))
	if _, err := io.ReadFull(connection, received); err != nil {
		return err
	}
	if string(received) != response {
		return errors.New("unexpected response transcript")
	}
	serverResult := <-server
	if serverResult.err != nil {
		return serverResult.err
	}
	if err := verifyPeer(serverResult.state, "r094-client"); err != nil {
		return err
	}
	if !bytes.Equal(clientExporter, serverResult.exporter) {
		return errors.New("TLS exporter mismatch")
	}
	return nil
}

func serveLane(connection io.ReadWriteCloser, state tls.ConnectionState) laneResult {
	result := laneResult{state: state, authenticated: true}
	exporter, err := state.ExportKeyingMaterial(label, nil, 32)
	if err != nil {
		result.err = err
		return result
	}
	received := make([]byte, len(transcript))
	if _, err := io.ReadFull(connection, received); err != nil {
		result.err = err
		return result
	}
	if string(received) != transcript {
		result.err = errors.New("unexpected request transcript")
		return result
	}
	if _, err := connection.Write([]byte(response)); err != nil {
		result.err = err
		return result
	}
	result.exporter = exporter
	return result
}

func rejectedServerResult(ctx context.Context, server <-chan laneResult) error {
	select {
	case result := <-server:
		if result.authenticated {
			return errors.New("server exposed authenticated lane to wrong peer")
		}
		return errRejectedPeer
	case <-ctx.Done():
		return ctx.Err()
	}
}

func serverTLS(material pki) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{material.server},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    material.roots,
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{alpn},
	}
}

func clientTLS(material pki, certificate tls.Certificate) *tls.Config {
	return &tls.Config{
		Certificates: []tls.Certificate{certificate},
		RootCAs:      material.roots,
		ServerName:   "r094-server",
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{alpn},
	}
}

func quicConfig() *quic.Config {
	return &quic.Config{Versions: []quic.Version{quic.Version1}, EnableDatagrams: false, Allow0RTT: false}
}

func verifyPeer(state tls.ConnectionState, expected string) error {
	if len(state.PeerCertificates) != 1 || state.PeerCertificates[0].Subject.CommonName != expected {
		return errors.New("unexpected authenticated peer")
	}
	return nil
}

func newPKI() (pki, error) {
	caPrivate, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return pki{}, err
	}
	serial, err := randomSerial()
	if err != nil {
		return pki{}, err
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "r094-test-ca"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caRaw, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caPrivate.PublicKey, caPrivate)
	if err != nil {
		return pki{}, err
	}
	ca, err := x509.ParseCertificate(caRaw)
	if err != nil {
		return pki{}, err
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	server, err := signedCertificate("r094-server", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, ca, caPrivate)
	if err != nil {
		return pki{}, err
	}
	client, err := signedCertificate("r094-client", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, ca, caPrivate)
	if err != nil {
		return pki{}, err
	}
	wrongCA, wrongKey, err := newCA()
	if err != nil {
		return pki{}, err
	}
	wrong, err := signedCertificate("r094-wrong-client", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, wrongCA, wrongKey)
	if err != nil {
		return pki{}, err
	}
	return pki{server: server, client: client, wrong: wrong, roots: roots}, nil
}

func newCA() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}
	template := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: "r094-wrong-ca"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, &private.PublicKey, private)
	if err != nil {
		return nil, nil, err
	}
	certificate, err := x509.ParseCertificate(raw)
	return certificate, private, err
}

func signedCertificate(name string, usage []x509.ExtKeyUsage, authority *x509.Certificate, authorityKey *ecdsa.PrivateKey) (tls.Certificate, error) {
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := randomSerial()
	if err != nil {
		return tls.Certificate{}, err
	}
	template := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: name}, DNSNames: []string{name},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: usage,
	}
	raw, err := x509.CreateCertificate(rand.Reader, template, authority, &private.PublicKey, authorityKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	key, err := x509.MarshalECPrivateKey(private)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: key}))
}

func randomSerial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}
