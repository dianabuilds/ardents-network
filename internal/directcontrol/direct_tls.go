package directcontrol

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

const (
	directServerName  = "carrier.invalid"
	directALPN        = "carrier-lab-direct/1"
	directRecordLimit = 1024 * 1024
	directDeadline    = 10 * time.Second
)

type directObservation struct {
	TLSVersion               string
	Curve                    string
	CipherSuite              string
	SessionResumed           bool
	SNI                      string
	ALPN                     string
	CanarySHA256             string
	PayloadSHA256            string
	PayloadBytes             int
	ApplicationBytesVerified bool
}

func runDirectTLSRole(ctx context.Context, config directRoleConfig, evidenceDir string) (directObservation, error) {
	if config.Role == "service" {
		return runDirectService(ctx, config, evidenceDir)
	}
	return runDirectUser(ctx, config)
}

func runDirectService(ctx context.Context, config directRoleConfig, evidenceDir string) (directObservation, error) {
	certificate, err := tls.LoadX509KeyPair(config.CertificatePath, config.PrivateKeyPath)
	if err != nil {
		return directObservation{}, fmt.Errorf("load active Instance fixture: %w", err)
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", config.Address)
	if err != nil {
		return directObservation{}, fmt.Errorf("listen for Direct TLS control: %w", err)
	}
	defer listener.Close()
	stopClose := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stopClose()
	if err := writeDirectJSON(evidenceDir+string(os.PathSeparator)+"ready.json", map[string]string{
		"schema_version": directResultSchema, "run_id": config.RunID, "case": config.Case, "role": config.Role, "status": "ready",
	}); err != nil {
		return directObservation{}, err
	}
	connection, err := acceptDirectConnection(listener, directDeadline)
	if err != nil {
		return directObservation{}, err
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(directDeadline)); err != nil {
		return directObservation{}, err
	}
	secured := tls.Server(connection, &tls.Config{
		Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519}, NextProtos: []string{directALPN}, SessionTicketsDisabled: true,
	})
	if err := secured.HandshakeContext(ctx); err != nil {
		return directObservation{}, fmt.Errorf("direct TLS service handshake: %w", err)
	}
	state := secured.ConnectionState()
	canary, err := readDirectRecord(secured, 32)
	if err != nil || len(canary) != 32 {
		return observationFromState(state), errors.New("direct TLS canary was not a complete 32-byte record")
	}
	payload, err := readDirectRecord(secured, directRecordLimit)
	if err != nil {
		return observationFromState(state), fmt.Errorf("read protected Application record: %w", err)
	}
	if err := writeDirectRecord(secured, canary); err != nil {
		return observationFromState(state), err
	}
	if err := writeDirectRecord(secured, payload); err != nil {
		return observationFromState(state), err
	}
	observation := observationFromState(state)
	observation.recordVerifiedApplication(canary, payload)
	return observation, nil
}

func runDirectUser(ctx context.Context, config directRoleConfig) (directObservation, error) {
	rootPEM, err := os.ReadFile(config.TargetRootPath)
	if err != nil {
		return directObservation{}, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(rootPEM) {
		return directObservation{}, errors.New("target root fixture is not a certificate")
	}
	expectedLeaf, err := hex.DecodeString(config.ExpectedLeafSHA256)
	if err != nil || len(expectedLeaf) != sha256.Size {
		return directObservation{}, errors.New("expected Instance leaf digest is invalid")
	}
	connection, err := (&net.Dialer{Timeout: directDeadline}).DialContext(ctx, "tcp", config.Address)
	if err != nil {
		return directObservation{}, fmt.Errorf("connect Direct TLS control: %w", err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(directDeadline)); err != nil {
		return directObservation{}, err
	}
	secured := tls.Client(connection, &tls.Config{
		RootCAs: roots, ServerName: directServerName, MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519}, NextProtos: []string{directALPN},
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) != 1 && len(state.PeerCertificates) != 2 {
				return errors.New("unexpected Instance certificate chain")
			}
			observed := sha256.Sum256(state.PeerCertificates[0].Raw)
			if !equalDirectBytes(observed[:], expectedLeaf) {
				return errors.New("active Instance leaf does not match the pinned fixture")
			}
			return nil
		},
	})
	if err := secured.HandshakeContext(ctx); err != nil {
		return directObservation{}, fmt.Errorf("authenticate exact Target/Instance: %w", err)
	}
	state := secured.ConnectionState()
	if err := validateDirectTLSState(state); err != nil {
		return observationFromState(state), err
	}
	canary, err := hex.DecodeString(config.CanaryHex)
	if err != nil || len(canary) != 32 {
		return observationFromState(state), errors.New("fresh canary must be exactly 32 bytes")
	}
	payload := directPayload(config.PayloadSeed, config.PayloadSize)
	if err := writeDirectRecord(secured, canary); err != nil {
		return observationFromState(state), err
	}
	if err := writeDirectRecord(secured, payload); err != nil {
		return observationFromState(state), err
	}
	returnedCanary, err := readDirectRecord(secured, 32)
	if err != nil || !equalDirectBytes(returnedCanary, canary) {
		return observationFromState(state), errors.New("direct TLS canary response was not verified")
	}
	returnedPayload, err := readDirectRecord(secured, directRecordLimit)
	if err != nil || !equalDirectBytes(returnedPayload, payload) {
		return observationFromState(state), errors.New("protected Application bytes were not verified in order")
	}
	observation := observationFromState(state)
	observation.recordVerifiedApplication(canary, payload)
	return observation, nil
}

