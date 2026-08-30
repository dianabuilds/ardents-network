package endpoint

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/reference"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// AlphaBrowserStateView is the narrow current State projection required to
// open one browser-demanded alpha name. It is implemented by
// state.ResolutionView. It contains no State source, persistence, or candidate
// ordering operation.
type AlphaBrowserStateView interface {
	Epoch(time.Time, time.Time) (state.ResolutionEpoch, bool)
	Gateway(time.Time, time.Time) (state.DestinationResolutionGateway, bool)
	Candidate([32]byte, time.Time, time.Time) (state.ResolutionCandidate, bool)
}

// AlphaBrowserEntry is the Endpoint's already-imported User Entry owner. It
// exposes one current contact so the Endpoint can bind an Initiator identity
// before a closed Entry acquisition. Entry retains invite validation, retry,
// and durable contact state.
type AlphaBrowserEntry interface {
	route.EntryAcquirer
	Contact() (entry.Candidate, error)
}

// AlphaBrowserRuntimeRequest binds the local durable alpha floor to current
// State and an already-imported Entry set. No caller may provide a Target,
// Gateway URL/profile, C-2 peer, Route, or browser destination.
type AlphaBrowserRuntimeRequest struct {
	Floor   *alpha.PersistentFloor
	Current func() (AlphaBrowserStateView, error)
	Entry   AlphaBrowserEntry
	// Principal is the preconfigured local connection grant principal. A fresh
	// capability is minted for each browser-demanded Service Connection.
	Principal          [32]byte
	BytesEachDirection uint32
	Clock              func() time.Time
}

// OpenAlphaBrowserRuntime makes the selected Alpha Browser Entry composition live:
// name.ard -> accepted alpha floor -> State-selected private lookup -> exact
// C-2 Service -> local transparent presentation. It leaves ordinary browser
// traffic untouched and the Endpoint itself has no DNS or direct-origin
// fallback; browser resolver behavior is a separate integration concern.
func (endpoint *endpoint) OpenAlphaBrowserRuntime(ctx context.Context, input AlphaBrowserRuntimeRequest) (*AlphaBrowserResolution, error) {
	if endpoint == nil || ctx == nil || input.Floor == nil || input.Current == nil || input.Entry == nil || input.Principal == [32]byte{} ||
		input.BytesEachDirection == 0 || input.BytesEachDirection > maximumEndpointStreamBytes {
		return nil, errors.New("alpha browser runtime input is incomplete")
	}
	clock := input.Clock
	if clock == nil {
		clock = time.Now
	}
	return endpoint.OpenAlphaBrowserResolution(ctx, AlphaBrowserResolutionRequest{Floor: input.Floor, Clock: clock,
		Open: func(openCtx context.Context, binding alpha.Binding) (AlphaBrowserSite, error) {
			return endpoint.openAlphaBrowserService(openCtx, binding, input, clock)
		}})
}

// ConnectionInterface is the Connection-surface owner shared by headless CLI
// and optional Adapters. Its Open input is only a Service Link; State, Entry,
// Target authentication, Route, and one-use transport inputs remain private.
type ConnectionInterface struct {
	endpoint *endpoint
	input    AlphaBrowserRuntimeRequest
	clock    func() time.Time
}

// OpenConnectionInterface binds current Endpoint-owned acquisition inputs to a
// narrow name-to-stream operation. The config is process composition, not an
// Adapter request or Route plan.
func (endpoint *endpoint) OpenConnectionInterface(input AlphaBrowserRuntimeRequest) (*ConnectionInterface, error) {
	if endpoint == nil || input.Floor == nil || input.Current == nil || input.Entry == nil || input.Principal == [32]byte{} ||
		input.BytesEachDirection == 0 || input.BytesEachDirection > maximumEndpointStreamBytes {
		return nil, errors.New("Application Connection Interface input is incomplete")
	}
	clock := input.Clock
	if clock == nil {
		clock = time.Now
	}
	return &ConnectionInterface{endpoint: endpoint, input: input, clock: clock}, nil
}

