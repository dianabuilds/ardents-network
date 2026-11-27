package connection

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"math/big"
	"net"
	"os"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/connection/recordio"
	"github.com/dianabuilds/ardents-network/internal/domain/connection/wire"
)

// This isolates certificate possession and the exact-key TLS verifier. The
// ephemeral fixture is not Publication/Execution authority or a genuine JOIN.
func TestRealTLSHandshakeRejectsForeignInstance(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(private)
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	var expected [32]byte
	copy(expected[:], public)
	expected[0] ^= 1
	rawClient, rawPublisher := net.Pipe()
	defer rawClient.Close()
	defer rawPublisher.Close()
	if err := rawClient.SetDeadline(now.Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := rawPublisher.SetDeadline(now.Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	client := newInstanceTLSClient(rawClient, expected)
	publisher := tls.Server(rawPublisher, &tls.Config{
		MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, SessionTicketsDisabled: true,
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.X25519},
		Certificates:     []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: private}},
	})
	joined := make(chan error, 1)
	go func() { joined <- publisher.HandshakeContext(t.Context()) }()
	clientErr := client.HandshakeContext(t.Context())
	// Interrupt the exact fixture peer and join its original handshake before
	// ending the test, whether the verifier rejected or accidentally accepted.
	rawClient.Close()
	publisherErr := <-joined
	if clientErr == nil || !strings.Contains(clientErr.Error(), "key differs from verified Publication") || client.ConnectionState().HandshakeComplete {
		t.Fatalf("foreign Instance was not refused by key binding: client=%v peer=%v", clientErr, publisherErr)
	}
}

func TestInstanceCertificateRequiresExactSingleEd25519Key(t *testing.T) {
	t.Parallel()
	expected := [32]byte{9}
	certificate := &x509.Certificate{PublicKey: ed25519.PublicKey(expected[:])}
	valid := tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{certificate}}
	if err := verifyInstancePeer(valid, expected); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		name  string
		state tls.ConnectionState
		key   [32]byte
	}{
		{"absent expected key", valid, [32]byte{}},
		{"foreign expected key", valid, [32]byte{8}},
		{"missing certificate", tls.ConnectionState{Version: tls.VersionTLS13}, expected},
		{"nil certificate", tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{nil}}, expected},
		{"multiple certificates", tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{certificate, certificate}}, expected},
		{"TLS 1.2", tls.ConnectionState{Version: tls.VersionTLS12, PeerCertificates: valid.PeerCertificates}, expected},
		{"resumed", tls.ConnectionState{Version: tls.VersionTLS13, DidResume: true, PeerCertificates: valid.PeerCertificates}, expected},
		{"wrong algorithm", tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{{PublicKey: &rsa.PublicKey{}}}}, expected},
		{"short key", tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{{PublicKey: ed25519.PublicKey{9}}}}, expected},
	} {
		t.Run(input.name, func(t *testing.T) {
			if err := verifyInstancePeer(input.state, input.key); err == nil {
				t.Fatal("foreign or unsupported Instance certificate accepted")
			}
		})
	}
}

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
	client := newInstanceTLSClient(rawClient, expectedInstance)
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

// This is the failed authentication-record boundary, not successful Service
// authority. The installed reproduction separately supplies real original
// Startup, Job, JOIN and TLS owners. Virtual time preserves their 60s/30s bounds
// while the same setup callback interrupts the physical reader.
func TestInitialSetupReadRetainsDeadlineBeforeAttachmentCancellation(t *testing.T) {
	for _, failure := range []struct {
		name string
		err  error
	}{{"original-io-timeout", os.ErrDeadlineExceeded}, {"cancellation-only", nil}} {
		t.Run(failure.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				original, cancelOriginal := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancelOriginal()
				setup, cancelSetup := context.WithTimeout(original, 30*time.Second)
				defer cancelSetup()
				logical, cancelLogical, physical, cancelPhysical := initiatorLifetimes(original, time.Now().Add(60*time.Second))
				defer cancelLogical()
				defer cancelPhysical()
				callbackDone := make(chan struct{})
				stop := context.AfterFunc(setup, func() {
					defer close(callbackDone)
					cancelLogical()
				})
				defer func() {
					if !stop() {
						<-callbackDone
					}
				}()
				flight := recordio.NewInitialResponseReader(setupInterruptedFlight{physical: physical, failure: failure.err})
				record, err := readInitialResponseRecord(setup, flight)
				if original.Err() != nil || logical.Err() != context.Canceled || setup.Err() != context.DeadlineExceeded {
					t.Fatal("fixture lost the original caller/setup/physical distinction", original.Err(), setup.Err(), logical.Err())
				}
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal("setup deadline became only Attachment cancellation", err)
				}
				if failure.err != nil && !errors.Is(err, failure.err) || !errors.Is(err, context.Canceled) || record.Proof != nil || record.Continuity != nil {
					t.Fatal("failed original read or refusal was lost", record, err)
				}
			})
		})
	}
}

