package main

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

type peer struct {
	NodeID, PublicKey, Endpoint, Certificate, PrivateKey string
}

// transitCredential is test-only closed-alpha provisioning material for one
// State-authorized Transit Grant and its matching ephemeral TLS key pair.
// It is never a participant configuration format.
type transitCredential struct {
	Grant, Certificate, PrivateKey string
}

type stateSource struct {
	Address, ServerName, Root, LeafKeyDigest string
}

type stateClient struct {
	Certificate, PrivateKey string
}

type config struct {
	Schema, Network, Digest, Deadline, PublicationPath, PublisherRoot, ReadyRoot, CompletePath, ResourceProofPath string
	Epoch                                                                                                         uint64
	Introduction, Rendezvous, Responder, Initiator, Gateway                                                       peer
	JoinHandle, Reachability, SlotAttachment, ServiceAttachment, ResolutionAttachment                             string
	GatewayRoot, GatewayProfilePath                                                                               string
	TransitAuthority, InviteID, Invite                                                                            string
	SlotCredential, ResponderCredential, IntroductionCredential                                                   transitCredential
	TransitStateRoots                                                                                             map[string]string
	TransitStateMaterials                                                                                         map[string]uint32
	TransitStateSources                                                                                           []stateSource
	TransitStateClient                                                                                            stateClient
	FirefoxExecutable                                                                                             string
	PublisherOffline                                                                                              bool
}

type publicationEnvelope struct {
	AuthorityPublic, Publication, TargetLink, Descriptor string
}