// Open resolves one exact accepted Service Link and returns only its
// authenticated opaque Application stream and bounded terminal outcome.
func (owner *ConnectionInterface) Open(ctx context.Context, serviceLink string) (*ApplicationConnection, error) {
	if owner == nil || ctx == nil {
		return nil, errors.New("Application Connection Interface is unavailable")
	}
	link, err := alpha.ParseServiceLink(serviceLink)
	if err != nil {
		return nil, err
	}
	at := owner.clock().UTC()
	binding, err := owner.endpoint.ResolveAcceptedAlpha(owner.input.Floor, link.String(), at)
	if err != nil {
		return nil, err
	}
	return owner.openBinding(ctx, binding)
}

func (owner *ConnectionInterface) openBinding(ctx context.Context, binding alpha.Binding) (*ApplicationConnection, error) {
	if owner == nil {
		return nil, errors.New("Application Connection Interface is unavailable")
	}
	return owner.endpoint.openAlphaApplicationForBinding(ctx, binding, owner.input, owner.clock)
}

func (endpoint *endpoint) openAlphaBrowserService(ctx context.Context, binding alpha.Binding, input AlphaBrowserRuntimeRequest,
	clock func() time.Time) (*UserReferenceSite, error) {
	owner, err := endpoint.OpenConnectionInterface(input)
	if err != nil {
		return nil, err
	}
	owner.clock = clock
	application, err := owner.openBinding(ctx, binding)
	if err != nil {
		return nil, err
	}
	return endpoint.openAlphaCompatibilityPresentation(binding, application)
}

