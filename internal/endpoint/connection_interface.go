package endpoint

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"time"

	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/interfacev1/connection"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

const maximumConnectionInterfaceBytes = uint32(768 << 20)

// ApplicationStateView is the narrow current State projection required to
// open one Target Link. It is implemented by
// state.ResolutionView. It contains no State source, persistence, or candidate
// ordering operation.
type resolutionCandidateView interface {
	Epoch(time.Time, time.Time) (state.ResolutionEpoch, bool)
	Candidate([32]byte, time.Time, time.Time) (state.ResolutionCandidate, bool)
}

type applicationStateView interface {
	resolutionCandidateView
	transitCredentialIssuerView
	Gateway(time.Time, time.Time) (state.DestinationResolutionGateway, bool)
}

// ApplicationEntry is the Endpoint's already-imported User Entry owner. It
// exposes one current contact so the Endpoint can bind an Initiator identity
// before a closed Entry acquisition. Entry retains invite validation, retry,
// and durable contact state.
type applicationEntry interface {
	route.EntryAcquirer
	Contact() (entry.Candidate, error)
}

// connectionInterfaceConfig binds the already-constructed Endpoint-owned Route
// and an optional preaccepted legacy Service Link floor. Its construction is process
// composition; the Connection Interface caller cannot provide a Target,
// Gateway URL/profile, C-2 peer, Route plan, or browser destination.
type connectionInterfaceConfig struct {
	Route                  *route.Route
	LegacyServiceLinkFloor *alpha.PersistentFloor
	// Principal is the preconfigured local connection grant principal. A fresh
	// capability is minted for each Application-requested Service Connection.
	Principal          [32]byte
	BytesEachDirection uint32
	Clock              func() time.Time
}

// connectionInterface is the Connection-surface owner shared by headless CLI
// and optional Adapters. Its Open input is a Target Link, except for the
// explicit preaccepted v1 Service Link migration path. State, Entry, Target
// authentication, Route, and one-use transport inputs remain private.
type connectionInterface struct {
	endpoint *endpoint
	input    connectionInterfaceConfig
	clock    func() time.Time
}

// openConnectionInterface binds accepted Endpoint-owned acquisition inputs to a
// narrow name-to-stream operation. The config is process composition, not an
// Adapter request or Route plan.
func (endpoint *endpoint) openConnectionInterface(input connectionInterfaceConfig) (*connectionInterface, error) {
	if endpoint == nil || input.Route == nil || input.Principal == [32]byte{} ||
		input.BytesEachDirection == 0 || input.BytesEachDirection > maximumConnectionInterfaceBytes {
		return nil, errors.New("application Connection Interface input is incomplete")
	}
	clock := input.Clock
	if clock == nil {
		clock = time.Now
	}
	return &connectionInterface{endpoint: endpoint, input: input, clock: clock}, nil
}

// Open resolves one exact Target Link and returns only its authenticated opaque
// Application stream and bounded terminal outcome. A legacy Service Link is
// recognized only when process composition supplied its complete preaccepted
// corpus floor; it never becomes a Target Link fallback for new C0 runtimes.
func (owner *connectionInterface) Open(ctx context.Context, targetLink string) (applicationconnection.Stream, error) {
	if owner == nil || ctx == nil {
		return nil, errors.New("application Connection Interface is unavailable")
	}
	target, err := owner.endpoint.TargetFromLink(targetLink)
	if err == nil {
		session, sessionErr := owner.endpoint.beginApplicationSession(ctx, owner.input.Principal)
		if sessionErr != nil {
			return nil, sessionErr
		}
		return owner.openTargetForSession(session, target)
	}
	if owner.input.LegacyServiceLinkFloor == nil {
		return nil, err
	}
	legacyLink, legacyErr := alpha.ParseServiceLink(targetLink)
	if legacyErr != nil {
		return nil, err
	}
	session, sessionErr := owner.endpoint.beginApplicationSession(ctx, owner.input.Principal)
	if sessionErr != nil {
		return nil, sessionErr
	}
	binding, bindingErr := owner.endpoint.ResolveAcceptedAlpha(owner.input.LegacyServiceLinkFloor, legacyLink.String(), owner.clock().UTC())
	if bindingErr != nil {
		session.Release()
		return nil, bindingErr
	}
	return owner.openTargetForSession(session, binding.Target())
}

