package endpoint

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
)

type endpointRunner interface {
	Admit([32]byte, broker.Surface) ([32]byte, error)
	Withdraw(context.Context, withdrawalRequest) (withdrawalResult, error)
	connectForHarness(context.Context, outboundConnectionRequest) (runtimeResult, error)
	acceptForHarness(context.Context, inboundConnectionRequest) (runtimeResult, error)
}

func (endpoint *endpoint) connectForHarness(ctx context.Context, input outboundConnectionRequest) (runtimeResult, error) {
	return endpoint.runOutbound(ctx, connectionInput{Principal: input.Principal, Session: input.Capability,
		Target: input.Target, AuthorityPublic: input.AuthorityPublic, Publication: input.Publication, Route: input.Route, Application: input.Application,
		OpenAttachment: input.OpenAttachment, RecoveryBinding: input.RecoveryBinding, NameBinding: input.NameBinding,
		NameUpdates: input.NameUpdates, closeApplicationOnRemoteTerminal: input.closeApplicationOnRemoteTerminal, OnAuthenticated: input.OnAuthenticated, BytesEachDirection: input.BytesEachDirection, SendBytes: input.SendBytes,
		ReceiveBytes: input.ReceiveBytes, At: input.At})
}

func (endpoint *endpoint) runOutbound(ctx context.Context, input connectionInput) (runtimeResult, error) {
	if endpoint == nil || input.At.IsZero() {
		return denied("local operation is incomplete")
	}
	if err := ctx.Err(); err != nil {
		return failed("local timeout or cancellation", "local operation was cancelled", err)
	}
	return endpoint.connect(ctx, input)
}

func (endpoint *endpoint) connect(ctx context.Context, input connectionInput) (runtimeResult, error) {
	session, err := endpoint.activateApplicationSession(ctx, input.Session, input.Principal)
	if err != nil {
		return denied(err.Error())
	}
	defer session.Release()
	result, err := endpoint.connectAuthorized(session.Context(), input, session.receipt)
	return preferCallerCancellation(ctx, result, err)
}

type fixture struct {
	now                                            time.Time
	networkID, clientPrincipal, publisherPrincipal [32]byte
	administrationPrincipal                        [32]byte
	authorityPublic, introductionPublic            ed25519.PublicKey
	authorityPrivate                               ed25519.PrivateKey
	first                                          publicationCredential
	binding                                        *instance.Binding
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	value := fixture{now: time.Unix(2_000_000_000, 0).UTC(), networkID: fixtureID(1),
		clientPrincipal: fixtureID(2), publisherPrincipal: fixtureID(3), administrationPrincipal: fixtureID(4)}
	var err error
	value.authorityPublic, value.authorityPrivate, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	value.introductionPublic, _, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root, binding := acceptedInstanceBinding(t, serviceInstanceFixtureRoot(t), value.networkID, value.authorityPrivate,
		value.now, value.now.Add(time.Minute))
	value.binding, value.first = binding, binding.Credential()
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("close Instance fixture: %v", closeErr)
		}
	})
	return value
}

