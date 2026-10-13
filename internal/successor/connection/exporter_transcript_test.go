package connection

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"github.com/dianabuilds/ardents-network/internal/successor/connection/wire"
	"math/big"
	"net"
	"testing"
	"time"
)

func TestExporterIndependentKeyAndCommitmentDomains(t *testing.T) {
	t.Parallel()
	material := make([]byte, 32)
	for i := range material {
		material[i] = byte(i)
	}
	mac := hmac.New(sha256.New, material)
	mac.Write([]byte("ardents-service-connection-continuity-key-v3\x00"))
	var wantKey [32]byte
	copy(wantKey[:], mac.Sum(nil))
	wantExporter := sha256.Sum256(append([]byte("ardents-service-connection-exporter-v3\x00"), wantKey[:]...))
	wantContinuity := sha256.Sum256(append([]byte("ardents-service-connection-continuity-commitment-v3\x00"), wantKey[:]...))
	key, exporter, err := wire.DeriveExporterSecrets(material)
	if err != nil || key != wantKey || exporter != wantExporter || wire.ContinuityCommitment(key) != wantContinuity {
		t.Fatal("accepted key/commitment transcript differs")
	}
	material[0]++
	replacementKey, replacementExporter, err := wire.DeriveExporterSecrets(material)
	if err != nil || replacementKey == key || replacementExporter == exporter {
		t.Fatal("fresh exporter did not change Attachment facts")
	}
	if err := wire.VerifyContinuity(key, mustContinuity(t, key, exporter), wire.RoleInitiator, 1, [32]byte{9}, replacementExporter); err == nil {
		t.Fatal("foreign Attachment exporter accepted")
	}
	for _, size := range []int{0, 31, 33, 64} {
		if _, _, err := wire.DeriveExporterSecrets(make([]byte, size)); err == nil {
			t.Fatal("wrong exporter length accepted")
		}
	}
}

// This positive isolates the real TLS exporter, with an ephemeral fixture
// certificate. It supplies no Route JOIN, Instance lease, local permission or
// initial authentication receipt and therefore attests no Service readiness.
func TestRealTLSExporterBindsExactAttachmentContext(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(private)
	now := time.Now()
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, public, private)
	if err != nil {
		t.Fatal(err)
	}
	rawClient, rawPublisher := net.Pipe()
	defer rawClient.Close()
	defer rawPublisher.Close()
	var expectedInstance [32]byte
	copy(expectedInstance[:], public)
	client := tls.Client(rawClient, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true, SessionTicketsDisabled: true, CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519},
		VerifyConnection: func(state tls.ConnectionState) error { return verifyInstanceCertificate(state, expectedInstance) }})
	publisher := tls.Server(rawPublisher, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, SessionTicketsDisabled: true, CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519}, Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: private}}})
	if err := rawClient.SetDeadline(now.Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := rawPublisher.SetDeadline(now.Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- publisher.HandshakeContext(t.Context()) }()
	clientErr := client.HandshakeContext(t.Context())
	if clientErr != nil {
		rawClient.Close()
	}
	publisherErr := <-result
	if clientErr != nil || publisherErr != nil {
		t.Fatalf("real TLS handshake: %v / %v", clientErr, publisherErr)
	}
	context := [32]byte{7}
	clientKey, clientCommitment, err := wire.AttachmentSecrets(client, context)
	if err != nil {
		t.Fatal(err)
	}
	publisherKey, publisherCommitment, err := wire.AttachmentSecrets(publisher, context)
	if err != nil || clientKey != publisherKey || clientCommitment != publisherCommitment {
		t.Fatal("opposite exporter transcripts differ")
	}
	foreignKey, foreignCommitment, err := wire.AttachmentSecrets(publisher, [32]byte{8})
	if err != nil || foreignKey == clientKey || foreignCommitment == clientCommitment {
		t.Fatal("foreign Attachment context reused exporter")
	}
	if wire.ExporterLabel != "EXPORTER-ardents-service-connection-v3" {
		t.Fatal("accepted TLS exporter label differs")
	}
	// Genuine TLS, isolated from Route/Publication/local permission. Receipt
	// checks cannot manufacture any of those neighboring owners.
	publisherBinding, err := wire.BindTLS(publisher, context)
	if err != nil {
		t.Fatal(err)
	}
	clientBinding, err := wire.BindTLS(client, context)
	if err != nil {
		t.Fatal(err)
	}
	expected := wire.Challenge{Network: [32]byte{1}, Target: [32]byte{2}, InstanceGeneration: 3, Context: [32]byte{9}}
	challenge := expected
	challenge.Nonce = [32]byte{10}
	continuity := mustContinuity(t, clientKey, clientCommitment)
	invalid := continuity
	invalid.MAC[0]++
	if _, err := wire.VerifyInitialRequest(publisherBinding, expected, challenge, invalid); err == nil {
		t.Fatal("altered Continuity supplied signing receipt")
	}
	invalid = continuity
	invalid.SendEnd = 1
	if _, err := wire.VerifyInitialRequest(publisherBinding, expected, challenge, invalid); err == nil {
		t.Fatal("nonzero initial offsets supplied signing receipt")
	}
	receipt, err := wire.VerifyInitialRequest(publisherBinding, expected, challenge, continuity)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := receipt.Consume(clientBinding, expected); err == nil {
		t.Fatal("other TLS object consumed Publisher receipt")
	}
	if actual, err := receipt.Consume(publisherBinding, expected); err != nil || actual != challenge {
		t.Fatal("original TLS receipt refused", err)
	}
	if _, err := receipt.Consume(publisherBinding, expected); err == nil {
		t.Fatal("initial receipt used twice")
	}
}

func mustContinuity(t *testing.T, key, exporter [32]byte) wire.Continuity {
	t.Helper()
	c, err := wire.NewContinuity(key, wire.RoleInitiator, 1, 0, 0, 0, [32]byte{9}, exporter)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestExporterRefusesUnfinishedResumedAndForeignTLSState(t *testing.T) {
	t.Parallel()
	for _, state := range []tls.ConnectionState{
		{}, {Version: tls.VersionTLS12, HandshakeComplete: true},
		{Version: tls.VersionTLS13, HandshakeComplete: true, DidResume: true},
	} {
		if err := wire.ValidateExporterState(state, [32]byte{1}); err == nil {
			t.Fatal("unsupported TLS state accepted")
		}
	}
	if err := wire.ValidateExporterState(tls.ConnectionState{Version: tls.VersionTLS13, HandshakeComplete: true}, [32]byte{}); err == nil {
		t.Fatal("missing Attachment context accepted")
	}
	if _, _, err := wire.AttachmentSecrets(nil, [32]byte{1}); err == nil {
		t.Fatal("absent TLS accepted")
	}
	if _, _, err := wire.AttachmentSecrets(&tls.Conn{}, [32]byte{1}); err == nil {
		t.Fatal("zero TLS connection accepted")
	}
}