type setupInterruptedFlight struct {
	physical context.Context
	failure  error
}

func (reader setupInterruptedFlight) Read([]byte) (int, error) {
	<-reader.physical.Done()
	return 0, errors.Join(reader.failure, reader.physical.Err())
}

func TestInitialSetupReadKeepsIndependentPhysicalAndGrammarFailures(t *testing.T) {
	authorityLoss := errors.New("independent current authority loss")
	for _, failure := range []error{io.EOF, io.ErrUnexpectedEOF, io.ErrClosedPipe, authorityLoss, errors.Join(io.ErrClosedPipe, authorityLoss)} {
		flight := recordio.NewInitialResponseReader(initialSetupReadFunc(func([]byte) (int, error) { return 0, failure }))
		record, err := readInitialResponseRecord(t.Context(), flight)
		if !errors.Is(err, failure) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errIntegrity) || record.Proof != nil {
			t.Fatal("live setup changed an independent failure into cancellation, expiry or integrity", failure, err)
		}
	}
	// A fully observed invalid prefix remains a structural contradiction.
	malformed := append([]byte("ardents-service-connection-v3\x01"), 0, 0)
	flight := recordio.NewInitialResponseReader(bytes.NewReader(malformed))
	if _, err := readInitialResponseRecord(t.Context(), flight); !errors.Is(err, wire.ErrInvalidRecord) || !errors.Is(err, errIntegrity) {
		t.Fatal("complete malformed record lost its integrity failure", err)
	}
}

func TestInitialSetupReadRefusesLateCompleteProofAndAlreadyCanceledCaller(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var encoded bytes.Buffer
	if err := wire.Write(&encoded, wire.Record{Proof: &wire.Proof{ChallengeDigest: [32]byte{1}, Signature: [64]byte{2}}}); err != nil {
		t.Fatal(err)
	}
	input := bytes.NewReader(encoded.Bytes())
	flight := recordio.NewInitialResponseReader(initialSetupReadFunc(func(body []byte) (int, error) {
		n, err := input.Read(body)
		cancel()
		return n, err
	}))
	if record, err := readInitialResponseRecord(ctx, flight); !errors.Is(err, context.Canceled) || errors.Is(err, errIntegrity) || record.Proof != nil {
		t.Fatal("late complete proof survived its original caller", record, err)
	}
	reads := 0
	flight = recordio.NewInitialResponseReader(initialSetupReadFunc(func([]byte) (int, error) {
		reads++
		return 0, io.EOF
	}))
	if _, err := readInitialResponseRecord(ctx, flight); !errors.Is(err, context.Canceled) || reads != 0 {
		t.Fatal("already canceled setup performed physical input", reads, err)
	}
}

func TestInitialSetupReadEarlierOriginalDeadlineAndLatePhysicalFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		original, cancelOriginal := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelOriginal()
		setup, cancelSetup := context.WithTimeout(original, 30*time.Second)
		defer cancelSetup()
		physicalFailure := errors.New("original late read failure")
		release := make(chan struct{})
		done := make(chan error, 1)
		flight := recordio.NewInitialResponseReader(initialSetupReadFunc(func([]byte) (int, error) {
			<-setup.Done()
			<-release
			return 0, physicalFailure
		}))
		go func() {
			_, err := readInitialResponseRecord(setup, flight)
			done <- err
		}()
		time.Sleep(10 * time.Second)
		synctest.Wait()
		select {
		case err := <-done:
			t.Fatal("deadline was treated as joined original input", err)
		default:
		}
		close(release)
		err := <-done
		if original.Err() != context.DeadlineExceeded || !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, physicalFailure) || errors.Is(err, context.Canceled) {
			t.Fatal("earlier original deadline or late physical failure was changed", original.Err(), err)
		}
	})
}

type initialSetupReadFunc func([]byte) (int, error)

func (read initialSetupReadFunc) Read(body []byte) (int, error) { return read(body) }
