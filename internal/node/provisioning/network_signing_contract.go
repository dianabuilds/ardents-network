package provisioning

import (
	"errors"
	"time"
)

var errNetworkSigning = errors.New("offline Network signing unavailable")

// IsNetworkSigning recognizes the four fixed offline signing purposes.
func IsNetworkSigning(operation string) bool {
	return operation == "sign-initial-node-record" || operation == "sign-initial-epoch" || operation == "sign-closed-profile" || operation == "prepare-node-certificate"
}

// NetworkSigningResult retains the signed byte commitment and operation interval.
type NetworkSigningResult struct {
	Digest   [32]byte
	interval networkSigningInterval
}

type networkSigningInterval struct{ reference, from, until time.Time }

func (s networkSigningInterval) checkTime(now time.Time) error {
	if s.reference.Before(now.Add(-5*time.Minute)) || s.reference.After(now.Add(time.Minute)) ||
		s.reference.Before(s.from) || !s.reference.Before(s.until) || now.Before(s.from) || !now.Before(s.until) || s.until.Sub(s.from) > 7*24*time.Hour {
		return errNetworkSigning
	}
	return nil
}

// CheckTime verifies that reporting uses the original signing interval.
func (result NetworkSigningResult) CheckTime(now time.Time) error {
	return result.interval.checkTime(now)
}
