package route

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// openIntroduction opens the exact Entry-to-Initiator attachment, authorizes
// its State-selected Rendezvous leg, then delivers a sealed instruction to
// the Descriptor-selected Introduction slot. A successful delivery result
// proves receiving-node admission, so only then does presented become true.
// Every ambiguous or later failure is conservatively burned by Endpoint's
// credential owner rather than made reusable.
func (route *Route) openIntroduction(ctx context.Context, target [32]byte, verified reachability.Verified,
	initiator, introduction, rendezvous TransitPeer, attachment, handshake [32]byte, authorization []byte, certificate tls.Certificate, at time.Time,
) (connection net.Conn, cleanup func() error, evidence Evidence, presented bool, err error) {
	slot := verified.Descriptor.Introduction
	if route == nil || ctx == nil || target == [32]byte{} || !validUserTransitPeer(initiator) || !validUserTransitPeer(introduction) ||
		!validUserTransitPeer(rendezvous) || attachment == [32]byte{} || handshake == [32]byte{} || len(authorization) == 0 ||
		slot.StateDigest == [32]byte{} || slot.Epoch == 0 || slot.IntroductionNodeID != introduction.NodeID ||
		slot.RendezvousNodeID != rendezvous.NodeID || initiator.NodeID == introduction.NodeID || initiator.NodeID == rendezvous.NodeID ||
		introduction.NodeID == rendezvous.NodeID || at.IsZero() || !at.Before(slot.NotAfter) {
		return nil, nil, Evidence{}, false, errors.New("user Introduction Route input is incomplete or outside its bound")
	}
	connection, cleanup, err = OpenEntryAttachment(ctx, route.config.Entry, EntryAttachmentRequest{NetworkID: route.config.NetworkID,
		Digest: slot.StateDigest, Epoch: slot.Epoch, AttachmentID: attachment, Deadline: slot.NotAfter})
	if err != nil || connection == nil || cleanup == nil {
		if connection != nil {
			_ = connection.Close()
		}
		return nil, nil, Evidence{}, false, errors.Join(errors.New("user Entry attachment is unavailable"), err)
	}
	fail := func(cause error) (net.Conn, func() error, Evidence, bool, error) {
		return nil, nil, Evidence{}, presented, errors.Join(cause, cleanup())
	}
	setup := RelaySetup{NetworkID: route.config.NetworkID, Digest: slot.StateDigest, AttachmentID: attachment, Epoch: slot.Epoch,
		TransitRole: InitiatorRole, NextRole: RendezvousRole, TransitNodeID: initiator.NodeID, NextNodeID: rendezvous.NodeID,
		NextNodePublicKey: rendezvous.PublicKey, NotAfter: slot.NotAfter}
	if err := WriteRelaySetup(connection, setup); err != nil {
		return fail(errors.Join(errors.New("user Initiator setup is unavailable"), err))
	}
	ready, readyErr := ReadRelayReady(connection)
	if readyErr != nil || setup.VerifyRelayReady(ready) != nil {
		return fail(errors.Join(errors.New("user Initiator RelayReady is invalid"), readyErr, setup.VerifyRelayReady(ready)))
	}
	current, err := publication.Decode(verified.Current.Record, ed25519.PublicKey(verified.Descriptor.AuthorityPublic[:]), route.config.NetworkID, at)
	if err != nil || current.Credential.Target != target || current.Credential.NotAfter < slot.NotAfter.Unix() {
		return fail(errors.Join(errors.New("target does not authenticate the current publication"), err))
	}
	public, err := ecdh.X25519().NewPublicKey(current.Credential.IntroductionHPKEPublic[:])
	if err != nil {
		return fail(errors.Join(errors.New("publication Introduction recipient is invalid"), err))
	}
	recipient, err := hpke.NewDHKEMPublicKey(public)
	if err != nil {
		return fail(errors.Join(errors.New("publication Introduction recipient is unavailable"), err))
	}
	plaintext, err := publication.EncodeIntroductionInstruction(publication.IntroductionInstruction{Target: target,
		Generation: current.Credential.Generation, PublicationDigest: current.Digest, AttachmentID: attachment})
	if err != nil {
		return fail(err)
	}
	sealed, err := SealIntroduction(SealedIntroduction{NetworkID: route.config.NetworkID, Digest: slot.StateDigest, Epoch: slot.Epoch,
		IntroductionNodeID: introduction.NodeID, RendezvousNodeID: rendezvous.NodeID, Reachability: slot.Reachability,
		NotAfter: slot.NotAfter, JoinHandle: slot.JoinHandle, EndpointHandshake: handshake}, recipient, plaintext)
	if err != nil {
		return fail(err)
	}
	transit, err := OpenEndpointTransitAttachment(ctx, EndpointTransitAttachmentRequest{NetworkID: route.config.NetworkID, Digest: slot.StateDigest,
		AttachmentID: attachment, TransitNodeID: introduction.NodeID, TransitNodePublicKey: introduction.PublicKey, Epoch: slot.Epoch,
		TransitRole: IntroductionRole, Endpoint: introduction.Endpoint, Deadline: slot.NotAfter, Authorization: authorization,
		ClientCertificate: certificate})
	if err != nil {
		return fail(errors.Join(errors.New("introduction submission is unavailable"), err))
	}
	defer transit.Close()
	if err := WriteSealedIntroduction(transit, sealed); err != nil {
		return fail(errors.Join(errors.New("introduction submission is unavailable"), err))
	}
	delivery, err := ReadIntroductionDeliveryResult(transit)
	if err != nil || !deliveryConfirmsIntroductionAdmission(delivery, attachment) {
		return fail(errors.Join(errors.New("introduction submission is unavailable"), err))
	}
	presented = true
	return connection, cleanup, Evidence{AuthenticatedTarget: target, AuthorityPublic: verified.Descriptor.AuthorityPublic,
		Publication: append([]byte(nil), verified.Current.Record...), Generation: current.Credential.Generation, AttachmentID: attachment}, presented, nil
}

func validUserTransitPeer(peer TransitPeer) bool {
	return peer.NodeID != [32]byte{} && peer.PublicKey != [32]byte{} && peer.Family != [32]byte{} && peer.Endpoint != ""
}

func deliveryConfirmsIntroductionAdmission(delivery IntroductionDeliveryResult, attachment [32]byte) bool {
	return attachment != [32]byte{} && delivery.AttachmentID == attachment && delivery.Outcome == IntroductionDelivered
}
