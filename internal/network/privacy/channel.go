package privacy

import (
	"crypto/ed25519"
	"fmt"
	"time"

	identityapi "ardents/internal/identity/api"
)

const CodeCapabilityMissing = "privacy.capability.missing"

type ChannelConfig struct {
	Resolver   identityapi.CapabilityResolver
	Authorizer identityapi.CapabilitySenderAuthorizer
	Replay     ReplayLedger
	Reference  identityapi.CapabilityRef
	Subject    string
	Scope      identityapi.CapabilityScope
	Signer     func() ed25519.PrivateKey
	Clock      func() time.Time
}

type Channel struct {
	resolver   identityapi.CapabilityResolver
	authorizer identityapi.CapabilitySenderAuthorizer
	replay     ReplayLedger
	reference  identityapi.CapabilityRef
	subject    string
	scope      identityapi.CapabilityScope
	signer     func() ed25519.PrivateKey
	clock      func() time.Time
}

func NewChannel(cfg ChannelConfig) (*Channel, error) {
	if cfg.Resolver == nil || cfg.Authorizer == nil || cfg.Replay == nil ||
		cfg.Reference == "" || cfg.Subject == "" || cfg.Scope == "" || cfg.Signer == nil {
		return nil, fmt.Errorf("private channel configuration is incomplete")
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &Channel{
		resolver: cfg.Resolver, authorizer: cfg.Authorizer, replay: cfg.Replay,
		reference: cfg.Reference, subject: cfg.Subject, scope: cfg.Scope,
		signer: cfg.Signer, clock: cfg.Clock,
	}, nil
}

func (c *Channel) ContentTopic() (string, error) {
	return c.contentTopic(identityapi.CapabilitySubscribe)
}

func (c *Channel) StoreContentTopic() (string, error) {
	return c.contentTopic(identityapi.CapabilityStoreFetch)
}

func (c *Channel) contentTopic(permission identityapi.CapabilityPermission) (string, error) {
	resolved, _, err := c.resolve(permission)
	if err != nil {
		return "", err
	}
	material, err := Derive(resolved)
	if err != nil {
		return "", envelopeError(CodeCapabilityMissing, "private channel material is unavailable")
	}
	return material.ContentTopic, nil
}

func (c *Channel) Seal(class MessageClass, payloadVersion uint32, payload []byte) (SealedEnvelope, error) {
	resolved, now, err := c.resolve(identityapi.CapabilityPublish)
	if err != nil {
		return SealedEnvelope{}, err
	}
	return Seal(SealRequest{
		Capability: resolved, Class: class, PayloadVersion: payloadVersion,
		Payload: payload, Signer: c.signer(), IssuedAt: now,
	})
}

func (c *Channel) Open(envelope SealedEnvelope) (OpenedMessage, error) {
	resolved, now, err := c.resolve(identityapi.CapabilitySubscribe)
	if err != nil {
		return OpenedMessage{}, err
	}
	return Open(OpenRequest{
		Capability: resolved, PubsubTopic: envelope.PubsubTopic,
		ContentTopic: envelope.ContentTopic, Payload: envelope.Payload,
		Authorizer: c.authorizer, Replay: c.replay, Now: now,
	})
}

func (c *Channel) resolve(permission identityapi.CapabilityPermission) (identityapi.ResolvedCapability, time.Time, error) {
	now := c.clock().UTC().Truncate(time.Second)
	resolved, err := c.resolver.ResolveCapability(identityapi.CapabilityUse{
		Ref: c.reference, Subject: c.subject, Permission: permission,
		Scope: c.scope, At: now,
	})
	if err != nil {
		return identityapi.ResolvedCapability{}, now, err
	}
	return resolved, now, nil
}
