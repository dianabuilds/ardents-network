//go:build linux

package connection

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// TLS, publication acceptance and local launch are explicit seams here.
// The tests exercise the real Instance proofs, Continuity MACs and stream owner.
type initialFlightCarrier struct {
	net.Conn
	writes atomic.Int32
}

func (carrier *initialFlightCarrier) Write(raw []byte) (int, error) {
	carrier.writes.Add(1)
	return carrier.Conn.Write(raw)
}

func initialAuthenticationFixture(t *testing.T, carrier net.Conn, client bool) (StreamConfig, InstanceAuthentication) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{37}, ed25519.SeedSize))
	identity := InstanceAuthentication{Network: [32]byte{1}, Target: [32]byte{2}, Generation: 3}
	copy(identity.Public[:], key.Public().(ed25519.PublicKey))
	if !client {
		identity.Signer = key
	}
	attachment, err := NewAttachment(carrier, 1, [32]byte{4}, [32]byte{5}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	t.Cleanup(cancel)
	t.Cleanup(func() { _ = carrier.Close() })
	return StreamConfig{Context: ctx, Application: &bufferApplication{}, NetworkID: identity.Network,
		Initial: attachment, ContinuityKey: [32]byte{6}, Authorized: time.Now(), Client: client}, identity
}

func TestInitialAuthenticationSingleFlightThenRealStream(t *testing.T) {
	left, right := net.Pipe()
	clientCarrier, publisherCarrier := &initialFlightCarrier{Conn: left}, &initialFlightCarrier{Conn: right}
	clientConfig, clientIdentity := initialAuthenticationFixture(t, clientCarrier, true)
	publisherConfig, publisherIdentity := initialAuthenticationFixture(t, publisherCarrier, false)
	clientApplication, clientUser := halfClosePair()
	publisherApplication, publisherUser := halfClosePair()
	defer clientUser.Close()
	defer publisherUser.Close()
	clientConfig.Application, publisherConfig.Application = clientApplication, publisherApplication
	type result struct {
		stream *Stream
		err    error
	}
	publisherDone := make(chan result, 1)
	go func() {
		stream, err := NewAuthenticatedStream(publisherConfig, publisherIdentity)
		publisherDone <- result{stream, err}
	}()
	client, err := NewAuthenticatedStream(clientConfig, clientIdentity)
	publisher := <-publisherDone
	if err != nil || publisher.err != nil {
		t.Fatalf("initial exchange: client=%v Publisher=%v", err, publisher.err)
	}
	if clientCarrier.writes.Load() != 1 || publisherCarrier.writes.Load() != 1 {
		t.Fatal("initial authentication used more than one flight per role")
	}
	// These are actual bounded stream lifecycles. A duplicate initial
	// Continuity would contradict the first Data/Terminal parser.
	results := runBoundedPair(boundedStreamPair{client: client, publisher: publisher.stream}, 32)
	sent := make(chan error, 1)
	go func() {
		_, err := clientUser.Write([]byte("one request"))
		sent <- errors.Join(err, clientUser.CloseInput())
	}()
	body, readErr := io.ReadAll(publisherUser)
	if readErr != nil || string(body) != "one request" {
		t.Fatalf("delivered request %q: %v", body, readErr)
	}
	if err := <-sent; err != nil {
		t.Fatal(err)
	}
	if err := publisherUser.CloseInput(); err != nil {
		t.Fatal(err)
	}
	if body, err := io.ReadAll(clientUser); err != nil || len(body) != 0 {
		t.Fatalf("response EOF: %q %v", body, err)
	}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	for _, stream := range []*Stream{client, publisher.stream} {
		stream.mu.Lock()
		received := stream.terminalAcknowledgedGeneration == 1
		confirmed := stream.terminalAckConfirmedGeneration == 1 && stream.terminalConfirmationSent
		stream.mu.Unlock()
		if !received || !confirmed {
			t.Fatalf("authenticated completion omitted Terminal receipt exchange: received=%v confirmed=%v", received, confirmed)
		}
	}
	if client.initialAuthentication != nil || publisher.stream.initialAuthentication != nil {
		t.Fatal("initial receipt was not consumed")
	}
}

type initialCountingSigner struct {
	key   ed25519.PrivateKey
	calls atomic.Int32
}

func (signer *initialCountingSigner) Public() crypto.PublicKey { return signer.key.Public() }
func (signer *initialCountingSigner) Sign(random io.Reader, raw []byte, opts crypto.SignerOpts) ([]byte, error) {
	signer.calls.Add(1)
	return signer.key.Sign(random, raw, opts)
}

