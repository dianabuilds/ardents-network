package endpoint_test

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"testing"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

func TestStartPublisherOwnsInstancePublicationAndReadySlot(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	deadline := now.Add(time.Minute)
	network, digest := c2Identifier(61), c2Identifier(62)
	introductionID := c2Identifier(63)
	introductionCertificate, introductionPublic := c2Certificate(t, 61, "start-introduction")
	introductionAddress := c2AvailableAddress(t)
	introduction, err := node.StartIntroduction(node.IntroductionConfig{
		ListenAddress: introductionAddress, Certificate: introductionCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: introductionID, NodePublicKey: introductionPublic,
		Epoch: 12, NotAfter: deadline, Admit: c2IntroductionAdmitForEpoch(network, digest, introductionID, deadline, 12),
		HandshakeLimit: 2, SlotLimit: 1, DeliveryLimit: 1, AdmissionTimeout: time.Second, DrainTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer introduction.Close()

	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	instancePath := t.TempDir()
	instanceRoot, binding := c2AcceptedInstanceBinding(t, instancePath, network, authorityPrivate, now, deadline)
	defer instanceRoot.Close()
	credential := binding.Credential()
	profile := endpointapi.PublisherIntroductionProfile{
		NetworkID: network, Digest: digest, Epoch: 12,
		Introduction:     endpointapi.TransitPeer{NodeID: introductionID, PublicKey: introductionPublic, Endpoint: introductionAddress},
		Rendezvous:       endpointapi.TransitPeer{NodeID: c2Identifier(64), PublicKey: c2Identifier(65), Endpoint: "127.0.0.1:26064"},
		Responder:        endpointapi.TransitPeer{NodeID: c2Identifier(66), PublicKey: c2Identifier(67), Endpoint: "127.0.0.1:26066"},
		SlotAttachmentID: c2Identifier(68), Reachability: c2Identifier(69), JoinHandle: c2Identifier(70), NotAfter: deadline,
		SlotAuthorization: []byte("start-slot"), ResponderAuthorization: []byte("start-responder"),
	}
	principal := c2Identifier(71)
	introductionAuthority := c2Identifier(73)
	owner, err := endpointapi.New(endpointapi.Setup{
		NetworkID: network, BrokerID: c2Identifier(72), AuthorityPublic: authorityPublic,
		IntroductionPublic: ed25519.PublicKey(introductionAuthority[:]), ConnectionPrincipal: c2Identifier(74),
		AdministrationPrincipal: principal, PublicationRoot: t.TempDir(),
		PublisherBinding: binding, PublisherIntroductionProfile: profile,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	administration, err := owner.OpenServiceAdministration(endpointapi.ServiceAdministrationConfig{
		Principal: principal, Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	started, err := administration.Publish(context.Background())
	if err != nil || started.Class != "published" {
		t.Fatalf("StartPublisher = %+v, %v", started, err)
	}
	current, err := publication.Decode(started.Record, authorityPublic, network, now)
	if err != nil || current.Credential != credential {
		t.Fatalf("published current = %+v, %v", current, err)
	}
	if _, err := binding.Sign(nil, []byte("live-consumed-binding"), crypto.Hash(0)); err != nil {
		t.Fatalf("live consumed binding unavailable before withdrawal: %v", err)
	}
	forbiddenRoute, forbiddenPeer := net.Pipe()
	defer forbiddenRoute.Close()
	defer forbiddenPeer.Close()
	if result, acceptErr := owner.AcceptPublisher(context.Background(), endpointapi.InboundConnectionRequest{
		Route: forbiddenRoute, At: now,
	}); acceptErr == nil || result.Class != "local authorization or policy denial" {
		t.Fatalf("Endpoint-owned Publisher accepted a caller-selected Route: %+v, %v", result, acceptErr)
	}
	if repeated, retryErr := administration.Publish(context.Background()); retryErr == nil || repeated.Class == "published" {
		t.Fatalf("repeated Publisher start = %+v, %v", repeated, retryErr)
	}
	if _, err := binding.Sign(nil, []byte("binding-after-retry"), crypto.Hash(0)); err != nil {
		t.Fatalf("rejected retry withdrew the live binding: %v", err)
	}

	withdrawn, err := administration.Withdraw(context.Background())
	if err != nil || withdrawn.Class != "unpublished" || withdrawn.Generation != credential.Generation {
		t.Fatalf("Withdraw = %+v, %v", withdrawn, err)
	}
	if _, err := binding.Sign(nil, []byte("withdrawn-binding"), crypto.Hash(0)); !errors.Is(err, instance.ErrUnavailable) {
		t.Fatalf("withdrawn binding remained usable: %v", err)
	}
}

func TestStartPublisherSlotFailureConsumesGenerationWithoutExposure(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	deadline := now.Add(time.Minute)
	network := c2Identifier(81)
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	instanceRoot, binding := c2AcceptedInstanceBinding(t, t.TempDir(), network, authorityPrivate, now, deadline)
	publicationRoot := t.TempDir()
	unavailableAddress := c2AvailableAddress(t)
	profile := endpointapi.PublisherIntroductionProfile{
		NetworkID: network, Digest: c2Identifier(82), Epoch: 13,
		Introduction:     endpointapi.TransitPeer{NodeID: c2Identifier(83), PublicKey: c2Identifier(84), Endpoint: unavailableAddress},
		Rendezvous:       endpointapi.TransitPeer{NodeID: c2Identifier(85), PublicKey: c2Identifier(86), Endpoint: "127.0.0.1:28085"},
		Responder:        endpointapi.TransitPeer{NodeID: c2Identifier(87), PublicKey: c2Identifier(88), Endpoint: "127.0.0.1:28087"},
		SlotAttachmentID: c2Identifier(89), Reachability: c2Identifier(90), JoinHandle: c2Identifier(91), NotAfter: deadline,
		SlotAuthorization: []byte("unavailable-slot"), ResponderAuthorization: []byte("unused-responder"),
	}
	principal, introductionAuthority := c2Identifier(92), c2Identifier(93)
	owner, err := endpointapi.New(endpointapi.Setup{
		NetworkID: network, BrokerID: c2Identifier(94), AuthorityPublic: authorityPublic,
		IntroductionPublic: ed25519.PublicKey(introductionAuthority[:]), ConnectionPrincipal: c2Identifier(95),
		AdministrationPrincipal: principal, PublicationRoot: publicationRoot,
		PublisherBinding: binding, PublisherIntroductionProfile: profile,
	})
	if err != nil {
		t.Fatal(err)
	}
	administration, err := owner.OpenServiceAdministration(endpointapi.ServiceAdministrationConfig{
		Principal: principal, Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if started, startErr := administration.Publish(context.Background()); startErr == nil || started.Class == "published" {
		t.Fatalf("unready Publisher start = %+v, %v", started, startErr)
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

func c2AcceptedInstanceBinding(t *testing.T, rootPath string, network [32]byte, authority ed25519.PrivateKey,
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

func c2IntroductionAdmitForEpoch(network, digest, nodeID [32]byte, deadline time.Time, epoch uint64) route.EndpointTransitBindingAdmitter {
	return func(authorization []byte, attachment, key [32]byte, role byte, receivedNode [32]byte,
		notAfter time.Time,
	) (route.EndpointTransitAdmission, error) {
		if len(authorization) == 0 || attachment == [32]byte{} || key == [32]byte{} || role != route.IntroductionRole ||
			receivedNode != nodeID || !notAfter.Equal(deadline) {
			return route.EndpointTransitAdmission{}, errors.New("unexpected Introduction admission")
		}
		return route.EndpointTransitAdmission{AuthorizationID: c2Identifier(75), NetworkID: network, Digest: digest,
			Epoch: epoch, TransitRole: route.IntroductionRole, TransitNodeID: nodeID, NotAfter: deadline}, nil
	}
}
