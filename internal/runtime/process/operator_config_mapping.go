package process

import (
	"time"

	runtimeconfig "ardents/internal/runtime/config"
)

func policyConfigFromOperator(in runtimeconfig.PolicyConfig) PolicyConfig {
	return PolicyConfig{
		MaxWorkloads: in.MaxWorkloads, AllowedPolicyRefs: cloneStrings(in.AllowedPolicyRefs),
		DeniedCapabilities:              cloneStrings(in.DeniedCapabilities),
		DisableServicePublication:       in.DisableServicePublication,
		DisableNetworkPublishedServices: in.DisableNetworkPublishedServices,
		DeniedServiceTypes:              cloneStrings(in.DeniedServiceTypes),
		DisableUntrustedRouteUse:        in.DisableUntrustedRouteUse,
		DeniedRouteSchemes:              cloneStrings(in.DeniedRouteSchemes),
		DisablePrivateCapabilityUse:     in.DisablePrivateCapabilityUse,
		DeniedCapabilityScopes:          cloneStrings(in.DeniedCapabilityScopes),
		DisableLocalBlobRetention:       in.DisableLocalBlobRetention,
		DisableRelayBlobRetention:       in.DisableRelayBlobRetention,
		DisableBlobPinning:              in.DisableBlobPinning,
		DisablePeerBlobReserving:        in.DisablePeerBlobReserving,
		AllowPinRelayRetainedBlobs:      in.AllowPinRelayRetainedBlobs,
		AllowReservingRelayBlobs:        in.AllowReservingRelayBlobs,
		MaxLocalRetentionTTL:            parseOperatorDuration(in.MaxLocalRetention),
		MaxRelayRetentionTTL:            parseOperatorDuration(in.MaxRelayRetention),
	}
}

func parseOperatorDuration(raw string) time.Duration {
	value, _ := time.ParseDuration(raw)
	return value
}
