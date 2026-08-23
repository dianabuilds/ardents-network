//go:build ignore

// R-093 synthetic cross-host native-leg tracer. See README.md and the research record.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

const profile = "ardents-interactive-route-v1"

var defaultNotAfter = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

type result struct {
	Schema              string `json:"schema"`
	Role                string `json:"role"`
	Outcome             string `json:"outcome"`
	Address             string `json:"address,omitempty"`
	TLSVersion          string `json:"tls_version,omitempty"`
	ALPN                string `json:"alpn,omitempty"`
	CarriedConnections  int    `json:"carried_connections"`
	RejectedConnections int    `json:"rejected_connections"`
	PayloadBytes        int    `json:"payload_bytes,omitempty"`
	PayloadSHA256       string `json:"payload_sha256,omitempty"`
	EchoSHA256          string `json:"echo_sha256,omitempty"`
	ElapsedNanoseconds  int64  `json:"elapsed_nanoseconds"`
	Reason              string `json:"reason,omitempty"`
}

func main() {
	mode := flag.String("mode", "", "identity, server, or client")
	certificate := flag.String("certificate", "", "local PEM certificate path")
	key := flag.String("key", "", "local PEM private-key path")
	peerCertificate := flag.String("peer-certificate", "", "expected peer PEM certificate path")
	listen := flag.String("listen", ":44393", "server TCP listen address")
	address := flag.String("address", "", "server TCP address for the client")
	payloadBytes := flag.Int("payload", 64<<10, "opaque payload bytes (1..1048576)")
	accepted := flag.Int("accepted-connections", 1, "finite valid server connections (0..32)")
	rejected := flag.Int("expected-rejections", 0, "finite rejected server connections (0..32)")
	bindingMode := flag.String("binding", "valid", "valid or changed-attachment")
	expectRejection := flag.Bool("expect-rejection", false, "client expects its binding attempt to be rejected")
	timeout := flag.Duration("timeout", 15*time.Second, "per-role completion deadline")
	flag.Parse()
	if *mode == "identity" {
		if err := createIdentity(*certificate, *key); err != nil {
			fail(err)
		}
		return
	}
	if *certificate == "" || *key == "" || *peerCertificate == "" || *timeout <= 0 {
		fail(errors.New("certificate, key, peer certificate, and positive timeout are required"))
	}
	if *payloadBytes < 1 || *payloadBytes > 1<<20 {
		fail(errors.New("payload is outside its bound"))
	}
	if *mode == "server" {
		if *accepted < 0 || *rejected < 0 || *accepted+*rejected < 1 || *accepted+*rejected > 32 {
			fail(errors.New("server connection counts are outside their bound"))
		}
		output, err := serve(*listen, *certificate, *key, *peerCertificate, *accepted, *rejected, *timeout)
		if err != nil {
			fail(err)
		}
		writeResult(output)
		return
	}
	if *mode == "client" && *address != "" && (*bindingMode == "valid" || *bindingMode == "changed-attachment") {
		output, err := carry(*address, *certificate, *key, *peerCertificate, *payloadBytes, *bindingMode, *expectRejection, *timeout)
		if err != nil {
			fail(err)
		}
		writeResult(output)
		return
	}
	fail(errors.New("mode or its arguments are invalid"))
}

func createIdentity(certificatePath, keyPath string) error {
	if certificatePath == "" || keyPath == "" || certificatePath == keyPath {
		return errors.New("distinct certificate and key paths are required")
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	issued := x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), NotBefore: time.Now().Add(-time.Minute),
		NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, &issued, &issued, public, private)
	if err != nil {
		return err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return err
	}
	if err := writeExclusivePEM(certificatePath, "CERTIFICATE", der); err != nil {
		return err
	}
	if err := writeExclusivePEM(keyPath, "PRIVATE KEY", privateDER); err != nil {
		_ = os.Remove(certificatePath)
		return err
	}
	certificateDigest := sha256.Sum256(der)
	writeResult(result{Schema: "ardents-r093-native-leg-v1", Role: "identity", Outcome: "created",
		PayloadSHA256: hex.EncodeToString(certificateDigest[:])})
	return nil
}

