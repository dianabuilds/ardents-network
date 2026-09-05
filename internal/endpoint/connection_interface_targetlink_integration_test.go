package endpoint

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// TestConnectionInterfaceOpensTargetLinkThroughTwoEndpoints proves the whole
// maintained destination path: a caller provides only a Target Link, Endpoint
// privately resolves it through Route, and two Endpoint instances exchange
// bounded opaque bytes over the authenticated Service Connection.
func TestConnectionInterfaceOpensTargetLinkThroughTwoEndpoints(t *testing.T) {
	fixture := openUserRouteCredentialFixture(t, route.IntroductionDelivered)
	defer fixture.close()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	link, err := targetlink.Encode(targetlink.Link{Network: fixture.network, Target: fixture.target})
	if err != nil {
		t.Fatal(err)
	}
	exchangeConnectionInterfaceLink(t, fixture, ctx, link, nil)
}

// TestConnectionInterfaceOpensPersistedLegacyServiceLinkThroughTwoEndpoints
// proves the bounded v1 adapter: only a complete accepted local corpus can
// resolve an exact historical Service Link to the same authenticated Target.
func TestConnectionInterfaceOpensPersistedLegacyServiceLinkThroughTwoEndpoints(t *testing.T) {
	fixture := openUserRouteCredentialFixture(t, route.IntroductionDelivered)
	defer fixture.close()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	floor, link := acceptedLegacyServiceLinkFloor(t, fixture)
	t.Cleanup(func() {
		if err := floor.Close(); err != nil {
			t.Errorf("close legacy Service Link floor: %v", err)
		}
	})
	exchangeConnectionInterfaceLink(t, fixture, ctx, link, floor)
}

func exchangeConnectionInterfaceLink(t *testing.T, fixture *userRouteCredentialFixture, ctx context.Context, link string,
	legacyFloor *alpha.PersistentFloor,
) {
	t.Helper()
	const byteLimit = uint32(19)
	publisherApplication, publisherResult, closePublisher := startTargetLinkPublisher(t, fixture, ctx, byteLimit)
	defer closePublisher()

	owner, err := fixture.endpoint.openConnectionInterface(connectionInterfaceConfig{Route: fixture.route,
		LegacyServiceLinkFloor: legacyFloor, Principal: [32]byte{86}, BytesEachDirection: byteLimit, Clock: func() time.Time { return fixture.now }})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := owner.Open(ctx, link)
	if err != nil {
		t.Fatalf("open exact Target Link: %v", err)
	}
	defer connection.Close()

	toPublisher, toCaller := []byte("opaque caller bytes"), []byte("opaque server bytes")
	if uint32(len(toPublisher)) != byteLimit || uint32(len(toCaller)) != byteLimit {
		t.Fatal("test byte bounds are inconsistent")
	}
	exchange := make(chan error, 4)
	go func() { _, err := connection.Write(toPublisher); exchange <- err }()
	go func() { _, err := publisherApplication.Write(toCaller); exchange <- err }()
	go func() {
		actual := make([]byte, len(toCaller))
		_, err := io.ReadFull(connection, actual)
		if err == nil && !bytes.Equal(actual, toCaller) {
			err = errors.New("Endpoint caller received altered opaque bytes")
		}
		exchange <- err
	}()
	go func() {
		actual := make([]byte, len(toPublisher))
		_, err := io.ReadFull(publisherApplication, actual)
		if err == nil && !bytes.Equal(actual, toPublisher) {
			err = errors.New("Endpoint publisher received altered opaque bytes")
		}
		exchange <- err
	}()
	for range 4 {
		if err := <-exchange; err != nil {
			t.Fatalf("bounded opaque exchange: %v", err)
		}
	}

	select {
	case outcome := <-connection.Done():
		if outcome.Class != "clean service connection close" {
			t.Fatalf("caller outcome = %+v", outcome)
		}
	case <-ctx.Done():
		t.Fatalf("caller Endpoint did not finish the bounded stream: %v", ctx.Err())
	}
	select {
	case outcome := <-publisherResult:
		if outcome.err != nil || outcome.result.Class != "clean service connection close" {
			t.Fatalf("publisher outcome = %+v / %v", outcome.result, outcome.err)
		}
	case <-ctx.Done():
		t.Fatalf("publisher Endpoint did not finish the bounded stream: %v", ctx.Err())
	}
	fixture.wait(t)
}

