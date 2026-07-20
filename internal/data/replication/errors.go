package replication

import (
	"errors"
	"strings"
)

var ErrDependencies = errors.New("replica control dependencies are unavailable")

func safeReason(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, ErrDependencies):
		return "dependencies_unavailable"
	case strings.Contains(err.Error(), "quota") || strings.Contains(err.Error(), "byte limit"):
		return "quota_refused"
	case strings.Contains(err.Error(), "untrusted") || strings.Contains(err.Error(), "not trusted"):
		return "peer_untrusted"
	case strings.Contains(err.Error(), "policy"):
		return "policy_denied"
	case strings.Contains(err.Error(), "capability"):
		return "capability_denied"
	case strings.Contains(err.Error(), "expired") || strings.Contains(err.Error(), "lease"):
		return "lease_invalid"
	case strings.Contains(err.Error(), "content identity") || strings.Contains(err.Error(), "ciphertext"):
		return "content_invalid"
	case strings.Contains(err.Error(), "replay") || strings.Contains(err.Error(), "already"):
		return "operation_replayed"
	case strings.Contains(err.Error(), "unsupported") || strings.Contains(err.Error(), "requires chunking"):
		return "transfer_unsupported"
	default:
		return "replica_control_rejected"
	}
}
