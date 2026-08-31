package endpoint_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
)

type endpointRunner interface {
	Admit([32]byte, broker.Surface) ([32]byte, error)
	Withdraw(context.Context, endpointapi.WithdrawalRequest) (endpointapi.WithdrawalResult, error)
	Connect(context.Context, endpointapi.OutboundConnectionRequest) (endpointapi.RuntimeResult, error)
	Accept(context.Context, endpointapi.InboundConnectionRequest) (endpointapi.RuntimeResult, error)
}

type fixture struct {
	now                                            time.Time
	networkID, clientPrincipal, publisherPrincipal [32]byte
	administrationPrincipal                        [32]byte
	authorityPublic, introductionPublic            ed25519.PublicKey
	authorityPrivate                               ed25519.PrivateKey
	first                                          endpointapi.Credential
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
	root, binding := acceptedInstanceBinding(t, t.TempDir(), value.networkID, value.authorityPrivate,
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
	introduction, err := node.StartIntroduction(node.IntroductionConfig{
		ListenAddress: address, Certificate: certificate, NetworkID: value.networkID, EpochDigest: digest,
		NodeID: introductionID, NodePublicKey: introductionPublic, Epoch: 1, NotAfter: value.now.Add(time.Minute),
		Admit:          introductionAdmitForEpoch(value.networkID, digest, introductionID, value.now.Add(time.Minute), 1),
		HandshakeLimit: 2, SlotLimit: 1, DeliveryLimit: 1, AdmissionTimeout: time.Second, DrainTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	profile := endpointapi.PublisherIntroductionProfile{
		NetworkID: value.networkID, Digest: digest, Epoch: 1,
		Introduction:     endpointapi.TransitPeer{NodeID: introductionID, PublicKey: introductionPublic, Endpoint: address},
		Rendezvous:       endpointapi.TransitPeer{NodeID: fixtureID(12), PublicKey: fixtureID(13), Endpoint: "127.0.0.1:26112"},
		Responder:        endpointapi.TransitPeer{NodeID: fixtureID(14), PublicKey: fixtureID(15), Endpoint: "127.0.0.1:26114"},
		SlotAttachmentID: fixtureID(16), Reachability: fixtureID(17), JoinHandle: fixtureID(18),
		NotAfter: value.now.Add(time.Minute), SlotAuthorization: []byte("test-slot"),
		ResponderAuthorization: []byte("test-responder"),
	}
	owner, err := endpointapi.New(endpointapi.Setup{
		NetworkID: value.networkID, BrokerID: fixtureID(19), ConnectionPrincipal: value.publisherPrincipal,
		AdministrationPrincipal: value.administrationPrincipal, PublicationRoot: t.TempDir(),
		PublisherBinding: value.binding, PublisherIntroductionProfile: profile,
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
	result, err := owner.StartPublisher(context.Background(), endpointapi.PublisherStartRequest{
		Principal: value.administrationPrincipal, Capability: capability, At: value.now,
	})
	if err != nil || result.Class != "published" || len(result.Record) == 0 {
		t.Fatalf("start Publisher: result=%+v err=%v", result, err)
	}
	return owner, result.Record
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
