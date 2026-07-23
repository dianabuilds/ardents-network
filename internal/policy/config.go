// Package policy owns allow and deny decisions with stable reasons.
// It does not own performing workload, publication, retention, or transport operations.
package policy

import (
	workloadregistry "ardents/internal/workload/registry"
	"time"
)

type Config struct {
	MaxWorkloads                    int
	AllowedPolicyRefs               []string
	DeniedWorkloadRequirements      []workloadregistry.WorkloadRequirement
	DisableServicePublication       bool
	DisableNetworkPublishedServices bool
	DeniedServiceTypes              []string
	DisableUntrustedRouteUse        bool
	DeniedRouteSchemes              []string
	DisablePrivateChannelGrantUse   bool
	DeniedChannelGrantScopes        []string
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
	cfg.DeniedWorkloadRequirements = append([]workloadregistry.WorkloadRequirement(nil), cfg.DeniedWorkloadRequirements...)
	cfg.DeniedServiceTypes = NormalizeStrings(cfg.DeniedServiceTypes)
	cfg.DeniedRouteSchemes = NormalizeStrings(cfg.DeniedRouteSchemes)
	cfg.DeniedChannelGrantScopes = NormalizeStrings(cfg.DeniedChannelGrantScopes)
	return cfg
}