func (owner *connectionInterface) openTargetForSession(session *applicationSession, target [32]byte) (applicationconnection.Stream, error) {
	stream, err := owner.openTarget(session.Context(), target, session)
	if err == nil {
		return stream, nil
	}
	session.Release()
	var acquisition transitAcquisitionOutcomeError
	if !errors.As(err, &acquisition) {
		return nil, err
	}
	switch acquisition.outcome {
	case credential.Exhausted:
		return nil, applicationconnection.Refuse(applicationconnection.Outcome{Class: "transit grant exhausted", Reason: "current issuer budget is exhausted"})
	case credential.Withdrawn:
		return nil, applicationconnection.Refuse(applicationconnection.Outcome{Class: "transit grant withdrawn", Reason: "current issuer duty is withdrawn"})
	default:
		return nil, applicationconnection.Refuse(applicationconnection.Outcome{Class: "transit grant unavailable", Reason: "current issuer could not provide a grant"})
	}
}

func (owner *connectionInterface) openTarget(ctx context.Context, target [32]byte, session *applicationSession) (applicationconnection.Stream, error) {
	if owner == nil {
		return nil, errors.New("application Connection Interface is unavailable")
	}
	return owner.endpoint.openTargetApplication(ctx, target, owner.input, owner.clock, session)
}

func (endpoint *endpoint) openTargetApplication(ctx context.Context, target [32]byte, input connectionInterfaceConfig,
	clock func() time.Time, session *applicationSession) (*applicationConnection, error) {
	if endpoint == nil || input.Route == nil || session == nil || target == [32]byte{} {
		return nil, errors.New("application Connection Route input is unavailable")
	}
	attachment, err := input.Route.Attach(ctx, route.Intent{Target: target})
	if err != nil {
		return nil, err
	}
	evidence, err := attachment.Evidence()
	if err != nil {
		return nil, errors.Join(err, attachment.Close())
	}
	return endpoint.openTargetRouteApplicationConnection(ctx, target, input, clock, session, attachment, evidence)
}

// applicationServiceAttachment preserves the one-use binding of a signed
// Transit Grant when a descriptor carries one. Decode failure is rejected
// before an attachment identifier can be allocated; State remains responsible
// for the Grant signature and current-authority checks below.
func applicationServiceAttachment(authorization []byte, epoch state.ResolutionEpoch, introduction [32]byte, notAfter time.Time) ([32]byte, error) {
	grant, err := route.DecodeTransitGrant(authorization)
	if err != nil {
		return [32]byte{}, errors.New("application Transit Grant is malformed")
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
		return [32]byte{}, errors.New("introduction transit grant does not bind the current Application route")
	}
	return grant.AttachmentID, nil
}

func applicationInitiator(view resolutionCandidateView, contact entry.Candidate, at, deadline time.Time) (transitPeer, error) {
	candidate, available := view.Candidate(contact.NodeID, at, deadline)
	if !available || candidate.Domain != "initiator" || candidate.PublicKey != contact.PublicKey || candidate.Endpoint != contact.Endpoint ||
		sha256.Sum256([]byte(candidate.Family)) != contact.FamilyID {
		return transitPeer{}, errors.New("user entry contact does not match current initiator state")
	}
	return transitPeer{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, Family: contact.FamilyID, Endpoint: candidate.Endpoint}, nil
}

func applicationAttachmentID() ([32]byte, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil || value == [32]byte{} {
		return [32]byte{}, errors.New("application Connection could not create a Route attachment identifier")
	}
	return value, nil
}
