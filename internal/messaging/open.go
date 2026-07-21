package messaging

import (
	"crypto/ed25519"
	"fmt"
	"time"

	identityapi "ardents/internal/identity"

	"golang.org/x/crypto/chacha20poly1305"
	"google.golang.org/protobuf/proto"
)

func Open(request OpenRequest) (OpenedMessage, error) {
	header, material, now, err := prepareOpen(request)
	if err != nil {
		return OpenedMessage{}, err
	}
	inner, err := authenticateEnvelope(request, header, material)
	if err != nil {
		return OpenedMessage{}, err
	}
	if err := admitAuthenticatedReplay(request, header, now); err != nil {
		return OpenedMessage{}, err
	}
	message, err := decodePrivateMessage(inner)
	if err != nil {
		return OpenedMessage{}, err
	}
	return authorizePrivateMessage(request, header, message)
}

func prepareOpen(request OpenRequest) (envelopeHeader, Material, time.Time, error) {
	if len(request.Payload) > maximumOuterSize {
		return envelopeHeader{}, Material{}, time.Time{}, envelopeError(CodeEnvelopeOversized, "envelope exceeds maximum size")
	}
	header, err := parseHeader(request.Payload)
	if err != nil {
		return envelopeHeader{}, Material{}, time.Time{}, err
	}
	now := request.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err := validateTimes(header, now); err != nil {
		return envelopeHeader{}, Material{}, time.Time{}, err
	}
	material, err := Derive(request.Capability)
	if err != nil || request.PubsubTopic != DefaultPubsubTopic ||
		request.ContentTopic != material.ContentTopic ||
		header.Generation != request.Capability.Generation {
		return envelopeHeader{}, Material{}, time.Time{}, envelopeError(CodeEnvelopeAuthentication, "envelope routing or capability does not match")
	}
	if request.Capability.Permissions&identityapi.CapabilitySubscribe == 0 {
		return envelopeHeader{}, Material{}, time.Time{}, envelopeError(CodeEnvelopeSenderUnauthorized, "capability does not permit subscribing")
	}
	return header, material, now, nil
}

func admitAuthenticatedReplay(request OpenRequest, header envelopeHeader, now time.Time) error {
	if request.Replay == nil {
		return envelopeError(CodeEnvelopeReplayed, "replay ledger is required")
	}
	return request.Replay.Admit(ReplayUse{
		CapabilityRef: request.Capability.Ref, Generation: header.Generation,
		MessageID: header.MessageID, ExpiresAt: time.Unix(header.ExpiresAt, 0).UTC(), Now: now,
	})
}

func decodePrivateMessage(inner []byte) (*PrivateMessageV1, error) {
	message := &PrivateMessageV1{}
	if err := (proto.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(inner, message); err != nil {
		return nil, envelopeError(CodeEnvelopeMalformed, "private message cannot be decoded")
	}
	if err := validateDecodedInner(inner, message); err != nil {
		return nil, envelopeError(CodeEnvelopeMalformed, err.Error())
	}
	return message, nil
}

func authorizePrivateMessage(request OpenRequest, header envelopeHeader, message *PrivateMessageV1) (OpenedMessage, error) {
	class := message.MessageClass
	scope, classLifetime, ok := classProperties(class)
	issuedAt := time.Unix(header.IssuedAt, 0).UTC()
	expiresAt := time.Unix(header.ExpiresAt, 0).UTC()
	if !ok || scope != request.Capability.Scope ||
		expiresAt.Sub(issuedAt) > classLifetime {
		return OpenedMessage{}, envelopeError(CodeEnvelopeSenderUnauthorized, "message class is not admitted by capability")
	}
	digest, err := signingDigest(header, message)
	if err != nil || !ed25519.Verify(message.SenderPublicKey, digest, message.Signature) {
		return OpenedMessage{}, envelopeError(CodeEnvelopeSignatureInvalid, "private message signature is invalid")
	}
	if request.Authorizer == nil {
		return OpenedMessage{}, envelopeError(CodeEnvelopeSenderUnauthorized, "sender authorizer is required")
	}
	var grantID [16]byte
	copy(grantID[:], message.GrantId)
	if err := request.Authorizer.AuthorizeCapabilitySender(identityapi.CapabilitySenderUse{
		GrantID: grantID, ChannelID: request.Capability.ChannelID,
		Generation: header.Generation, Subject: message.SenderPrincipal,
		Permission: identityapi.CapabilityPublish, Scope: scope, At: issuedAt,
		ObservedAt: request.Now.UTC(),
	}); err != nil {
		return OpenedMessage{}, envelopeError(CodeEnvelopeSenderUnauthorized, "sender capability is not authorized")
	}
	return OpenedMessage{
		Class: class, PayloadVersion: message.PayloadVersion,
		Payload: append([]byte(nil), message.Payload...), Sender: message.SenderPrincipal,
		IssuedAt: issuedAt, ExpiresAt: expiresAt,
	}, nil
}

func authenticateEnvelope(request OpenRequest, header envelopeHeader, material Material) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(material.EnvelopeKey())
	if err != nil {
		return nil, err
	}
	headerRaw := request.Payload[:headerSize]
	aad, err := associatedData(headerRaw, request.PubsubTopic, request.ContentTopic)
	if err != nil {
		return nil, envelopeError(CodeEnvelopeMalformed, err.Error())
	}
	inner, err := aead.Open(nil, header.Nonce[:], request.Payload[headerSize:], aad)
	if err != nil {
		return nil, envelopeError(CodeEnvelopeAuthentication, "envelope authentication failed")
	}
	if len(inner) > maximumInnerSize {
		return nil, envelopeError(CodeEnvelopeOversized, fmt.Sprintf("inner message exceeds %d bytes", maximumInnerSize))
	}
	return inner, nil
}