func startPublishedEndpoint(t *testing.T, value fixture) (endpointRunner, []byte) {
	t.Helper()
	digest, introductionID := fixtureID(10), fixtureID(11)
	certificate, introductionPublic := testCertificate(t, 10, "test-introduction")
	address := availableAddress(t)
	introduction, err := startIntroductionTestHarness(address, certificate, value.networkID, digest, introductionID, 1,
		value.now.Add(time.Minute), introductionAdmitForEpoch(value.networkID, digest, introductionID, value.now.Add(time.Minute), 1))
	if err != nil {
		t.Fatal(err)
	}
	slotCertificate, _ := testCertificate(t, 16, "test-slot-client")
	responderCertificate, _ := testCertificate(t, 20, "test-responder-client")
	slotGrantID, responderGrantID := fixtureID(21), fixtureID(22)
	slotAuthorization := issueEndpointTransitFixtureGrant(t, value.networkID, digest, fixtureID(16), introductionID, 1,
		route.IntroductionRole, slotGrantID, value.now.Add(time.Minute), slotCertificate)
	responderAuthorization := issueEndpointTransitFixtureGrant(t, value.networkID, digest, fixtureID(20), fixtureID(14), 1,
		route.ResponderRole, responderGrantID, value.now.Add(time.Minute), responderCertificate)
	profile := publisherIntroductionProfile{
		NetworkID: value.networkID, Digest: digest, Epoch: 1,
		Introduction:     transitPeer{NodeID: introductionID, PublicKey: introductionPublic, Endpoint: address},
		Rendezvous:       transitPeer{NodeID: fixtureID(12), PublicKey: fixtureID(13), Endpoint: "127.0.0.1:26112"},
		Responder:        transitPeer{NodeID: fixtureID(14), PublicKey: fixtureID(15), Endpoint: "127.0.0.1:26114"},
		SlotAttachmentID: fixtureID(16), ResponderAttachmentID: fixtureID(20), Reachability: fixtureID(17), JoinHandle: fixtureID(18),
		NotAfter: value.now.Add(time.Minute), SlotAuthorization: slotAuthorization, SlotClientCertificate: slotCertificate,
		ResponderAuthorization: responderAuthorization, ResponderClientCertificate: responderCertificate,
	}
	owner, err := newEndpoint(setup{
		NetworkID: value.networkID, BrokerID: fixtureID(19), ConnectionPrincipal: value.publisherPrincipal,
		AdministrationPrincipal: value.administrationPrincipal, PublicationRoot: publicationStoreRoot(t),
		PublisherBinding: value.binding, publisherIntroductionProfile: profile,
		TransitClientCertificates: map[[32]byte]tls.Certificate{slotGrantID: slotCertificate, responderGrantID: responderCertificate},
	})

	if err != nil {
		_ = introduction.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := owner.Close(); closeErr != nil {
			t.Errorf("close Publisher fixture: %v", closeErr)
		}
		if closeErr := introduction.Close(); closeErr != nil {
			t.Errorf("close Introduction fixture: %v", closeErr)
		}
	})
	capability := admit(t, owner, "administration", value.administrationPrincipal, value.now)
	result, err := owner.StartPublisher(context.Background(), publisherStartRequest{
		Principal: value.administrationPrincipal, Capability: capability, At: value.now,
	})
	if err != nil || result.Class != "published" || len(result.Record) == 0 {
		t.Fatalf("start Publisher: result=%+v err=%v", result, err)
	}
	return owner, result.Record
}

func issueEndpointTransitFixtureGrant(t *testing.T, network, digest, attachment, node [32]byte, epoch uint64, role byte,
	grantID [32]byte, notAfter time.Time, certificate tls.Certificate,
) []byte {
	t.Helper()
	digestKey, err := route.ClientTLSKeyDigest(certificate.Leaf)
	if err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := route.IssueTransitGrant(route.TransitGrant{IssuerID: sha256.Sum256(public), GrantID: grantID,
		NetworkID: network, Digest: digest, AttachmentID: attachment, TransitNodeID: node, ClientKeyDigest: digestKey,
		Epoch: epoch, TransitRole: role, NotAfter: notAfter}, private)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func admit(t *testing.T, endpoint endpointRunner, surface string, principal [32]byte, at time.Time) [32]byte {
	t.Helper()
	if at.IsZero() {
		t.Fatal("admit time is absent")
	}
	result, err := endpoint.Admit(principal, broker.Surface(surface))
	if err != nil || result == [32]byte{} {
		t.Fatalf("admit %s: session=%x err=%v", surface, result, err)
	}
	return result
}

func seededBytes(length, seed int) []byte {
	value := make([]byte, length)
	for index := range value {
		value[index] = byte((index*131 + seed) % 251)
	}
	return value
}

func assertExchange(t *testing.T, client, publisher net.Conn, clientBytes, publisherBytes []byte) {
	t.Helper()
	type result struct {
		got []byte
		err error
	}
	results := make(chan result, 2)
	var writers sync.WaitGroup
	writers.Add(2)
	go func() {
		defer writers.Done()
		_, err := client.Write(clientBytes)
		if err != nil {
			results <- result{err: err}
		}
	}()
	go func() {
		defer writers.Done()
		_, err := publisher.Write(publisherBytes)
		if err != nil {
			results <- result{err: err}
		}
	}()
	go func() {
		got := make([]byte, len(publisherBytes))
		_, err := netReadFull(client, got)
		results <- result{got, err}
	}()
	go func() {
		got := make([]byte, len(clientBytes))
		_, err := netReadFull(publisher, got)
		results <- result{got, err}
	}()
	writers.Wait()
	first, second := <-results, <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("Application exchange failed: %v %v", first.err, second.err)
	}
	if !(bytes.Equal(first.got, publisherBytes) && bytes.Equal(second.got, clientBytes) ||
		bytes.Equal(second.got, publisherBytes) && bytes.Equal(first.got, clientBytes)) {
		t.Fatal("Application bytes changed length or order")
	}
}

func netReadFull(connection net.Conn, destination []byte) (int, error) {
	total := 0
	for total < len(destination) {
		count, err := connection.Read(destination[total:])
		total += count
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
