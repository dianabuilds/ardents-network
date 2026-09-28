//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
	"github.com/dianabuilds/ardents-network/internal/endpoint/service"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	nativeconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

// This is the first protected-text recovery seam: Service TLS, the native
// Connection and the Application exchange are real, while the already
// authorized Route replacement is supplied as an opaque Attachment transport.
// The full protected Route opener is integrated at the next boundary.
func TestTextServiceRecoveryDoesNotReplayAcceptedDocumentRequest(t *testing.T) {
	client, publisher, _ := serviceFixture(t)
	initialClient, initialPublisher := net.Pipe()
	replacementClient, replacementPublisher := net.Pipe()
	// Recovery performs a fresh protected Route attachment under the race
	// detector. Keep the test harness deadline above the accepted operation's
	// own bounded stages so a loaded runner does not cancel valid recovery.
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	testOwner := newServiceRecoveryTestOwner(t, cancel, initialClient, initialPublisher, replacementClient, replacementPublisher)
	testOwner.retainContext(client.owner)
	testOwner.retainContext(publisher.owner)
	t.Cleanup(testOwner.Close)

	digest := fixtureID(91)
	requests := make(chan nativeconnection.Recovery, 2)
	clientOpener := service.AttachmentOpener(func(ctx context.Context, request nativeconnection.Recovery) (net.Conn, [32]byte, error) {
		select {
		case requests <- request:
		case <-ctx.Done():
			return nil, [32]byte{}, ctx.Err()
		}
		return replacementClient, digest, nil
	})
	publisherOpener := service.AttachmentOpener(func(ctx context.Context, request nativeconnection.Recovery) (net.Conn, [32]byte, error) {
		select {
		case requests <- request:
		case <-ctx.Done():
			return nil, [32]byte{}, ctx.Err()
		}
		return replacementPublisher, digest, nil
	})

	type opened struct {
		stream *service.Stream
		err    error
	}
	publisherOpened := make(chan opened, 1)
	testOwner.Go(func() {
		stream, err := publisher.openServiceStreamWithRecovery(ctx, initialPublisher, fixtureID(90), publisherOpener)
		testOwner.retainStream(stream)
		publisherOpened <- opened{stream: stream, err: err}
	})
	clientStream, err := client.openServiceStreamWithRecovery(ctx, initialClient, fixtureID(90), clientOpener)
	testOwner.retainStream(clientStream)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	remote := <-publisherOpened
	if remote.err != nil {
		cancel()
		t.Fatal(remote.err)
	}

	body := bytes.Repeat([]byte("r"), 64<<10)
	snapshot, err := textdocument.NewSnapshot(body)
	if err != nil {
		t.Fatal(err)
	}
	requestAccepted := make(chan struct{})
	publisherDone := make(chan error, 1)
	testOwner.Go(func() {
		reader := &interruptAfterAcceptedRequest{Reader: remote.stream, accepted: requestAccepted,
			interrupt: func() { _ = initialPublisher.Close() }}
		err := snapshot.Respond(reader, remote.stream)
		if err == nil {
			err = remote.stream.CloseInput()
		}
		if err == nil {
			outcome := <-remote.stream.Done()
			if outcome.Class != applicationconnection.CleanClose {
				err = errors.New("recovered Publisher stream did not close cleanly")
			}
		}
		publisherDone <- errors.Join(err, remote.stream.Close())
	})

	received, readErr := textdocument.Read(ctx, clientStream)
	select {
	case <-requestAccepted:
	default:
		t.Fatal("Publisher did not accept the original document request")
	}
	publisherErr := <-publisherDone
	if err := errors.Join(readErr, publisherErr); err != nil {
		t.Fatalf("protected text recovery failed: %v", err)
	}
	if !bytes.Equal(received, body) {
		t.Fatalf("recovered body length = %d, want %d", len(received), len(body))
	}
	for range 2 {
		request := <-requests
		if request.Generation != 2 || request.NetworkID != client.facts.Network ||
			request.CandidateView != client.candidateView || request.RouteProfile != nativeconnection.Profile ||
			request.WorkSafetyNotAfter != client.facts.WorkSafetyNotAfter ||
			request.WorkSafetyMaximum != client.facts.WorkSafetyMaximum ||
			request.NoNewRecoveryAfter != client.facts.NoNewRecoveryAfter {
			t.Fatalf("replacement lost immutable authority: %+v", request)
		}
	}
}