func acceptedLegacyServiceLinkFloor(t *testing.T, fixture *userRouteCredentialFixture) (*alpha.PersistentFloor, string) {
	t.Helper()
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://reference")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: fixture.network, Serial: 1,
		Bindings: []alpha.BindingInput{{Link: link, Target: fixture.target}}, NotBefore: fixture.now.Add(-time.Minute),
		NotAfter: fixture.now.Add(time.Minute)}, authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(authorityPublic, raw)
	if err != nil {
		t.Fatal(err)
	}
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: alphaPersistentFloorRoot(t), Authority: authorityPublic,
		Cohort: "closed-alpha-1", Network: fixture.network})
	if err != nil {
		t.Fatal(err)
	}
	if err := floor.Observe(corpus); err != nil {
		if closeErr := floor.Close(); closeErr != nil {
			t.Fatalf("observe legacy Service Link corpus: %v; close floor: %v", err, closeErr)
		}
		t.Fatal(err)
	}
	return floor, link.String()
}

func startTargetLinkPublisher(t *testing.T, fixture *userRouteCredentialFixture, ctx context.Context, byteLimit uint32,
) (net.Conn, <-chan serviceOutcome, func()) {
	t.Helper()
	const principal = byte(91)
	publisher, err := newEndpoint(setup{NetworkID: fixture.network, BrokerID: [32]byte{90}, AuthorityPublic: fixture.authority,
		ConnectionPrincipal: [32]byte{principal}, AdministrationPrincipal: [32]byte{92}, PublicationRoot: publicationStoreRoot(t),
		Clock: func() time.Time { return fixture.now }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.publications.Publish(ctx, publication.PublishInput{Credential: fixture.credential,
		InstanceSigner: fixture.instanceSigner, Acknowledgement: []byte("user route credential lifecycle"), At: fixture.now}); err != nil {
		_ = publisher.Close()
		t.Fatal(err)
	}
	publisherEndpoint, publisherApplication := net.Pipe()
	result := make(chan serviceOutcome, 1)
	fixture.connectionHandler = func(routeConnection net.Conn) error {
		capability, admitErr := publisher.Admit([32]byte{principal}, broker.Connection)
		if admitErr != nil {
			result <- serviceOutcome{err: admitErr}
			return nil
		}
		runtime, runErr := publisher.acceptForHarness(ctx, inboundConnectionRequest{Principal: [32]byte{principal}, Capability: capability,
			Route: routeConnection, Application: publisherEndpoint, BytesEachDirection: byteLimit, At: fixture.now})
		result <- serviceOutcome{result: runtime, err: runErr}
		return nil
	}
	return publisherApplication, result, func() {
		_ = publisherApplication.Close()
		_ = publisherEndpoint.Close()
		_ = publisher.Close()
	}
}

func serveTargetLinkIntroduction(listener net.Listener, network, digest [32]byte, epoch uint64, node [32]byte,
	deadline time.Time, certificate tls.Certificate, outcome byte, onDelivered func(net.Conn) error,
) <-chan error {
	result := make(chan error, 1)
	go func() {
		defer close(result)
		connection, err := listener.Accept()
		if err != nil {
			result <- err
			return
		}
		accepted, err := route.AcceptEndpointTransitAttachment(context.Background(), connection, route.EndpointTransitAttachmentAcceptance{
			NetworkID: network, Digest: digest, TransitNodeID: node, Epoch: epoch, TransitRole: route.IntroductionRole,
			Deadline: deadline, Certificate: certificate,
			Admit: func(authorization []byte, attachment, key [32]byte, role byte, transit [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
				if len(authorization) == 0 || attachment == [32]byte{} || key == [32]byte{} || role != route.IntroductionRole || transit != node || !notAfter.Equal(deadline) {
					return route.EndpointTransitAdmission{}, errors.New("Introduction admission input is invalid")
				}
				return route.EndpointTransitAdmission{AuthorizationID: [32]byte{87}, NetworkID: network, Digest: digest, TransitNodeID: node,
					Epoch: epoch, TransitRole: route.IntroductionRole, NotAfter: deadline}, nil
			}})
		if err != nil {
			result <- err
			return
		}
		defer accepted.Connection.Close()
		control, err := route.ReadIntroductionControlRecord(accepted.Connection)
		if err != nil || control.Sealed == nil {
			result <- errors.Join(errors.New("read sealed Introduction"), err)
			return
		}
		if err := route.WriteIntroductionDeliveryResult(accepted.Connection, route.IntroductionDeliveryResult{AttachmentID: accepted.Binding.AttachmentID, Outcome: outcome}); err != nil {
			result <- err
			return
		}
		if onDelivered != nil {
			result <- onDelivered(accepted.Connection)
			return
		}
		result <- nil
	}()
	return result
}

func targetLinkIntroductionRelayHandler(connection net.Conn, onReady func(net.Conn) error) error {
	setup, err := route.ReadRelaySetup(connection)
	if err != nil {
		return err
	}
	if err := route.WriteRelayReady(connection, route.RelayReady{Setup: setup}); err != nil {
		return err
	}
	if onReady != nil {
		return onReady(connection)
	}
	_, err = io.Copy(io.Discard, connection)
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
