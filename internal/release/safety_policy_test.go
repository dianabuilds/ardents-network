package release

import (
	"testing"
	"time"
)

func TestEmergencyFiniteExpiryIsExclusive(t *testing.T) {
	end := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	policy := targetIdentityDescriptor{targetIdentity: targetIdentity{ProtocolPhase: "required"}, EmergencyReason: emergencyExploitableFlaw, EmergencyExpiry: end}
	for _, s := range []struct {
		at   time.Time
		want Outcome
	}{{end.Add(-time.Nanosecond), OutcomeReleaseAccepted}, {end, OutcomeReleaseUnavailable}, {end.Add(time.Nanosecond), OutcomeReleaseUnavailable}} {
		if got := classifyProtocol(policy, s.at).classification; got != s.want {
			t.Fatalf("finite emergency at%s got%s want%s", s.at, got, s.want)
		}
	}
}
