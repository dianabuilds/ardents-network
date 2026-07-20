package evaluation

import (
	"time"

	dataapi "ardents/internal/data/api"
	"ardents/internal/policy/decision"
)

type RetentionConfig struct {
	DisableLocalBlobRetention  bool
	DisableRelayBlobRetention  bool
	DisableBlobPinning         bool
	DisablePeerBlobReserving   bool
	AllowPinRelayRetainedBlobs bool
	AllowReservingRelayBlobs   bool
	MaxLocalRetentionTTL       time.Duration
	MaxRelayRetentionTTL       time.Duration
}

func CheckRetention(cfg RetentionConfig, blob dataapi.BlobSnapshot, relay bool, expiresAt time.Time, now time.Time) decision.Result {
	if relay {
		if cfg.DisableRelayBlobRetention {
			return decision.Deny("policy_retention_denied", "relay retention is disabled by policy")
		}
		if cfg.MaxRelayRetentionTTL > 0 && !expiresAt.IsZero() && expiresAt.After(now.Add(cfg.MaxRelayRetentionTTL)) {
			return decision.Deny("policy_retention_denied", "relay retention exceeds policy ttl")
		}
	} else {
		if cfg.DisableLocalBlobRetention {
			return decision.Deny("policy_retention_denied", "local retention is disabled by policy")
		}
		if cfg.MaxLocalRetentionTTL > 0 && !expiresAt.IsZero() && expiresAt.After(now.Add(cfg.MaxLocalRetentionTTL)) {
			return decision.Deny("policy_retention_denied", "local retention exceeds policy ttl")
		}
	}
	if relay && !blob.Encrypted {
		return decision.Deny("policy_retention_denied", "relay retention requires encrypted blob")
	}
	return decision.Allow()
}

func CheckPin(cfg RetentionConfig, blob dataapi.BlobSnapshot) decision.Result {
	if cfg.DisableBlobPinning {
		return decision.Deny("policy_pin_denied", "blob pinning is disabled by policy")
	}
	if blob.Retention == "relay-temporary" && !cfg.AllowPinRelayRetainedBlobs {
		return decision.Deny("policy_pin_denied", "relay-retained blobs cannot be pinned by policy")
	}
	return decision.Allow()
}

func CheckPeerReserving(cfg RetentionConfig, blob dataapi.BlobSnapshot) decision.Result {
	if cfg.DisablePeerBlobReserving {
		return decision.Deny("policy_reserving_denied", "peer blob re-serving is disabled by policy")
	}
	if blob.Retention == "relay-temporary" && !cfg.AllowReservingRelayBlobs {
		return decision.Deny("policy_reserving_denied", "relay-retained blobs cannot be re-served by policy")
	}
	switch blob.State {
	case "available-local", "retained-temporary", "pinned":
		return decision.Allow()
	default:
		return decision.Deny("policy_reserving_denied", "blob state is not eligible for peer re-serving")
	}
}
