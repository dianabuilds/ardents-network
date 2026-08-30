package endpoint

import (
	"context"
	"crypto/hpke"
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// TransitPeer is one State-selected Introduction, Responder, or Rendezvous
// identity. Endpoint owns the chosen profile; this type does not discover or
// select a peer.
type TransitPeer struct {
	NodeID, PublicKey [32]byte
	Family            [32]byte
	Endpoint          string
}

// PublisherIntroductionProfile is the complete finite C-2 context selected
// for one Publisher live slot. Its authorizations are opaque capabilities for
// the receiving transit duties, not Service material or route candidates.
type PublisherIntroductionProfile struct {
	NetworkID, Digest                                 [32]byte
	Epoch                                             uint64
	Introduction, Rendezvous, Responder               TransitPeer
	SlotAttachmentID, Reachability, JoinHandle        [32]byte
	NotAfter                                          time.Time
	SlotAuthorization, ResponderAuthorization         []byte
	SlotClientCertificate, ResponderClientCertificate tls.Certificate
}

// PublisherIntroductionRecipient is the non-exporting host capability needed
// to authenticate and open one SealedIntroduction v1 record.
type PublisherIntroductionRecipient interface {
	route.IntroductionRecipient
	IntroductionPublic() [32]byte
}

// PublisherIntroductionRequest supplies the non-public HPKE recipient for a
// live Publisher publication and one State-selected C-2 profile.
type PublisherIntroductionRequest struct {
	Profile   PublisherIntroductionProfile
	Recipient PublisherIntroductionRecipient
	// HPKEPrivate is retained only for lower-level compatibility evidence.
	// The supported headless runtime passes Recipient instead.
	HPKEPrivate hpke.PrivateKey
	At          time.Time
}

// PublisherIntroduction owns one registered live slot and its retained
// publication lease. Wait returns one authenticated Responder carrier only
// after the sealed instruction matches that exact retained publication.
type PublisherIntroduction struct {
	endpoint  *endpoint
	profile   PublisherIntroductionProfile
	recipient PublisherIntroductionRecipient
	lease     *publication.Lease
	slot      net.Conn

	mu   sync.Mutex
	used bool
	once sync.Once
}

// OpenPublisherIntroduction registers one finite outbound slot. Its caller
// must retain the returned session until it has handed the accepted carrier to
// the Publisher Endpoint; Close then releases the current publication lease.
func (endpoint *endpoint) OpenPublisherIntroduction(ctx context.Context, input PublisherIntroductionRequest) (*PublisherIntroduction, error) {
	recipient, recipientErr := publisherIntroductionRecipient(input)
	if endpoint == nil || endpoint.publications == nil || ctx == nil || recipientErr != nil || input.At.IsZero() ||
		input.Profile.NetworkID != endpoint.network || !validPublisherIntroductionProfile(input.Profile) {
		return nil, errors.New("publisher Introduction input is incomplete or outside its bound")
	}
	lease, err := endpoint.publications.AcquireAt(ctx, input.At)
	if err != nil {
		return nil, errors.New("current Publisher publication is unavailable")
	}
	closeLease := func(cause error) (*PublisherIntroduction, error) {
		return nil, errors.Join(cause, lease.Close())
	}
	current := lease.Current()
	if input.Profile.NotAfter.Unix() > current.Credential.NotAfter || !matchesIntroductionRecipient(recipient, current.Credential.IntroductionHPKEPublic) {
		return closeLease(errors.New("publisher Introduction recipient does not match the current Credential"))
	}
	slotCertificate, err := endpoint.transitClientCertificate(input.Profile.SlotAuthorization, input.Profile.SlotClientCertificate)
	if err != nil {
		return closeLease(errors.Join(errors.New("publisher Introduction slot lacks its enrolled transit credential"), err))
	}
	connection, err := route.OpenEndpointTransitAttachment(ctx, route.EndpointTransitAttachmentRequest{
		NetworkID: input.Profile.NetworkID, Digest: input.Profile.Digest, AttachmentID: input.Profile.SlotAttachmentID,
		TransitNodeID: input.Profile.Introduction.NodeID, TransitNodePublicKey: input.Profile.Introduction.PublicKey,
		Epoch: input.Profile.Epoch, TransitRole: route.IntroductionRole, Endpoint: input.Profile.Introduction.Endpoint,
		Deadline: input.Profile.NotAfter, Authorization: input.Profile.SlotAuthorization, ClientCertificate: slotCertificate,
	})
	if err != nil {
		return closeLease(errors.Join(errors.New("publisher Introduction slot is unavailable"), err))
	}
	closeConnection := func(cause error) (*PublisherIntroduction, error) {
		return nil, errors.Join(cause, connection.Close(), lease.Close())
	}
	registration := route.IntroductionSlotRegistration{Reachability: input.Profile.Reachability,
		JoinHandle: input.Profile.JoinHandle, NotAfter: input.Profile.NotAfter}
	if err := route.WriteIntroductionSlotRegistration(connection, registration); err != nil {
		return closeConnection(err)
	}
	ready, err := route.ReadIntroductionSlotReady(connection)
	if err != nil || ready.Reachability != registration.Reachability || ready.JoinHandle != registration.JoinHandle ||
		!ready.NotAfter.Equal(registration.NotAfter) {
		return closeConnection(errors.Join(err, errors.New("publisher Introduction slot acknowledgement is invalid")))
	}
	return &PublisherIntroduction{endpoint: endpoint, profile: clonePublisherIntroductionProfile(input.Profile), recipient: recipient,
		lease: lease, slot: connection}, nil
}

// Wait receives one sealed slot delivery and creates the separately admitted
// Publisher-to-Responder attachment it authorizes. Any malformed, expired,
// replayed, foreign-publication, or cancelled delivery closes the slot and
// releases its lease without creating a Responder attachment.
func (session *PublisherIntroduction) Wait(ctx context.Context) (net.Conn, error) {
	if session == nil || ctx == nil {
		return nil, errors.New("publisher Introduction session is unavailable")
	}
	session.mu.Lock()
	if session.used || session.slot == nil || session.lease == nil {
		session.mu.Unlock()
		return nil, errors.New("publisher Introduction session is already closed or consumed")
	}
	session.used = true
	slot := session.slot
	session.mu.Unlock()
	deadline := session.profile.NotAfter
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := slot.SetDeadline(deadline); err != nil {
		session.Close()
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = slot.Close() })
	record, err := route.ReadIntroductionControlRecord(slot)
	stop()
	if err != nil || record.Sealed == nil || !session.matchesHeader(*record.Sealed) {
		session.Close()
		return nil, errors.Join(err, errors.New("publisher Introduction delivery is unavailable or invalid"))
	}
	plaintext, err := route.OpenSealedIntroductionWith(*record.Sealed, session.recipient)
	if err != nil {
		session.Close()
		return nil, errors.Join(errors.New("publisher Introduction ciphertext is invalid"), err)
	}
	instruction, err := publication.DecodeIntroductionInstruction(plaintext)
	if err != nil || instruction.AttachmentID == session.profile.SlotAttachmentID ||
		session.lease.Current().ValidateIntroductionInstruction(instruction) != nil {
		session.Close()
		return nil, errors.Join(err, errors.New("publisher Introduction does not match the current publication"))
	}
	responderCertificate, err := session.endpoint.transitClientCertificate(session.profile.ResponderAuthorization, session.profile.ResponderClientCertificate)
	if err != nil {
		session.Close()
		return nil, errors.Join(errors.New("publisher Responder attachment lacks its enrolled transit credential"), err)
	}
	carrier, err := route.OpenEndpointTransitAttachment(ctx, route.EndpointTransitAttachmentRequest{
		NetworkID: session.profile.NetworkID, Digest: session.profile.Digest, AttachmentID: instruction.AttachmentID,
		TransitNodeID: session.profile.Responder.NodeID, TransitNodePublicKey: session.profile.Responder.PublicKey,
		Epoch: session.profile.Epoch, TransitRole: route.ResponderRole, Endpoint: session.profile.Responder.Endpoint,
		Deadline: session.profile.NotAfter, Authorization: session.profile.ResponderAuthorization,
		ClientCertificate: responderCertificate,
	})
	if err != nil {
		session.Close()
		return nil, errors.Join(errors.New("publisher Responder attachment is unavailable"), err)
	}
	_ = slot.Close()
	return carrier, nil
}