func TestInitialAuthenticationPublisherRejectsBeforeSigning(t *testing.T) {
	for _, change := range []string{"target", "generation", "context", "role", "exporter", "mac", "offset", "early-data"} {
		t.Run(change, func(t *testing.T) {
			local, remote := net.Pipe()
			defer remote.Close()
			config, identity := initialAuthenticationFixture(t, local, false)
			signer := &initialCountingSigner{key: identity.Signer.(ed25519.PrivateKey)}
			identity.Signer = signer
			result := make(chan error, 1)
			go func() {
				stream, err := NewAuthenticatedStream(config, identity)
				if stream != nil {
					err = errors.New("unauthenticated stream returned")
				}
				result <- err
			}()
			challenge := Challenge{Network: identity.Network, Target: identity.Target, InstanceGeneration: identity.Generation,
				Context: config.Initial.context, Nonce: [32]byte{7}}
			continuity, err := NewContinuity(config.ContinuityKey, RoleClient, 1, 0, 0, 0, config.Initial.context, config.Initial.exporterCommitment)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "target":
				challenge.Target[0]++
			case "generation":
				challenge.InstanceGeneration++
			case "context":
				challenge.Context[0]++
			case "role":
				continuity.Role = RolePublisher
			case "exporter":
				continuity.ExporterCommitment[0]++
			case "mac":
				continuity.MAC[0]++
			case "offset":
				continuity, err = NewContinuity(config.ContinuityKey, RoleClient, 1, 0, 1, 0, config.Initial.context, config.Initial.exporterCommitment)
				if err != nil {
					t.Fatal(err)
				}
			}
			var flight bytes.Buffer
			if err := Write(&flight, Record{Challenge: &challenge}); err != nil {
				t.Fatal(err)
			}
			second := Record{Continuity: &continuity}
			if change == "early-data" {
				second = Record{Data: &Data{AttachmentGeneration: 1, Payload: []byte("early")}}
			}
			if err := Write(&flight, second); err != nil {
				t.Fatal(err)
			}
			_, _ = flight.WriteTo(remote) // Rejection can close before the full flight is read.
			if err := <-result; !errors.Is(err, ErrActiveViolation) {
				t.Fatalf("rejection: %v", err)
			}
			if signer.calls.Load() != 0 {
				t.Fatal("signed before complete client authentication")
			}
		})
	}
}

func TestInitialAuthenticationClientRequiresBothProofs(t *testing.T) {
	for _, invalid := range []string{"proof", "continuity", "offset", "missing"} {
		t.Run(invalid, func(t *testing.T) {
			local, remote := net.Pipe()
			config, identity := initialAuthenticationFixture(t, local, true)
			peerDone := make(chan error, 1)
			go func() {
				defer remote.Close()
				first, err := Read(remote)
				if err != nil {
					peerDone <- err
					return
				}
				if _, err = Read(remote); err != nil {
					peerDone <- err
					return
				}
				digest, err := ChallengeDigest(*first.Challenge)
				if err != nil {
					peerDone <- err
					return
				}
				key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{37}, ed25519.SeedSize))
				proof := Proof{ChallengeDigest: digest}
				copy(proof.Signature[:], ed25519.Sign(key, digest[:]))
				if invalid == "proof" {
					proof.Signature[0]++
				}
				continuity, err := NewContinuity(config.ContinuityKey, RolePublisher, 1, 0, 0, 0, config.Initial.context, config.Initial.exporterCommitment)
				if err != nil {
					peerDone <- err
					return
				}
				if invalid == "continuity" {
					continuity.MAC[0]++
				}
				if invalid == "offset" {
					continuity, err = NewContinuity(config.ContinuityKey, RolePublisher, 1, 1, 1, 0, config.Initial.context, config.Initial.exporterCommitment)
					if err != nil {
						peerDone <- err
						return
					}
				}
				var flight bytes.Buffer
				if err := Write(&flight, Record{Proof: &proof}); err != nil {
					peerDone <- err
					return
				}
				if invalid != "missing" {
					if err := Write(&flight, Record{Continuity: &continuity}); err != nil {
						peerDone <- err
						return
					}
				}
				_, _ = flight.WriteTo(remote)
				peerDone <- nil
			}()
			stream, err := NewAuthenticatedStream(config, identity)
			if err == nil || stream != nil {
				t.Fatal("incomplete authentication exposed stream")
			}
			if err := <-peerDone; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInitialAuthenticationCancellationJoinsCarrierClose(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	config, identity := initialAuthenticationFixture(t, local, true)
	ctx, cancel := context.WithCancel(config.Context)
	config.Context = ctx
	entered, release := make(chan struct{}), make(chan struct{})
	config.Initial.close = func() {
		_ = local.Close()
		select {
		case <-entered:
		default:
			close(entered)
		}
		<-release
	}
	result := make(chan error, 1)
	go func() { _, err := NewAuthenticatedStream(config, identity); result <- err }()
	// Drain the complete request flight, then deliberately withhold the
	// response. Cancellation must interrupt the real pending response read.
	if _, err := Read(remote); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(remote); err != nil {
		t.Fatal(err)
	}
	cancel()
	<-entered
	select {
	case <-result:
		t.Fatal("returned before cancellation transport cleanup joined")
	default:
	}
	close(release)
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestInitialAuthenticationCancelledBeforeRunRejectsReceipt(t *testing.T) {
	left, right := net.Pipe()
	config, identity := initialAuthenticationFixture(t, left, true)
	publisherConfig, publisherIdentity := initialAuthenticationFixture(t, right, false)
	ctx, cancel := context.WithCancel(config.Context)
	config.Context = ctx
	publisherDone := make(chan error, 1)
	go func() {
		stream, err := NewAuthenticatedStream(publisherConfig, publisherIdentity)
		if stream != nil {
			stream.close()
		}
		publisherDone <- err
	}()
	stream, err := NewAuthenticatedStream(config, identity)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.close()
	if err := <-publisherDone; err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := stream.establishInitialAttachment(); err == nil || stream.established {
		t.Fatal("cancelled context consumed receipt and established a stream")
	}
}