func serve(address, certificatePath, keyPath, peerPath string, accepted, rejected int, timeout time.Duration) (result, error) {
	certificate, peer, err := loadMaterials(certificatePath, keyPath, peerPath)
	if err != nil {
		return result{}, err
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return result{}, err
	}
	defer listener.Close()
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stop()
	output := result{Schema: "ardents-r093-native-leg-v1", Role: "server", Address: listener.Addr().String()}
	for range accepted + rejected {
		raw, acceptErr := listener.Accept()
		if acceptErr != nil {
			return result{}, acceptErr
		}
		connection := tls.Server(raw, tlsConfig(certificate, peer, true))
		handshakeErr := connection.HandshakeContext(ctx)
		if handshakeErr == nil {
			handshakeErr = route.AcceptNodeLegBinding(connection, binding(false, false))
		}
		if handshakeErr != nil {
			_ = connection.Close()
			output.RejectedConnections++
			continue
		}
		payload, readErr := readFrame(connection)
		if readErr != nil {
			_ = connection.Close()
			return result{}, readErr
		}
		if writeErr := writeFrame(connection, payload); writeErr != nil {
			_ = connection.Close()
			return result{}, writeErr
		}
		state := connection.ConnectionState()
		output.TLSVersion, output.ALPN = tlsVersion(state.Version), state.NegotiatedProtocol
		output.CarriedConnections++
		output.PayloadBytes = len(payload)
		output.PayloadSHA256 = digest(payload)
		output.EchoSHA256 = output.PayloadSHA256
		if closeErr := connection.Close(); closeErr != nil {
			return result{}, closeErr
		}
	}
	if output.CarriedConnections != accepted || output.RejectedConnections != rejected {
		return result{}, fmt.Errorf("server results carried=%d rejected=%d, want %d/%d", output.CarriedConnections, output.RejectedConnections, accepted, rejected)
	}
	output.Outcome, output.ElapsedNanoseconds = "completed", time.Since(started).Nanoseconds()
	return output, nil
}

