package messaging

import "slices"

import identityapi "ardents/internal/identity"

const (
	ProfileV1                 = "ardents-private/1"
	StatusActive              = "active"
	StatusDegraded            = "degraded"
	ReasonCapabilityReady     = "capability_ready"
	RecoverySteady            = "steady"
	RecoveryPending           = "recovery_pending"
	RecoveryBlocked           = "blocked"
	CodeCapabilityUnavailable = "privacy.capability.unavailable"
)

type StatusSnapshot struct {
	Profile             string
	State               string
	SwitchReason        string
	RecoveryState       string
	ReducedCapabilities []string
	ErrorCategories     []string
}

func Snapshot(discovery, data *Channel) StatusSnapshot {
	status := StatusSnapshot{Profile: ProfileV1, State: StatusActive, SwitchReason: ReasonCapabilityReady, RecoveryState: RecoverySteady}
	blocked := false
	check := func(channel *Channel, capability string, permissions ...identityapi.CapabilityPermission) {
		if channel == nil {
			status.reduce(capability, CodeCapabilityMissing)
			blocked = true
			return
		}
		for _, permission := range permissions {
			if _, _, err := channel.resolve(permission); err != nil {
				status.reduce(capability, safeCapabilityCode(err))
				return
			}
		}
	}
	check(discovery, "private_publication", identityapi.CapabilityPublish)
	check(discovery, "private_discovery", identityapi.CapabilitySubscribe, identityapi.CapabilityStoreFetch)
	check(data, "private_data_exchange", identityapi.CapabilityPublish, identityapi.CapabilitySubscribe)
	if status.State == StatusDegraded {
		status.RecoveryState = RecoveryPending
		if blocked {
			status.RecoveryState = RecoveryBlocked
		}
	}
	return status
}

func (s *StatusSnapshot) reduce(capability, code string) {
	s.State = StatusDegraded
	s.ReducedCapabilities = appendUnique(s.ReducedCapabilities, capability)
	s.ErrorCategories = appendUnique(s.ErrorCategories, code)
	if s.SwitchReason == ReasonCapabilityReady {
		s.SwitchReason = code
	}
}

func safeCapabilityCode(err error) string {
	if code := CodeOf(err); IsSafeErrorCategory(code) {
		return code
	}
	return CodeCapabilityUnavailable
}

func IsSafeErrorCategory(code string) bool {
	switch code {
	case CodeCapabilityMissing,
		"privacy.capability.not_yet_valid",
		"privacy.capability.expired",
		"privacy.capability.revoked",
		"privacy.capability.scope_denied",
		"privacy.capability.issuer_untrusted",
		"privacy.capability.invalid",
		CodeCapabilityUnavailable:
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