type result struct {
	Schema, Role, Class string
	Passed              bool
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: reference-c2 (publisher|user|rendezvous|initiator|introduction|responder) CONFIG")
		os.Exit(2)
	}
	input, err := readConfig(os.Args[2])
	if err == nil {
		switch os.Args[1] {
		case "publisher":
			err = runPublisher(input)
		case "user":
			err = runUser(input)
		case "gateway":
			err = runGateway(input)
		case "rendezvous", "initiator", "introduction", "responder":
			err = runTransitRole(input, os.Args[1])
		default:
			err = errors.New("C2 fixture role is unsupported")
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func readConfig(path string) (config, error) {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 || len(raw) > 16<<10 {
		return config{}, errors.New("C2 fixture configuration is unavailable")
	}
	var input config
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return config{}, err
	}
	if input.Schema != "ardents-e2e-reference-c2-v1" || input.Epoch == 0 || input.PublicationPath == "" || input.PublisherRoot == "" || input.GatewayRoot == "" || input.GatewayProfilePath == "" || input.ReadyRoot == "" || input.CompletePath == "" || input.ResourceProofPath == "" ||
		input.TransitAuthority == "" || input.Invite == "" || !input.SlotCredential.valid() ||
		!input.ResponderCredential.valid() || !input.IntroductionCredential.valid() || len(input.TransitStateRoots) != 4 || len(input.TransitStateMaterials) != 4 ||
		len(input.TransitStateSources) != 2 || input.TransitStateClient.Certificate == "" || input.TransitStateClient.PrivateKey == "" {
		return config{}, errors.New("C2 fixture configuration is incomplete")
	}
	if _, err := input.deadline(); err != nil {
		return config{}, err
	}
	for _, value := range []string{input.Network, input.Digest, input.JoinHandle, input.Reachability, input.SlotAttachment, input.ServiceAttachment, input.ResolutionAttachment, input.InviteID} {
		if _, err := fixed(value); err != nil {
			return config{}, err
		}
	}
	for _, value := range []peer{input.Introduction, input.Rendezvous, input.Responder, input.Initiator, input.Gateway} {
		if _, err := value.decode(); err != nil {
			return config{}, err
		}
	}
	for _, role := range []string{"rendezvous", "initiator", "introduction", "responder"} {
		if input.TransitStateRoots[role] == "" {
			return config{}, errors.New("C2 fixture transit State root is unavailable")
		}
		if _, present := input.TransitStateMaterials[role]; !present {
			return config{}, errors.New("C2 fixture transit State materialization is unavailable")
		}
	}
	for _, source := range input.TransitStateSources {
		if source.Address == "" || source.ServerName == "" || source.Root == "" {
			return config{}, errors.New("C2 fixture transit State source is unavailable")
		}
		if _, err := fixed(source.LeafKeyDigest); err != nil {
			return config{}, err
		}
	}
	if _, err := input.entryInvite(); err != nil {
		return config{}, err
	}
	if input.FirefoxExecutable != "" && !filepath.IsAbs(input.FirefoxExecutable) {
		return config{}, errors.New("C2 fixture Firefox executable path is invalid")
	}
	return input, nil
}

func (input config) deadline() (time.Time, error) {
	deadline, err := time.Parse(time.RFC3339, input.Deadline)
	if err != nil || !time.Now().UTC().Before(deadline) {
		return time.Time{}, errors.New("C2 fixture deadline is invalid")
	}
	return deadline.UTC(), nil
}

func (value peer) decode() (endpointapi.TransitPeer, error) {
	nodeID, err := fixed(value.NodeID)
	if err != nil {
		return endpointapi.TransitPeer{}, err
	}
	public, err := fixed(value.PublicKey)
	if err != nil || value.Endpoint == "" {
		return endpointapi.TransitPeer{}, errors.New("C2 fixture peer is invalid")
	}
	return endpointapi.TransitPeer{NodeID: nodeID, PublicKey: public, Family: sha256.Sum256(nodeID[:]), Endpoint: value.Endpoint}, nil
}

func runPublisher(input config) error {
	deadline, _ := input.deadline()
	now := time.Now().UTC().Truncate(time.Second)
	network, _ := fixed(input.Network)
	digest, _ := fixed(input.Digest)
	introduction, _ := input.Introduction.decode()
	rendezvous, _ := input.Rendezvous.decode()
	responder, _ := input.Responder.decode()
	join, _ := fixed(input.JoinHandle)
	slotReachability, _ := fixed(input.Reachability)
	slotAttachment, _ := fixed(input.SlotAttachment)
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	introductionPublic, introductionPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	instancePublic, instancePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	hpkePrivate, err := hpkePrivateKey()
	if err != nil {
		return err
	}
	var authority, instance, hpkePublic [32]byte
	copy(authority[:], authorityPublic)
	copy(instance[:], instancePublic)
	copy(hpkePublic[:], hpkePrivate.PublicKey().Bytes())
	credential, err := (publication.Credential{AuthorityPublic: authority, InstancePublic: instance, IntroductionHPKEPublic: hpkePublic,
		Generation: 1, NotBefore: now.Add(-time.Second).Unix(), NotAfter: deadline.Unix(), NetworkID: network, Capabilities: 3}).Issue(authorityPrivate)
	if err != nil {
		return err
	}
	brokerID, principal, administrator := identifier(41), identifier(42), identifier(43)
	slotAuthorization, slotCertificate, slotGrant, err := input.SlotCredential.decode()
	if err != nil {
		return err
	}
	responderAuthorization, responderCertificate, responderGrant, err := input.ResponderCredential.decode()
	if err != nil {
		return err
	}
	publisher, err := endpointapi.New(endpointapi.Setup{NetworkID: network, BrokerID: brokerID, AuthorityPublic: authorityPublic,
		IntroductionPublic: introductionPublic, ConnectionPrincipal: principal, AdministrationPrincipal: administrator, PublicationRoot: input.PublisherRoot,
		TransitClientCertificates: map[[32]byte]tls.Certificate{slotGrant.GrantID: slotCertificate, responderGrant.GrantID: responderCertificate}})
	if err != nil {
		return err
	}
	defer publisher.Close()
	administration, err := publisher.Admit(administrator, broker.Administration)
	if err != nil {
		return err
	}
	published, err := publisher.Publish(context.Background(), endpointapi.PublicationRequest{Principal: administrator, Capability: administration,
		Credential: credential, InstancePrivate: instancePrivate, IntroductionAcknowledgement: acknowledgement(credential, introductionPrivate, brokerID), At: now})
	if err != nil {
		return err
	}
	link, err := targetlink.Encode(targetlink.Link{Network: network, Target: credential.Target})
	if err != nil {
		return err
	}
	slot, err := publisher.OpenPublisherIntroduction(context.Background(), endpointapi.PublisherIntroductionRequest{
		Profile: endpointapi.PublisherIntroductionProfile{NetworkID: network, Digest: digest, Epoch: input.Epoch, Introduction: introduction,
			Rendezvous: rendezvous, Responder: responder, SlotAttachmentID: slotAttachment, Reachability: slotReachability, JoinHandle: join,
			NotAfter: deadline, SlotAuthorization: slotAuthorization, ResponderAuthorization: responderAuthorization},
		HPKEPrivate: hpkePrivate, At: now})
	if err != nil {
		return err
	}
	defer slot.Close()
	current, err := publication.Decode(published.Record, authorityPublic, network, now)
	if err != nil {
		return err
	}
	introductionAuthorization, _, _, err := input.IntroductionCredential.decode()
	if err != nil {
		return err
	}
	descriptor, _, err := reachability.Issue(reachability.IssueInput{Current: current, InstanceSigner: instancePrivate,
		Introduction: reachability.Introduction{StateDigest: digest, Epoch: input.Epoch, IntroductionNodeID: introduction.NodeID,
			RendezvousNodeID: rendezvous.NodeID, Reachability: slotReachability, JoinHandle: join, NotAfter: deadline, SubmissionAuthorization: introductionAuthorization}})
	if err != nil {
		return err
	}
	if err := writePublication(input.PublicationPath, publicationEnvelope{AuthorityPublic: hex.EncodeToString(authorityPublic),
		Publication: base64.RawStdEncoding.EncodeToString(published.Record), TargetLink: link, Descriptor: base64.RawStdEncoding.EncodeToString(descriptor)}); err != nil {
		return err
	}
	if input.PublisherOffline {
		return json.NewEncoder(os.Stdout).Encode(result{Schema: "ardents-e2e-reference-c2-result-v1", Role: "publisher", Class: "offline", Passed: true})
	}
	capability, err := publisher.Admit(principal, broker.Connection)
	if err != nil {
		return err
	}
	application, service := net.Pipe()
	defer service.Close()
	go serveStatic(service, input.ResourceProofPath)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	outcome, err := slot.Accept(ctx, endpointapi.InboundConnectionRequest{Principal: principal, Capability: capability,
		Application: application, BytesEachDirection: 64 << 10, At: now})
	if outcome.Class == "" {
		return errors.Join(err, errors.New("publisher C2 fixture did not complete a classified Service Connection"))
	}
	return json.NewEncoder(os.Stdout).Encode(result{Schema: "ardents-e2e-reference-c2-result-v1", Role: "publisher", Class: outcome.Class, Passed: true})
}

func runUser(input config) error {
	deadline, _ := input.deadline()
	envelope, err := readPublication(input.PublicationPath)
	if err != nil {
		return err
	}
	network, _ := fixed(input.Network)
	digest, _ := fixed(input.Digest)
	authority, err := hex.DecodeString(envelope.AuthorityPublic)
	if err != nil || len(authority) != ed25519.PublicKeySize {
		return errors.New("publisher authority is invalid")
	}
	introduction, _ := input.Introduction.decode()
	rendezvous, _ := input.Rendezvous.decode()
	initiator, _ := input.Initiator.decode()
	gateway, err := input.Gateway.decode()
	if err != nil {
		return err
	}
	profile, err := readGatewayProfile(input.GatewayProfilePath)
	if err != nil {
		return err
	}
	serviceAttachment, _ := fixed(input.ServiceAttachment)
	resolutionAttachment, _ := fixed(input.ResolutionAttachment)
	inviteID, _ := fixed(input.InviteID)
	invite, err := input.entryInvite()
	if err != nil {
		return err
	}
	introductionAuthorization, introductionCertificate, introductionGrant, err := input.IntroductionCredential.decode()
	if err != nil {
		return err
	}
	if len(introductionAuthorization) == 0 {
		return errors.New("user C2 fixture Introduction grant is unavailable")
	}
	userPrincipal := identifier(42)
	var browser endpointapi.ReferenceBrowser
	if input.FirefoxExecutable != "" {
		browser, err = endpointapi.NewFirefoxBrowser(input.FirefoxExecutable)
		if err != nil {
			return err
		}
	}
	user, err := endpointapi.New(endpointapi.Setup{NetworkID: network, BrokerID: identifier(44), AuthorityPublic: authority,
		IntroductionPublic: make([]byte, ed25519.PublicKeySize), ConnectionPrincipal: userPrincipal,
		TransitClientCertificates: map[[32]byte]tls.Certificate{introductionGrant.GrantID: introductionCertificate}})
	if err != nil {
		return err
	}
	defer user.Close()
	capability, err := user.Admit(userPrincipal, broker.Connection)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Second)
	resolutionDeadline := now.Add(5 * time.Second)
	request := endpointapi.UserReferenceSiteRequest{
		Reachability: &endpointapi.UserReachabilityRouteRequest{TargetLink: envelope.TargetLink,
			Private: &endpointapi.UserPrivateReachabilityRequest{GatewayNodeID: gateway.NodeID, GatewayNodePublicKey: gateway.PublicKey, GatewayFamily: gateway.Family,
				GatewayProfile: profile, StateDigest: digest, Epoch: input.Epoch, Initiator: initiator,
				Entry: entryAcquirer{candidate: entry.Candidate{NodeID: initiator.NodeID, PublicKey: initiator.PublicKey, Endpoint: initiator.Endpoint},
					presentation: entry.Presentation{InviteID: inviteID, Invite: invite}}, AttachmentID: resolutionAttachment, At: now, Deadline: resolutionDeadline},
			Introduction: introduction, Entry: entryAcquirer{candidate: entry.Candidate{NodeID: initiator.NodeID, PublicKey: initiator.PublicKey, Endpoint: initiator.Endpoint},
				presentation: entry.Presentation{InviteID: inviteID, Invite: invite}}, Initiator: initiator, Rendezvous: rendezvous,
			AttachmentID: serviceAttachment, EndpointHandshake: identifier(45), At: now},
		Routes: map[string]string{"": "/", "site.css": "/site.css", "mark.svg": "/mark.svg"}, Principal: userPrincipal, Capability: capability, BytesEachDirection: 64 << 10, Browser: browser}
	if input.PublisherOffline {
		return runOfflineUser(user, request)
	}
	site, err := user.OpenUserReferenceSite(context.Background(), request)
	if err != nil {
		return fmt.Errorf("user C2 fixture exact route: %w", err)
	}
	defer site.Close()
	var ready endpointapi.ReferenceReady
	select {
	case ready = <-site.Ready():
		if ready.URL == "" {
			return errors.New("user C2 fixture did not receive Reference Site readiness")
		}
	case <-time.After(time.Until(deadline)):
		return errors.New("user C2 fixture timed out before Reference Site readiness")
	}
	if browser == nil {
		for resource, expected := range map[string]string{"": referenceDocument, "site.css": referenceStylesheet, "mark.svg": referenceMark} {
			response, requestErr := (&http.Client{Transport: &http.Transport{Proxy: nil}}).Get(ready.URL + resource)
			if requestErr != nil {
				return requestErr
			}
			body, readErr := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if readErr != nil || response.StatusCode != http.StatusOK || string(body) != expected {
				return errors.New("user C2 fixture did not receive its declared Reference Site resource")
			}
		}
	}
	if err := waitForResourceProof(deadline, input.ResourceProofPath); err != nil {
		return err
	}
	if err := site.Close(); err != nil {
		return err
	}
	select {
	case outcome := <-site.Done():
		if outcome.Result.Class == "" {
			return errors.New("user C2 fixture did not receive a classified Service Connection result")
		}
		return json.NewEncoder(os.Stdout).Encode(result{Schema: "ardents-e2e-reference-c2-result-v1", Role: "user", Class: outcome.Result.Class, Passed: true})
	case <-time.After(time.Until(deadline)):
		return errors.New("user C2 fixture did not receive a terminal result")
	}
}

