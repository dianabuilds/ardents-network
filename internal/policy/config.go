// Package policy owns allow and deny decisions with stable reasons.
// It does not own performing workload, publication, retention, or transport operations.
package policy

import (
	"time"
)

type Config struct {
	MaxWorkloads                    int
	AllowedPolicyRefs               []string
	DeniedCapabilities              []string
	DisableServicePublication       bool
	DisableNetworkPublishedServices bool
	DeniedServiceTypes              []string
	DisableUntrustedRouteUse        bool
	DeniedRouteSchemes              []string
	DisablePrivateCapabilityUse     bool
	DeniedCapabilityScopes          []string
	DisableLocalBlobRetention       bool
	DisableRelayBlobRetention       bool
	DisableBlobPinning              bool
	DisablePeerBlobReserving        bool
	AllowPinRelayRetainedBlobs      bool
	AllowReservingRelayBlobs        bool
	MaxLocalRetentionTTL            time.Duration
	MaxRelayRetentionTTL            time.Duration
}

func normalizeConfig(cfg Config) Config {
	cfg.AllowedPolicyRefs = NormalizeStrings(cfg.AllowedPolicyRefs)
	cfg.DeniedCapabilities = NormalizeStrings(cfg.DeniedCapabilities)
	cfg.DeniedServiceTypes = NormalizeStrings(cfg.DeniedServiceTypes)
	cfg.DeniedRouteSchemes = NormalizeStrings(cfg.DeniedRouteSchemes)
	cfg.DeniedCapabilityScopes = NormalizeStrings(cfg.DeniedCapabilityScopes)
	return cfg
}
