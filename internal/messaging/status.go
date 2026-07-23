package messaging

import "slices"

import identityapi "ardents/internal/identity"
import "ardents/internal/network"

const (
	ProfileV1                   = "ardents-private/1"
	StatusActive                = "active"
	StatusDegraded              = "degraded"
	ReasonChannelGrantReady     = "channel_grant_ready"
	RecoverySteady              = "steady"
	RecoveryPending             = "recovery_pending"
	RecoveryBlocked             = "blocked"
	CodeChannelGrantUnavailable = "privacy.channel_grant.unavailable"
)

type StatusSnapshot struct {
	Profile         string
	State           string
	SwitchReason    string
	RecoveryState   string
	ReducedFeatures []network.TransportFeature
	ErrorCategories []string
}

func Snapshot(discovery, data *Channel) StatusSnapshot {
	status := StatusSnapshot{Profile: ProfileV1, State: StatusActive, SwitchReason: ReasonChannelGrantReady, RecoveryState: RecoverySteady}
	blocked := false
	check := func(channel *Channel, feature network.TransportFeature, permissions ...identityapi.CapabilityPermission) {
		if channel == nil {
			status.reduce(feature, CodeChannelGrantMissing)
			blocked = true
			return
		}
		for _, permission := range permissions {
			if _, _, err := channel.resolve(permission); err != nil {
				status.reduce(feature, safeChannelGrantCode(err))
				return
			}
		}
	}
	check(discovery, network.TransportFeaturePrivatePublication, identityapi.CapabilityPublish)
	check(discovery, network.TransportFeaturePrivateDiscovery, identityapi.CapabilitySubscribe, identityapi.CapabilityStoreFetch)
	check(data, network.TransportFeaturePrivateDataExchange, identityapi.CapabilityPublish, identityapi.CapabilitySubscribe)
	if status.State == StatusDegraded {
		status.RecoveryState = RecoveryPending
		if blocked {
			status.RecoveryState = RecoveryBlocked
		}
	}
	return status
}

func (s *StatusSnapshot) reduce(feature network.TransportFeature, code string) {
	s.State = StatusDegraded
	s.ReducedFeatures = appendUniqueFeature(s.ReducedFeatures, feature)
	s.ErrorCategories = appendUnique(s.ErrorCategories, code)
	if s.SwitchReason == ReasonChannelGrantReady {
		s.SwitchReason = code
	}
}

func safeChannelGrantCode(err error) string {
	if code := CodeOf(err); IsSafeErrorCategory(code) {
		return code
	}
	return CodeChannelGrantUnavailable
}

func IsSafeErrorCategory(code string) bool {
	switch code {
	case CodeChannelGrantMissing,
		"privacy.channel_grant.not_yet_valid",
		"privacy.channel_grant.expired",
		"privacy.channel_grant.revoked",
		"privacy.channel_grant.scope_denied",
		"privacy.channel_grant.issuer_untrusted",
		"privacy.channel_grant.invalid",
		CodeChannelGrantUnavailable:
		return true
	default:
		return false
	}
}

func appendUnique(items []string, value string) []string {
	if slices.Contains(items, value) {
		return items
	}
	return append(items, value)
}

func appendUniqueFeature(items []network.TransportFeature, value network.TransportFeature) []network.TransportFeature {
	if slices.Contains(items, value) {
		return items
	}
	return append(items, value)
}
