//go:build referencec2

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: reference-c2 (publisher|publisher-app|user|alpha-observer|gateway|alpha-gateway|alpha-relay|carrier-relay|rendezvous|initiator|introduction|responder) CONFIG")
		os.Exit(2)
	}
	input, err := readConfig(os.Args[2])
	if err == nil {
		switch os.Args[1] {
		case "publisher":
			err = runPublisher(input)
		case "publisher-app":
			err = runPublisherApplication(input)
		case "user":
			err = runUser(input)
		case "alpha-observer":
			err = runAlphaObserver(input)
		case "gateway":
			err = runGateway(input)
		case "alpha-gateway":
			err = runAlphaGateway(input)
		case "alpha-relay":
			err = runAlphaRelay(input)
		case "carrier-relay":
			err = runCarrierRelay(input)
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
		Credential: credential, InstanceSigner: instancePrivate, IntroductionAcknowledgement: acknowledgement(credential, introductionPrivate, brokerID), At: now})
	if err != nil {
		return err
	}
	alphaAuthorityPublic, alphaAuthorityPrivate, err := input.alphaCorpusSigner()
	if err != nil {
		return err
	}
	alphaLink, err := alpha.ParseServiceLink("ardents-alpha://reference")
	if err != nil {
		return err
	}
	alphaCorpus, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "reference-c2", Network: network, Serial: 1,
		NotBefore: now.Add(-time.Second), NotAfter: deadline, Bindings: []alpha.BindingInput{{Link: alphaLink, Target: credential.Target}}}, alphaAuthorityPrivate)
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
	var publisherApplication *publisherApplicationListener
	if !input.PublisherOffline {
		publisherApplication, err = openPublisherApplication(input)
		if err != nil {
			return err
		}
		defer publisherApplication.Close()
		if err := writePublisherApplicationAddress(input.PublisherApplicationAddressPath, publisherApplication.Address()); err != nil {
			return err
		}
	}
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
		Publication: base64.RawStdEncoding.EncodeToString(published.Record), Descriptor: base64.RawStdEncoding.EncodeToString(descriptor),
		AlphaAuthorityPublic: hex.EncodeToString(alphaAuthorityPublic), AlphaCorpus: base64.RawStdEncoding.EncodeToString(alphaCorpus), AlphaLink: alphaLink.String()}); err != nil {
		return err
	}
	if input.PublisherOffline {
		return json.NewEncoder(os.Stdout).Encode(result{Schema: "ardents-e2e-reference-c2-result-v1", Role: "publisher", Class: "offline", Passed: true})
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	application, err := publisherApplication.Accept(ctx)
	if err != nil {
		return err
	}
	defer application.Close()
	if err := writePublisherApplicationReady(input.PublisherApplicationReady); err != nil {
		return err
	}
	capability, err := publisher.Admit(principal, broker.Connection)
	if err != nil {
		return err
	}
	outcome, err := slot.Accept(ctx, endpointapi.InboundConnectionRequest{Principal: principal, Capability: capability,
		Application: application, BytesEachDirection: input.streamBytesEachDirection(), At: now})
	if outcome.Class == "" {
		return errors.Join(err, errors.New("publisher C2 fixture did not complete a classified Service Connection"))
	}
	if input.PublisherTerminal == publisherTerminalWithdrawal {
		withdrawalCapability, admitErr := publisher.Admit(administrator, broker.Administration)
		if admitErr != nil {
			return admitErr
		}
		withdrawn, withdrawErr := publisher.Withdraw(ctx, endpointapi.WithdrawalRequest{
			Principal: administrator, Capability: withdrawalCapability, At: time.Now().UTC().Truncate(time.Second),
		})
		if withdrawErr != nil || withdrawn.Class != "unpublished" || withdrawn.AuthenticatedTarget != published.AuthenticatedTarget ||
			withdrawn.Generation != published.Generation {
			return errors.Join(withdrawErr, errors.New("publisher C2 fixture publication withdrawal was not classified or exact"))
		}
		verificationCapability, admitErr := publisher.Admit(administrator, broker.Administration)
		if admitErr != nil {
			return admitErr
		}
		stillPublished, verificationErr := publisher.Withdraw(ctx, endpointapi.WithdrawalRequest{
			Principal: administrator, Capability: verificationCapability, At: time.Now().UTC().Truncate(time.Second),
		})
		if verificationErr == nil || stillPublished.Class != "service unavailable" {
			return errors.New("publisher C2 fixture withdrawn publication remained live")
		}
		outcome.Class = withdrawn.Class
	}
	var runtimeResult *endpointapi.RuntimeResult
	if input.DynamicWorkload.configured() {
		if outcome.AuthenticatedTarget != published.AuthenticatedTarget || outcome.Generation != published.Generation {
			return errors.New("publisher C2 configured workload lost its authenticated terminal identity")
		}
		runtimeResult = &outcome
	}
	return json.NewEncoder(os.Stdout).Encode(result{Schema: "ardents-e2e-reference-c2-result-v1", Role: "publisher", Class: outcome.Class,
		Passed: true, Runtime: runtimeResult})
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
	userSetup := endpointapi.Setup{NetworkID: network, BrokerID: identifier(44), AuthorityPublic: authority,
		IntroductionPublic: make([]byte, ed25519.PublicKeySize), ConnectionPrincipal: userPrincipal,
		TransitClientCertificates: map[[32]byte]tls.Certificate{introductionGrant.GrantID: introductionCertificate}}
	if input.BrowserEntryStatePath != "" {
		userSetup.BrowserEntryStatePath = input.BrowserEntryStatePath
	}
	user, err := endpointapi.New(userSetup)
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
	alphaFloor, err := input.openAlphaCorpusFloor(network)
	if err != nil {
		return err
	}
	defer alphaFloor.Close()
	alphaResolver, err := input.openPrivateAlpha(alphaFloor, network, now)
	if err != nil {
		return err
	}
	alphaBinding, err := user.ResolveAlpha(context.Background(), alphaResolver, input.AlphaServiceLink, now)
	if err != nil {
		return err
	}
	acceptedAlphaBinding, err := user.ResolveAcceptedAlpha(alphaFloor, input.AlphaServiceLink, now)
	if err != nil || alphaBinding.Link() != acceptedAlphaBinding.Link() || alphaBinding.Network() != acceptedAlphaBinding.Network() ||
		alphaBinding.Target() != acceptedAlphaBinding.Target() || alphaBinding.Serial() != acceptedAlphaBinding.Serial() {
		return errors.New("user C2 fixture exact private binding disagrees with its accepted floor")
	}
	request := endpointapi.UserReferenceSiteRequest{
		Reachability: &endpointapi.UserReachabilityRouteRequest{
			Private: &endpointapi.UserPrivateReachabilityRequest{GatewayNodeID: gateway.NodeID, GatewayNodePublicKey: gateway.PublicKey, GatewayFamily: gateway.Family,
				GatewayProfile: profile, StateDigest: digest, Epoch: input.Epoch, Initiator: initiator,
				Entry: entryAcquirer{candidate: entry.Candidate{NodeID: initiator.NodeID, PublicKey: initiator.PublicKey, Endpoint: initiator.Endpoint},
					presentation: entry.Presentation{InviteID: inviteID, Invite: invite}}, AttachmentID: resolutionAttachment, At: now, Deadline: resolutionDeadline},
			Introduction: introduction, Entry: entryAcquirer{candidate: entry.Candidate{NodeID: initiator.NodeID, PublicKey: initiator.PublicKey, Endpoint: initiator.Endpoint},
				presentation: entry.Presentation{InviteID: inviteID, Invite: invite}}, Initiator: initiator, Rendezvous: rendezvous,
			AttachmentID: serviceAttachment, EndpointHandshake: identifier(45), At: now},
		Routes: map[string]string{"": "/", "site.css": "/site.css", "mark.svg": "/mark.svg"}, Principal: userPrincipal, Capability: capability,
		BytesEachDirection: input.streamBytesEachDirection(), Browser: browser}
	if input.PublisherOffline {
		return runOfflineAlphaUser(user, endpointapi.AlphaUserReferenceSiteRequest{Binding: alphaBinding, Route: request})
	}
	var site *endpointapi.UserReferenceSite
	var referenceClient *http.Client
	var dynamicWorkload *dynamicWorkloadResult
	if input.TransparentApplication {
		site, err = user.OpenAlphaTransparentUserReferenceSite(context.Background(), endpointapi.AlphaTransparentUserReferenceSiteRequest{Binding: alphaBinding, Route: request})
	} else {
		site, err = user.OpenAlphaUserReferenceSite(context.Background(), endpointapi.AlphaUserReferenceSiteRequest{Binding: alphaBinding, Route: request})
	}
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
	if ready.AuthenticatedTarget != alphaBinding.Target() {
		return errors.New("user C2 fixture did not authenticate the selected alpha binding")
	}
	if input.HeldRouteReady != "" || input.HeldRouteRelease != "" {
		return finishHeldUserRoute(input, deadline, site)
	}
	externalBrowserEntry := input.BrowserEntryStatePath != ""
	if browser == nil && !externalBrowserEntry {
		var clientErr error
		referenceClient, clientErr = alphaReferenceClient(ready.URL, ready.AlphaProxyURL)
		if clientErr != nil {
			return clientErr
		}
		defer referenceClient.CloseIdleConnections()
		if input.TransparentApplication {
			var exerciseErr error
			dynamicWorkload, exerciseErr = exerciseDynamicInput(input, referenceClient, ready.URL)
			if dynamicWorkload != nil {
				dynamicWorkload.ProxyTCPDialCount, dynamicWorkload.RejectedProxyRedials = alphaReferenceProxyDialCounts(referenceClient)
			}
			if exerciseErr != nil {
				return fmt.Errorf("%w; local Service Connection outcome: %s", exerciseErr, userReferenceTerminalSnapshot(site))
			}
		} else {
			for resource, expected := range map[string]string{"": referenceDocument, "site.css": referenceStylesheet, "mark.svg": referenceMark} {
				response, requestErr := referenceClient.Get(ready.URL + resource)
				if requestErr != nil {
					return requestErr
				}
				body, readErr := io.ReadAll(response.Body)
				_ = response.Body.Close()
				if readErr != nil || response.StatusCode != http.StatusOK || string(body) != expected ||
					response.Header.Get("Content-Security-Policy") != "sandbox allow-same-origin; default-src 'none'; script-src 'none'; connect-src 'none'; img-src 'self'; style-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; worker-src 'none'" ||
					response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Referrer-Policy") != "no-referrer" ||
					response.Header.Get("X-Content-Type-Options") != "nosniff" {
					return errors.New("user C2 fixture did not receive its declared Reference Site resource")
				}
			}
		}
	}
	if input.TransparentApplication {
		expectedProof := dynamicProofForPublisherTerminal(input.PublisherTerminal, input.TransitFault)
		if externalBrowserEntry {
			expectedProof = "browser-dynamic-http\n"
		}
		if err := waitForDynamicProof(deadline, input.ResourceProofPath, expectedProof); err != nil {
			return fmt.Errorf("user C2 fixture dynamic Service %s: %w", ready.URL, err)
		}
		runtimeResult, waitErr := waitForDynamicPublisherWithdrawal(deadline, site, referenceClient, ready.URL, input.BrowserEntryStatePath)
		if waitErr != nil {
			return fmt.Errorf("user C2 fixture dynamic Publisher withdrawal: %w", waitErr)
		}
		var retainedRuntime *endpointapi.RuntimeResult
		if input.DynamicWorkload.configured() {
			if runtimeResult.AuthenticatedTarget != ready.AuthenticatedTarget || runtimeResult.Generation != 1 {
				return errors.New("user C2 configured workload lost its authenticated terminal identity")
			}
			retainedRuntime = &runtimeResult
		}
		return json.NewEncoder(os.Stdout).Encode(result{Schema: "ardents-e2e-reference-c2-result-v1", Role: "user", Class: runtimeResult.Class,
			Passed: true, Workload: dynamicWorkload, Runtime: retainedRuntime})
	} else if err := waitForResourceProof(deadline, input.ResourceProofPath); err != nil {
		return fmt.Errorf("user C2 fixture Reference Site %s: %w", ready.URL, err)
	}
	if err := site.Close(); err != nil {
		return err
	}
	class, err := waitForUserReferenceOutcome(deadline, site)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result{Schema: "ardents-e2e-reference-c2-result-v1", Role: "user", Class: class, Passed: true})
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
