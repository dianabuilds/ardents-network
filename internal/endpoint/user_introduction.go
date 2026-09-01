package endpoint

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/tls"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// UserIntroductionProfile is the finite State-selected C-2 submission path
// for one already chosen Service Connection attachment. It cannot select the
// next Initiator or Rendezvous endpoint; it names only the Introduction peer
// required to submit the sealed request.
type userIntroductionProfile struct {
	NetworkID, Digest                          [32]byte
	Epoch                                      uint64
	Introduction                               transitPeer
	RendezvousNodeID, Reachability, JoinHandle [32]byte
	NotAfter                                   time.Time
	SubmissionAuthorization                    []byte
	// SubmissionClientCertificate is the private one-use TLS key paired with
	// a pre-issued State Transit Grant. It is local Endpoint material and is
	// never part of a Reachability Descriptor.
	SubmissionClientCertificate tls.Certificate
}

// UserIntroductionRequest binds a shared Target Link, a current public
// publication record, and one C-5 attachment chosen by the Endpoint. Neither
// the Target Link nor this operation may fall back to another destination.
type userIntroductionRequest struct {
	TargetLink  string
	Publication []byte
	// AuthorityPublic is present only when a verified Private Reachability
	// Descriptor supplied the exact Target authority. Legacy direct callers
	// retain the Endpoint's configured authority.
	AuthorityPublic                 [32]byte
	Profile                         userIntroductionProfile
	AttachmentID, EndpointHandshake [32]byte
	At                              time.Time
}

// UserIntroductionResult exposes only the exact authenticated Target and the
// selected attachment after Introduction reports sealed-byte delivery. It is
// not a Publisher, Responder, or Service availability result.
type userIntroductionResult struct {
	AuthenticatedTarget [32]byte
	Generation          uint64
	AttachmentID        [32]byte
}

// SubmitIntroductionFromLink authenticates the exact Target Link against one
// current publication, seals the Service-only instruction, and submits it on
// the selected Introduction path. A delivered result means only that the
// sealed bytes entered the live slot; the caller must separately establish
// the already-selected C-5 attachment.
func (endpoint *endpoint) SubmitIntroductionFromLink(ctx context.Context, input userIntroductionRequest) (userIntroductionResult, error) {
	if endpoint == nil || ctx == nil || input.At.IsZero() || input.AttachmentID == [32]byte{} || input.EndpointHandshake == [32]byte{} ||
		!validUserIntroductionProfile(input.Profile) {
		return userIntroductionResult{}, errors.New("user Introduction input is incomplete or outside its bound")
	}
	target, err := endpoint.TargetFromLink(input.TargetLink)
	if err != nil {
		return userIntroductionResult{}, err
	}
	authority := endpoint.authority
	if input.AuthorityPublic != [32]byte{} {
		authority = input.AuthorityPublic
	}
	current, err := publication.Decode(input.Publication, ed25519.PublicKey(authority[:]), endpoint.network, input.At)
	if err != nil || current.Credential.Target != target || input.Profile.NotAfter.Unix() > current.Credential.NotAfter {
		return userIntroductionResult{}, errors.Join(err, errors.New("target Link does not authenticate the current publication"))
	}
	public, err := ecdh.X25519().NewPublicKey(current.Credential.IntroductionHPKEPublic[:])
	if err != nil {
		return userIntroductionResult{}, errors.Join(errors.New("publication Introduction recipient is invalid"), err)
	}
	recipient, err := hpke.NewDHKEMPublicKey(public)
	if err != nil {
		return userIntroductionResult{}, errors.Join(errors.New("publication Introduction recipient is unavailable"), err)
	}
	plaintext, err := publication.EncodeIntroductionInstruction(publication.IntroductionInstruction{Target: target,
		Generation: current.Credential.Generation, PublicationDigest: current.Digest, AttachmentID: input.AttachmentID})
	if err != nil {
		return userIntroductionResult{}, err
	}
	sealed, err := route.SealIntroduction(route.SealedIntroduction{NetworkID: input.Profile.NetworkID, Digest: input.Profile.Digest,
		Epoch: input.Profile.Epoch, IntroductionNodeID: input.Profile.Introduction.NodeID, RendezvousNodeID: input.Profile.RendezvousNodeID,
		Reachability: input.Profile.Reachability, NotAfter: input.Profile.NotAfter, JoinHandle: input.Profile.JoinHandle,
		EndpointHandshake: input.EndpointHandshake}, recipient, plaintext)
	if err != nil {
		return userIntroductionResult{}, err
	}
	certificate, err := endpoint.transitClientCertificate(input.Profile.SubmissionAuthorization, input.Profile.SubmissionClientCertificate)
	if err != nil {
		return userIntroductionResult{}, errors.Join(errors.New("introduction submission lacks its enrolled transit credential"), err)
	}
	connection, err := route.OpenEndpointTransitAttachment(ctx, route.EndpointTransitAttachmentRequest{NetworkID: input.Profile.NetworkID,
		Digest: input.Profile.Digest, AttachmentID: input.AttachmentID, TransitNodeID: input.Profile.Introduction.NodeID,
		TransitNodePublicKey: input.Profile.Introduction.PublicKey, Epoch: input.Profile.Epoch, TransitRole: route.IntroductionRole,
		Endpoint: input.Profile.Introduction.Endpoint, Deadline: input.Profile.NotAfter, Authorization: input.Profile.SubmissionAuthorization,
		ClientCertificate: certificate})
	if err != nil {
		return userIntroductionResult{}, errors.Join(errors.New("introduction submission is unavailable"), err)
	}
	defer connection.Close()
	if err := route.WriteSealedIntroduction(connection, sealed); err != nil {
		return userIntroductionResult{}, errors.Join(errors.New("introduction submission is unavailable"), err)
	}
	delivery, err := route.ReadIntroductionDeliveryResult(connection)
	if err != nil || delivery.AttachmentID != input.AttachmentID || delivery.Outcome != route.IntroductionDelivered {
		return userIntroductionResult{}, errors.Join(err, errors.New("introduction submission is unavailable"))
	}
	return userIntroductionResult{AuthenticatedTarget: target, Generation: current.Credential.Generation,
		AttachmentID: input.AttachmentID}, nil
}

func validUserIntroductionProfile(value userIntroductionProfile) bool {
	return value.NetworkID != [32]byte{} && value.Digest != [32]byte{} && value.Epoch != 0 &&
		value.Introduction.NodeID != [32]byte{} && value.Introduction.PublicKey != [32]byte{} && value.Introduction.Endpoint != "" &&
		value.RendezvousNodeID != [32]byte{} && value.RendezvousNodeID != value.Introduction.NodeID && value.Reachability != [32]byte{} &&
		value.JoinHandle != [32]byte{} && !value.NotAfter.IsZero() && value.NotAfter.Equal(value.NotAfter.UTC().Truncate(time.Second)) &&
		time.Now().UTC().Before(value.NotAfter) && len(value.SubmissionAuthorization) > 0 && len(value.SubmissionAuthorization) <= 1024 &&
		validOptionalTransitClientCertificate(value.SubmissionClientCertificate)
}