func acceptDirectConnection(listener net.Listener, timeout time.Duration) (net.Conn, error) {
	if tcpListener, ok := listener.(*net.TCPListener); ok {
		if err := tcpListener.SetDeadline(time.Now().Add(timeout)); err != nil {
			return nil, err
		}
	}
	connection, err := listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("accept Direct TLS control: %w", err)
	}
	return connection, nil
}

func validateDirectTLSState(state tls.ConnectionState) error {
	if state.Version != tls.VersionTLS13 || state.CurveID != tls.X25519 || state.DidResume || state.ServerName != directServerName || state.NegotiatedProtocol != directALPN {
		return errors.New("direct TLS negotiated outside the fixed TLS 1.3/X25519/SNI/ALPN contract")
	}
	return nil
}

func observationFromState(state tls.ConnectionState) directObservation {
	return directObservation{
		TLSVersion: directTLSVersion(state.Version), Curve: state.CurveID.String(), CipherSuite: tls.CipherSuiteName(state.CipherSuite),
		SessionResumed: state.DidResume, SNI: state.ServerName, ALPN: state.NegotiatedProtocol,
	}
}

func (observation *directObservation) recordVerifiedApplication(canary, payload []byte) {
	canaryDigest := sha256.Sum256(canary)
	payloadDigest := sha256.Sum256(payload)
	observation.CanarySHA256 = hex.EncodeToString(canaryDigest[:])
	observation.PayloadSHA256 = hex.EncodeToString(payloadDigest[:])
	observation.PayloadBytes = len(payload)
	observation.ApplicationBytesVerified = true
}

func directTLSVersion(version uint16) string {
	if version == tls.VersionTLS13 {
		return "TLS1.3"
	}
	return fmt.Sprintf("0x%04x", version)
}

func writeDirectRecord(writer io.Writer, payload []byte) error {
	if len(payload) > directRecordLimit {
		return errors.New("direct TLS Application record exceeds its fixed limit")
	}
	header := [4]byte{}
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	if err := writeDirectBytes(writer, header[:]); err != nil {
		return err
	}
	return writeDirectBytes(writer, payload)
}

func readDirectRecord(reader io.Reader, maximum int) ([]byte, error) {
	header := [4]byte{}
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, err
	}
	size := int(binary.BigEndian.Uint32(header[:]))
	if size < 0 || size > maximum {
		return nil, errors.New("direct TLS Application record has an invalid length")
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func directPayload(seed string, size int) []byte {
	payload := make([]byte, size)
	var counter uint64
	for offset := 0; offset < len(payload); {
		input := make([]byte, len(seed)+8)
		copy(input, seed)
		binary.BigEndian.PutUint64(input[len(seed):], counter)
		block := sha256.Sum256(input)
		offset += copy(payload[offset:], block[:])
		counter++
	}
	return payload
}

func equalDirectBytes(left, right []byte) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare(left, right) == 1
}

func writeDirectBytes(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}
