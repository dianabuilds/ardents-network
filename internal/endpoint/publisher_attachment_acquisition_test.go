package endpoint

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func TestPublisherPlanOrdersStateProjectedRolesSeparately(t *testing.T) {
	t.Parallel()
	now := time.Unix(2_100_000_000, 0).UTC()
	network, digest := acquisitionID(81), acquisitionID(82)
	initiatorFamily := "publisher-initiator"
	initiator := transitPeer{NodeID: acquisitionID(83), PublicKey: acquisitionID(84), Family: sha256.Sum256([]byte(initiatorFamily)), Endpoint: "127.0.0.1:3183"}
	projected := state.PublisherAttachment{NetworkID: network, Digest: digest, Epoch: 86,
		Introduction: state.PublisherTransitPeer{NodeID: acquisitionID(87), PublicKey: acquisitionID(88), Family: acquisitionID(89), Endpoint: "127.0.0.1:3187"},
		Rendezvous:   state.PublisherTransitPeer{NodeID: acquisitionID(90), PublicKey: acquisitionID(91), Family: acquisitionID(92), Endpoint: "127.0.0.1:3190"},
		Responder:    state.PublisherTransitPeer{NodeID: acquisitionID(93), PublicKey: acquisitionID(94), Family: acquisitionID(95), Endpoint: "127.0.0.1:3193"}}
	view := publisherAcquisitionView{epoch: state.ResolutionEpoch{NetworkID: network, Digest: digest, Number: 86}, attachment: projected,
		initiator: state.ResolutionCandidate{NodeID: initiator.NodeID, PublicKey: initiator.PublicKey, Family: initiatorFamily,
			Endpoint: initiator.Endpoint, Domain: "initiator"}}
	contact := entry.Candidate{NodeID: initiator.NodeID, PublicKey: initiator.PublicKey, FamilyID: initiator.Family, Endpoint: initiator.Endpoint}
	owner := &endpoint{network: network}
	plan, err := owner.planPublisherAcquisition(view, publisherAcquisitionEntry{contact: contact},
		publication.Credential{NetworkID: network, NotBefore: now.Add(-time.Minute).Unix(), NotAfter: now.Add(time.Hour).Unix()}, now)
	if err != nil {
		t.Fatal(err)
	}
	calls := plan.requests(view)
	if calls[0].role != route.IntroductionRole || calls[1].role != route.ResponderRole ||
		calls[0].transit.NodeID != projected.Introduction.NodeID || calls[1].transit.NodeID != projected.Responder.NodeID ||
		calls[0].initiator != initiator || calls[1].initiator != initiator || calls[0].slot.SubmissionMode != reachability.SubmissionMembershipGrant ||
		calls[0].deadline != calls[1].deadline {
		t.Fatalf("publisher acquisitions = %+v", calls)
	}
	if plan.introduction.NodeID != projected.Introduction.NodeID || plan.rendezvous.NodeID != projected.Rendezvous.NodeID ||
		plan.responder.NodeID != projected.Responder.NodeID || plan.epoch.NetworkID != view.epoch.NetworkID ||
		plan.epoch.Digest != view.epoch.Digest || plan.epoch.Number != view.epoch.Number || plan.slot.NotAfter != plan.deadline {
		t.Fatalf("Publisher plan = %+v", plan)
	}
}

func TestPublisherConfigurationReadsCurrentStateOnlyOnStart(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	network, principal := acquisitionID(101), acquisitionID(102)
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	instanceRoot, binding := publisherAcquisitionBinding(t, network, private, now)
	credential := binding.Credential()
	owner, err := newEndpoint(setup{NetworkID: network, BrokerID: acquisitionID(103),
		AuthorityPublic: ed25519.PublicKey(credential.AuthorityPublic[:]), IntroductionPublic: ed25519.PublicKey(credential.IntroductionHPKEPublic[:]),
		ConnectionPrincipal: acquisitionID(104), AdministrationPrincipal: principal, PublicationRoot: publicationStoreRoot(t),
		TransitAcquisitionRoot: transitAcquisitionRoot(t), CreateTransitAcquisitionRoot: true, Clock: func() time.Time { return now }})

	if err != nil {
		t.Fatal(err)
	}
	var reads int
	if err := owner.configurePublisher(func() (publisherAttachmentStateView, error) {
		reads++
		return nil, errors.New("test State unavailable")
	}, publisherAcquisitionEntry{}, binding); err != nil {
		t.Fatal(err)
	}
	if reads != 0 {
		t.Fatal("Publisher configuration read State before a Publish operation")
	}
	capability, err := owner.Admit(principal, broker.Administration)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.StartPublisher(t.Context(), publisherStartRequest{Principal: principal, Capability: capability, At: now}); err == nil || reads != 1 {
		t.Fatalf("StartPublisher read count = %d, err = %v", reads, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := instanceRoot.Close(); err != nil {
		t.Fatal(err)
	}
}

func publisherAcquisitionBinding(t *testing.T, network [32]byte, authority ed25519.PrivateKey, now time.Time) (*instance.Root, *instance.Binding) {
	t.Helper()
	root, err := instance.Initialize(instance.InitializeConfig{Root: serviceInstanceFixtureRoot(t), NetworkID: network,
		NotBefore: now, NotAfter: now.Add(time.Hour)})
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
	credential, err := (publication.Credential{InstancePublic: view.InstancePublic, IntroductionHPKEPublic: view.IntroductionPublic,
		Generation: 1, NotBefore: view.NotBefore, NotAfter: view.NotAfter, NetworkID: view.NetworkID,
		Capabilities: publication.CapabilityPublish | publication.CapabilityConnect}).Issue(authority)
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

type publisherAcquisitionView struct {
	epoch      state.ResolutionEpoch
	attachment state.PublisherAttachment
	initiator  state.ResolutionCandidate
}

func (view publisherAcquisitionView) Epoch(_, deadline time.Time) (state.ResolutionEpoch, bool) {
	view.attachment.NotAfter = deadline
	return view.epoch, true
}

func (view publisherAcquisitionView) PublisherAttachment(_, deadline time.Time) (state.PublisherAttachment, bool) {
	view.attachment.NotAfter = deadline
	return view.attachment, true
}

func (publisherAcquisitionView) CredentialIssuer(time.Time, time.Time) (state.TransitIssuer, bool) {
	return state.TransitIssuer{}, false
}

func (view publisherAcquisitionView) Candidate(nodeID [32]byte, _, _ time.Time) (state.ResolutionCandidate, bool) {
	return view.initiator, nodeID == view.initiator.NodeID
}

func (publisherAcquisitionView) Gateway(time.Time, time.Time) (state.DestinationResolutionGateway, bool) {
	return state.DestinationResolutionGateway{}, false
}

type publisherAcquisitionEntry struct{ contact entry.Candidate }

func (value publisherAcquisitionEntry) Contact() (entry.Candidate, error) { return value.contact, nil }

func (publisherAcquisitionEntry) Acquire(context.Context, entry.Attempt, entry.CandidateOpener) (net.Conn, func() error, error) {
	return nil, nil, nil
}
