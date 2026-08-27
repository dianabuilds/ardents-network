package endpoint_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/browserentry"
	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func TestAlphaBrowserRuntimeUsesStateGatewayAndEntryForDemandedName(t *testing.T) {
	testAlphaBrowserRuntime(t, false)
}

func TestAlphaBrowserRuntimeIssuesMembershipGrantThroughStateCredentialRelay(t *testing.T) {
	testAlphaBrowserRuntime(t, true)
}

func testAlphaBrowserRuntime(t *testing.T, membership bool) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	deadline := now.Add(time.Minute)
	credentialDeadline := deadline
	if membership {
		credentialDeadline = now.Add(10 * time.Second)
	}
	slotDeadline := credentialDeadline
	network, digest := c2Identifier(131), c2Identifier(132)
	introductionID, rendezvousID := c2Identifier(133), c2Identifier(134)
	responderID, initiatorID, gatewayID := c2Identifier(135), c2Identifier(136), c2Identifier(137)
	introductionCertificate, introductionPublic := c2Certificate(t, 131, "introduction")
	rendezvousCertificate, rendezvousPublic := c2Certificate(t, 132, "rendezvous")
	responderCertificate, responderPublic := c2Certificate(t, 133, "responder")
	initiatorCertificate, initiatorPublic := c2Certificate(t, 134, "initiator")
	introductionAddress, rendezvousAddress := c2AvailableAddress(t), c2AvailableAddress(t)
	responderAddress, initiatorAddress := c2AvailableAddress(t), c2AvailableAddress(t)
	join, slotReachability, slotAttachment := c2Identifier(138), c2Identifier(139), c2Identifier(140)
	presentation := entry.Presentation{InviteID: c2Identifier(141), Invite: []byte("alpha-runtime-entry")}
	gatewayCertificate, gatewayPublic, gatewayPrivate := c2GatewayCertificate(t)
	transitAuthorityPublic, transitAuthorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	transitAuthorityID := sha256.Sum256(transitAuthorityPublic)
	var (
		issuerConfig node.CredentialIssuer
		issuerState  state.TransitIssuer
		issued       chan credential.Request
	)
	if membership {
		issuerCertificate, issuerPublic, issuerPrivate := c2GatewayCertificate(t)
		issuerID := c2Identifier(149)
		issued = make(chan credential.Request, 1)
		issuer, issueErr := credential.NewIssuer(credential.IssuerConfig{NetworkID: network, NodeID: issuerID,
			IdentityKey: issuerPrivate, GrantSigner: transitAuthorityPrivate, InitiatorNodeID: initiatorID, InitiatorPublicKey: initiatorPublic,
			CurrentDuty: func() (credential.StateDuty, bool) {
				return credential.StateDuty{NetworkID: network, Digest: digest, IssuerNodeID: issuerID, IssuerPublicKey: issuerPublic,
					InitiatorNodeID: initiatorID, InitiatorPublicKey: initiatorPublic, GrantAuthorityID: transitAuthorityID, Epoch: 10, NotAfter: deadline}, true
			}, Clock: func() time.Time { return now }, Authorize: func(request credential.Request, at time.Time) bool {
				if request.NetworkID != network || request.Digest != digest || request.Epoch != 10 ||
					request.IntroductionNodeID != introductionID || !request.NotAfter.Equal(credentialDeadline) || !at.Equal(now) {
					return false
				}
				issued <- request
				return true
			}})
		if issueErr != nil {
			t.Fatal(issueErr)
		}
		server := httptest.NewUnstartedServer(issuer.Handler())
		server.TLS, issueErr = issuer.TLSConfig(issuerCertificate)
		if issueErr != nil {
			t.Fatal(issueErr)
		}
		server.StartTLS()
		defer server.Close()
		profile, profileErr := credential.EncodeProfile(issuer.Profile())
		if profileErr != nil {
			t.Fatal(profileErr)
		}
		issuerConfig = node.CredentialIssuer{NodeID: issuerID, PublicKey: issuerPublic, ProfileDigest: sha256.Sum256(profile), URL: server.URL}
		issuerState = state.TransitIssuer{NodeID: issuerID, PublicKey: issuerPublic, Family: c2Identifier(149), Profile: profile,
			AssignmentNotAfter: deadline}
	}
	transitCertificate, _ := c2Certificate(t, 142, "alpha-runtime-transit-client")
	transitKey, err := route.ClientTLSKeyDigest(transitCertificate.Leaf)
	if err != nil {
		t.Fatal(err)
	}
	serviceAttachment := c2Identifier(143)
	introductionGrant := route.TransitGrant{IssuerID: transitAuthorityID, GrantID: c2Identifier(144), NetworkID: network,
		Digest: digest, AttachmentID: serviceAttachment, TransitNodeID: introductionID, ClientKeyDigest: transitKey, Epoch: 10,
		TransitRole: route.IntroductionRole, NotAfter: credentialDeadline}
	introductionAuthorization, err := route.IssueTransitGrant(introductionGrant, transitAuthorityPrivate)
	if err != nil {
		t.Fatal(err)
	}

	store, err := reachability.OpenStore(reachability.StoreConfig{Root: t.TempDir(), NetworkID: network})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	gateway, err := reachability.NewGateway(reachability.GatewayConfig{NetworkID: network, NodeID: gatewayID, IdentityKey: gatewayPrivate,
		AssignmentNotAfter: deadline, Store: store, Clock: func() time.Time { return now },
		AuthorizeDescriptor: func(value reachability.Descriptor, at time.Time) bool {
			return at.Equal(now) && value.Introduction.StateDigest == digest && value.Introduction.Epoch == 10 &&
				value.Introduction.IntroductionNodeID == introductionID && value.Introduction.RendezvousNodeID == rendezvousID
		}})
	if err != nil {
		t.Fatal(err)
	}
	gatewayServer := httptest.NewUnstartedServer(gateway.Handler())
	gatewayServer.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{gatewayCertificate}}
	gatewayServer.StartTLS()
	defer gatewayServer.Close()

	rendezvous, err := node.StartRendezvous(node.RendezvousConfig{ListenAddress: rendezvousAddress, CarrierProfile: route.CarrierTCP, Certificate: rendezvousCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: rendezvousID, NodePublicKey: rendezvousPublic, Epoch: 10, NotAfter: deadline,
		Peers: []node.RendezvousPeer{{NodeID: initiatorID, PublicKey: initiatorPublic, Role: route.InitiatorRole},
			{NodeID: responderID, PublicKey: responderPublic, Role: route.ResponderRole}},
		HandshakeLimit: 2, WaitingLimit: 2, PairLimit: 1, PairByteLimit: 256 << 10, AdmissionTimeout: time.Second, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer rendezvous.Close()
	initiator, err := node.StartInitiator(node.InitiatorConfig{ListenAddress: initiatorAddress, Certificate: initiatorCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: initiatorID, NodePublicKey: initiatorPublic, Epoch: 10, NotAfter: deadline,
		Rendezvous:        node.InitiatorPeer{NodeID: rendezvousID, PublicKey: rendezvousPublic, Endpoint: rendezvousAddress, CarrierProfile: route.CarrierTCP},
		ResolutionGateway: node.ResolutionGateway{NodeID: gatewayID, PublicKey: gatewayPublic, URL: gatewayServer.URL},
		CredentialIssuer:  issuerConfig,
		Admit:             alphaRuntimeEntryAdmit(presentation, network, digest, initiatorID, deadline),
		HandshakeLimit:    2, RelayLimit: 2, RelayByteLimit: 256 << 10, AdmissionTimeout: time.Second, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer initiator.Close()
	responder, err := node.StartResponder(node.ResponderConfig{ListenAddress: responderAddress, Certificate: responderCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: responderID, NodePublicKey: responderPublic, Epoch: 10, NotAfter: deadline,
		Rendezvous:     node.ResponderPeer{NodeID: rendezvousID, PublicKey: rendezvousPublic, Endpoint: rendezvousAddress, CarrierProfile: route.CarrierTCP},
		Admit:          alphaRuntimeResponderAdmit([]byte("alpha-runtime-responder"), network, digest, responderID, slotDeadline),
		HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 256 << 10, AdmissionTimeout: time.Second, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close()
	introductionAdmit := alphaRuntimeIntroductionAdmit([]byte("alpha-runtime-slot"), transitAuthorityPublic, introductionGrant, network, digest, introductionID, slotDeadline)
	if membership {
		introductionAdmit = alphaRuntimeMembershipIntroductionAdmit([]byte("alpha-runtime-slot"), transitAuthorityPublic, network, digest, introductionID, slotDeadline, credentialDeadline)
	}
	introduction, err := node.StartIntroduction(node.IntroductionConfig{ListenAddress: introductionAddress, Certificate: introductionCertificate,
		NetworkID: network, EpochDigest: digest, NodeID: introductionID, NodePublicKey: introductionPublic, Epoch: 10, NotAfter: deadline,
		Admit:          introductionAdmit,
		HandshakeLimit: 3, SlotLimit: 1, DeliveryLimit: 1, AdmissionTimeout: time.Second, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer introduction.Close()

	publisher, current, hpkePrivate, instancePrivate := c2PublishedEndpoint(t, network, now)
	defer publisher.Close()
	publisherSlot, err := publisher.OpenPublisherIntroduction(t.Context(), endpointapi.PublisherIntroductionRequest{
		Profile: endpointapi.PublisherIntroductionProfile{NetworkID: network, Digest: digest, Epoch: 10,
			Introduction:     endpointapi.TransitPeer{NodeID: introductionID, PublicKey: introductionPublic, Endpoint: introductionAddress},
			Rendezvous:       endpointapi.TransitPeer{NodeID: rendezvousID, PublicKey: rendezvousPublic, Endpoint: rendezvousAddress},
			Responder:        endpointapi.TransitPeer{NodeID: responderID, PublicKey: responderPublic, Endpoint: responderAddress},
			SlotAttachmentID: slotAttachment, Reachability: slotReachability, JoinHandle: join, NotAfter: slotDeadline,
			SlotAuthorization: []byte("alpha-runtime-slot"), ResponderAuthorization: []byte("alpha-runtime-responder")}, HPKEPrivate: hpkePrivate, At: now})
	if err != nil {
		t.Fatal(err)
	}
	defer publisherSlot.Close()
	submissionMode, submissionAuthorization := reachability.SubmissionFixedGrant, introductionAuthorization
	if membership {
		submissionMode, submissionAuthorization = reachability.SubmissionMembershipGrant, nil
	}
	descriptor, _, err := reachability.Issue(reachability.IssueInput{Current: current, InstanceSigner: instancePrivate,
		Introduction: reachability.Introduction{StateDigest: digest, Epoch: 10, IntroductionNodeID: introductionID, RendezvousNodeID: rendezvousID,
			Reachability: slotReachability, JoinHandle: join, NotAfter: slotDeadline, SubmissionMode: submissionMode,
			SubmissionAuthorization: submissionAuthorization}})
	if err != nil {
		t.Fatal(err)
	}
	if result, err := gateway.Publish(descriptor, now); err != nil || result.Class != reachability.StoreAccepted {
		t.Fatalf("Gateway Publish = %+v, %v", result, err)
	}

	stateProfile, err := reachability.EncodeGatewayProfile(gateway.Profile())
	if err != nil {
		t.Fatal(err)
	}
	var transitAuthorityStateKey [32]byte
	copy(transitAuthorityStateKey[:], transitAuthorityPublic)
	currentState := alphaRuntimeState{epoch: state.ResolutionEpoch{Generation: "alpha-runtime", NetworkID: network, Number: 10, Digest: digest,
		ViewRoot: c2Identifier(145), Authorities: []state.ResolutionAuthority{{ID: transitAuthorityID, PublicKey: transitAuthorityStateKey}}, Threshold: 1},
		gateway: state.DestinationResolutionGateway{NodeID: gatewayID, PublicKey: gatewayPublic, Family: c2Identifier(137), Profile: stateProfile,
			AssignmentNotAfter: deadline}, issuer: issuerState, candidates: map[[32]byte]state.ResolutionCandidate{
			initiatorID:    {NodeID: initiatorID, PublicKey: initiatorPublic, Family: "initiator-family", Endpoint: initiatorAddress, Domain: "initiator", AssignmentNotAfter: deadline},
			introductionID: {NodeID: introductionID, PublicKey: introductionPublic, Family: "introduction-family", Endpoint: introductionAddress, Domain: "introduction", AssignmentNotAfter: deadline},
			rendezvousID:   {NodeID: rendezvousID, PublicKey: rendezvousPublic, Family: "rendezvous-family", Endpoint: rendezvousAddress, Domain: "rendezvous", AssignmentNotAfter: deadline},
		}}
	entryCandidate := entry.Candidate{NodeID: initiatorID, PublicKey: initiatorPublic, FamilyID: sha256.Sum256([]byte("initiator-family")), Endpoint: initiatorAddress}
	alphaFloor := alphaRuntimeFloor(t, network, current.Credential.Target, now)
	defer alphaFloor.Close()
	statePath := filepath.Join(t.TempDir(), "browser-entry.json")
	transitClients := map[[32]byte]tls.Certificate{introductionGrant.GrantID: transitCertificate}
	if membership {
		transitClients = nil
	}
	user, err := endpointapi.New(endpointapi.Setup{NetworkID: network, BrokerID: c2Identifier(146), AuthorityPublic: current.Credential.AuthorityPublic[:],
		IntroductionPublic: make([]byte, 32), ConnectionPrincipal: c2Identifier(147), BrowserEntryStatePath: statePath,
		TransitClientCertificates: transitClients})
	if err != nil {
		t.Fatal(err)
	}
	defer user.Close()
	publisherApplication, serviceApplication := net.Pipe()
	defer serviceApplication.Close()
	publisherSession, err := publisher.Admit(c2Identifier(41), "connection")
	if err != nil {
		t.Fatal(err)
	}
	publisherDone := make(chan serviceOutcome, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	go func() {
		result, runErr := publisherSlot.Accept(ctx, endpointapi.InboundConnectionRequest{Principal: c2Identifier(41), Capability: publisherSession,
			Application: publisherApplication, BytesEachDirection: 64 << 10, At: now})
		publisherDone <- serviceOutcome{result, runErr}
	}()
	requestSeen := make(chan *http.Request, 1)
	go serveOneTransparentReference(serviceApplication, requestSeen)
	runtime, err := user.OpenAlphaBrowserRuntime(ctx, endpointapi.AlphaBrowserRuntimeRequest{Floor: alphaFloor,
		Current:   func() (endpointapi.AlphaBrowserStateView, error) { return currentState, nil },
		Entry:     alphaRuntimeEntry{c2EntryAcquirer: c2EntryAcquirer{candidate: entryCandidate, presentation: presentation}, contact: entryCandidate},
		Principal: c2Identifier(147), BytesEachDirection: 64 << 10, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	port, err := nativeBrowserEntryPortResult(statePath)
	if err != nil {
		t.Fatal(err)
	}
	authentication, err := nativeBrowserEntryProxyAuthentication(statePath)
	if err != nil || authentication.Port != port {
		t.Fatalf("Browser Entry authentication = %+v, %v", authentication, err)
	}
	proxy, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	proxy.User = url.UserPassword(browserentry.ProxyUsername, authentication.Password)
	browser := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy), DisableCompression: true}}
	browserRequest, err := http.NewRequest(http.MethodPost, "http://reference.ard/publish?draft=1", strings.NewReader("post=ready"))
	if err != nil {
		t.Fatal(err)
	}
	browserRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := browser.Do(browserRequest)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusCreated || string(body) != "published" {
		t.Logf("runtime node usage: initiator=%+v responder=%+v rendezvous=%+v", initiator.Usage(), responder.Usage(), rendezvous.Usage())
		t.Fatalf("runtime browser response = %d %q %v", response.StatusCode, body, readErr)
	}
	if request := <-requestSeen; request == nil || request.Method != http.MethodPost || request.Host != "reference.ard" || request.URL.String() != "/publish?draft=1" {
		t.Fatalf("Publisher request through State runtime = %#v", request)
	}
	if membership {
		select {
		case request := <-issued:
			if request.AttachmentID == [32]byte{} || request.ClientKeyDigest == [32]byte{} {
				t.Fatalf("membership credential request omitted its adjacent-hop binding: %+v", request)
			}
		default:
			t.Fatal("State-selected issuer did not authorize a membership credential")
		}
	}
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(statePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("closed runtime retained Browser Entry state: %v", err)
	}
	select {
	case <-publisherDone:
	case <-time.After(time.Second):
		t.Fatal("publisher did not terminate after runtime withdrawal")
	}
}

type alphaRuntimeState struct {
	epoch      state.ResolutionEpoch
	gateway    state.DestinationResolutionGateway
	issuer     state.TransitIssuer
	candidates map[[32]byte]state.ResolutionCandidate
}

func (value alphaRuntimeState) Epoch(at, deadline time.Time) (state.ResolutionEpoch, bool) {
	return value.epoch, !at.IsZero() && at.Before(deadline) && !deadline.After(at.Add(15*time.Second))
}

func (value alphaRuntimeState) Gateway(at, deadline time.Time) (state.DestinationResolutionGateway, bool) {
	return value.gateway, !at.IsZero() && at.Before(deadline) && !deadline.After(at.Add(15*time.Second))
}

func (value alphaRuntimeState) CredentialIssuer(at, deadline time.Time) (state.TransitIssuer, bool) {
	return value.issuer, value.issuer.NodeID != [32]byte{} && !at.IsZero() && at.Before(deadline) &&
		!deadline.After(at.Add(15*time.Second)) && !value.issuer.AssignmentNotAfter.Before(deadline)
}

func (value alphaRuntimeState) Candidate(nodeID [32]byte, at, deadline time.Time) (state.ResolutionCandidate, bool) {
	candidate, found := value.candidates[nodeID]
	return candidate, found && at.Before(deadline) && !candidate.AssignmentNotAfter.Before(deadline)
}

type alphaRuntimeEntry struct {
	c2EntryAcquirer
	contact entry.Candidate
}

func (value alphaRuntimeEntry) Contact() (entry.Candidate, error) { return value.contact, nil }

func alphaRuntimeEntryAdmit(presentation entry.Presentation, network, digest, initiator [32]byte, deadline time.Time) route.EntryBindingAdmitter {
	return func(invite []byte, attachment, key [32]byte, notAfter time.Time) (route.EntryAdmission, error) {
		if string(invite) != string(presentation.Invite) || attachment == [32]byte{} || key == [32]byte{} || notAfter.After(deadline) {
			return route.EntryAdmission{}, errors.New("unexpected alpha runtime Entry admission")
		}
		return route.EntryAdmission{InviteID: presentation.InviteID, NetworkID: network, Digest: digest, Epoch: 10, InitiatorNodeID: initiator, NotAfter: notAfter}, nil
	}
}

func alphaRuntimeIntroductionAdmit(slotAuthorization []byte, authority ed25519.PublicKey, expected route.TransitGrant,
	network, digest, introduction [32]byte, deadline time.Time) route.EndpointTransitBindingAdmitter {
	return func(received []byte, attachment, key [32]byte, role byte, nodeID [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
		if string(received) == string(slotAuthorization) {
			if attachment == [32]byte{} || key == [32]byte{} || role != route.IntroductionRole || nodeID != introduction || !notAfter.Equal(deadline) {
				return route.EndpointTransitAdmission{}, errors.New("unexpected alpha runtime Publisher slot admission")
			}
			return route.EndpointTransitAdmission{AuthorizationID: c2Identifier(148), NetworkID: network, Digest: digest, Epoch: 10,
				TransitRole: route.IntroductionRole, TransitNodeID: introduction, NotAfter: deadline}, nil
		}
		grant, err := route.VerifyTransitGrant(received, authority)
		if err != nil || grant != expected || attachment != grant.AttachmentID || key != grant.ClientKeyDigest || role != grant.TransitRole ||
			nodeID != grant.TransitNodeID || !notAfter.Equal(grant.NotAfter) {
			return route.EndpointTransitAdmission{}, errors.New("unexpected alpha runtime User Transit Grant admission")
		}
		return route.EndpointTransitAdmission{AuthorizationID: grant.GrantID, NetworkID: grant.NetworkID, Digest: grant.Digest, Epoch: grant.Epoch,
			TransitRole: grant.TransitRole, TransitNodeID: grant.TransitNodeID, NotAfter: grant.NotAfter}, nil
	}
}

func alphaRuntimeMembershipIntroductionAdmit(slotAuthorization []byte, authority ed25519.PublicKey, network, digest, introduction [32]byte,
	slotDeadline, grantDeadline time.Time) route.EndpointTransitBindingAdmitter {
	return func(received []byte, attachment, key [32]byte, role byte, nodeID [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
		if string(received) == string(slotAuthorization) {
			if attachment == [32]byte{} || key == [32]byte{} || role != route.IntroductionRole || nodeID != introduction || !notAfter.Equal(slotDeadline) {
				return route.EndpointTransitAdmission{}, errors.New("unexpected alpha runtime Publisher slot admission")
			}
			return route.EndpointTransitAdmission{AuthorizationID: c2Identifier(148), NetworkID: network, Digest: digest, Epoch: 10,
				TransitRole: route.IntroductionRole, TransitNodeID: introduction, NotAfter: slotDeadline}, nil
		}
		grant, err := route.VerifyTransitGrant(received, authority)
		if err != nil || grant.NetworkID != network || grant.Digest != digest || grant.Epoch != 10 ||
			grant.AttachmentID != attachment || grant.ClientKeyDigest != key || grant.TransitRole != role ||
			grant.TransitNodeID != nodeID || nodeID != introduction || !notAfter.Equal(grantDeadline) || !grant.NotAfter.Equal(grantDeadline) {
			return route.EndpointTransitAdmission{}, errors.New("unexpected alpha runtime membership Transit Grant admission")
		}
		return route.EndpointTransitAdmission{AuthorizationID: grant.GrantID, NetworkID: network, Digest: digest, Epoch: 10,
			TransitRole: route.IntroductionRole, TransitNodeID: introduction, NotAfter: grantDeadline}, nil
	}
}

func alphaRuntimeResponderAdmit(authorization []byte, network, digest, responder [32]byte, deadline time.Time) route.EndpointTransitBindingAdmitter {
	return func(received []byte, attachment, key [32]byte, role byte, nodeID [32]byte, notAfter time.Time) (route.EndpointTransitAdmission, error) {
		if string(received) != string(authorization) || attachment == [32]byte{} || key == [32]byte{} || role != route.ResponderRole ||
			nodeID != responder || !notAfter.Equal(deadline) {
			return route.EndpointTransitAdmission{}, errors.New("unexpected alpha runtime Responder admission")
		}
		return route.EndpointTransitAdmission{AuthorizationID: c2Identifier(147), NetworkID: network, Digest: digest, Epoch: 10,
			TransitRole: route.ResponderRole, TransitNodeID: responder, NotAfter: deadline}, nil
	}
}

func alphaRuntimeFloor(t *testing.T, network, target [32]byte, now time.Time) *alpha.PersistentFloor {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://reference")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "alpha-browser-runtime", Network: network, Serial: 1,
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute), Bindings: []alpha.BindingInput{{Link: link, Target: target}}}, private)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(public, raw)
	if err != nil {
		t.Fatal(err)
	}
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: t.TempDir(), Authority: public, Cohort: "alpha-browser-runtime", Network: network})
	if err != nil {
		t.Fatal(err)
	}
	if err := floor.Observe(corpus); err != nil {
		t.Fatal(err)
	}
	return floor
}
