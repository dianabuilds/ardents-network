package main

import (
	"crypto/ed25519"
	"fmt"
	"time"

	identityapi "ardents/internal/identity/api"
	identitycapability "ardents/internal/identity/capability"
	networkprivacy "ardents/internal/network/privacy"
	apppolicy "ardents/internal/policy"
	runtimeconfig "ardents/internal/runtime/config"
	runtimeprocess "ardents/internal/runtime/process"
)

func operatorPrivacyChannels(
	doc runtimeconfig.Document,
	policyConfig runtimeprocess.PolicyConfig,
) (*networkprivacy.Channel, *networkprivacy.Channel, *apppolicy.Service, error) {
	policyService := runtimeprocess.NewPolicyService(policyConfig)
	if !doc.Privacy.Required {
		return nil, nil, policyService, nil
	}
	private, storeKey, replayKey, issuers, err := operatorPrivacyInputs(doc)
	if err != nil {
		return nil, nil, nil, err
	}
	authority, err := identitycapability.NewService(
		doc.Privacy.CapabilityStore, storeKey, doc.Privacy.Subject, issuers, policyService, time.Now,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("protected privacy capability store is unavailable or invalid")
	}
	discovery, err := buildOperatorPrivacyChannel(authority, private, replayKey, doc.Privacy.Subject,
		doc.Privacy.Discovery, identityapi.CapabilityRealmDiscovery)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("configure discovery privacy channel: %w", err)
	}
	data, err := buildOperatorPrivacyChannel(authority, private, replayKey, doc.Privacy.Subject,
		doc.Privacy.Data, identityapi.CapabilityDataExchange)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("configure data privacy channel: %w", err)
	}
	return discovery, data, policyService, nil
}

func operatorPrivacyInputs(doc runtimeconfig.Document) (
	ed25519.PrivateKey, []byte, []byte, map[string]ed25519.PublicKey, error,
) {
	private, err := operatorIdentityPrivate(doc.Node.DataDir, doc.Privacy.Subject)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	storeKey, err := readProtectedKey(doc.Privacy.CapabilityStoreKeyFile, "privacy capability-store key file")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	replayKey, err := readProtectedKey(doc.Privacy.ReplayKeyFile, "privacy replay key file")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	issuers, err := operatorTrustedIssuers(doc.Privacy.TrustedIssuers)
	return private, storeKey, replayKey, issuers, err
}

func buildOperatorPrivacyChannel(
	authority *identitycapability.Service,
	private ed25519.PrivateKey,
	replayKey []byte,
	subject string,
	cfg runtimeconfig.PrivacyChannelConfig,
	scope identityapi.CapabilityScope,
) (*networkprivacy.Channel, error) {
	ref := identityapi.CapabilityRef(cfg.Reference)
	now := time.Now().UTC().Truncate(time.Second)
	for _, permission := range []identityapi.CapabilityPermission{
		identityapi.CapabilityPublish, identityapi.CapabilitySubscribe, identityapi.CapabilityStoreFetch,
	} {
		if _, err := authority.ResolveCapability(identityapi.CapabilityUse{
			Ref: ref, Subject: subject, Permission: permission, Scope: scope, At: now,
		}); err != nil {
			return nil, fmt.Errorf("required capability is unavailable: %w", err)
		}
	}
	replay, err := networkprivacy.NewDurableReplayLedger(cfg.ReplayPath, replayKey, 4096, 16384)
	if err != nil {
		return nil, fmt.Errorf("durable privacy replay ledger is unavailable or invalid")
	}
	return networkprivacy.NewChannel(networkprivacy.ChannelConfig{
		Resolver: authority, Authorizer: authority, Replay: replay,
		Reference: ref, Subject: subject, Scope: scope,
		Signer: func() ed25519.PrivateKey { return private }, Clock: time.Now,
	})
}
