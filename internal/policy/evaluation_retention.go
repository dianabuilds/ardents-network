package policy

import (
	"ardents/internal/content"
	"time"
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

func CheckRetention(cfg RetentionConfig, blob content.BlobPolicyView, relay bool, expiresAt time.Time, now time.Time) Result {
	if relay {
		if cfg.DisableRelayBlobRetention {
			return Deny("policy_retention_denied", "relay retention is disabled by policy")
		}
		if cfg.MaxRelayRetentionTTL > 0 && !expiresAt.IsZero() && expiresAt.After(now.Add(cfg.MaxRelayRetentionTTL)) {
			return Deny("policy_retention_denied", "relay retention exceeds policy ttl")
		}
	} else {
		if cfg.DisableLocalBlobRetention {
			return Deny("policy_retention_denied", "local retention is disabled by policy")
		}
		if cfg.MaxLocalRetentionTTL > 0 && !expiresAt.IsZero() && expiresAt.After(now.Add(cfg.MaxLocalRetentionTTL)) {
			return Deny("policy_retention_denied", "local retention exceeds policy ttl")
		}
	}
	if relay && !blob.Encrypted {
		return Deny("policy_retention_denied", "relay retention requires encrypted blob")
	}
	return Allow()
}

func CheckPin(cfg RetentionConfig, blob content.BlobPolicyView) Result {
	if cfg.DisableBlobPinning {
		return Deny("policy_pin_denied", "blob pinning is disabled by policy")
	}
	if blob.Retention == "relay-temporary" && !cfg.AllowPinRelayRetainedBlobs {
		return Deny("policy_pin_denied", "relay-retained blobs cannot be pinned by policy")
	}
	return Allow()
}

func CheckPeerReserving(cfg RetentionConfig, blob content.BlobPolicyView) Result {
	if cfg.DisablePeerBlobReserving {
		return Deny("policy_reserving_denied", "peer blob re-serving is disabled by policy")
	}
	if blob.Retention == "relay-temporary" && !cfg.AllowReservingRelayBlobs {
		return Deny("policy_reserving_denied", "relay-retained blobs cannot be re-served by policy")
	}
	switch blob.State {
	case "available-local", "retained-temporary", "pinned":
		return Allow()
	default:
		return Deny("policy_reserving_denied", "blob state is not eligible for peer re-serving")
	}
}