func TestTextJoinedServiceRecoversAcceptedRequestAcrossFreshProtectedRoute(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			reader, publisher, destination := joinedNetworkFixture(t, carrier)
			readerJob, publisherJob := liveCapsuleJob(t, reader), liveCapsuleJob(t, publisher)
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			testOwner := newServiceRecoveryTestOwner(t, cancel)
			testOwner.retainContext(reader)
			testOwner.retainContext(publisher)
			t.Cleanup(testOwner.Close)
			now := time.Now().UTC()
			bounds := [3]int64{now.Add(25 * time.Second).Unix(), now.Add(25 * time.Second).Unix(), now.Add(25 * time.Second).Unix()}
			prepared, err := reader.prepareIntroduction(ctx, readerJob, destination, bounds)
			if err != nil {
				t.Fatal(err)
			}

			type joined struct {
				attempt *introductionAttempt
				raw     *joinedTransport
				err     error
			}
			publisherJoined := make(chan joined, 1)
			testOwner.Go(func() {
				accepted, err := publisher.receiveIntroduction(ctx, publisherJob)
				if err != nil {
					publisherJoined <- joined{err: err}
					return
				}
				raw, err := publisher.openJoinedTransport(ctx, publisherJob, accepted)
				publisherJoined <- joined{attempt: accepted, raw: raw, err: err}
			})
			readerRaw, err := reader.openJoinedTransport(ctx, readerJob, prepared)
			testOwner.retainConnection(readerRaw)
			remoteJoined := <-publisherJoined
			testOwner.retainConnection(remoteJoined.raw)
			if err != nil {
				cancel()
				t.Fatal(errors.Join(err, remoteJoined.err))
			}
			if remoteJoined.err != nil {
				cancel()
				t.Fatal(remoteJoined.err)
			}

			clientRecovery := &observedServiceOpener{open: reader.serviceRouteRecoveryOpener(readerJob, prepared.binding)}
			publisherRecovery := &observedServiceOpener{open: publisher.serviceRouteRecoveryOpener(publisherJob, remoteJoined.attempt.binding)}
			publisherOpened := make(chan openedService, 1)
			testOwner.Go(func() {
				stream, err := remoteJoined.attempt.binding.openServiceStreamWithRecovery(ctx, remoteJoined.raw,
					remoteJoined.attempt.digest, publisherRecovery.openObserved)
				testOwner.retainStream(stream)
				publisherOpened <- openedService{stream: stream, err: err}
			})
			clientStream, err := prepared.binding.openServiceStreamWithRecovery(ctx, readerRaw, prepared.digest,
				clientRecovery.openObserved)
			testOwner.retainStream(clientStream)
			if err != nil {
				cancel()
				t.Fatal(errors.Join(err, (<-publisherOpened).err))
			}
			remote := <-publisherOpened
			if remote.err != nil {
				cancel()
				t.Fatal(remote.err)
			}
			stopInitialReceiver := holdInitialIntroductionReceiver(t, ctx, publisher, publisherJob)
			initialTokens := tokenAttemptSnapshot(t, reader.endpoint)

			body := bytes.Repeat([]byte("network recovery\n"), 4096)
			snapshot, err := textdocument.NewSnapshot(body)
			if err != nil {
				t.Fatal(err)
			}
			accepted := make(chan struct{})
			publisherDone := make(chan error, 1)
			testOwner.Go(func() {
				reader := &interruptAfterAcceptedRequest{Reader: remote.stream, accepted: accepted,
					interrupt: func() { _ = remoteJoined.raw.Close() }}
				err := snapshot.Respond(reader, remote.stream)
				if err == nil {
					err = remote.stream.CloseInput()
				}
				if err == nil {
					outcome := <-remote.stream.Done()
					if outcome.Class != applicationconnection.CleanClose {
						err = errors.New("protected-route Publisher stream did not close cleanly")
					}
				}
				// Keep the terminal-control tail alive through observation. Closing
				// either side here would introduce a second physical failure into
				// this one-failure recovery test; separate tests own tail loss.
				publisherDone <- err
			})

			received, readErr := textdocument.Read(ctx, clientStream)
			if readErr != nil {
				cancel()
			}
			publisherErr := <-publisherDone
			if err := errors.Join(readErr, publisherErr); err != nil || !bytes.Equal(received, body) {
				t.Fatalf("recovered protected Route document=%d/%d: %v; client route=%v; Publisher route=%v",
					len(received), len(body), err, clientRecovery.outcome(), publisherRecovery.outcome())
			}
			recoveredTokens := tokenAttemptSnapshot(t, reader.endpoint)
			assertFreshRecoveryTokenAttempts(t, initialTokens, recoveredTokens)
			clientAttempts, clientDigests, clientRouteErr := clientRecovery.observation()
			publisherAttempts, publisherDigests, publisherRouteErr := publisherRecovery.observation()
			if clientAttempts != 1 || publisherAttempts != 1 || len(clientDigests) != 1 || len(publisherDigests) != 1 ||
				clientRouteErr != nil || publisherRouteErr != nil || clientDigests[0] != publisherDigests[0] ||
				clientDigests[0] == prepared.digest || publisherDigests[0] == remoteJoined.attempt.digest {
				t.Fatalf("recovery did not use one matching fresh Route: client=%v Publisher=%v",
					clientRecovery.outcome(), publisherRecovery.outcome())
			}
			cancel()
			stopInitialReceiver()
			if err := errors.Join(clientStream.Close(), remote.stream.Close()); err != nil {
				t.Fatalf("recovered Route cleanup: %v", err)
			}
			clientAttempts, clientDigests, clientRouteErr = clientRecovery.observation()
			publisherAttempts, publisherDigests, publisherRouteErr = publisherRecovery.observation()
			if clientAttempts < 1 || clientAttempts > 2 || publisherAttempts < 1 || publisherAttempts > 2 ||
				len(clientDigests) != 1 || len(publisherDigests) != 1 ||
				clientRouteErr != nil && !errors.Is(clientRouteErr, context.Canceled) ||
				publisherRouteErr != nil && !errors.Is(publisherRouteErr, context.Canceled) {
				t.Fatalf("recovery cleanup revived a Route instead of joining cancellation: client=%v Publisher=%v",
					clientRecovery.outcome(), publisherRecovery.outcome())
			}
			if afterCleanup := tokenAttemptSnapshot(t, reader.endpoint); !sameTokenAttemptSnapshot(recoveredTokens, afterCleanup) {
				t.Fatal("recovery cleanup spent another receiver token")
			}
			for _, owner := range []*dutyContext{reader, publisher} {
				owner.mu.Lock()
				pending := introduction.ActiveExchangeCount(&owner.introduction.exchanges)
				owner.mu.Unlock()
				if pending != 0 {
					t.Errorf("recovery retained %d Introduction exchanges", pending)
				}
			}
		})
	}
}

