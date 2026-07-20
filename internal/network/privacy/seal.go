package privacy

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"time"

	identityapi "ardents/internal/identity/api"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/network/privacy/wire"

	"golang.org/x/crypto/chacha20poly1305"
)

func Seal(request SealRequest) (SealedEnvelope, error) {
	public, lifetime, err := validateSealRequest(request)
	if err != nil {
		return SealedEnvelope{}, err
	}
	material, err := Derive(request.Capability)
	if err != nil {
		return SealedEnvelope{}, envelopeError(CodeEnvelopeSenderUnauthorized, "capability material is invalid")
	}
	header, err := newEnvelopeHeader(request, lifetime)
	if err != nil {
		return SealedEnvelope{}, err
	}
	message := newPrivateMessage(request, public)
	digest, err := signingDigest(header, message)
	if err != nil {
		return SealedEnvelope{}, err
	}
	message.Signature = ed25519.Sign(request.Signer, digest)
	inner, err := marshalPadded(message)
	if err != nil {
		return SealedEnvelope{}, err
	}
	return encryptPrivateMessage(material, header, inner)
}

func validateSealRequest(request SealRequest) (ed25519.PublicKey, time.Duration, error) {
	scope, classLifetime, ok := classProperties(request.Class)
	if !ok || request.Capability.Scope != scope {
		return nil, 0, envelopeError(CodeEnvelopeSenderUnauthorized, "message class is outside capability scope")
	}
	if request.Capability.Permissions&identityapi.CapabilityPublish == 0 {
		return nil, 0, envelopeError(CodeEnvelopeSenderUnauthorized, "capability does not permit publishing")
	}
	if len(request.Signer) != ed25519.PrivateKeySize || request.PayloadVersion == 0 {
		return nil, 0, envelopeError(CodeEnvelopeMalformed, "private message signer or payload version is invalid")
	}
	if zeroID(request.Capability.GrantID) {
		return nil, 0, envelopeError(CodeEnvelopeSenderUnauthorized, "capability grant identifier is invalid")
	}
	public := request.Signer.Public().(ed25519.PublicKey)
	if identityprincipal.DeriveID("p", public) != request.Capability.Subject {
		return nil, 0, envelopeError(CodeEnvelopeSenderUnauthorized, "signer does not match capability subject")
	}
	issuedAt := request.IssuedAt.UTC()
	if issuedAt.IsZero() || issuedAt.Nanosecond() != 0 {
		return nil, 0, envelopeError(CodeEnvelopeTimeInvalid, "issue time must use whole UTC seconds")
	}
	lifetime := request.Lifetime
	if lifetime == 0 {
		lifetime = classLifetime
	}
	if lifetime <= 0 || lifetime > classLifetime || lifetime%time.Second != 0 {
		return nil, 0, envelopeError(CodeEnvelopeTimeInvalid, "message lifetime is invalid for its class")
	}
	return public, lifetime, nil
}

func newEnvelopeHeader(request SealRequest, lifetime time.Duration) (envelopeHeader, error) {
	issuedAt := request.IssuedAt.UTC()
	header := envelopeHeader{
		Version: envelopeVersion, Suite: envelopeSuite,
		Generation: request.Capability.Generation,
		IssuedAt:   issuedAt.Unix(), ExpiresAt: issuedAt.Add(lifetime).Unix(),
	}
	random := request.Random
	if random == nil {
		random = rand.Reader
	}
	if _, err := io.ReadFull(random, header.MessageID[:]); err != nil {
		return envelopeHeader{}, err
	}
	if _, err := io.ReadFull(random, header.Nonce[:]); err != nil {
		return envelopeHeader{}, err
	}
	return header, nil
}

func newPrivateMessage(request SealRequest, public ed25519.PublicKey) *wire.PrivateMessageV1 {
	return &wire.PrivateMessageV1{
		ProtocolVersion: 1, MessageClass: wire.MessageClass(request.Class),
		GrantId:         request.Capability.GrantID[:],
		SenderPrincipal: request.Capability.Subject,
		SenderPublicKey: append([]byte(nil), public...),
		PayloadVersion:  request.PayloadVersion,
		Payload:         append([]byte(nil), request.Payload...),
	}
}

func encryptPrivateMessage(material Material, header envelopeHeader, inner []byte) (SealedEnvelope, error) {
	aead, err := chacha20poly1305.NewX(material.EnvelopeKey())
	if err != nil {
		return SealedEnvelope{}, err
	}
	header.CiphertextLength = uint32(len(inner) + aead.Overhead())
	headerRaw := header.marshal()
	aad, err := associatedData(headerRaw, DefaultPubsubTopic, material.ContentTopic)
	if err != nil {
		return SealedEnvelope{}, err
	}
	ciphertext := aead.Seal(nil, header.Nonce[:], inner, aad)
	payload := append(headerRaw, ciphertext...)
	if len(payload) > maximumOuterSize {
		return SealedEnvelope{}, envelopeError(CodeEnvelopeOversized, "sealed envelope exceeds maximum size")
	}
	return SealedEnvelope{
		PubsubTopic: DefaultPubsubTopic, ContentTopic: material.ContentTopic, Payload: payload,
	}, nil
}