// Accept waits for the one C-2-authorized Responder carrier and passes it to
// the Publisher Endpoint's existing native Connection acceptance path. The
// caller supplies only the authorized local Application attachment; it cannot
// substitute a route or install a recovery opener for this one-use handoff.
func (session *PublisherIntroduction) Accept(ctx context.Context, input InboundConnectionRequest) (RuntimeResult, error) {
	if session == nil || session.endpoint == nil || ctx == nil || input.Route != nil || input.OpenAttachment != nil || input.Application == nil || input.At.IsZero() ||
		(input.SendBytes == 0 && input.ReceiveBytes == 0 && input.BytesEachDirection == 0) {
		return failed("local authorization or policy denial", "Publisher Introduction local handoff is incomplete or attempts to select a route", errors.New("publisher Introduction local handoff is invalid"))
	}
	carrier, err := session.Wait(ctx)
	if err != nil {
		return failed("service unavailable", "Publisher Introduction delivery or Responder attachment is unavailable", err)
	}
	defer carrier.Close()
	defer session.Close()
	input.Route = carrier
	return session.endpoint.Accept(ctx, input)
}

// Close withdraws this local slot and releases the retained publication. It
// does not withdraw the Publisher's publication itself.
func (session *PublisherIntroduction) Close() error {
	if session == nil {
		return nil
	}
	var result error
	session.once.Do(func() {
		session.mu.Lock()
		slot, lease := session.slot, session.lease
		session.slot, session.lease = nil, nil
		session.mu.Unlock()
		if slot != nil {
			result = errors.Join(result, slot.Close())
		}
		if lease != nil {
			result = errors.Join(result, lease.Close())
		}
	})
	return result
}