func TestTextRecoveryPreparesFreshAttachmentUnderRetainedAuthority(t *testing.T) {
	reader, _, destination := joinedNetworkFixture(t, routecarrier.ClosedCarrierTCP)
	job := liveCapsuleJob(t, reader)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	now := time.Now().UTC()
	bounds := [3]int64{now.Add(12 * time.Second).Unix(), now.Add(12 * time.Second).Unix(), now.Add(12 * time.Second).Unix()}
	initial, err := reader.prepareIntroduction(ctx, job, destination, bounds)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(initial.operation)
	request := initial.binding.serviceRecovery()
	request.Generation, request.Role, request.Deadline = 2, "client", now.Add(10*time.Second).UTC().Truncate(time.Second)
	first, err := reader.prepareRecovery(ctx, job, initial.binding, request)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(first.operation)
	second, err := reader.prepareRecovery(ctx, job, initial.binding, request)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(second.operation)

	for index, attempt := range []*introductionAttempt{first, second} {
		facts := attempt.plaintext
		if facts.Network != initial.binding.facts.Network || facts.Target != initial.binding.facts.Target ||
			facts.PublicationDigest != initial.binding.facts.PublicationDigest || facts.ProfileDigest != initial.binding.facts.ProfileDigest ||
			facts.ConnectionNonce != initial.binding.facts.ConnectionNonce || facts.InitiatorBinding != initial.binding.facts.InitiatorBinding ||
			facts.WorkSafetyNotAfter != initial.binding.facts.WorkSafetyNotAfter || facts.WorkSafetyMaximum != initial.binding.facts.WorkSafetyMaximum ||
			facts.NoNewRecoveryAfter != initial.binding.facts.NoNewRecoveryAfter || facts.AttachmentGeneration != request.Generation ||
			facts.RendezvousNode != initial.plaintext.RendezvousNode || facts.RendezvousDutyGeneration != initial.plaintext.RendezvousDutyGeneration ||
			facts.Deadline.After(request.Deadline) {
			t.Fatalf("replacement %d changed retained authority: %+v", index, facts)
		}
	}
	if first.digest == initial.digest || second.digest == initial.digest || first.digest == second.digest ||
		first.plaintext.JoinSecret == initial.plaintext.JoinSecret || second.plaintext.JoinSecret == initial.plaintext.JoinSecret ||
		first.plaintext.JoinSecret == second.plaintext.JoinSecret ||
		first.plaintext.HandshakeContext == initial.plaintext.HandshakeContext ||
		second.plaintext.HandshakeContext == initial.plaintext.HandshakeContext ||
		first.plaintext.HandshakeContext == second.plaintext.HandshakeContext ||
		bytes.Equal(first.operation, second.operation) {
		t.Fatal("replacement reused capsule, JOIN or handshake material")
	}
}

