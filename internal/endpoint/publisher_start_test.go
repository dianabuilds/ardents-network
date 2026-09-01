package endpoint

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

func TestStartPublisherOwnsInstancePublicationAndReadySlot(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	deadline := now.Add(time.Minute)
	network, digest := fixtureID(61), fixtureID(62)
	introductionID := fixtureID(63)
	introductionCertificate, introductionPublic := testCertificate(t, 61, "start-introduction")
	introductionAddress := availableAddress(t)
	introduction, err := startIntroductionTestHarness(introductionAddress, introductionCertificate, network, digest,
		introductionID, 12, deadline, introductionAdmitForEpoch(network, digest, introductionID, deadline, 12))
	if err != nil {
		t.Fatal(err)
	}
	defer introduction.Close()

	_, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	instancePath := t.TempDir()
	instanceRoot, binding := acceptedInstanceBinding(t, instancePath, network, authorityPrivate, now, deadline)
	defer instanceRoot.Close()
	profile := publisherIntroductionProfile{
		NetworkID: network, Digest: digest, Epoch: 12,
		Introduction:     transitPeer{NodeID: introductionID, PublicKey: introductionPublic, Endpoint: introductionAddress},
		Rendezvous:       transitPeer{NodeID: fixtureID(64), PublicKey: fixtureID(65), Endpoint: "127.0.0.1:26064"},
		Responder:        transitPeer{NodeID: fixtureID(66), PublicKey: fixtureID(67), Endpoint: "127.0.0.1:26066"},
		SlotAttachmentID: fixtureID(68), ResponderAttachmentID: fixtureID(73), Reachability: fixtureID(69), JoinHandle: fixtureID(70), NotAfter: deadline,
		SlotAuthorization: []byte("start-slot"), ResponderAuthorization: []byte("start-responder"),
	}
	principal := fixtureID(71)
	owner, err := newEndpoint(setup{
		NetworkID: network, BrokerID: fixtureID(72), ConnectionPrincipal: fixtureID(74),
		AdministrationPrincipal: principal, PublicationRoot: t.TempDir(),
		PublisherBinding: binding, publisherIntroductionProfile: profile,
	})

	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	administration, err := owner.OpenServiceAdministration(serviceAdministrationConfig{
		Principal: principal, Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := administration.Publish(context.Background()); err != nil {
		t.Fatalf("StartPublisher = %v", err)
	}
	if _, err := binding.Sign(nil, []byte("live-consumed-binding"), crypto.Hash(0)); err != nil {
		t.Fatalf("live consumed binding unavailable before withdrawal: %v", err)
	}
	forbiddenRoute, forbiddenPeer := net.Pipe()
	defer forbiddenRoute.Close()
	defer forbiddenPeer.Close()
	if result, acceptErr := owner.AcceptPublisher(context.Background(), inboundConnectionRequest{
		Route: forbiddenRoute, At: now,
	}); acceptErr == nil || result.Class != "local authorization or policy denial" {
		t.Fatalf("Endpoint-owned Publisher accepted a caller-selected Route: %+v, %v", result, acceptErr)
	}
	if retryErr := administration.Publish(context.Background()); retryErr == nil {
		t.Fatal("repeated Publisher start succeeded")
	}
	if _, err := binding.Sign(nil, []byte("binding-after-retry"), crypto.Hash(0)); err != nil {
		t.Fatalf("rejected retry withdrew the live binding: %v", err)
	}

	if err := administration.Withdraw(context.Background()); err != nil {
		t.Fatalf("Withdraw = %v", err)
	}
	if _, err := binding.Sign(nil, []byte("withdrawn-binding"), crypto.Hash(0)); !errors.Is(err, instance.ErrUnavailable) {
		t.Fatalf("withdrawn binding remained usable: %v", err)
	}
}

func TestStartPublisherSlotFailureConsumesGenerationWithoutExposure(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	deadline := now.Add(time.Minute)
	network := fixtureID(81)
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	instanceRoot, binding := acceptedInstanceBinding(t, t.TempDir(), network, authorityPrivate, now, deadline)
	publicationRoot := t.TempDir()
	unavailableAddress := availableAddress(t)
	profile := publisherIntroductionProfile{
		NetworkID: network, Digest: fixtureID(82), Epoch: 13,
		Introduction:     transitPeer{NodeID: fixtureID(83), PublicKey: fixtureID(84), Endpoint: unavailableAddress},
		Rendezvous:       transitPeer{NodeID: fixtureID(85), PublicKey: fixtureID(86), Endpoint: "127.0.0.1:28085"},
		Responder:        transitPeer{NodeID: fixtureID(87), PublicKey: fixtureID(88), Endpoint: "127.0.0.1:28087"},
		SlotAttachmentID: fixtureID(89), ResponderAttachmentID: fixtureID(93), Reachability: fixtureID(90), JoinHandle: fixtureID(91), NotAfter: deadline,
		SlotAuthorization: []byte("unavailable-slot"), ResponderAuthorization: []byte("unused-responder"),
	}
	principal := fixtureID(92)
	owner, err := newEndpoint(setup{
		NetworkID: network, BrokerID: fixtureID(94), ConnectionPrincipal: fixtureID(95),
		AdministrationPrincipal: principal, PublicationRoot: publicationRoot,
		PublisherBinding: binding, publisherIntroductionProfile: profile,
	})

	if err != nil {
		t.Fatal(err)
	}
	administration, err := owner.OpenServiceAdministration(serviceAdministrationConfig{
		Principal: principal, Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if startErr := administration.Publish(context.Background()); startErr == nil {
		t.Fatal("unready Publisher start succeeded")
	}
	if _, err := binding.Sign(nil, []byte("failed-generation"), crypto.Hash(0)); !errors.Is(err, instance.ErrUnavailable) {
		t.Fatalf("failed generation remained usable: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := instanceRoot.Close(); err != nil {
		t.Fatal(err)
	}
	publicationOwner, err := publication.Open(publication.Config{Root: publicationRoot, NetworkID: network,
		Authority: authorityPublic, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	defer publicationOwner.Close()
	floor, err := publicationOwner.Floor()
	if err != nil || floor != 1 {
		t.Fatalf("failed Publisher floor = %d, %v", floor, err)
	}
	if _, err := publicationOwner.Acquire(t.Context()); err == nil {
		t.Fatal("failed Publisher generation became discoverable")
	}
}

func acceptedInstanceBinding(t *testing.T, rootPath string, network [32]byte, authority ed25519.PrivateKey,
	now, deadline time.Time,
) (*instance.Root, *instance.Binding) {
	t.Helper()
	root, err := instance.Initialize(instance.InitializeConfig{Root: rootPath, NetworkID: network, NotBefore: now, NotAfter: deadline})
	if err != nil {
		t.Fatal(err)
	}
	request, err := root.Request()
	if err != nil {
		t.Fatal(err)
	}
	view, err := instance.ParseRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := (publication.Credential{
		InstancePublic: view.InstancePublic, IntroductionHPKEPublic: view.IntroductionPublic,
		Generation: 1, NotBefore: view.NotBefore, NotAfter: view.NotAfter, NetworkID: view.NetworkID,
		Capabilities: publication.CapabilityPublish | publication.CapabilityConnect,
	}).Issue(authority)
	if err != nil {
		t.Fatal(err)
	}
	response, err := instance.BuildResponse(request, credential)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := root.Accept(response); err != nil {
		t.Fatal(err)
	}
	binding, err := root.OpenBinding(0)
	if err != nil {
		t.Fatal(err)
	}
	return root, binding
}

func introductionAdmitForEpoch(network, digest, nodeID [32]byte, deadline time.Time, epoch uint64) route.EndpointTransitBindingAdmitter {
	return func(authorization []byte, attachment, key [32]byte, role byte, receivedNode [32]byte,
		notAfter time.Time,
	) (route.EndpointTransitAdmission, error) {
		if len(authorization) == 0 || attachment == [32]byte{} || key == [32]byte{} || role != route.IntroductionRole ||
			receivedNode != nodeID || !notAfter.Equal(deadline) {
			return route.EndpointTransitAdmission{}, errors.New("unexpected Introduction admission")
		}
		return route.EndpointTransitAdmission{AuthorizationID: fixtureID(75), NetworkID: network, Digest: digest,
			Epoch: epoch, TransitRole: route.IntroductionRole, TransitNodeID: nodeID, NotAfter: deadline}, nil
	}
}
