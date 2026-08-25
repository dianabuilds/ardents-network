package endpoint

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// UserIntroductionRouteRequest is the complete Endpoint-owned composition of
// one selected Entry attachment and one selected C-2 Introduction delivery.
// Entry retains its invite/contact lifecycle, and the caller supplies only
// State-selected peers; this operation neither discovers a Node nor tries an
// alternate destination.
type UserIntroductionRouteRequest struct {
	TargetLink            string
	Publication           []byte
	AuthorityPublic       [32]byte
	Introduction          UserIntroductionProfile
	Entry                 route.EntryAcquirer
	Initiator, Rendezvous TransitPeer
	AttachmentID          [32]byte
	EndpointHandshake     [32]byte
	At                    time.Time
}

// UserIntroductionRoute is the one authenticated User-side carrier that has
// completed both exact Initiator RelaySetup and Introduction delivery. Its
// attachment identity remains State-selected because the Publisher-side
// Responder authorization binds that same opaque value. The caller passes
// Connection only to the selected Endpoint Service Connection operation, then
// must Close it when that operation ends.
type UserIntroductionRoute struct {
	Connection          net.Conn
	AuthenticatedTarget [32]byte
	AuthorityPublic     [32]byte
	Publication         []byte
	Generation          uint64
	AttachmentID        [32]byte

	cleanup func() error
	once    sync.Once
}

// OpenUserIntroductionRoute opens one Entry-to-Initiator carrier, authorizes
// its exact Rendezvous leg, then delivers the same State-selected Attachment
// ID to the selected Publisher Introduction slot. It closes the carrier on every
// refusal, never falling back to another peer, publication, or Target.
func (endpoint *endpoint) OpenUserIntroductionRoute(ctx context.Context, input UserIntroductionRouteRequest) (*UserIntroductionRoute, error) {
	if endpoint == nil || ctx == nil || input.At.IsZero() || input.Entry == nil || input.AttachmentID == [32]byte{} ||
		input.EndpointHandshake == [32]byte{} || !validTransitPeer(input.Initiator) ||
		!validTransitPeer(input.Rendezvous) || input.Initiator.NodeID == input.Rendezvous.NodeID ||
		input.Rendezvous.NodeID != input.Introduction.RendezvousNodeID || input.Introduction.NetworkID != endpoint.network ||
		!validUserIntroductionProfile(input.Introduction) {
		return nil, errors.New("user Introduction Route input is incomplete or outside its bound")
	}
	target, err := endpoint.TargetFromLink(input.TargetLink)
	if err != nil {
		return nil, err
	}
	connection, cleanup, err := route.OpenEntryAttachment(ctx, input.Entry, route.EntryAttachmentRequest{
		NetworkID: input.Introduction.NetworkID, Digest: input.Introduction.Digest, Epoch: input.Introduction.Epoch,
		AttachmentID: input.AttachmentID, Deadline: input.Introduction.NotAfter})
	if err != nil {
		return nil, errors.Join(errors.New("user Entry attachment is unavailable"), err)
	}
	if connection == nil || cleanup == nil {
		if connection != nil {
			_ = connection.Close()
		}
		return nil, errors.New("user Entry attachment lacks its owned cleanup")
	}
	closeRoute := func(cause error) (*UserIntroductionRoute, error) {
		return nil, errors.Join(cause, cleanup())
	}
	setup := route.RelaySetup{NetworkID: input.Introduction.NetworkID, Digest: input.Introduction.Digest, AttachmentID: input.AttachmentID,
		Epoch: input.Introduction.Epoch, TransitRole: route.InitiatorRole, NextRole: route.RendezvousRole,
		TransitNodeID: input.Initiator.NodeID, NextNodeID: input.Rendezvous.NodeID, NextNodePublicKey: input.Rendezvous.PublicKey,
		NotAfter: input.Introduction.NotAfter}
	if err := route.WriteRelaySetup(connection, setup); err != nil {
		return closeRoute(errors.Join(errors.New("user Initiator setup is unavailable"), err))
	}
	ready, err := route.ReadRelayReady(connection)
	if err != nil || setup.VerifyRelayReady(ready) != nil {
		return closeRoute(errors.Join(errors.New("user Initiator RelayReady is invalid"), err, setup.VerifyRelayReady(ready)))
	}
	delivery, err := endpoint.SubmitIntroductionFromLink(ctx, UserIntroductionRequest{TargetLink: input.TargetLink, Publication: input.Publication,
		AuthorityPublic: input.AuthorityPublic, Profile: input.Introduction, AttachmentID: input.AttachmentID, EndpointHandshake: input.EndpointHandshake, At: input.At})
	if err != nil {
		return closeRoute(err)
	}
	if delivery.AuthenticatedTarget != target {
		return closeRoute(errors.New("user Introduction Route authenticated a different Target"))
	}
	authority := input.AuthorityPublic
	if authority == [32]byte{} {
		authority = endpoint.authority
	}
	return &UserIntroductionRoute{Connection: connection, AuthenticatedTarget: delivery.AuthenticatedTarget, AuthorityPublic: authority,
		Publication: append([]byte(nil), input.Publication...), Generation: delivery.Generation, AttachmentID: input.AttachmentID, cleanup: cleanup}, nil
}

// Close releases the Entry-owned carrier exactly once. It does not withdraw a
// Publisher publication, alter Node duties, or retain Route material.
func (route *UserIntroductionRoute) Close() error {
	if route == nil || route.cleanup == nil {
		return nil
	}
	var err error
	route.once.Do(func() {
		err = route.cleanup()
		if errors.Is(err, net.ErrClosed) {
			err = nil
		}
	})
	return err
}

func validTransitPeer(peer TransitPeer) bool {
	return peer.NodeID != [32]byte{} && peer.PublicKey != [32]byte{} && peer.Endpoint != ""
}
