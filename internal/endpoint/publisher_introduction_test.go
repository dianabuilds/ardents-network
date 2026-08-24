package endpoint_test

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"errors"
	"io"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

func TestPublisherIntroductionDeliversOnlyCurrentPublicationToResponder(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	deadline := now.Add(time.Minute)
	network, digest := c2Identifier(1), c2Identifier(2)
	introductionID, rendezvousID, responderID := c2Identifier(3), c2Identifier(4), c2Identifier(5)
	introductionCertificate, introductionPublic := c2Certificate(t, 1, "introduction")
	responderCertificate, responderPublic := c2Certificate(t, 2, "responder")
	introductionAddress := c2AvailableAddress(t)
	responderAddress := c2AvailableAddress(t)
	join, reachability, slotAttachment, serviceAttachment := c2Identifier(6), c2Identifier(7), c2Identifier(8), c2Identifier(9)
	slotAuthorization, responderAuthorization := []byte("publisher-slot"), []byte("publisher-responder")
	introduction, err := node.StartIntroduction(node.IntroductionConfig{ListenAddress: introductionAddress, Certificate: introductionCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: introductionID, NodePublicKey: introductionPublic, Epoch: 10, NotAfter: deadline,
		Admit: c2IntroductionAdmit(network, digest, introductionID, deadline), HandshakeLimit: 3, SlotLimit: 1, DeliveryLimit: 1, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer introduction.Close()
	responder := c2StartResponder(t, responderAddress, responderCertificate, network, digest, responderID, deadline, serviceAttachment, responderAuthorization)
	defer responder.listener.Close()

	publisher, current, private := c2PublishedEndpoint(t, network, now)
	defer publisher.Close()
	profile := endpointapi.PublisherIntroductionProfile{NetworkID: network, Digest: digest, Epoch: 10,
		Introduction:     endpointapi.TransitPeer{NodeID: introductionID, PublicKey: introductionPublic, Endpoint: introductionAddress},
		Rendezvous:       endpointapi.TransitPeer{NodeID: rendezvousID, PublicKey: c2Identifier(11), Endpoint: "127.0.0.1:24011"},
		Responder:        endpointapi.TransitPeer{NodeID: responderID, PublicKey: responderPublic, Endpoint: responderAddress},
		SlotAttachmentID: slotAttachment, Reachability: reachability, JoinHandle: join, NotAfter: deadline,
		SlotAuthorization: slotAuthorization, ResponderAuthorization: responderAuthorization}
	session, err := publisher.OpenPublisherIntroduction(context.Background(), endpointapi.PublisherIntroductionRequest{Profile: profile, HPKEPrivate: private, At: now})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	waited := make(chan struct {
		connection net.Conn
		err        error
	}, 1)
	go func() {
		connection, waitErr := session.Wait(context.Background())
		waited <- struct {
			connection net.Conn
			err        error
		}{connection, waitErr}
	}()
	plaintext, err := publication.EncodeIntroductionInstruction(publication.IntroductionInstruction{Target: current.Credential.Target,
		Generation: current.Credential.Generation, PublicationDigest: current.Digest, AttachmentID: serviceAttachment})
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := route.SealIntroduction(route.SealedIntroduction{NetworkID: network, Digest: digest, Epoch: 10,
		IntroductionNodeID: introductionID, RendezvousNodeID: rendezvousID, Reachability: reachability, NotAfter: deadline,
		JoinHandle: join, EndpointHandshake: c2Identifier(12)}, private.PublicKey(), plaintext)
	if err != nil {
		t.Fatal(err)
	}
	submission, err := route.OpenEndpointTransitAttachment(context.Background(), route.EndpointTransitAttachmentRequest{NetworkID: network,
		Digest: digest, AttachmentID: c2Identifier(13), TransitNodeID: introductionID, TransitNodePublicKey: introductionPublic,
		Epoch: 10, TransitRole: route.IntroductionRole, Endpoint: introductionAddress, Deadline: deadline, Authorization: join[:]})
	if err != nil {
		t.Fatal(err)
	}
	if err := route.WriteSealedIntroduction(submission, sealed); err != nil {
		t.Fatal(err)
	}
	delivery, err := route.ReadIntroductionDeliveryResult(submission)
	_ = submission.Close()
	if err != nil || delivery.Outcome != route.IntroductionDelivered {
		t.Fatalf("delivery = %+v, %v", delivery, err)
	}
	result := <-waited
	if result.err != nil {
		t.Fatal(result.err)
	}
	defer result.connection.Close()
	if _, err := result.connection.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 4)
	if _, err := io.ReadFull(result.connection, response); err != nil || string(response) != "pong" {
		t.Fatalf("Responder carrier response = %q, %v", response, err)
	}
	select {
	case err := <-responder.done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Responder did not receive the exact Publisher attachment")
	}
}

func TestPublisherIntroductionRejectsForeignRecipientBeforeOpeningSlot(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	publisher, _, private := c2PublishedEndpoint(t, c2Identifier(21), now)
	defer publisher.Close()
	foreign, err := c2HPKEPrivate()
	if err != nil {
		t.Fatal(err)
	}
	_, err = publisher.OpenPublisherIntroduction(context.Background(), endpointapi.PublisherIntroductionRequest{HPKEPrivate: foreign, At: now,
		Profile: endpointapi.PublisherIntroductionProfile{NetworkID: c2Identifier(21), Digest: c2Identifier(22), Epoch: 1,
			Introduction:     endpointapi.TransitPeer{NodeID: c2Identifier(23), PublicKey: c2Identifier(24), Endpoint: "127.0.0.1:25023"},
			Rendezvous:       endpointapi.TransitPeer{NodeID: c2Identifier(25), PublicKey: c2Identifier(26), Endpoint: "127.0.0.1:25025"},
			Responder:        endpointapi.TransitPeer{NodeID: c2Identifier(27), PublicKey: c2Identifier(28), Endpoint: "127.0.0.1:25027"},
			SlotAttachmentID: c2Identifier(29), Reachability: c2Identifier(30), JoinHandle: c2Identifier(31), NotAfter: now.Add(time.Minute),
			SlotAuthorization: []byte("slot"), ResponderAuthorization: []byte("responder")}})
	if err == nil {
		t.Fatal("foreign HPKE recipient opened a Publisher slot")
	}
	if private == nil {
		t.Fatal("test Publisher key is absent")
	}
}

type c2Responder struct {
	listener net.Listener
	done     chan error
}

type c2Publisher interface {
	Admit([32]byte, broker.Surface) ([32]byte, error)
	Publish(context.Context, endpointapi.PublicationRequest) (endpointapi.PublicationResult, error)
	OpenPublisherIntroduction(context.Context, endpointapi.PublisherIntroductionRequest) (*endpointapi.PublisherIntroduction, error)
	Close() error
}

func c2StartResponder(t *testing.T, address string, certificate tls.Certificate, network, digest, nodeID [32]byte, deadline time.Time,
	attachment [32]byte, authorization []byte) c2Responder {
	t.Helper()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		raw, acceptErr := listener.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		accepted, acceptErr := route.AcceptEndpointTransitAttachment(context.Background(), raw, route.EndpointTransitAttachmentAcceptance{
			NetworkID: network, Digest: digest, TransitNodeID: nodeID, Epoch: 10, TransitRole: route.ResponderRole, Deadline: deadline, Certificate: certificate,
			Admit: func(received []byte, gotAttachment, key [32]byte, role byte, gotNode [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
				if string(received) != string(authorization) || gotAttachment != attachment || key == [32]byte{} || role != route.ResponderRole || gotNode != nodeID || !notAfter.Equal(deadline) {
					return route.EndpointTransitAdmission{}, errors.New("unexpected Responder admission")
				}
				return route.EndpointTransitAdmission{AuthorizationID: c2Identifier(32), NetworkID: network, Digest: digest, Epoch: 10,
					TransitRole: route.ResponderRole, TransitNodeID: nodeID, NotAfter: deadline}, nil
			}})
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		defer accepted.Connection.Close()
		request := make([]byte, 4)
		if _, acceptErr = io.ReadFull(accepted.Connection, request); acceptErr == nil && string(request) == "ping" {
			_, acceptErr = accepted.Connection.Write([]byte("pong"))
		}
		done <- acceptErr
	}()
	return c2Responder{listener: listener, done: done}
}

func c2PublishedEndpoint(t *testing.T, network [32]byte, now time.Time) (c2Publisher, publication.Current, hpke.PrivateKey) {
	t.Helper()
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	introductionPublic, introductionPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	instancePublic, instancePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	private, err := c2HPKEPrivate()
	if err != nil {
		t.Fatal(err)
	}
	var authority, instance, hpkePublic [32]byte
	copy(authority[:], authorityPublic)
	copy(instance[:], instancePublic)
	copy(hpkePublic[:], private.PublicKey().Bytes())
	credential, err := (publication.Credential{AuthorityPublic: authority, InstancePublic: instance, IntroductionHPKEPublic: hpkePublic,
		Generation: 1, NotBefore: now.Add(-time.Second).Unix(), NotAfter: now.Add(time.Minute).Unix(), NetworkID: network, Capabilities: 3}).Issue(authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	brokerID := c2Identifier(40)
	publisher, err := endpointapi.New(endpointapi.Setup{NetworkID: network, BrokerID: brokerID, AuthorityPublic: authorityPublic,
		IntroductionPublic: introductionPublic, ConnectionPrincipal: c2Identifier(41), AdministrationPrincipal: c2Identifier(42), PublicationRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	capability, err := publisher.Admit(c2Identifier(42), broker.Administration)
	if err != nil {
		t.Fatal(err)
	}
	result, err := publisher.Publish(context.Background(), endpointapi.PublicationRequest{Principal: c2Identifier(42), Capability: capability,
		Credential: credential, InstancePrivate: instancePrivate, IntroductionAcknowledgement: c2Acknowledgement(credential, introductionPrivate, brokerID), At: now})
	if err != nil {
		t.Fatal(err)
	}
	current, err := publication.Decode(result.Record, authorityPublic, network, now)
	if err != nil {
		t.Fatal(err)
	}
	return publisher, current, private
}

func c2Acknowledgement(credential publication.Credential, private ed25519.PrivateKey, brokerID [32]byte) []byte {
	body := make([]byte, 149)
	copy(body[:4], "ARIA")
	body[4] = 1
	copy(body[5:37], credential.Target[:])
	binary.BigEndian.PutUint64(body[37:45], credential.Generation)
	binary.BigEndian.PutUint64(body[45:53], uint64(credential.NotAfter))
	copy(body[53:85], credential.NetworkID[:])
	copy(body[85:117], brokerID[:])
	body[117] = 1
	signature := ed25519.Sign(private, append([]byte("ardents-service-introduction-ack-v1\x00"), body...))
	return append(body, signature...)
}

func c2HPKEPrivate() (hpke.PrivateKey, error) {
	private, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return hpke.NewDHKEMPrivateKey(private)
}

func c2Certificate(t *testing.T, serial int64, name string) (tls.Certificate, [32]byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, NotBefore: time.Now().Add(-time.Minute),
		NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	var fixed [32]byte
	copy(fixed[:], public)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private, Leaf: leaf}, fixed
}

func c2IntroductionAdmit(network, digest, nodeID [32]byte, deadline time.Time) route.EndpointTransitBindingAdmitter {
	return func(authorization []byte, attachment, key [32]byte, role byte, receivedNode [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
		if len(authorization) == 0 || attachment == [32]byte{} || key == [32]byte{} || role != route.IntroductionRole || receivedNode != nodeID || !notAfter.Equal(deadline) {
			return route.EndpointTransitAdmission{}, errors.New("unexpected Introduction admission")
		}
		return route.EndpointTransitAdmission{AuthorizationID: c2Identifier(50), NetworkID: network, Digest: digest, Epoch: 10,
			TransitRole: route.IntroductionRole, TransitNodeID: nodeID, NotAfter: deadline}, nil
	}
}

func c2AvailableAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func c2Identifier(value byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = value + byte(index)
	}
	return result
}
