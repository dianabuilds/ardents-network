//go:build h4_8_a11

package service_test

import (
	"context"
	"crypto/ed25519"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/route"
	servicepublication "github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

type h48A11ExpiryEntryOracle struct{ calls atomic.Int32 }

func (oracle *h48A11ExpiryEntryOracle) Acquire(context.Context, entry.Attempt, entry.CandidateOpener) (net.Conn, func() error, error) {
	oracle.calls.Add(1)
	return nil, nil, errors.New("expired descriptor reached Entry acquisition")
}

type h48A11ExpiryBrowserOracle struct{ calls atomic.Int32 }

func (oracle *h48A11ExpiryBrowserOracle) OpenReference(context.Context, endpointapi.ReferenceReady) error {
	oracle.calls.Add(1)
	return errors.New("expired outbound path invoked presentation opener")
}

func assertH48A11ReachabilityExpiry(t *testing.T, record []byte, authorityPublic, introductionPublic ed25519.PublicKey,
	instancePrivate ed25519.PrivateKey, network, target [32]byte, before, boundary time.Time) {
	t.Helper()
	current, err := servicepublication.Decode(record, authorityPublic, network, before)
	if err != nil {
		t.Fatalf("decode current publication for expiry Descriptor: %v", err)
	}
	introductionID, rendezvousID := referenceC2ID(181), referenceC2ID(182)
	descriptor, _, err := reachability.Issue(reachability.IssueInput{Current: current, InstanceSigner: instancePrivate,
		Introduction: reachability.Introduction{StateDigest: referenceC2ID(183), Epoch: 1,
			IntroductionNodeID: introductionID, RendezvousNodeID: rendezvousID, Reachability: referenceC2ID(184),
			JoinHandle: referenceC2ID(185), NotAfter: boundary, SubmissionAuthorization: []byte("a11-expiry-introduction")}})
	if err != nil {
		t.Fatalf("issue bounded Service Reachability Descriptor: %v", err)
	}
	if _, err := reachability.Verify(descriptor, target, network, before); err != nil {
		t.Fatalf("Service Reachability Descriptor at NotAfter-1s: %v", err)
	}
	if _, err := reachability.Verify(descriptor, target, network, boundary); err == nil ||
		!strings.Contains(err.Error(), "reachability descriptor publication or Introduction is invalid") {
		t.Fatalf("stale Service Reachability Descriptor exact-NotAfter refusal: %v", err)
	}

	principal := referenceC2ID(186)
	user, err := endpointapi.New(endpointapi.Setup{NetworkID: network, BrokerID: referenceC2ID(187),
		AuthorityPublic: authorityPublic, IntroductionPublic: introductionPublic, ConnectionPrincipal: principal,
		Clock: func() time.Time { return boundary }})
	if err != nil {
		t.Fatal(err)
	}
	defer user.Close()
	targetLink, err := targetlink.Encode(targetlink.Link{Network: network, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	capability, err := user.Admit(principal, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	entryOracle, browserOracle := &h48A11ExpiryEntryOracle{}, &h48A11ExpiryBrowserOracle{}
	peer := func(marker byte) endpointapi.TransitPeer {
		return endpointapi.TransitPeer{NodeID: referenceC2ID(marker), PublicKey: referenceC2ID(marker + 1),
			Family: referenceC2ID(marker + 2), Endpoint: "127.0.0.1:49" + string([]byte{'0' + marker%10, '0' + (marker+1)%10})}
	}
	site, openErr := user.OpenUserReferenceSite(context.Background(), endpointapi.UserReferenceSiteRequest{
		Reachability: &endpointapi.UserReachabilityRouteRequest{TargetLink: targetLink, Descriptor: descriptor,
			Introduction: peer(188), Initiator: peer(192), Rendezvous: peer(196), Entry: entryOracle,
			AttachmentID: referenceC2ID(200), EndpointHandshake: referenceC2ID(201), At: boundary},
		Principal: principal, Capability: capability, BytesEachDirection: 1, Browser: browserOracle})
	if openErr == nil || site != nil || !strings.Contains(openErr.Error(), "private reachability evidence is invalid") {
		if site != nil {
			_ = site.Close()
		}
		t.Fatalf("expired Descriptor opened a User Reference Site: site=%v err=%v", site != nil, openErr)
	}
	if entryOracle.calls.Load() != 0 || browserOracle.calls.Load() != 0 {
		t.Fatalf("expired Descriptor spent downstream work: entry=%d presentation=%d", entryOracle.calls.Load(), browserOracle.calls.Load())
	}

	outboundCapability, err := user.Admit(principal, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	routeSide, routePeer := net.Pipe()
	applicationSide, applicationPeer := net.Pipe()
	defer routeSide.Close()
	defer routePeer.Close()
	defer applicationSide.Close()
	defer applicationPeer.Close()
	var presentationCalls atomic.Int32
	result, connectErr := user.Connect(context.Background(), endpointapi.OutboundConnectionRequest{Principal: principal,
		Capability: outboundCapability, Target: target, Publication: record, Route: routeSide, Application: applicationSide,
		OnAuthenticated: func([32]byte) error { presentationCalls.Add(1); return nil }, BytesEachDirection: 1, At: boundary})
	if connectErr == nil || result.Class != "service target authentication failure" {
		t.Fatalf("expired outbound User work: class=%q err=%v", result.Class, connectErr)
	}
	if presentationCalls.Load() != 0 {
		t.Fatalf("expired outbound User path invoked OnAuthenticated presentation opener %d time(s)", presentationCalls.Load())
	}
}

var _ route.EntryAcquirer = (*h48A11ExpiryEntryOracle)(nil)
var _ endpointapi.ReferenceBrowser = (*h48A11ExpiryBrowserOracle)(nil)