func carry(address, certificatePath, keyPath, peerPath string, size int, bindingMode string, expectRejection bool, timeout time.Duration) (result, error) {
	certificate, peer, err := loadMaterials(certificatePath, keyPath, peerPath)
	if err != nil {
		return result{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	started := time.Now()
	raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		return result{}, err
	}
	connection := tls.Client(raw, tlsConfig(certificate, peer, false))
	defer connection.Close()
	if err := connection.HandshakeContext(ctx); err != nil {
		return rejectionOrError(started, err, expectRejection)
	}
	changed := bindingMode == "changed-attachment"
	if err := route.ConfirmNodeLegBinding(connection, binding(true, changed)); err != nil {
		return rejectionOrError(started, err, expectRejection)
	}
	if expectRejection {
		return result{}, errors.New("client binding was accepted despite expected rejection")
	}
	payload := make([]byte, size)
	if _, err := rand.Read(payload); err != nil {
		return result{}, err
	}
	if err := writeFrame(connection, payload); err != nil {
		return result{}, err
	}
	echo, err := readFrame(connection)
	if err != nil {
		return result{}, err
	}
	if !bytes.Equal(payload, echo) {
		return result{}, errors.New("opaque payload echo is invalid")
	}
	state := connection.ConnectionState()
	return result{Schema: "ardents-r093-native-leg-v1", Role: "client", Outcome: "completed", Address: address,
		TLSVersion: tlsVersion(state.Version), ALPN: state.NegotiatedProtocol, CarriedConnections: 1,
		PayloadBytes: len(payload), PayloadSHA256: digest(payload), EchoSHA256: digest(echo),
		ElapsedNanoseconds: time.Since(started).Nanoseconds()}, nil
}

func rejectionOrError(started time.Time, err error, expected bool) (result, error) {
	if !expected {
		return result{}, err
	}
	return result{Schema: "ardents-r093-native-leg-v1", Role: "client", Outcome: "rejected", RejectedConnections: 1,
		ElapsedNanoseconds: time.Since(started).Nanoseconds(), Reason: err.Error()}, nil
}

func loadMaterials(certificatePath, keyPath, peerPath string) (tls.Certificate, ed25519.PublicKey, error) {
	certificate, err := tls.LoadX509KeyPair(certificatePath, keyPath)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	peerPEM, err := os.ReadFile(peerPath)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	block, rest := pem.Decode(peerPEM)
	if block == nil || block.Type != "CERTIFICATE" || len(rest) != 0 {
		return tls.Certificate{}, nil, errors.New("peer certificate PEM is invalid")
	}
	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	peer, ok := parsed.PublicKey.(ed25519.PublicKey)
	if !ok {
		return tls.Certificate{}, nil, errors.New("peer certificate key is not Ed25519")
	}
	return certificate, peer, nil
}

func tlsConfig(certificate tls.Certificate, expected ed25519.PublicKey, server bool) *tls.Config {
	output := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		NextProtos: []string{profile}, SessionTicketsDisabled: true, InsecureSkipVerify: true}
	if server {
		output.ClientAuth = tls.RequireAnyClientCert
	}
	output.VerifyConnection = func(state tls.ConnectionState) error {
		if state.Version != tls.VersionTLS13 || state.NegotiatedProtocol != profile || len(state.PeerCertificates) != 1 {
			return errors.New("synthetic native TLS contract is invalid")
		}
		peer, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
		if !ok || !peer.Equal(expected) {
			return errors.New("synthetic native TLS peer key is invalid")
		}
		return nil
	}
	return output
}

func binding(initiator, changed bool) route.LegBinding {
	first, second := identifier(4), identifier(5)
	firstRole, secondRole := byte(1), byte(3)
	if !initiator {
		first, second, firstRole, secondRole = second, first, secondRole, firstRole
	}
	attachment := identifier(10)
	if changed {
		attachment = identifier(11)
	}
	return route.LegBinding{NetworkID: identifier(1), Digest: identifier(2), AttachmentID: attachment, Epoch: 1,
		SenderRole: firstRole, PeerRole: secondRole, SenderNodeID: first, PeerNodeID: second, NotAfter: defaultNotAfter}
}

func readFrame(reader io.Reader) ([]byte, error) {
	var size [4]byte
	if _, err := io.ReadFull(reader, size[:]); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(size[:])
	if length == 0 || length > 1<<20 {
		return nil, errors.New("opaque payload frame is outside its bound")
	}
	payload := make([]byte, length)
	_, err := io.ReadFull(reader, payload)
	return payload, err
}

func writeFrame(writer io.Writer, payload []byte) error {
	if len(payload) == 0 || len(payload) > 1<<20 {
		return errors.New("opaque payload frame is outside its bound")
	}
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(payload)))
	if err := writeAll(writer, size[:]); err != nil {
		return err
	}
	return writeAll(writer, payload)
}

func writeAll(writer io.Writer, payload []byte) error {
	for len(payload) != 0 {
		written, err := writer.Write(payload)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		payload = payload[written:]
	}
	return nil
}

func writeExclusivePEM(path, kind string, value []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	encodeErr := pem.Encode(file, &pem.Block{Type: kind, Bytes: value})
	closeErr := file.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}

func tlsVersion(version uint16) string {
	if version == tls.VersionTLS13 {
		return "TLS 1.3"
	}
	return fmt.Sprintf("unknown-%d", version)
}

func digest(payload []byte) string {
	value := sha256.Sum256(payload)
	return hex.EncodeToString(value[:])
}

func identifier(value byte) (result [32]byte) { result[0] = value; return result }

func writeResult(output result) {
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