func (endpoint *endpoint) openAlphaApplicationForBinding(ctx context.Context, binding alpha.Binding, input AlphaBrowserRuntimeRequest,
	clock func() time.Time) (*ApplicationConnection, error) {
	at := clock().UTC()
	if at.IsZero() {
		return nil, errors.New("alpha browser runtime clock is unavailable")
	}
	lookupDeadline := at.Add(15 * time.Second)
	view, err := input.Current()
	if err != nil || view == nil {
		return nil, errors.New("current State resolution view is unavailable")
	}
	epoch, available := view.Epoch(at, lookupDeadline)
	if !available || epoch.NetworkID != endpoint.network {
		return nil, errors.New("current State resolution epoch is unavailable")
	}
	gateway, available := view.Gateway(at, lookupDeadline)
	if !available {
		return nil, errors.New("state destination resolution gateway is unavailable")
	}
	profile, err := reachability.DecodeGatewayProfile(gateway.Profile)
	if err != nil {
		return nil, errors.New("state destination resolution gateway profile is invalid")
	}
	if err := reachability.VerifyGatewayProfile(profile, endpoint.network, gateway.NodeID, gateway.PublicKey, at, lookupDeadline); err != nil {
		return nil, errors.New("state destination resolution gateway profile is invalid")
	}
	initiatorContact, err := input.Entry.Contact()
	if err != nil {
		return nil, errors.New("current User Entry contact is unavailable")
	}
	initiator, err := alphaBrowserInitiator(view, initiatorContact, at, lookupDeadline)
	if err != nil {
		return nil, err
	}
	lookupAttachment, err := alphaBrowserRandomID()
	if err != nil {
		return nil, err
	}
	targetLink, err := targetlink.Encode(targetlink.Link{Network: endpoint.network, Target: binding.Target()})
	if err != nil {
		return nil, err
	}
	descriptor, err := endpoint.ResolveUserReachability(ctx, targetLink, UserPrivateReachabilityRequest{
		GatewayNodeID: gateway.NodeID, GatewayNodePublicKey: gateway.PublicKey, GatewayFamily: gateway.Family, GatewayProfile: profile,
		StateDigest: epoch.Digest, Epoch: epoch.Number, Initiator: initiator, Entry: input.Entry,
		AttachmentID: lookupAttachment, At: at, Deadline: lookupDeadline,
	})
	if err != nil {
		return nil, err
	}
	verified, err := reachability.Verify(descriptor, binding.Target(), endpoint.network, at)
	if err != nil || verified.Descriptor.Introduction.StateDigest != epoch.Digest || verified.Descriptor.Introduction.Epoch != epoch.Number {
		return nil, errors.New("private reachability descriptor is not current State evidence")
	}
	slot := verified.Descriptor.Introduction
	introduction, err := alphaBrowserStatePeer(view, slot.IntroductionNodeID, "introduction", at, slot.NotAfter)
	if err != nil {
		return nil, err
	}
	rendezvous, err := alphaBrowserStatePeer(view, slot.RendezvousNodeID, "rendezvous", at, slot.NotAfter)
	if err != nil {
		return nil, err
	}
	initiator, err = alphaBrowserInitiator(view, initiatorContact, at, slot.NotAfter)
	if err != nil {
		return nil, err
	}
	if !distinctAlphaBrowserPeers(gateway, initiator, introduction, rendezvous) {
		return nil, errors.New("state private lookup and C-2 peers overlap")
	}
	credentialDeadline := slot.NotAfter
	if slot.SubmissionMode == reachability.SubmissionMembershipGrant && (!at.Before(credentialDeadline) || credentialDeadline.After(lookupDeadline)) {
		return nil, errors.New("reachability descriptor exceeds the membership credential window")
	}
	submission, err := endpoint.alphaBrowserSubmission(ctx, view, epoch, input.Entry, initiator, introduction, slot, at, credentialDeadline)
	if err != nil {
		return nil, err
	}
	presented := false
	if submission.finish != nil {
		defer func() { _ = submission.finish(presented) }()
	}
	handshake, err := alphaBrowserRandomID()
	if err != nil {
		return nil, err
	}
	// Descriptor is intentionally passed only after this runtime obtained it
	// through ResolveUserReachability and revalidated it above. No external
	// caller can provide this branch or substitute a target.
	application, err := endpoint.openAlphaApplicationConnection(ctx, binding, UserApplicationConnectionRequest{
		Reachability: &UserReachabilityRouteRequest{Descriptor: descriptor,
			Introduction: introduction, Initiator: initiator, Rendezvous: rendezvous, Entry: input.Entry,
			AttachmentID: submission.attachment, EndpointHandshake: handshake, At: at,
			SubmissionAuthorization: submission.authorization, SubmissionClientCertificate: submission.certificate}, Principal: input.Principal,
		BytesEachDirection: input.BytesEachDirection})
	presented = err == nil
	return application, err
}

func (endpoint *endpoint) openAlphaCompatibilityPresentation(binding alpha.Binding, application *ApplicationConnection) (*UserReferenceSite, error) {
	if endpoint == nil || application == nil {
		return nil, errors.New("alpha Browser compatibility presentation is unavailable")
	}
	hostname := alphaBrowserHostname(binding)
	presentation, err := reference.OpenTransparent(reference.TransparentConfig{Target: binding.Target(), Hostname: hostname, Connection: application})
	if err != nil {
		_ = application.Close()
		return nil, err
	}
	proxyURL, release, err := endpoint.openAlphaTransparentBrowserRoute(hostname, presentation)
	if err != nil {
		_ = presentation.Close()
		return nil, err
	}
	running := &ReferenceConnection{cancel: func() { _ = application.Close() }, ready: make(chan ReferenceReady, 1),
		done: make(chan ReferenceOutcome, 1), closed: make(chan struct{}), presentation: presentation, release: release}
	running.publishReady(ReferenceReady{URL: "http://" + hostname + "/", AlphaProxyURL: proxyURL,
		AuthenticatedTarget: binding.Target()})
	go func() {
		outcome, open := <-application.Done()
		result := RuntimeResult{Class: outcome.Class, Reason: outcome.Reason}
		if !open {
			result = RuntimeResult{Class: "indeterminate failure", Reason: "Application Connection ended without an outcome"}
		}
		running.completeReferenceConnection(result, nil)
	}()
	return &UserReferenceSite{reference: running}, nil
}