func TestTextServiceRecoveryRefusesChangedImmutableRequest(t *testing.T) {
	client, publisher, _ := serviceFixture(t)
	valid := client.serviceRecovery()
	valid.Generation, valid.Role, valid.Deadline = 2, "client", time.Now().Add(5*time.Second)
	if err := client.validateServiceRecovery(valid); err != nil {
		t.Fatal(err)
	}
	publisherValid := publisher.serviceRecovery()
	publisherValid.Generation, publisherValid.Role, publisherValid.Deadline = 2, "publisher", valid.Deadline
	if err := publisher.validateServiceRecovery(publisherValid); err != nil {
		t.Fatal(err)
	}

	mutations := map[string]func(*nativeconnection.Recovery){
		"network":        func(value *nativeconnection.Recovery) { value.NetworkID[0]++ },
		"candidate-view": func(value *nativeconnection.Recovery) { value.CandidateView[0]++ },
		"context":        func(value *nativeconnection.Recovery) { value.IsolationContext[0]++ },
		"destination":    func(value *nativeconnection.Recovery) { value.DestinationBinding[0]++ },
		"profile":        func(value *nativeconnection.Recovery) { value.RouteProfile += "-other" },
		"work-safety":    func(value *nativeconnection.Recovery) { value.WorkSafetyNotAfter++ },
		"maximum":        func(value *nativeconnection.Recovery) { value.WorkSafetyMaximum++ },
		"recovery-end":   func(value *nativeconnection.Recovery) { value.NoNewRecoveryAfter++ },
		"role":           func(value *nativeconnection.Recovery) { value.Role = "publisher" },
		"generation":     func(value *nativeconnection.Recovery) { value.Generation = 1 },
		"deadline":       func(value *nativeconnection.Recovery) { value.Deadline = time.Time{} },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := valid
			mutate(&changed)
			if err := client.validateServiceRecovery(changed); err == nil {
				t.Fatal("changed recovery request was accepted")
			}
		})
	}
	state := client.owner.endpoint.closedState.(*permissionStateFixture)
	originalProfile := state.profile
	defer func() { state.profile = originalProfile }()
	state.profile.StateDigest[0]++
	if err := client.validateServiceRecovery(valid); err == nil {
		t.Fatal("changed Candidate View retained an old recovery binding")
	}
}