type entryAcquirer struct {
	candidate    entry.Candidate
	presentation entry.Presentation
}

func (input entryAcquirer) Acquire(ctx context.Context, attempt entry.Attempt, opener entry.CandidateOpener) (net.Conn, func() error, error) {
	connection, cleanup, complete, err := opener(ctx, input.candidate, input.presentation, attempt.Deadline)
	if err != nil || !complete {
		return nil, cleanup, err
	}
	return connection, cleanup, nil
}

func writePublication(path string, value publicationEnvelope) error {
	if filepath.Dir(path) == "." {
		return errors.New("publisher publication path is not absolute")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func readPublication(path string) (publicationEnvelope, error) {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 || len(raw) > 8<<10 {
		return publicationEnvelope{}, errors.New("publisher publication is unavailable")
	}
	var value publicationEnvelope
	if err := json.Unmarshal(raw, &value); err != nil || value.AuthorityPublic == "" || value.Publication == "" || value.TargetLink == "" || value.Descriptor == "" {
		return publicationEnvelope{}, errors.New("publisher publication is invalid")
	}
	return value, nil
}

func acknowledgement(credential publication.Credential, private ed25519.PrivateKey, brokerID [32]byte) []byte {
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

func hpkePrivateKey() (hpke.PrivateKey, error) {
	private, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return hpke.NewDHKEMPrivateKey(private)
}

func fixed(encoded string) ([32]byte, error) {
	var value [32]byte
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != len(value) {
		return value, errors.New("C2 fixture fixed value is invalid")
	}
	copy(value[:], raw)
	return value, nil
}

func identifier(value byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = value + byte(index)
	}
	return result
}
