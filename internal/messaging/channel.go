package messaging

import (
	"crypto/ed25519"
	"fmt"
	"time"

	identityapi "ardents/internal/identity"
)

const CodeChannelGrantMissing = "privacy.channel_grant.missing"

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

type capabilityGenerationResolver interface {
	ResolveCapabilityGeneration(identityapi.CapabilityUse, uint32) (identityapi.ResolvedCapability, error)
	AvailableCapabilityGenerations(
		identityapi.CapabilityRef,
		string,
		identityapi.CapabilityScope,
		time.Time,
	) []uint32
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

func (c *Channel) ContentTopics() ([]string, error) {
	return c.receiveContentTopics(identityapi.CapabilitySubscribe)
}

func (c *Channel) StoreContentTopics() ([]string, error) {
	return c.receiveContentTopics(identityapi.CapabilityStoreFetch)
}

func (c *Channel) contentTopic(permission identityapi.CapabilityPermission) (string, error) {
	resolved, _, err := c.resolve(permission)
	if err != nil {
		return "", err
	}
	material, err := Derive(resolved)
	if err != nil {
		return "", envelopeError(CodeChannelGrantMissing, "private channel material is unavailable")
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
	header, err := parseHeader(envelope.Payload)
	if err != nil {
		return OpenedMessage{}, err
	}
	resolved, now, err := c.resolveGeneration(
		identityapi.CapabilitySubscribe, header.Generation,
	)
	if err != nil {
		return OpenedMessage{}, err
	}
	return Open(OpenRequest{
		Capability: resolved, PubsubTopic: envelope.PubsubTopic,
		ContentTopic: envelope.ContentTopic, Payload: envelope.Payload,
		Authorizer: c.authorizer, Replay: c.replay, Now: now,
	})
}

func (c *Channel) receiveContentTopics(
	permission identityapi.CapabilityPermission,
) ([]string, error) {
	now := c.clock().UTC().Truncate(time.Second)
	generations := []uint32{0}
	if resolver, ok := c.resolver.(capabilityGenerationResolver); ok {
		generations = resolver.AvailableCapabilityGenerations(
			c.reference, c.subject, c.scope, now,
		)
	}
	topics := make([]string, 0, len(generations))
	seen := make(map[string]struct{}, len(generations))
	for _, generation := range generations {
		var resolved identityapi.ResolvedCapability
		var err error
		if generation == 0 {
			resolved, _, err = c.resolve(permission)
		} else {
			resolved, _, err = c.resolveGeneration(permission, generation)
		}
		if err != nil {
			return nil, err
		}
		material, err := Derive(resolved)
		if err != nil {
			return nil, envelopeError(CodeChannelGrantMissing, "private channel material is unavailable")
		}
		if _, duplicate := seen[material.ContentTopic]; duplicate {
			continue
		}
		seen[material.ContentTopic] = struct{}{}
		topics = append(topics, material.ContentTopic)
	}
	if len(topics) == 0 {
		return nil, envelopeError(CodeChannelGrantMissing, "private channel material is unavailable")
	}
	return topics, nil
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

func (c *Channel) resolveGeneration(
	permission identityapi.CapabilityPermission,
	generation uint32,
) (identityapi.ResolvedCapability, time.Time, error) {
	now := c.clock().UTC().Truncate(time.Second)
	if resolver, ok := c.resolver.(capabilityGenerationResolver); ok {
		resolved, err := resolver.ResolveCapabilityGeneration(identityapi.CapabilityUse{
			Ref: c.reference, Subject: c.subject, Permission: permission,
			Scope: c.scope, At: now,
		}, generation)
		return resolved, now, err
	}
	resolved, _, err := c.resolve(permission)
	if err != nil || resolved.Generation != generation {
		return identityapi.ResolvedCapability{}, now,
			envelopeError(CodeChannelGrantMissing, "private channel generation is unavailable")
	}
	return resolved, now, nil
}