type openedService struct {
	stream *service.Stream
	err    error
}

func tokenAttemptSnapshot(t *testing.T, current *endpoint) map[[32]byte]tokenReceipt {
	t.Helper()
	return snapshotTokenReceipts(t, current.closedTokenRoot, current.network)
}

func assertFreshRecoveryTokenAttempts(t *testing.T, initial, recovered map[[32]byte]tokenReceipt) {
	t.Helper()
	classes := [4]int{}
	nonces := make(map[[32]byte]struct{}, len(recovered)-len(initial))
	for digest, attempt := range recovered {
		if _, existed := initial[digest]; existed {
			continue
		}
		for _, prior := range initial {
			if attempt.attempt == prior.attempt {
				t.Fatal("recovery reused an initial Route token attempt")
			}
		}
		if _, repeated := nonces[attempt.attempt]; repeated {
			t.Fatal("recovery reused a token attempt across fresh Route legs")
		}
		nonces[attempt.attempt] = struct{}{}
		classes[attempt.class]++
	}
	// Fresh resolution adds two class-1 attempts before capsule submission;
	// the other class-1 attempts replenish the exhausted Route stocks and two
	// class-2 attempts admit the independently opened JOIN legs.
	if len(recovered) != len(initial)+8 || len(nonces) != 8 || classes[1] != 6 || classes[2] != 2 || classes[3] != 0 {
		t.Fatalf("recovery token receipts = %d; class-1:%d class-2:%d class-3:%d distinct attempts:%d; want 8, 6, 2, 0, 8",
			len(recovered)-len(initial), classes[1], classes[2], classes[3], len(nonces))
	}
}

func sameTokenAttemptSnapshot(left, right map[[32]byte]tokenReceipt) bool {
	if len(left) != len(right) {
		return false
	}
	for digest, attempt := range left {
		if right[digest] != attempt {
			return false
		}
	}
	return true
}

type observedServiceOpener struct {
	open    service.AttachmentOpener
	mu      sync.Mutex
	count   int
	digests [][32]byte
	err     error
}

func (opener *observedServiceOpener) openObserved(ctx context.Context, request nativeconnection.Recovery) (net.Conn, [32]byte, error) {
	connection, digest, err := opener.open(ctx, request)
	opener.mu.Lock()
	opener.count++
	if err == nil {
		opener.digests = append(opener.digests, digest)
	}
	opener.err = errors.Join(opener.err, err)
	opener.mu.Unlock()
	return connection, digest, err
}

func (opener *observedServiceOpener) outcome() error {
	count, digests, err := opener.observation()
	return fmt.Errorf("attempts=%d successful-digests=%x error=%v", count, digests, err)
}

func (opener *observedServiceOpener) observation() (int, [][32]byte, error) {
	opener.mu.Lock()
	defer opener.mu.Unlock()
	return opener.count, append([][32]byte(nil), opener.digests...), opener.err
}

type interruptAfterAcceptedRequest struct {
	io.Reader
	accepted  chan<- struct{}
	interrupt func()
	once      sync.Once
	read      int
}

func (reader *interruptAfterAcceptedRequest) Read(body []byte) (int, error) {
	n, err := reader.Reader.Read(body)
	reader.read += n
	if reader.read >= 512 {
		reader.once.Do(func() {
			close(reader.accepted)
			reader.interrupt()
		})
	}
	return n, err
}
