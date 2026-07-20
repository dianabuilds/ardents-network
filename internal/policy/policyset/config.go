package policyset

import (
	"time"

	"ardents/internal/policy/rule"
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

func Normalize(cfg Config) Config {
	cfg.AllowedPolicyRefs = rule.NormalizeStrings(cfg.AllowedPolicyRefs)
	cfg.DeniedCapabilities = rule.NormalizeStrings(cfg.DeniedCapabilities)
	cfg.DeniedServiceTypes = rule.NormalizeStrings(cfg.DeniedServiceTypes)
	cfg.DeniedRouteSchemes = rule.NormalizeStrings(cfg.DeniedRouteSchemes)
	cfg.DeniedCapabilityScopes = rule.NormalizeStrings(cfg.DeniedCapabilityScopes)
	return cfg
}
