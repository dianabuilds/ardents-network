package nameauthority

import "time"

// Request captures one identity-authority action intent.
type Request struct {
	Kind              string
	Actor             string
	Name              string
	Generation        uint64
	NewAuthority      string
	NewTarget         string
	Parent            string
	LeaseDuration     time.Duration
	GraceDuration     time.Duration
	RecoveryPolicy    *RecoveryPolicy
	RecoveryWitnesses []string
	ConflictContext   string
}