func (session *PublisherIntroduction) matchesHeader(value route.SealedIntroduction) bool {
	profile := session.profile
	return value.NetworkID == profile.NetworkID && value.Digest == profile.Digest && value.Epoch == profile.Epoch &&
		value.IntroductionNodeID == profile.Introduction.NodeID && value.RendezvousNodeID == profile.Rendezvous.NodeID &&
		value.Reachability == profile.Reachability && value.JoinHandle == profile.JoinHandle && value.NotAfter.Equal(profile.NotAfter)
}

func validPublisherIntroductionProfile(value PublisherIntroductionProfile) bool {
	if value.NetworkID == [32]byte{} || value.Digest == [32]byte{} || value.Epoch == 0 || value.SlotAttachmentID == [32]byte{} ||
		value.Reachability == [32]byte{} || value.JoinHandle == [32]byte{} || value.NotAfter.IsZero() ||
		!value.NotAfter.Equal(value.NotAfter.UTC().Truncate(time.Second)) || !time.Now().UTC().Before(value.NotAfter) ||
		len(value.SlotAuthorization) == 0 || len(value.SlotAuthorization) > 1024 ||
		len(value.ResponderAuthorization) == 0 || len(value.ResponderAuthorization) > 1024 ||
		!validOptionalTransitClientCertificate(value.SlotClientCertificate) || !validOptionalTransitClientCertificate(value.ResponderClientCertificate) {
		return false
	}
	peers := []TransitPeer{value.Introduction, value.Rendezvous, value.Responder}
	for index, peer := range peers {
		if peer.NodeID == [32]byte{} || peer.PublicKey == [32]byte{} || peer.Endpoint == "" {
			return false
		}
		for prior := 0; prior < index; prior++ {
			if peer.NodeID == peers[prior].NodeID {
				return false
			}
		}
	}
	return true
}

func matchesIntroductionRecipient(recipient PublisherIntroductionRecipient, expected [32]byte) bool {
	if recipient == nil || expected == [32]byte{} {
		return false
	}
	public := recipient.IntroductionPublic()
	return subtle.ConstantTimeCompare(public[:], expected[:]) == 1
}

func publisherIntroductionRecipient(input PublisherIntroductionRequest) (PublisherIntroductionRecipient, error) {
	if input.Recipient != nil {
		if input.HPKEPrivate != nil {
			return nil, errors.New("publisher Introduction supplied two recipients")
		}
		return input.Recipient, nil
	}
	if input.HPKEPrivate == nil {
		return nil, errors.New("publisher Introduction recipient is unavailable")
	}
	return legacyIntroductionRecipient{private: input.HPKEPrivate}, nil
}

type legacyIntroductionRecipient struct {
	private hpke.PrivateKey
}

func (recipient legacyIntroductionRecipient) IntroductionPublic() [32]byte {
	var public [32]byte
	copy(public[:], recipient.private.PublicKey().Bytes())
	return public
}

func (recipient legacyIntroductionRecipient) OpenIntroduction(encapsulation, info, authenticatedHeader, ciphertext []byte) ([]byte, error) {
	opened, err := hpke.NewRecipient(encapsulation, recipient.private, hpke.HKDFSHA256(), hpke.AES128GCM(), info)
	if err != nil {
		return nil, err
	}
	return opened.Open(authenticatedHeader, ciphertext)
}

func clonePublisherIntroductionProfile(value PublisherIntroductionProfile) PublisherIntroductionProfile {
	value.SlotAuthorization = append([]byte(nil), value.SlotAuthorization...)
	value.ResponderAuthorization = append([]byte(nil), value.ResponderAuthorization...)
	return value
}