// alphaBrowserServiceAttachment preserves the one-use binding of a signed
// Transit Grant when a descriptor carries one. Legacy opaque authorizations
// retain a freshly chosen attachment for the lower-level compatibility path;
// a byte sequence that identifies itself as a Transit Grant must instead
// validate against current State and cannot fall back.
func alphaBrowserServiceAttachment(authorization []byte, epoch state.ResolutionEpoch, introduction [32]byte, notAfter time.Time) ([32]byte, error) {
	grant, err := route.DecodeTransitGrant(authorization)
	if err != nil {
		return alphaBrowserRandomID()
	}
	var authority ed25519.PublicKey
	for _, candidate := range epoch.Authorities {
		if candidate.ID == grant.IssuerID {
			authority = ed25519.PublicKey(candidate.PublicKey[:])
			break
		}
	}
	if authority == nil {
		return [32]byte{}, errors.New("introduction transit grant issuer is absent from current state")
	}
	grant, err = route.VerifyTransitGrant(authorization, authority)
	if err != nil || grant.NetworkID != epoch.NetworkID || grant.Digest != epoch.Digest || grant.Epoch != epoch.Number ||
		grant.TransitRole != route.IntroductionRole || grant.TransitNodeID != introduction || grant.AttachmentID == [32]byte{} ||
		notAfter.IsZero() || notAfter.After(grant.NotAfter) {
		return [32]byte{}, errors.New("introduction transit grant does not bind the current browser route")
	}
	return grant.AttachmentID, nil
}

func alphaBrowserInitiator(view AlphaBrowserStateView, contact entry.Candidate, at, deadline time.Time) (TransitPeer, error) {
	candidate, available := view.Candidate(contact.NodeID, at, deadline)
	if !available || candidate.Domain != "initiator" || candidate.PublicKey != contact.PublicKey || candidate.Endpoint != contact.Endpoint ||
		sha256.Sum256([]byte(candidate.Family)) != contact.FamilyID {
		return TransitPeer{}, errors.New("user entry contact does not match current initiator state")
	}
	return TransitPeer{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, Family: contact.FamilyID, Endpoint: candidate.Endpoint}, nil
}

func alphaBrowserStatePeer(view AlphaBrowserStateView, nodeID [32]byte, domain string, at, deadline time.Time) (TransitPeer, error) {
	candidate, available := view.Candidate(nodeID, at, deadline)
	if !available || candidate.Domain != domain || candidate.NodeID == [32]byte{} || candidate.PublicKey == [32]byte{} || candidate.Endpoint == "" {
		return TransitPeer{}, errors.New("current State C-2 peer is unavailable")
	}
	return TransitPeer{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, Family: sha256.Sum256([]byte(candidate.Family)), Endpoint: candidate.Endpoint}, nil
}

func distinctAlphaBrowserPeers(gateway state.DestinationResolutionGateway, peers ...TransitPeer) bool {
	if gateway.NodeID == [32]byte{} || gateway.Family == [32]byte{} || len(peers) != 3 {
		return false
	}
	identities := [][32]byte{gateway.NodeID}
	families := [][32]byte{gateway.Family}
	for _, peer := range peers {
		if !validTransitPeer(peer) || peer.Family == [32]byte{} {
			return false
		}
		identities = append(identities, peer.NodeID)
		families = append(families, peer.Family)
	}
	for index := range identities {
		for other := 0; other < index; other++ {
			if identities[index] == identities[other] || families[index] == families[other] {
				return false
			}
		}
	}
	return true
}

func alphaBrowserRandomID() ([32]byte, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil || value == [32]byte{} {
		return [32]byte{}, errors.New("alpha browser runtime could not create a Route attachment identifier")
	}
	return value, nil
}
