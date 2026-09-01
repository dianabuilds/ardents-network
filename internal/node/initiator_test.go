package node

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
	"github.com/openpcc/ohttp"
)

func TestInitiatorRelaysOnlyAfterExactSetupAndReady(t *testing.T) {
	rendezvous, material, rendezvousConfig := rendezvousFixture(t)
	attachment := [32]byte{21}
	responder, err := openRendezvousLeg(t.Context(), rendezvousConfig.ListenAddress, material.responder, material.serverPublic,
		legFor(material, attachment, route.ResponderRole, rendezvousConfig.NotAfter))
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close()
	presentation := entry.Presentation{InviteID: [32]byte{22}, Invite: []byte{2, 4, 6, 8}}
	initiator, err := startInitiator(initiatorConfig{ListenAddress: availableLoopbackEndpoint(t), Certificate: material.initiator,
		NetworkID: rendezvousConfig.NetworkID, EpochDigest: rendezvousConfig.EpochDigest, NodeID: [32]byte{4},
		NodePublicKey: material.initiatorPublic, Epoch: rendezvousConfig.Epoch, NotAfter: rendezvousConfig.NotAfter,
		rendezvous: initiatorPeer{NodeID: rendezvousConfig.NodeID, PublicKey: material.serverPublic, Endpoint: rendezvousConfig.ListenAddress, CarrierProfile: route.CarrierTCP},
		Admit:      initiatorAdmission(presentation, attachment, rendezvousConfig), HandshakeLimit: 2, RelayLimit: 1,
		RelayByteLimit: 1024, AdmissionTimeout: time.Second, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer initiator.Close()
	candidate := entry.Candidate{NodeID: [32]byte{4}, PublicKey: material.initiatorPublic, Endpoint: initiator.listener.Addr().String()}
	acquirer := initiatorEntryAcquirer{candidate: candidate, presentation: presentation}
	connection, cleanup, err := route.OpenEntryAttachment(t.Context(), acquirer, route.EntryAttachmentRequest{
		NetworkID: rendezvousConfig.NetworkID, Digest: rendezvousConfig.EpochDigest, Epoch: rendezvousConfig.Epoch,
		AttachmentID: attachment, Deadline: rendezvousConfig.NotAfter})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	setup := route.RelaySetup{NetworkID: rendezvousConfig.NetworkID, Digest: rendezvousConfig.EpochDigest, AttachmentID: attachment,
		Epoch: rendezvousConfig.Epoch, TransitRole: route.InitiatorRole, NextRole: route.RendezvousRole,
		TransitNodeID: [32]byte{4}, NextNodeID: rendezvousConfig.NodeID, NextNodePublicKey: material.serverPublic,
		NotAfter: rendezvousConfig.NotAfter}
	if err := route.WriteRelaySetup(connection, setup); err != nil {
		t.Fatal(err)
	}
	ready, err := route.ReadRelayReady(connection)
	if err != nil || setup.VerifyRelayReady(ready) != nil {
		t.Fatalf("RelayReady err=%v verify=%v", err, setup.VerifyRelayReady(ready))
	}
	if _, err := connection.Write([]byte("from user")); err != nil {
		t.Fatal(err)
	}
	if got := readExact(t, responder, len("from user")); string(got) != "from user" {
		t.Fatalf("Responder bytes = %q", got)
	}
	if _, err := responder.Write([]byte("from responder")); err != nil {
		t.Fatal(err)
	}
	if got := readExact(t, connection, len("from responder")); string(got) != "from responder" {
		t.Fatalf("User bytes = %q", got)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := initiator.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if usage := initiator.Usage(); usage.ActiveRelays != 0 || usage.Connections != 0 || usage.CompletedRelays != 1 {
		t.Fatalf("Initiator terminal usage = %+v", usage)
	}
	if err := rendezvous.Drain(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestInitiatorDutyUsesOnlyStateAssignedRendezvous(t *testing.T) {
	certificate, public := rendezvousCertificate(t, 31, "state-initiator")
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := dutyFacts{NetworkID: [32]byte{31}, Epoch: 32, Digest: [32]byte{33}, Profile: route.Profile,
		NodeID: [32]byte{34}, NodePublicKey: public, Assignment: "initiator", ProbeEndpoint: "127.0.0.1:30234",
		EpochValidFrom: now.Add(-time.Second), ValidUntil: now.Add(time.Minute), RecordValidUntil: now.Add(time.Minute),
		Candidates: [64]dutyCandidate{{NodeID: [32]byte{35}, PublicKey: [32]byte{36}, Endpoint: "127.0.0.1:30235", CarrierProfile: string(route.CarrierQUIC),
			Assignment: "rendezvous", ValidFrom: now.Add(-time.Second), ValidUntil: now.Add(time.Minute)}}, CandidateCount: 1}
	profile := InitiatorProfile{Certificate: certificate, HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 1024, AdmissionTimeout: time.Second, DrainTimeout: time.Second}
	admit := func([]byte, [32]byte, [32]byte, time.Time) (route.EntryAdmission, error) {
		return route.EntryAdmission{}, nil
	}
	plan, err := initiatorDuty(profile, snapshot, admit)
	if err != nil {
		t.Fatal(err)
	}
	if plan.rendezvous.NodeID != snapshot.Candidates[0].NodeID || plan.rendezvous.PublicKey != snapshot.Candidates[0].PublicKey ||
		plan.rendezvous.Endpoint != snapshot.Candidates[0].Endpoint || plan.rendezvous.CarrierProfile != route.CarrierQUIC {
		t.Fatalf("Initiator State duty peer = %+v", plan.rendezvous)
	}
	snapshot.Candidates[1] = dutyCandidate{NodeID: [32]byte{37}, PublicKey: [32]byte{38}, Endpoint: "127.0.0.1:30236",
		CarrierProfile: string(route.CarrierTCP), Assignment: "rendezvous", ValidFrom: now.Add(-time.Second), ValidUntil: now.Add(time.Minute)}
	snapshot.CandidateCount = 2
	if _, err := initiatorDuty(profile, snapshot, admit); err == nil {
		t.Fatal("Initiator accepted an ambiguous State Rendezvous peer set")
	}
}

func TestEntryViewRetainsOnlyAuthenticatedInitiatorVerificationFacts(t *testing.T) {
	until := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	snapshot := dutyFacts{NetworkID: [32]byte{71}, Epoch: 72, Digest: [32]byte{73}, Profile: route.Profile, Fresh: true,
		CandidateCount: 1, Candidates: [64]dutyCandidate{{NodeID: [32]byte{74}, PublicKey: [32]byte{75}, KeyID: [32]byte{76},
			FamilyID: [32]byte{77}, RecordDigest: [32]byte{78}, DomainProofDigest: [32]byte{79}, Endpoint: "127.0.0.1:3074",
			Capacity: 1, Assignment: "initiator", ValidFrom: until.Add(-time.Minute), ValidUntil: until, AssignmentNotAfter: until}}}
	view, err := entryView(snapshot)
	if err != nil || len(view.Candidates) != 1 || view.Candidates[0].NodeID != snapshot.Candidates[0].NodeID ||
		view.Candidates[0].FamilyID != snapshot.Candidates[0].FamilyID || view.Candidates[0].AssignmentNotAfter != until {
		t.Fatalf("Entry verification view = %+v, %v", view, err)
	}
}

func TestInitiatorForwardsOneOpaqueResolutionEnvelopeToExactGateway(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	runningRendezvous, material, rendezvousConfig := rendezvousFixture(t)
	defer runningRendezvous.Close()
	gatewayCertificate, gatewayPublic, gatewayPrivate := resolutionGatewayCertificate(t)
	store, err := reachability.OpenStore(reachability.StoreConfig{Root: t.TempDir(), NetworkID: rendezvousConfig.NetworkID})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	gateway, err := reachability.NewGateway(reachability.GatewayConfig{NetworkID: rendezvousConfig.NetworkID, NodeID: [32]byte{48},
		IdentityKey: gatewayPrivate, AssignmentNotAfter: now.Add(time.Minute), Store: store, Clock: func() time.Time { return now },
		AuthorizeDescriptor: func(reachability.Descriptor, time.Time) bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(gateway.Handler())
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{gatewayCertificate}}
	server.StartTLS()
	defer server.Close()
	attachment := [32]byte{49}
	presentation := entry.Presentation{InviteID: [32]byte{50}, Invite: []byte{5, 0, 5}}
	initiator, err := startInitiator(initiatorConfig{ListenAddress: availableLoopbackEndpoint(t), Certificate: material.initiator,
		NetworkID: rendezvousConfig.NetworkID, EpochDigest: rendezvousConfig.EpochDigest, NodeID: [32]byte{4},
		NodePublicKey: material.initiatorPublic, Epoch: rendezvousConfig.Epoch, NotAfter: rendezvousConfig.NotAfter,
		rendezvous:        initiatorPeer{NodeID: rendezvousConfig.NodeID, PublicKey: material.serverPublic, Endpoint: rendezvousConfig.ListenAddress, CarrierProfile: route.CarrierTCP},
		resolutionGateway: resolutionGateway{NodeID: [32]byte{48}, PublicKey: gatewayPublic, URL: server.URL},
		Admit:             initiatorAdmission(presentation, attachment, rendezvousConfig), HandshakeLimit: 2, RelayLimit: 1,
		RelayByteLimit: 1024, AdmissionTimeout: time.Second, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer initiator.Close()
	candidate := entry.Candidate{NodeID: [32]byte{4}, PublicKey: material.initiatorPublic, Endpoint: initiator.listener.Addr().String()}
	connection, cleanup, err := route.OpenEntryAttachment(t.Context(), initiatorEntryAcquirer{candidate: candidate, presentation: presentation}, route.EntryAttachmentRequest{
		NetworkID: rendezvousConfig.NetworkID, Digest: rendezvousConfig.EpochDigest, Epoch: rendezvousConfig.Epoch,
		AttachmentID: attachment, Deadline: rendezvousConfig.NotAfter})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	profile := gateway.Profile()
	var key ohttp.KeyConfig
	if err := key.UnmarshalBinary(profile.KeyConfig); err != nil {
		t.Fatal(err)
	}
	transport, err := ohttp.NewTransport(key, "https://relay.invalid/ohttp")
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://ohttp.invalid/resolve", bytes.NewReader(make([]byte, 4096)))
	if err != nil {
		t.Fatal(err)
	}
	encapsulated, decapsulator, err := transport.Encapsulate(request)
	if err != nil {
		t.Fatal(err)
	}
	opaque, err := io.ReadAll(encapsulated.Body)
	if err != nil {
		t.Fatal(err)
	}
	setup := route.ResolutionRelaySetup{NetworkID: rendezvousConfig.NetworkID, Digest: rendezvousConfig.EpochDigest, AttachmentID: attachment,
		InitiatorNodeID: [32]byte{4}, GatewayNodeID: [32]byte{48}, GatewayNodePublicKey: gatewayPublic, Epoch: rendezvousConfig.Epoch,
		NotAfter: rendezvousConfig.NotAfter, EnvelopeCapacity: route.ResolutionEnvelopeCapacity}
	if err := route.WriteResolutionRelaySetup(connection, setup); err != nil {
		t.Fatal(err)
	}
	ready, err := route.ReadResolutionRelayReady(connection)
	if err != nil || setup.VerifyResolutionRelayReady(ready) != nil {
		t.Fatalf("ResolutionRelayReady = %+v, %v", ready, err)
	}
	if err := route.WriteResolutionRelayEnvelope(connection, route.ResolutionRelayEnvelope{OHTTP: opaque}); err != nil {
		t.Fatal(err)
	}
	response, err := route.ReadResolutionRelayResponse(connection)
	if err != nil {
		t.Fatal(err)
	}
	contentType := ohttp.ResponseMediaType
	if response.Framing == route.ResolutionOHTTPChunkedResponse {
		contentType = ohttp.ChunkedResponseMediaType
	}
	plain, err := decapsulator.Decapsulate(t.Context(), &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{contentType}},
		Body: io.NopCloser(bytes.NewReader(response.OHTTP)), Request: encapsulated})
	if err != nil {
		t.Fatal(err)
	}
	defer plain.Body.Close()
	body, err := io.ReadAll(plain.Body)
	if err != nil || len(body) != 4096 {
		t.Fatalf("resolution Gateway response = %d bytes, %v", len(body), err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := initiator.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if usage := initiator.Usage(); usage.CompletedRelays != 1 || usage.RelayedBytes != uint64(len(opaque)) || usage.Connections != 0 {
		t.Fatalf("resolution Initiator usage = %+v", usage)
	}
}

func TestInitiatorForwardsOneOpaqueCredentialEnvelopeToExactIssuer(t *testing.T) {
	_, material, rendezvousConfig := rendezvousFixture(t)
	issuerCertificate, issuerPublic, issuerPrivate := resolutionGatewayCertificate(t)
	request := credential.Request{RequestID: [32]byte{80}, NetworkID: rendezvousConfig.NetworkID, Digest: rendezvousConfig.EpochDigest,
		TransitNodeID: [32]byte{81}, AttachmentID: [32]byte{82}, ClientKeyDigest: [32]byte{83}, Epoch: rendezvousConfig.Epoch,
		TransitRole: route.IntroductionRole, NotAfter: rendezvousConfig.NotAfter.Add(-time.Second)}
	issuer := openNodeTestIssuer(t, request.NetworkID, [32]byte{84}, issuerPrivate, [32]byte{4}, material.initiatorPublic,
		rendezvousConfig.NotAfter, 2, func() time.Time { return time.Now().UTC() },
		func(profile credential.Profile, profileDigest [32]byte) (credential.StateDuty, bool) {
			return credential.StateDuty{Generation: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", NetworkID: request.NetworkID, Digest: request.Digest, IssuerNodeID: [32]byte{84},
				IssuerPublicKey: issuerPublic, InitiatorNodeID: [32]byte{4}, InitiatorPublicKey: material.initiatorPublic,
				GrantSignerPublicKey: profile.GrantSignerPublicKey, ProfileDigest: profileDigest,
				Epoch: request.Epoch, NotAfter: rendezvousConfig.NotAfter, Fresh: true}, true
		})
	defer func() { _ = issuer.Close() }()
	server := httptest.NewUnstartedServer(issuer.Handler())
	serverTLS, err := issuer.TLSConfig(issuerCertificate)
	if err != nil {
		t.Fatal(err)
	}
	server.TLS = serverTLS
	server.StartTLS()
	defer server.Close()
	profile, err := credential.EncodeProfile(issuer.Profile())
	if err != nil {
		t.Fatal(err)
	}
	carrierAttachment := [32]byte{85}
	presentation := entry.Presentation{InviteID: [32]byte{86}, Invite: []byte{8, 6, 4, 2}}
	initiator, err := startInitiator(initiatorConfig{ListenAddress: availableLoopbackEndpoint(t), Certificate: material.initiator,
		NetworkID: rendezvousConfig.NetworkID, EpochDigest: rendezvousConfig.EpochDigest, NodeID: [32]byte{4}, NodePublicKey: material.initiatorPublic,
		Epoch: rendezvousConfig.Epoch, NotAfter: rendezvousConfig.NotAfter,
		rendezvous:       initiatorPeer{NodeID: rendezvousConfig.NodeID, PublicKey: material.serverPublic, Endpoint: rendezvousConfig.ListenAddress, CarrierProfile: route.CarrierTCP},
		credentialIssuer: credentialIssuer{NodeID: [32]byte{84}, PublicKey: issuerPublic, ProfileDigest: sha256.Sum256(profile), URL: server.URL},
		Admit:            initiatorAdmission(presentation, carrierAttachment, rendezvousConfig), HandshakeLimit: 2, RelayLimit: 1, RelayByteLimit: 1024, AdmissionTimeout: time.Second, DrainTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer initiator.Close()
	wrongSetup := route.CredentialRelaySetup{NetworkID: rendezvousConfig.NetworkID, Digest: rendezvousConfig.EpochDigest,
		AttachmentID: carrierAttachment, InitiatorNodeID: [32]byte{4}, IssuerNodeID: [32]byte{84}, IssuerNodePublicKey: issuerPublic,
		IssuerProfileDigest: sha256.Sum256([]byte("substituted-profile")), Epoch: rendezvousConfig.Epoch,
		NotAfter: request.NotAfter, EnvelopeCapacity: route.CredentialEnvelopeCapacity}
	if err := initiator.validateCredentialSetup(wrongSetup); err == nil {
		t.Fatal("Initiator accepted a Credential Relay setup with a substituted issuer profile")
	}
	candidate := entry.Candidate{NodeID: [32]byte{4}, PublicKey: material.initiatorPublic, Endpoint: initiator.listener.Addr().String()}
	connection, cleanup, err := route.OpenEntryAttachment(t.Context(), initiatorEntryAcquirer{candidate: candidate, presentation: presentation}, route.EntryAttachmentRequest{
		NetworkID: rendezvousConfig.NetworkID, Digest: rendezvousConfig.EpochDigest, Epoch: rendezvousConfig.Epoch, AttachmentID: carrierAttachment, Deadline: rendezvousConfig.NotAfter})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	setup := route.CredentialRelaySetup{NetworkID: rendezvousConfig.NetworkID, Digest: rendezvousConfig.EpochDigest, AttachmentID: carrierAttachment,
		InitiatorNodeID: [32]byte{4}, IssuerNodeID: [32]byte{84}, IssuerNodePublicKey: issuerPublic, IssuerProfileDigest: sha256.Sum256(profile), Epoch: rendezvousConfig.Epoch,
		NotAfter: request.NotAfter, EnvelopeCapacity: route.CredentialEnvelopeCapacity}
	if err := route.WriteCredentialRelaySetup(connection, setup); err != nil {
		t.Fatal(err)
	}
	ready, err := route.ReadCredentialRelayReady(connection)
	if err != nil || setup.VerifyCredentialRelayReady(ready) != nil {
		t.Fatalf("CredentialRelayReady = %+v, %v", ready, err)
	}
	client, err := credential.OpenClient(credential.ClientConfig{NetworkID: request.NetworkID, IssuerPublic: issuerPublic, Profile: issuer.Profile(),
		At: time.Now().UTC(), Deadline: request.NotAfter,
		Exchange: func(_ context.Context, envelope []byte) ([]byte, error) {
			if err := route.WriteCredentialRelayEnvelope(connection, route.CredentialRelayEnvelope{OHTTP: envelope}); err != nil {
				return nil, err
			}
			response, readErr := route.ReadCredentialRelayResponse(connection)
			if readErr != nil || response.Framing != route.CredentialOHTTPResponse {
				return nil, errors.New("credential relay response is invalid")
			}
			return response.OHTTP, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Issue(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != credential.Issued {
		t.Fatalf("credential Relay outcome = %q", result.Outcome)
	}
	issuerProfile := issuer.Profile()
	grant, err := route.VerifyTransitGrant(result.Grant, ed25519.PublicKey(issuerProfile.GrantSignerPublicKey[:]))
	if err != nil || grant.AttachmentID != request.AttachmentID || grant.ClientKeyDigest != request.ClientKeyDigest ||
		grant.TransitNodeID != request.TransitNodeID || grant.TransitRole != request.TransitRole {
		t.Fatalf("credential Relay Grant = %+v, %v", grant, err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := initiator.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if usage := initiator.Usage(); usage.CompletedRelays != 1 || usage.Connections != 0 || usage.RelayedBytes == 0 {
		t.Fatalf("credential Initiator usage = %+v", usage)
	}
}

func resolutionGatewayCertificate(t *testing.T) (tls.Certificate, [32]byte, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(90), Subject: pkix.Name{CommonName: "resolution-gateway"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	var identifier [32]byte
	copy(identifier[:], public)
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}, identifier, private
}

func initiatorAdmission(presentation entry.Presentation, attachment [32]byte, config rendezvousConfig) route.EntryBindingAdmitter {
	return func(invite []byte, received, key [32]byte, notAfter time.Time) (route.EntryAdmission, error) {
		if string(invite) != string(presentation.Invite) || received != attachment || key == [32]byte{} || !notAfter.Equal(config.NotAfter) {
			return route.EntryAdmission{}, errors.New("unexpected Entry admission")
		}
		return route.EntryAdmission{InviteID: presentation.InviteID, NetworkID: config.NetworkID, Digest: config.EpochDigest,
			Epoch: config.Epoch, InitiatorNodeID: [32]byte{4}, NotAfter: config.NotAfter}, nil
	}
}

type initiatorEntryAcquirer struct {
	candidate    entry.Candidate
	presentation entry.Presentation
}

func (input initiatorEntryAcquirer) Acquire(ctx context.Context, attempt entry.Attempt, opener entry.CandidateOpener) (net.Conn, func() error, error) {
	connection, cleanup, complete, err := opener(ctx, input.candidate, input.presentation, attempt.Deadline)
	if err != nil || !complete {
		return nil, cleanup, err
	}
	return connection, cleanup, nil
}
